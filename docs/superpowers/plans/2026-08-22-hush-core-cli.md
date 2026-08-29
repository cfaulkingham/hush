# hush Core CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a local-first Go CLI (`hush`) that stores per-project secrets in an encrypted `.hush/store` blob, imports/exports `.env`, and injects secrets via `hush run -- <cmd>`.

**Architecture:** Single encrypted JSON document (`HUSH1` + XChaCha20-Poly1305) in `.hush/store`; plaintext `.hush/config.json` holds `project_id` and `active_env`; 32-byte project key in the OS keychain or `HUSH_KEY`. Packages are strictly layered: `store`/`keyring`/`project`/`dotenv`/`run`/`ui` have no cobra; `internal/cli` wires commands and is tested through `cobra.Command.Execute` with an injected memory keyring.

**Tech Stack:** Go 1.23, cobra, lipgloss, godotenv, zalando/go-keyring, golang.org/x/crypto, golang.org/x/term.

## Global Constraints

- Module path: `github.com/cfaulkingham/hush`. Go version: `1.23`.
- Store magic: exactly `HUSH1` (5 bytes). Cipher: XChaCha20-Poly1305, 32-byte key, 24-byte nonce, AAD = magic bytes. File mode `0600`.
- Key string: `hush_key_v1_` + lowercase hex of 32 bytes. `HUSH_KEY` overrides the keychain when non-empty.
- Secret keys: `^[A-Za-z_][A-Za-z0-9_]*$`, 1–256 chars. Env names: `^[A-Za-z][A-Za-z0-9_-]*$`, 1–64 chars. Values: UTF-8, 0–65536 bytes.
- Named environments, no inheritance. Init creates `development`.
- Secret values never printed unless the command is `get`, `ls --values`, `export`, or `key backup`.
- Exit 0 success, 1 user/usage/project error, 2 unexpected I/O or keychain backend failure.
- No network, no telemetry, no TUI wizard, no doctor/copy/rename/diff.
- Quiet one-line successes. MIT license.
- Tests: `go test ./...`. No live OS keychain. Use `keyring.Memory` and `t.Setenv`.
- Unix `syscall.Exec` for `run`; Windows start-and-wait. Inject `App.Exec` in CLI tests.

## File map

| Path | Responsibility |
|------|----------------|
| `go.mod` / `go.sum` | Module `hush`, Go 1.23 |
| `LICENSE` | MIT |
| `.gitignore` | binaries, coverage |
| `README.md` | install + quickstart |
| `.github/workflows/test.yml` | Linux/macOS test, vet, and race; Windows vet and cross-build |
| `cmd/hush/main.go` | `os.Exit(cli.Main(os.Args[1:]))` |
| `internal/store/document.go` | JSON document, env/secret helpers |
| `internal/store/crypt.go` | Encrypt/Decrypt |
| `internal/store/file.go` | atomic 0600 write, symlink refuse, Load/Save |
| `internal/store/validate.go` | key/env/value validators |
| `internal/keyring/keyring.go` | Ring interface, Format/Parse/Resolve, errors |
| `internal/keyring/memory.go` | in-memory Ring |
| `internal/keyring/os.go` | zalando/go-keyring adapter |
| `internal/project/project.go` | walk-up find, config.json, paths |
| `internal/dotenv/dotenv.go` | godotenv parse + round-trip serialize |
| `internal/run/overlay.go` | env overlay |
| `internal/run/exec_unix.go` | syscall.Exec |
| `internal/run/exec_windows.go` | run-and-wait |
| `internal/ui/ui.go` | color enable, lipgloss, error print |
| `internal/cli/app.go` | App deps, Root, Main, exit codes |
| `internal/cli/*.go` | one file per command group |
| `testdata/dotenv/` | parse fixtures |

Package import rules: `store` imports nothing in this repo. `keyring` imports nothing in this repo except stdlib + zalando. `project` imports stdlib only. `cli` may import all internals. `run` does not import `store`.

---

### Task 1: Store encrypt/decrypt (`HUSH1`)

**Files:**
- Create: `go.mod`, `.gitignore`, `LICENSE`, `internal/store/crypt.go`, `internal/store/crypt_test.go`, `internal/store/document.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `const Magic = "HUSH1"`
  - `const Version = 1`
  - `const KeySize = 32`
  - `type Secret struct { Value string; UpdatedAt time.Time }`
  - `type Environment struct { UpdatedAt time.Time; Secrets map[string]Secret }`
  - `type Document struct { Version int; ProjectID string; Name string; CreatedAt, UpdatedAt time.Time; Environments map[string]*Environment }`
  - `func NewDocument(projectID, name string, now time.Time) *Document`
  - `func Encrypt(doc *Document, key []byte) ([]byte, error)`
  - `func Decrypt(blob []byte, key []byte) (*Document, error)`
  - `var ErrBadMagic, ErrBadKeySize, ErrDecrypt, ErrVersion error`

- [ ] **Step 1: Scaffold module files**

Create `.gitignore`:

```
/hush
/hush.exe
/coverage.out
*.test
```

Create `LICENSE` with the MIT text, copyright `Copyright (c) 2026 hush contributors`.

Create `go.mod`:

```
module github.com/cfaulkingham/hush

go 1.23
```

Run: `go get golang.org/x/crypto@latest`

- [ ] **Step 2: Write the failing test**

Create `internal/store/crypt_test.go`:

```go
package store

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	doc := NewDocument("550e8400-e29b-41d4-a716-446655440000", "api", now)
	doc.Environments["development"].Secrets["DATABASE_URL"] = Secret{
		Value:     "postgres://localhost/api",
		UpdatedAt: now,
	}

	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !bytes.HasPrefix(blob, []byte(Magic)) {
		t.Fatalf("blob missing magic, got %q", blob[:min(len(blob), 8)])
	}
	if len(blob) < 5+24+16 {
		t.Fatalf("blob too short: %d", len(blob))
	}

	got, err := Decrypt(blob, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got.ProjectID != doc.ProjectID || got.Name != "api" || got.Version != 1 {
		t.Fatalf("meta mismatch: %+v", got)
	}
	sec, ok := got.Environments["development"].Secrets["DATABASE_URL"]
	if !ok || sec.Value != "postgres://localhost/api" {
		t.Fatalf("secret mismatch: %#v", got.Environments)
	}

	blob2, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(blob, blob2) {
		t.Fatal("expected fresh nonce to change ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	other := bytes.Repeat([]byte{2}, KeySize)
	doc := NewDocument("id", "n", time.Now().UTC())
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(blob, other); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptBadMagic(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	if _, err := Decrypt([]byte("NOPE1xxxxxxxx"), key); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptTruncated(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	doc := NewDocument("id", "n", time.Now().UTC())
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(blob[:len(blob)-2], key); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncryptRejectsBadKeySize(t *testing.T) {
	doc := NewDocument("id", "n", time.Now().UTC())
	if _, err := Encrypt(doc, []byte("short")); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1`

Expected: FAIL to compile (`Encrypt` / `NewDocument` undefined) or FAIL assertions.

- [ ] **Step 4: Write minimal implementation**

Create `internal/store/document.go`:

```go
package store

import "time"

const (
	Magic   = "HUSH1"
	Version = 1
	KeySize = 32
)

type Secret struct {
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Environment struct {
	UpdatedAt time.Time          `json:"updated_at"`
	Secrets   map[string]Secret  `json:"secrets"`
}

type Document struct {
	Version      int                      `json:"version"`
	ProjectID    string                   `json:"project_id"`
	Name         string                   `json:"name"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	Environments map[string]*Environment  `json:"environments"`
}

func NewDocument(projectID, name string, now time.Time) *Document {
	now = now.UTC()
	return &Document{
		Version:   Version,
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		Environments: map[string]*Environment{
			"development": {
				UpdatedAt: now,
				Secrets:   map[string]Secret{},
			},
		},
	}
}
```

Create `internal/store/crypt.go`:

```go
package store

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrBadMagic   = errors.New("unrecognized hush store (magic). This CLI reads HUSH1 only.")
	ErrBadKeySize = errors.New("project key must be 32 bytes")
	ErrDecrypt    = errors.New("could not decrypt .hush/store (wrong key or corrupted file)")
	ErrVersion    = errors.New("unsupported hush store version")
)

func Encrypt(doc *Document, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrBadKeySize
	}
	plain, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plain, []byte(Magic))
	out := make([]byte, 0, 5+len(nonce)+len(ct))
	out = append(out, Magic...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func Decrypt(blob []byte, key []byte) (*Document, error) {
	if len(key) != KeySize {
		return nil, ErrBadKeySize
	}
	if len(blob) < 5+chacha20poly1305.NonceSizeX+16 {
		return nil, ErrDecrypt
	}
	if string(blob[:5]) != Magic {
		return nil, fmt.Errorf("%w: got %q", ErrBadMagic, blob[:min(5, len(blob))])
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := blob[5 : 5+chacha20poly1305.NonceSizeX]
	ct := blob[5+chacha20poly1305.NonceSizeX:]
	plain, err := aead.Open(nil, nonce, ct, []byte(Magic))
	if err != nil {
		return nil, ErrDecrypt
	}
	var doc Document
	if err := json.Unmarshal(plain, &doc); err != nil {
		return nil, ErrDecrypt
	}
	if doc.Version != Version {
		return nil, fmt.Errorf("%w: %d", ErrVersion, doc.Version)
	}
	if doc.Environments == nil {
		doc.Environments = map[string]*Environment{}
	}
	for _, env := range doc.Environments {
		if env.Secrets == nil {
			env.Secrets = map[string]Secret{}
		}
	}
	return &doc, nil
}
```

- [ ] **Step 5: Run tests and make sure they pass**

Run: `go test ./internal/store/ -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .gitignore LICENSE internal/store/crypt.go internal/store/crypt_test.go internal/store/document.go
git commit -m "feat: encrypt and decrypt HUSH1 store blobs"
```

---

### Task 2: Store validation, helpers, atomic file I/O

**Files:**
- Create: `internal/store/validate.go`, `internal/store/validate_test.go`, `internal/store/document_ops.go`, `internal/store/document_ops_test.go`, `internal/store/file.go`, `internal/store/file_test.go`

**Interfaces:**
- Consumes: `Document`, `Encrypt`, `Decrypt`, `KeySize` from Task 1
- Produces:
  - `func ValidateKey(key string) error`
  - `func ValidateEnvName(name string) error`
  - `func ValidateValue(value string) error`
  - `const MaxValueBytes = 65536`
  - `func (d *Document) PutSecret(env, key, value string, now time.Time) error`
  - `func (d *Document) GetSecret(env, key string) (string, error)`
  - `func (d *Document) DeleteSecret(env, key string) error`
  - `func (d *Document) ListKeys(env string) ([]string, error)`
  - `func (d *Document) SecretMap(env string) (map[string]string, error)`
  - `func (d *Document) NewEnv(name string, now time.Time) error`
  - `func (d *Document) Env(name string) (*Environment, error)`
  - `func WriteFile(path string, blob []byte) error`
  - `func ReadFile(path string) ([]byte, error)`
  - `func Save(path string, doc *Document, key []byte) error`
  - `func Load(path string, key []byte) (*Document, error)`
  - `var ErrSymlink, ErrSecretNotFound, ErrEnvNotFound, ErrEnvExists error`

- [ ] **Step 1: Write the failing tests**

`internal/store/validate_test.go`:

```go
package store

import "testing"

func TestValidateKey(t *testing.T) {
	ok := []string{"FOO", "_X", "A", "Database_URL", "A1"}
	for _, k := range ok {
		if err := ValidateKey(k); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
	}
	bad := []string{"", "1FOO", "FOO-BAR", "FOO.BAR", "FOO BAR", string(make([]byte, 257))}
	for _, k := range bad {
		if err := ValidateKey(k); err == nil {
			t.Fatalf("expected error for %q", k)
		}
	}
}

func TestValidateEnvName(t *testing.T) {
	if err := ValidateEnvName("development"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvName("staging-2"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvName("1x"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateEnvName(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateValueSize(t *testing.T) {
	if err := ValidateValue(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateValue(string(make([]byte, MaxValueBytes))); err != nil {
		t.Fatal(err)
	}
	if err := ValidateValue(string(make([]byte, MaxValueBytes+1))); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateValue("hello\x00"); err == nil {
		t.Fatal("expected reject NUL / non-text")
	}
}
```

Treat values as UTF-8 text. Reject if `!utf8.ValidString(value)` or if the value contains a NUL byte. Size is `len(value)` in bytes.

`internal/store/document_ops_test.go`:

```go
package store

import (
	"testing"
	"time"
)

func TestPutGetDeleteSecret(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDocument("id", "api", now)
	if err := d.PutSecret("development", "FOO", "bar", now); err != nil {
		t.Fatal(err)
	}
	v, err := d.GetSecret("development", "FOO")
	if err != nil || v != "bar" {
		t.Fatalf("got %q %v", v, err)
	}
	keys, err := d.ListKeys("development")
	if err != nil || len(keys) != 1 || keys[0] != "FOO" {
		t.Fatalf("keys %v %v", keys, err)
	}
	m, err := d.SecretMap("development")
	if err != nil || m["FOO"] != "bar" {
		t.Fatalf("map %v %v", m, err)
	}
	if err := d.DeleteSecret("development", "FOO"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.GetSecret("development", "FOO"); err == nil {
		t.Fatal("expected missing")
	}
}

func TestPutSecretMissingEnv(t *testing.T) {
	d := NewDocument("id", "api", time.Now().UTC())
	if err := d.PutSecret("staging", "FOO", "bar", time.Now().UTC()); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewEnv(t *testing.T) {
	now := time.Now().UTC()
	d := NewDocument("id", "api", now)
	if err := d.NewEnv("staging", now); err != nil {
		t.Fatal(err)
	}
	if err := d.NewEnv("staging", now); err == nil {
		t.Fatal("expected duplicate error")
	}
	if err := d.PutSecret("staging", "FOO", "x", now); err != nil {
		t.Fatal(err)
	}
}
```

`internal/store/file_test.go`:

```go
package store

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := make([]byte, KeySize)
	rand.Read(key)
	doc := NewDocument("pid", "api", time.Now().UTC())
	if err := doc.PutSecret("development", "FOO", "bar", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, doc, key); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
	got, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := got.GetSecret("development", "FOO")
	if v != "bar" {
		t.Fatalf("got %q", v)
	}
}

func TestWriteFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "store")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("HUSH1notreal")); err == nil {
		t.Fatal("expected symlink error")
	}
}

func TestWriteFileLeavesExistingOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	first := bytes.Repeat([]byte("a"), 32)
	if err := WriteFile(path, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	if err := WriteFile(path, bytes.Repeat([]byte("b"), 32)); err == nil {
		t.Fatal("expected failure")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		os.Chmod(dir, 0755)
		got, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("existing file mutated: %q", got)
	}
}

func TestLoadVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := bytes.Repeat([]byte{9}, KeySize)
	doc := NewDocument("pid", "api", time.Now().UTC())
	doc.Version = 99
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	// bypass Decrypt version check by writing ciphertext of v99 — Encrypt does not check version
	if err := WriteFile(path, blob); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, key); err == nil {
		t.Fatal("expected version error")
	}
}
```

`TestLoadVersionMismatch` depends on `Encrypt` not rejecting `doc.Version != 1`. Keep it that way: validation of version happens on decrypt/load only.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -count=1`

Expected: FAIL compile (missing funcs).

- [ ] **Step 3: Write minimal implementation**

`internal/store/validate.go`:

```go
package store

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

const MaxValueBytes = 65536

var (
	keyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
)

func ValidateKey(key string) error {
	if len(key) < 1 || len(key) > 256 || !keyRe.MatchString(key) {
		return fmt.Errorf("invalid secret key %q (use KEY_NAME)", key)
	}
	return nil
}

func ValidateEnvName(name string) error {
	if len(name) < 1 || len(name) > 64 || !envRe.MatchString(name) {
		return fmt.Errorf("invalid environment name %q", name)
	}
	return nil
}

func ValidateValue(value string) error {
	if len(value) > MaxValueBytes {
		return fmt.Errorf("secret value exceeds %d bytes", MaxValueBytes)
	}
	if !utf8.ValidString(value) || containsNUL(value) {
		return fmt.Errorf("secret value must be UTF-8 text")
	}
	return nil
}

func containsNUL(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}
```

`internal/store/document_ops.go`:

```go
package store

import (
	"fmt"
	"sort"
	"time"
)

func (d *Document) Env(name string) (*Environment, error) {
	if err := ValidateEnvName(name); err != nil {
		return nil, err
	}
	env, ok := d.Environments[name]
	if !ok {
		return nil, fmt.Errorf("environment %s not found. Create it with: hush env new %s", name, name)
	}
	return env, nil
}

func (d *Document) NewEnv(name string, now time.Time) error {
	if err := ValidateEnvName(name); err != nil {
		return err
	}
	if _, ok := d.Environments[name]; ok {
		return fmt.Errorf("environment %s already exists", name)
	}
	now = now.UTC()
	d.Environments[name] = &Environment{UpdatedAt: now, Secrets: map[string]Secret{}}
	d.UpdatedAt = now
	return nil
}

func (d *Document) PutSecret(env, key, value string, now time.Time) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	e, err := d.Env(env)
	if err != nil {
		return err
	}
	now = now.UTC()
	e.Secrets[key] = Secret{Value: value, UpdatedAt: now}
	e.UpdatedAt = now
	d.UpdatedAt = now
	return nil
}

func (d *Document) GetSecret(env, key string) (string, error) {
	e, err := d.Env(env)
	if err != nil {
		return "", err
	}
	sec, ok := e.Secrets[key]
	if !ok {
		return "", fmt.Errorf("secret %s not found in %s", key, env)
	}
	return sec.Value, nil
}

func (d *Document) DeleteSecret(env, key string) error {
	e, err := d.Env(env)
	if err != nil {
		return err
	}
	if _, ok := e.Secrets[key]; !ok {
		return fmt.Errorf("secret %s not found in %s", key, env)
	}
	delete(e.Secrets, key)
	now := time.Now().UTC()
	e.UpdatedAt = now
	d.UpdatedAt = now
	return nil
}

func (d *Document) ListKeys(env string) ([]string, error) {
	e, err := d.Env(env)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(e.Secrets))
	for k := range e.Secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (d *Document) SecretMap(env string) (map[string]string, error) {
	e, err := d.Env(env)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(e.Secrets))
	for k, sec := range e.Secrets {
		m[k] = sec.Value
	}
	return m, nil
}
```

`internal/store/file.go`:

```go
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrSymlink = errors.New("refusing to write through symlink .hush/store")

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func WriteFile(path string, blob []byte) error {
	fi, err := os.Lstat(path)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return ErrSymlink
	}
	dir := filepath.Dir(path)
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "store.tmp."+hex.EncodeToString(rnd[:]))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(blob); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	ok = true
	return nil
}

func Save(path string, doc *Document, key []byte) error {
	blob, err := Encrypt(doc, key)
	if err != nil {
		return err
	}
	return WriteFile(path, blob)
}

func Load(path string, key []byte) (*Document, error) {
	blob, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decrypt(blob, key)
}
```

On `TestWriteFileLeavesExistingOnFailure`, `os.ReadFile` after a failed write may fail because the directory is 0555. The test already chmods back in Cleanup and has a fallback read after chmod. Implementation must not truncate the existing file in place.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/store/ -count=1`

Expected: PASS. If the chmod failure test is flaky as root (chmod 0555 still writable), skip it with `if os.Geteuid() == 0 { t.Skip() }`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: validate secrets and write store files atomically"
```

---

### Task 3: Keyring interface, memory backend, HUSH_KEY

**Files:**
- Create: `internal/keyring/keyring.go`, `internal/keyring/memory.go`, `internal/keyring/os.go`, `internal/keyring/keyring_test.go`

**Interfaces:**
- Consumes: nothing from `store`
- Produces:
  - `const Service = "hush"`
  - `const Prefix = "hush_key_v1_"`
  - `type Ring interface { Get(service, account string) (string, error); Set(service, account, secret string) error; Delete(service, account string) error }`
  - `func FormatKey(raw []byte) (string, error)`
  - `func ParseKey(s string) ([]byte, error)`
  - `func Resolve(r Ring, projectID string) ([]byte, error)`
  - `func NewMemory() *Memory`
  - `type OS struct{}`
  - `var ErrNoKey, ErrNotFound, ErrBadKey, ErrBackend error`

- [ ] **Step 1: Write the failing test**

`internal/keyring/keyring_test.go`:

```go
package keyring

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestFormatParseKey(t *testing.T) {
	raw := bytes.Repeat([]byte{0xab}, 32)
	s, err := FormatKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := Prefix + hex.EncodeToString(raw)
	if s != want {
		t.Fatalf("got %s want %s", s, want)
	}
	got, err := ParseKey(s)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
	got, err = ParseKey("HUSH_KEY_V1_" + hex.EncodeToString(raw))
	if err == nil {
		t.Fatal("prefix must be exact hush_key_v1_")
	}
	_, err = ParseKey(Prefix + hex.EncodeToString(bytes.Repeat([]byte{1}, 31)))
	if err == nil {
		t.Fatal("expected length error")
	}
	upper := Prefix + "AB" + hex.EncodeToString(raw)[2:]
	got, err = ParseKey(upper)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("hex should be case-insensitive: %v", err)
	}
}

func TestMemorySetGet(t *testing.T) {
	m := NewMemory()
	if err := m.Set(Service, "pid", "secret"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(Service, "pid")
	if err != nil || got != "secret" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := m.Get(Service, "missing"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestResolvePrefersHUSH_KEY(t *testing.T) {
	raw := bytes.Repeat([]byte{3}, 32)
	s, err := FormatKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUSH_KEY", s)
	m := NewMemory()
	other, _ := FormatKey(bytes.Repeat([]byte{4}, 32))
	_ = m.Set(Service, "pid", other)
	got, err := Resolve(m, "pid")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
}

func TestResolveKeychain(t *testing.T) {
	t.Setenv("HUSH_KEY", "")
	raw := bytes.Repeat([]byte{5}, 32)
	s, _ := FormatKey(raw)
	m := NewMemory()
	_ = m.Set(Service, "pid", s)
	got, err := Resolve(m, "pid")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Setenv("HUSH_KEY", "")
	_, err := Resolve(NewMemory(), "pid")
	if err == nil {
		t.Fatal("expected ErrNoKey")
	}
}
```

`ParseKey` must require prefix `hush_key_v1_` (lowercase). Hex body is case-insensitive. Empty `HUSH_KEY` means unset for `Resolve` (treat empty as absent).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/keyring/ -count=1`

Expected: FAIL compile.

- [ ] **Step 3: Write minimal implementation**

`internal/keyring/keyring.go`:

```go
package keyring

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

const (
	Service = "hush"
	Prefix  = "hush_key_v1_"
)

var (
	ErrNoKey    = errors.New("no project key. Restore with hush key restore <key>, or set HUSH_KEY.")
	ErrNotFound = errors.New("not found")
	ErrBadKey   = errors.New("invalid project key (want hush_key_v1_ + 64 hex chars)")
	ErrBackend  = errors.New("keychain backend failure")
)

type Ring interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

func FormatKey(raw []byte) (string, error) {
	if len(raw) != 32 {
		return "", ErrBadKey
	}
	return Prefix + hex.EncodeToString(raw), nil
}

func ParseKey(s string) ([]byte, error) {
	if !strings.HasPrefix(s, Prefix) {
		return nil, ErrBadKey
	}
	body := s[len(Prefix):]
	raw, err := hex.DecodeString(body)
	if err != nil || len(raw) != 32 {
		return nil, ErrBadKey
	}
	return raw, nil
}

func Resolve(r Ring, projectID string) ([]byte, error) {
	if s := os.Getenv("HUSH_KEY"); s != "" {
		return ParseKey(s)
	}
	if r == nil {
		return nil, ErrNoKey
	}
	s, err := r.Get(Service, projectID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNoKey
		}
		return nil, errors.Join(ErrBackend, err)
	}
	return ParseKey(s)
}
```

`internal/keyring/memory.go`:

```go
package keyring

type Memory struct {
	m map[string]string
}

func NewMemory() *Memory {
	return &Memory{m: map[string]string{}}
}

func key(service, account string) string { return service + "\x00" + account }

func (m *Memory) Get(service, account string) (string, error) {
	v, ok := m.m[key(service, account)]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *Memory) Set(service, account, secret string) error {
	m.m[key(service, account)] = secret
	return nil
}

func (m *Memory) Delete(service, account string) error {
	delete(m.m, key(service, account))
	return nil
}
```

`internal/keyring/os.go`:

```go
package keyring

import (
	"errors"

	oskeyring "github.com/zalando/go-keyring"
)

type OS struct{}

func (OS) Get(service, account string) (string, error) {
	s, err := oskeyring.Get(service, account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", errors.Join(ErrBackend, err)
	}
	return s, nil
}

func (OS) Set(service, account, secret string) error {
	if err := oskeyring.Set(service, account, secret); err != nil {
		return errors.Join(ErrBackend, err)
	}
	return nil
}

func (OS) Delete(service, account string) error {
	err := oskeyring.Delete(service, account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrBackend, err)
	}
	return nil
}
```

Run: `go get github.com/zalando/go-keyring@latest`

- [ ] **Step 4: Run tests**

Run: `go test ./internal/keyring/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/keyring/ go.mod go.sum
git commit -m "feat: add keyring interface with HUSH_KEY override"
```

---

### Task 4: Project discovery and config.json

**Files:**
- Create: `internal/project/project.go`, `internal/project/project_test.go`

**Interfaces:**
- Consumes: nothing from store/keyring
- Produces:
  - `type Config struct { ProjectID string json:"project_id"; ActiveEnv string json:"active_env" }`
  - `type Project struct { Root string; Config Config }`
  - `func Find(startDir string) (*Project, error)`
  - `func LoadConfig(root string) (Config, error)`
  - `func SaveConfig(root string, cfg Config) error`
  - `func Dir(root string) string` → `filepath.Join(root, ".hush")`
  - `func StorePath(root string) string`
  - `func ConfigPath(root string) string`
  - `var ErrNotFound error` with message `no hush project (run hush init)`

- [ ] **Step 1: Write the failing test**

```go
package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	hush := filepath.Join(root, ".hush")
	if err := os.Mkdir(hush, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hush, "store"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{ProjectID: "pid", ActiveEnv: "development"}
	if err := SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	p, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root {
		t.Fatalf("root %s want %s", p.Root, root)
	}
	if p.Config.ProjectID != "pid" || p.Config.ActiveEnv != "development" {
		t.Fatalf("%+v", p.Config)
	}
}

func TestFindNone(t *testing.T) {
	_, err := Find(t.TempDir())
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestSaveConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".hush"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(root, Config{ProjectID: "abc", ActiveEnv: "staging"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != "abc" || got.ActiveEnv != "staging" {
		t.Fatalf("%+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -count=1`

Expected: FAIL compile.

- [ ] **Step 3: Write minimal implementation**

```go
package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("no hush project (run hush init)")

type Config struct {
	ProjectID string `json:"project_id"`
	ActiveEnv string `json:"active_env"`
}

type Project struct {
	Root   string
	Config Config
}

func Dir(root string) string        { return filepath.Join(root, ".hush") }
func StorePath(root string) string  { return filepath.Join(Dir(root), "store") }
func ConfigPath(root string) string { return filepath.Join(Dir(root), "config.json") }

func Find(startDir string) (*Project, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		store := StorePath(dir)
		if fi, err := os.Stat(store); err == nil && fi.Mode().IsRegular() {
			cfg, err := LoadConfig(dir)
			if err != nil {
				return nil, err
			}
			return &Project{Root: dir, Config: cfg}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNotFound
		}
		dir = parent
	}
}

func LoadConfig(root string) (Config, error) {
	b, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func SaveConfig(root string, cfg Config) error {
	if err := os.MkdirAll(Dir(root), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(ConfigPath(root), b, 0644)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/project/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/project/
git commit -m "feat: find hush projects by walking up for .hush/store"
```

---

### Task 5: dotenv parse and serialize

**Files:**
- Create: `internal/dotenv/dotenv.go`, `internal/dotenv/dotenv_test.go`, `testdata/dotenv/simple.env`, `testdata/dotenv/comments.env`, `testdata/dotenv/export.env`, `testdata/dotenv/quotes.env`, `testdata/dotenv/blank.env`, `testdata/dotenv/unicode.env`, `testdata/dotenv/multiline.env`, `testdata/dotenv/duplicate.env`

**Interfaces:**
- Consumes: none
- Produces:
  - `func Parse(r io.Reader) (map[string]string, error)`
  - `func ParseFile(path string) (map[string]string, error)`
  - `func Serialize(secrets map[string]string) ([]byte, error)`
  - `func SerializeJSON(secrets map[string]string) ([]byte, error)`

- [ ] **Step 1: Write fixtures and failing tests**

`testdata/dotenv/simple.env`:

```
FOO=bar
BAZ=qux
```

`testdata/dotenv/comments.env`:

```
# heading
FOO=bar

BAZ=qux
```

`testdata/dotenv/export.env`:

```
export FOO=bar
```

`testdata/dotenv/quotes.env`:

```
SINGLE='hello world'
DOUBLE="hello world"
HASH="foo#bar"
```

`testdata/dotenv/blank.env`:

```
EMPTY=
```

`testdata/dotenv/unicode.env`:

```
GREETING=こんにちは
```

`testdata/dotenv/multiline.env`:

```
CERT="line1\nline2"
```

`testdata/dotenv/duplicate.env`:

```
FOO=first
FOO=second
```

`internal/dotenv/dotenv_test.go`:

```go
package dotenv

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dotenv", name)
}

func TestParseFileFixtures(t *testing.T) {
	cases := []struct {
		file string
		key  string
		want string
	}{
		{"simple.env", "FOO", "bar"},
		{"simple.env", "BAZ", "qux"},
		{"comments.env", "FOO", "bar"},
		{"export.env", "FOO", "bar"},
		{"quotes.env", "SINGLE", "hello world"},
		{"quotes.env", "DOUBLE", "hello world"},
		{"quotes.env", "HASH", "foo#bar"},
		{"blank.env", "EMPTY", ""},
		{"unicode.env", "GREETING", "こんにちは"},
		{"duplicate.env", "FOO", "second"},
	}
	for _, tc := range cases {
		m, err := ParseFile(testdata(t, tc.file))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if m[tc.key] != tc.want {
			t.Fatalf("%s %s: got %q want %q", tc.file, tc.key, m[tc.key], tc.want)
		}
	}
}

func TestParseMultiline(t *testing.T) {
	m, err := ParseFile(testdata(t, "multiline.env"))
	if err != nil {
		t.Fatal(err)
	}
	if m["CERT"] != "line1\nline2" {
		t.Fatalf("got %q", m["CERT"])
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	in := map[string]string{
		"FOO":       "bar",
		"SPACED":    "hello world",
		"HASH":      "foo#bar",
		"EMPTY":     "",
		"NL":        "a\nb",
		"QUOTE":     `say "hi"`,
		"GREETING":  "こんにちは",
	}
	b, err := Serialize(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range in {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q\nserialized:\n%s", k, got[k], v, b)
		}
	}
}

func TestSerializeJSON(t *testing.T) {
	b, err := SerializeJSON(map[string]string{"B": "2", "A": "1"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"A\": \"1\",\n  \"B\": \"2\"\n}\n"
	if string(b) != want {
		t.Fatalf("got %q", b)
	}
}

func TestParseMissingFile(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "nope.env"))
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dotenv/ -count=1`

Expected: FAIL compile.

- [ ] **Step 3: Write minimal implementation**

Run: `go get github.com/joho/godotenv@latest`

```go
package dotenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func Parse(r io.Reader) (map[string]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return godotenv.Unmarshal(string(b))
}

func ParseFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	return godotenv.Unmarshal(string(b))
}

func Serialize(secrets map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(quoteDotenv(secrets[k]))
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func quoteDotenv(v string) string {
	if v == "" {
		return ""
	}
	if needsQuote(v) {
		return strconv.Quote(v) // double quotes, Go-style escapes; godotenv understands \n \"
	}
	return v
}

func needsQuote(v string) bool {
	if strings.ContainsAny(v, " \t#\"'`$\\") {
		return true
	}
	if strings.ContainsAny(v, "\n\r") {
		return true
	}
	return false
}

func SerializeJSON(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		secrets = map[string]string{}
	}
	b, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
```

`encoding/json` sorts map keys, so `SerializeJSON` output matches the test (`A` then `B`).

If `strconv.Quote` does not round-trip through godotenv for a fixture, switch that case to a hand-rolled escape (`\\`, `"`, `\n`, `\r`) wrapped in double quotes — keep the round-trip test as the spec.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/dotenv/ -count=1`

Expected: PASS. Also: `go test ./... -count=1` still PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dotenv/ testdata/dotenv/ go.mod go.sum
git commit -m "feat: parse and serialize dotenv files via godotenv"
```

---

### Task 6: Env overlay and platform exec

**Files:**
- Create: `internal/run/overlay.go`, `internal/run/overlay_test.go`, `internal/run/exec_unix.go`, `internal/run/exec_windows.go`, `internal/run/exec.go`

**Interfaces:**
- Consumes: none
- Produces:
  - `func Overlay(parent []string, secrets map[string]string) []string`
  - `func Exec(argv []string, env []string) error`
  - `var ErrNoCommand error` message `hush run requires a command. Example: hush run -- npm start`

- [ ] **Step 1: Write the failing test**

`internal/run/overlay_test.go`:

```go
package run

import "testing"

func TestOverlayWins(t *testing.T) {
	parent := []string{"PATH=/bin", "FOO=from-parent", "BAR=keep"}
	out := Overlay(parent, map[string]string{"FOO": "from-hush", "BAZ": "new"})
	got := map[string]string{}
	for _, kv := range out {
		i := indexByte(kv, '=')
		if i < 0 {
			t.Fatalf("bad entry %q", kv)
		}
		got[kv[:i]] = kv[i+1:]
	}
	if got["FOO"] != "from-hush" {
		t.Fatalf("FOO %q", got["FOO"])
	}
	if got["BAR"] != "keep" || got["BAZ"] != "new" || got["PATH"] != "/bin" {
		t.Fatalf("%v", got)
	}
}

func TestExecNoCommand(t *testing.T) {
	if err := Exec(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
```

Do not actually `syscall.Exec` in tests.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/run/ -count=1`

Expected: FAIL compile.

- [ ] **Step 3: Write minimal implementation**

`internal/run/overlay.go`:

```go
package run

import "strings"

func Overlay(parent []string, secrets map[string]string) []string {
	seen := map[string]int{}
	out := make([]string, 0, len(parent)+len(secrets))
	for _, kv := range parent {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, take := secrets[k]; take {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = len(out)
		out = append(out, kv)
	}
	for k, v := range secrets {
		out = append(out, k+"="+v)
	}
	return out
}
```

`internal/run/exec.go`:

```go
package run

import "errors"

var ErrNoCommand = errors.New("hush run requires a command. Example: hush run -- npm start")
```

`internal/run/exec_unix.go`:

```go
//go:build unix

package run

import (
	"os/exec"
	"syscall"
)

func Exec(argv []string, env []string) error {
	if len(argv) == 0 {
		return ErrNoCommand
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}
```

`internal/run/exec_windows.go`:

```go
//go:build windows

package run

import (
	"os"
	"os/exec"
)

func Exec(argv []string, env []string) error {
	if len(argv) == 0 {
		return ErrNoCommand
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/run/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/run/
git commit -m "feat: overlay process env and exec on unix/windows"
```

---

### Task 7: CLI harness, ui, `hush init`

**Files:**
- Create: `internal/ui/ui.go`, `internal/cli/app.go`, `internal/cli/exit.go`, `internal/cli/load.go`, `internal/cli/init.go`, `internal/cli/init_test.go`, `cmd/hush/main.go`
- Modify: `go.mod` (cobra, lipgloss, google/uuid)

**Interfaces:**
- Consumes: `store.*`, `keyring.*`, `project.*`
- Produces:
  - `type App struct { Ring keyring.Ring; Stdout, Stderr io.Writer; Stdin io.Reader; Getwd func() (string, error); Environ func() []string; Exec func(argv, env []string) error; Now func() time.Time }`
  - `func NewOSApp() *App`
  - `func (a *App) Root() *cobra.Command`
  - `func Main(args []string) int`
  - `func (a *App) initCmd() *cobra.Command`
  - `func ensureGitignore(dir string) error`
  - Persistent flags: `env` string, `plain` bool
  - `func (a *App) colorEnabled(w interface{ Fd() uintptr }) bool` — simpler: `func (a *App) plain() bool` from flag + `NO_COLOR`

Init behavior (from spec, implement exactly):

- Refuse if `./.hush/store` exists: `already a hush project. See hush status.`
- Refuse if `./.hush` exists and is not a directory.
- `--name` defaults to `filepath.Base(cwd)`.
- Create `.hush/`, write config (`project_id` UUID v4, `active_env=development`).
- 32-byte key → keychain `service=hush`, `account=project_id`, value `FormatKey`.
- Encrypted store with empty `development`.
- Append `.hush/` to `.gitignore` (create file if missing; if a trimmed line already equals `.hush/`, do not duplicate).
- Print:
  ```
  Initialized hush project "api" (gitignored .hush/).
  Key stored in keychain. Next: hush import .env
  ```
  If `HUSH_KEY` is set, second line still says `Key stored in keychain` only if we wrote the keychain. Spec says init stores in the keychain. Always `Ring.Set`. If `HUSH_KEY` is also set, still write keychain so `status` can show `HUSH_KEY` as the resolver. Init does not require HUSH_KEY to be unset.

- [ ] **Step 1: Write the failing test**

`internal/cli/init_test.go`:

```go
package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func newTestApp(t *testing.T, dir string) (*App, *bytes.Buffer, *bytes.Buffer, *keyring.Memory) {
	t.Helper()
	t.Setenv("HUSH_KEY", "")
	t.Setenv("NO_COLOR", "1")
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	ring := keyring.NewMemory()
	app := &App{
		Ring:    ring,
		Stdout:  out,
		Stderr:  errb,
		Stdin:   bytes.NewReader(nil),
		Getwd:   func() (string, error) { return dir, nil },
		Environ: func() []string { return []string{} },
		Exec:    func(argv, env []string) error { return nil },
		Now:     func() time.Time { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) },
	}
	return app, out, errb, ring
}

func runApp(t *testing.T, app *App, args ...string) error {
	t.Helper()
	cmd := app.Root()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func TestInitCreatesStoreAndGitignore(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `Initialized hush project "api"`) {
		t.Fatalf("stdout: %s", out.String())
	}
	if !strings.Contains(out.String(), "Next: hush import .env") {
		t.Fatalf("stdout: %s", out.String())
	}
	p, err := project.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.ActiveEnv != "development" || p.Config.ProjectID == "" {
		t.Fatalf("%+v", p.Config)
	}
	key, err := keyring.Resolve(ring, p.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load(project.StorePath(dir), key)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "api" || doc.ProjectID != p.Config.ProjectID {
		t.Fatalf("doc %+v", doc)
	}
	keys, _ := doc.ListKeys("development")
	if len(keys) != 0 {
		t.Fatalf("keys %v", keys)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), ".hush/") {
		t.Fatalf("gitignore %s", gi)
	}
	fi, err := os.Stat(project.StorePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	err := runApp(t, app, "init")
	if err == nil || !strings.Contains(err.Error(), "already a hush project") {
		t.Fatalf("got %v", err)
	}
}

func TestInitDoesNotDuplicateGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".hush/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(gi), ".hush/") != 1 {
		t.Fatalf("%s", gi)
	}
}

func TestInitDefaultNameIsDirBase(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "myapp")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"myapp"`) {
		t.Fatalf("stdout %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -count=1`

Expected: FAIL compile.

- [ ] **Step 3: Write minimal implementation**

Run:

```
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/google/uuid@latest
go get golang.org/x/term@latest
```

`internal/ui/ui.go` — `Enabled(plain bool) bool` returns false if `plain` or `os.Getenv("NO_COLOR") != ""`. `Success` is just the string; commands write it. Keep lipgloss for bold headers used by later `ls`/`env ls`. Export:

```go
func ColorOn(plain bool) bool {
	if plain || os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func Bold(s string, on bool) string {
	if !on {
		return s
	}
	return lipgloss.NewStyle().Bold(true).Render(s)
}
```

`internal/cli/app.go`:

```go
package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/run"
	"github.com/cfaulkingham/hush/internal/ui"
)

type App struct {
	Ring    keyring.Ring
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Getwd   func() (string, error)
	Environ func() []string
	Exec    func(argv, env []string) error
	Now     func() time.Time
}

func NewOSApp() *App {
	return &App{
		Ring:    keyring.OS{},
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Getwd:   os.Getwd,
		Environ: os.Environ,
		Exec:    run.Exec,
		Now:     time.Now,
	}
}

func (a *App) Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hush",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().String("env", "", "environment (overrides active)")
	cmd.PersistentFlags().Bool("plain", false, "disable color")
	cmd.AddCommand(a.initCmd())
	return cmd
}

func Main(args []string) int {
	app := NewOSApp()
	cmd := app.Root()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(app.Stderr, err.Error())
		return exitCode(err)
	}
	return 0
}

func (a *App) color(cmd *cobra.Command) bool {
	plain, _ := cmd.Flags().GetBool("plain")
	return ui.ColorOn(plain)
}

func (a *App) envFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("env")
	return v
}
```

`ui.ColorOn(plain bool)` returns `!plain && os.Getenv("NO_COLOR") == ""`. Honor `--plain` and `NO_COLOR` only (no TTY check); tests set `NO_COLOR=1`.

`internal/cli/exit.go`:

```go
package cli

import (
	"errors"
	"os"

	"github.com/cfaulkingham/hush/internal/keyring"
)

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, keyring.ErrBackend) {
		return 2
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return 2
	}
	return 1
}
```

`internal/cli/gitignore.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
)

func ensureGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == ".hush/" {
			return nil
		}
	}
	add := ".hush/\n"
	if len(b) > 0 && b[len(b)-1] != '\n' {
		add = "\n" + add
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(add)
	return err
}
```

`internal/cli/init.go`:

```go
package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) initCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a hush project in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			hushDir := project.Dir(cwd)
			if fi, err := os.Lstat(hushDir); err == nil && !fi.IsDir() {
				return fmt.Errorf(".hush exists and is not a directory")
			}
			if _, err := os.Lstat(project.StorePath(cwd)); err == nil {
				return errors.New("already a hush project. See hush status.")
			}
			if name == "" {
				name = filepath.Base(cwd)
			}
			id := uuid.NewString()
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return err
			}
			keyStr, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(hushDir, 0755); err != nil {
				return err
			}
			if err := project.SaveConfig(cwd, project.Config{ProjectID: id, ActiveEnv: "development"}); err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, id, keyStr); err != nil {
				return err
			}
			doc := store.NewDocument(id, name, a.Now())
			if err := store.Save(project.StorePath(cwd), doc, raw); err != nil {
				return err
			}
			if err := ensureGitignore(cwd); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Initialized hush project %q (gitignored .hush/).\n", name)
			fmt.Fprintln(a.Stdout, "Key stored in keychain. Next: hush import .env")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: directory name)")
	return cmd
}
```

`cmd/hush/main.go`:

```go
package main

import (
	"os"
	"github.com/cfaulkingham/hush/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -count=1` and `go test ./... -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/hush/main.go internal/cli/ internal/ui/ go.mod go.sum
git commit -m "feat: add hush init with keychain key and gitignored store"
```

---

### Task 8: `status`, `use`, `env ls`, `env new`

**Files:**
- Create: `internal/cli/load.go`, `internal/cli/status.go`, `internal/cli/use.go`, `internal/cli/env.go`, `internal/cli/env_test.go`
- Modify: `internal/cli/app.go` (register commands)

**Interfaces:**
- Consumes: `App`, `project.Find`, `keyring.Resolve`, `store.Load`
- Produces:
  - `func (a *App) open(cmd *cobra.Command) (cwd string, p *project.Project, key []byte, doc *store.Document, envName string, err error)`
  - `envName` = `--env` if set, else `p.Config.ActiveEnv`
  - After load, if `doc.ProjectID != p.Config.ProjectID` return `project_id mismatch between config and store`
  - `status` output format from spec
  - `use` persists config
  - `env new --use` also persists

`open` must use `a.Getwd()` then `project.Find`. Key via `keyring.Resolve(a.Ring, p.Config.ProjectID)`.

- [ ] **Step 1: Write the failing tests**

`internal/cli/env_test.go`:

```go
package cli

import (
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/project"
)

func TestStatusAfterInit(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"project:", "api", "env:", "development", "secrets:", "0", "store:", "key:", "keychain"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, "hush_key_v1_") {
		t.Fatal("status leaked key")
	}
}

func TestEnvNewUseLs(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "env", "new", "staging"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "env", "ls"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "development") || !strings.Contains(s, "staging") {
		t.Fatalf("%s", s)
	}
	if err := runApp(t, app, "use", "staging"); err != nil {
		t.Fatal(err)
	}
	cfg, err := project.LoadConfig(dir)
	if err != nil || cfg.ActiveEnv != "staging" {
		t.Fatalf("%+v %v", cfg, err)
	}
	out.Reset()
	if err := runApp(t, app, "env", "new", "production", "--use"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = project.LoadConfig(dir)
	if cfg.ActiveEnv != "production" {
		t.Fatalf("%+v", cfg)
	}
}

func TestUseMissingEnv(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "use", "nope")
	if err == nil || !strings.Contains(err.Error(), "hush env new nope") {
		t.Fatalf("got %v", err)
	}
}

func TestStatusHUSH_KEYLabel(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	p, _ := project.Find(dir)
	s, err := ring.Get("hush", p.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUSH_KEY", s)
	out.Reset()
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "HUSH_KEY") {
		t.Fatalf("%s", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1`

Expected: FAIL (unknown commands).

- [ ] **Step 3: Write minimal implementation**

`internal/cli/load.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) loadStore() (string, *project.Project, []byte, *store.Document, error) {
	cwd, err := a.Getwd()
	if err != nil {
		return "", nil, nil, nil, err
	}
	p, err := project.Find(cwd)
	if err != nil {
		return "", nil, nil, nil, err
	}
	key, err := keyring.Resolve(a.Ring, p.Config.ProjectID)
	if err != nil {
		return "", nil, nil, nil, err
	}
	doc, err := store.Load(project.StorePath(p.Root), key)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if doc.ProjectID != p.Config.ProjectID {
		return "", nil, nil, nil, fmt.Errorf("project_id mismatch between config and store")
	}
	return cwd, p, key, doc, nil
}

func (a *App) open(cmd *cobra.Command) (string, *project.Project, []byte, *store.Document, string, error) {
	cwd, p, key, doc, err := a.loadStore()
	if err != nil {
		return "", nil, nil, nil, "", err
	}
	env := a.envFlag(cmd)
	if env == "" {
		env = p.Config.ActiveEnv
	}
	if _, err := doc.Env(env); err != nil {
		return "", nil, nil, nil, "", err
	}
	return cwd, p, key, doc, env, nil
}

func keySource() string {
	if os.Getenv("HUSH_KEY") != "" {
		return "HUSH_KEY"
	}
	return "keychain"
}

func (a *App) save(p *project.Project, doc *store.Document, key []byte) error {
	return store.Save(project.StorePath(p.Root), doc, key)
}
```

`internal/cli/status.go`:

```go
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "status",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			key, kerr := keyring.Resolve(a.Ring, p.Config.ProjectID)
			name := "(unknown)"
			nsecrets := 0
			storeLine := ".hush/store"
			var loadErr error
			if kerr != nil {
				storeLine = "decrypt failed"
				loadErr = kerr
			} else {
				doc, err := store.Load(project.StorePath(p.Root), key)
				if err != nil {
					storeLine = "decrypt failed"
					loadErr = err
				} else {
					name = doc.Name
					if env, err := doc.Env(p.Config.ActiveEnv); err == nil {
						nsecrets = len(env.Secrets)
					}
				}
			}
			fmt.Fprintf(a.Stdout, "project:  %s\n", name)
			fmt.Fprintf(a.Stdout, "id:       %s\n", p.Config.ProjectID)
			fmt.Fprintf(a.Stdout, "env:      %s\n", p.Config.ActiveEnv)
			fmt.Fprintf(a.Stdout, "secrets:  %d\n", nsecrets)
			fmt.Fprintf(a.Stdout, "store:    %s\n", storeLine)
			fmt.Fprintf(a.Stdout, "key:      %s\n", keySource())
			if loadErr != nil {
				if errors.Is(loadErr, store.ErrDecrypt) {
					return store.ErrDecrypt
				}
				return loadErr
			}
			return nil
		},
	}
}
```

`internal/cli/use.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/project"
)

func (a *App) useCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <env>",
		Args:  cobra.ExactArgs(1),
		Short: "Set the active environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			env, err := doc.Env(args[0])
			if err != nil {
				return err
			}
			p.Config.ActiveEnv = args[0]
			if err := project.SaveConfig(p.Root, p.Config); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Now using %s (%d secrets).\n", args[0], len(env.Secrets))
			return nil
		},
	}
}
```

`internal/cli/env.go`:

```go
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/project"
)

func (a *App) envCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Manage environments"}
	cmd.AddCommand(a.envLsCmd(), a.envNewCmd())
	return cmd
}

func (a *App) envLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "ls",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(doc.Environments))
			width := 0
			for name := range doc.Environments {
				names = append(names, name)
				if len(name) > width {
					width = len(name)
				}
			}
			sort.Strings(names)
			for _, name := range names {
				n := len(doc.Environments[name].Secrets)
				mark := ""
				if name == p.Config.ActiveEnv {
					mark = "  ●"
				}
				fmt.Fprintf(a.Stdout, "  %-*s  %d secrets%s\n", width, name, n, mark)
			}
			return nil
		},
	}
}

func (a *App) envNewCmd() *cobra.Command {
	var use bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Create an empty environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			if err := doc.NewEnv(args[0], a.Now()); err != nil {
				return err
			}
			if err := a.save(p, doc, key); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Created environment %s.\n", args[0])
			if use {
				p.Config.ActiveEnv = args[0]
				if err := project.SaveConfig(p.Root, p.Config); err != nil {
					return err
				}
				fmt.Fprintf(a.Stdout, "Now using %s.\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&use, "use", false, "switch to the new environment")
	return cmd
}
```

Update `Root()`: `cmd.AddCommand(a.initCmd(), a.statusCmd(), a.useCmd(), a.envCmd())`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: add status, env, and use commands"
```

---

### Task 9: `set`, `get`, `ls`, `rm`

**Files:**
- Create: `internal/cli/set.go`, `internal/cli/get.go`, `internal/cli/ls.go`, `internal/cli/rm.go`, `internal/cli/secrets_test.go`
- Modify: `internal/cli/app.go`

**Interfaces:**
- Consumes: `open`/`loadStore`, `store.PutSecret/GetSecret/DeleteSecret/ListKeys`, `store.Save`
- Produces: the four commands. `set` input priority: `KEY=VALUE` → `--from-stdin` → TTY prompt via `golang.org/x/term.ReadPassword`. If form 3 and stdin is not a terminal (`term.IsTerminal` on stdin fd), error `no value: pass KEY=VALUE, --from-stdin, or run on a TTY`. Tests use form 1 and `--from-stdin`.

- [ ] **Step 1: Write the failing tests**

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetGetLsRm(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "set", "DATABASE_URL=postgres://localhost/api"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "postgres://") {
		t.Fatal("set leaked value")
	}
	out.Reset()
	if err := runApp(t, app, "get", "DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "postgres://localhost/api\n" {
		t.Fatalf("get %q", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "ls"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DATABASE_URL") {
		t.Fatalf("%s", out.String())
	}
	if strings.Contains(out.String(), "postgres://") {
		t.Fatal("ls leaked value")
	}
	out.Reset()
	if err := runApp(t, app, "ls", "--values"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "postgres://localhost/api") {
		t.Fatalf("%s", out.String())
	}
	if err := runApp(t, app, "rm", "DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "get", "DATABASE_URL"); err == nil {
		t.Fatal("expected missing")
	}
}

func TestSetFromStdin(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	app.Stdin = bytes.NewBufferString("s3cret\n")
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "set", "TOKEN", "--from-stdin"); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	app.Stdout = out
	if err := runApp(t, app, "get", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "s3cret\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestSetEqualsInValue(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "K=a=b=c"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	_ = runApp(t, app, "get", "K")
	if out.String() != "a=b=c\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestGetMissing(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "get", "NOPE")
	if err == nil || !strings.Contains(err.Error(), "secret NOPE not found in development") {
		t.Fatalf("%v", err)
	}
}

func TestSetRejectsBadKey(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "FOO-BAR=1"); err == nil {
		t.Fatal("expected validation error")
	}
}
```

`--from-stdin` trims **one** trailing newline only (`bytes.TrimSuffix(b, []byte("\n"))` then also `\r` for `\r\n`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestSet`

Expected: FAIL unknown command.

- [ ] **Step 3: Write minimal implementation**

`internal/cli/set.go`:

```go
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) setCmd() *cobra.Command {
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "set <KEY=VALUE|KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Set a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			name, value, err := a.readSetValue(args[0], fromStdin)
			if err != nil {
				return err
			}
			if err := doc.PutSecret(envName, name, value, a.Now()); err != nil {
				return err
			}
			if err := store.Save(project.StorePath(p.Root), doc, key); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Set %s in %s.\n", name, envName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read value from stdin")
	return cmd
}

func (a *App) readSetValue(arg string, fromStdin bool) (string, string, error) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i], arg[i+1:], nil
	}
	name := arg
	if fromStdin {
		b, err := io.ReadAll(a.Stdin)
		if err != nil {
			return "", "", err
		}
		if len(b) > 0 && b[len(b)-1] == '\n' {
			b = b[:len(b)-1]
		}
		if len(b) > 0 && b[len(b)-1] == '\r' {
			b = b[:len(b)-1]
		}
		return name, string(b), nil
	}
	f, ok := a.Stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", "", fmt.Errorf("no value: pass KEY=VALUE, --from-stdin, or run on a TTY")
	}
	fmt.Fprintf(a.Stderr, "Value: ")
	pw, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(a.Stderr)
	if err != nil {
		return "", "", err
	}
	return name, string(pw), nil
}
```

`internal/cli/get.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Print a secret value",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			v, err := doc.GetSecret(envName, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, v)
			return nil
		},
	}
}
```

`internal/cli/ls.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) lsCmd() *cobra.Command {
	var values bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List secret keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			keys, err := doc.ListKeys(envName)
			if err != nil {
				return err
			}
			active := ""
			if envName == p.Config.ActiveEnv {
				active = "  ● active"
			}
			fmt.Fprintf(a.Stdout, "%s  %d secrets%s\n", envName, len(keys), active)
			m, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			for _, k := range keys {
				if values {
					fmt.Fprintf(a.Stdout, "  %s  %s\n", k, m[k])
				} else {
					fmt.Fprintf(a.Stdout, "  %s\n", k)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&values, "values", false, "print secret values")
	return cmd
}
```

`internal/cli/rm.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			if err := doc.DeleteSecret(envName, args[0]); err != nil {
				return err
			}
			if err := store.Save(project.StorePath(p.Root), doc, key); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Removed %s from %s.\n", args[0], envName)
			return nil
		},
	}
}
```

Register: `cmd.AddCommand(..., a.setCmd(), a.getCmd(), a.lsCmd(), a.rmCmd())`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: add set, get, ls, and rm commands"
```

---

### Task 10: `import` and `export`

**Files:**
- Create: `internal/cli/import.go`, `internal/cli/export.go`, `internal/cli/io_test.go`
- Modify: `internal/cli/app.go`

**Interfaces:**
- Consumes: `dotenv.ParseFile`, `dotenv.Serialize`, `dotenv.SerializeJSON`, `open`
- Produces: `import <file> [--overwrite]`, `export [--format dotenv|json] [-o file] [--overwrite]`

Import: merge into existing env; skip existing keys unless `--overwrite`; **abort with no writes** if any key fails `ValidateKey` or any value fails `ValidateValue`. Report the first bad key. Summary strings from spec.

Export default stdout dotenv. `-o` mode 0600, refuse existing file unless `--overwrite`. Empty env → empty dotenv / `{}\n`.

- [ ] **Step 1: Write the failing tests**

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportExportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\nBAZ=qux\n"), 0644); err != nil {
		t.Fatal(err)
	}
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "import", envPath); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Imported 2 keys into development (0 skipped)") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "export"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "FOO=bar") || !strings.Contains(out.String(), "BAZ=qux") {
		t.Fatalf("%s", out.String())
	}
}

func TestImportSkipAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=old")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=new\nBAR=x\n"), 0644)
	out.Reset()
	if err := runApp(t, app, "import", p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Imported 1 keys into development (1 skipped)") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	_ = runApp(t, app, "get", "FOO")
	if out.String() != "old\n" {
		t.Fatalf("skipped failed: %q", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "import", p, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 overwritten") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	_ = runApp(t, app, "get", "FOO")
	if out.String() != "new\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestImportRejectsBadKeyNoPartialWrite(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("GOOD=1\nBAD-KEY=2\n"), 0644)
	if err := runApp(t, app, "import", p); err == nil {
		t.Fatal("expected error")
	}
	out.Reset()
	if err := runApp(t, app, "get", "GOOD"); err == nil {
		t.Fatal("partial import wrote GOOD")
	}
}

func TestExportJSONAndOutputFile(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "A=1")
	out.Reset()
	if err := runApp(t, app, "export", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"A": "1"`) {
		t.Fatalf("%s", out.String())
	}
	dest := filepath.Join(dir, "out.env")
	if err := runApp(t, app, "export", "-o", dest); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "export", "-o", dest); err == nil {
		t.Fatal("expected refuse overwrite")
	}
	if err := runApp(t, app, "export", "-o", dest, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(dest)
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
}

func TestImportMissingFile(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "import", filepath.Join(dir, "nope.env"))
	if err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("%v", err)
	}
}

func TestImportMissingEnv(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=bar\n"), 0644)
	err := runApp(t, app, "import", p, "--env", "staging")
	if err == nil || !strings.Contains(err.Error(), "staging") {
		t.Fatalf("%v", err)
	}
}
```

English: `Imported 1 keys` is grammatically off. Spec examples use `Imported 14 keys` — keep `keys` even for 1.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestImport`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`internal/cli/import.go`:

```go
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/dotenv"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) importCmd() *cobra.Command {
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Args:  cobra.ExactArgs(1),
		Short: "Import secrets from a .env file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			parsed, err := dotenv.ParseFile(args[0])
			if err != nil {
				return err
			}
			names := make([]string, 0, len(parsed))
			for k := range parsed {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				if err := store.ValidateKey(k); err != nil {
					return err
				}
				if err := store.ValidateValue(parsed[k]); err != nil {
					return err
				}
			}
			existing, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			added, skipped, overwritten := 0, 0, 0
			for _, k := range names {
				_, had := existing[k]
				if had && !overwrite {
					skipped++
					continue
				}
				if err := doc.PutSecret(envName, k, parsed[k], a.Now()); err != nil {
					return err
				}
				if had {
					overwritten++
				} else {
					added++
				}
			}
			if err := store.Save(project.StorePath(p.Root), doc, key); err != nil {
				return err
			}
			if overwrite {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d overwritten).\n", added+overwritten, envName, overwritten)
			} else {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d skipped).\n", added, envName, skipped)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing keys")
	return cmd
}
```

`internal/cli/export.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/dotenv"
)

func (a *App) exportCmd() *cobra.Command {
	var format, output string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "export",
		Args:  cobra.NoArgs,
		Short: "Export secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			m, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			var b []byte
			switch format {
			case "", "dotenv":
				b, err = dotenv.Serialize(m)
			case "json":
				b, err = dotenv.SerializeJSON(m)
			default:
				return fmt.Errorf("unknown format %q (dotenv|json)", format)
			}
			if err != nil {
				return err
			}
			if output == "" {
				_, err = a.Stdout.Write(b)
				return err
			}
			flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
			if overwrite {
				flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			}
			f, err := os.OpenFile(output, flags, 0600)
			if err != nil {
				if os.IsExist(err) {
					return fmt.Errorf("%s exists (pass --overwrite)", output)
				}
				return err
			}
			defer f.Close()
			if _, err := f.Write(b); err != nil {
				return err
			}
			return os.Chmod(output, 0600)
		},
	}
	cmd.Flags().StringVar(&format, "format", "dotenv", "dotenv or json")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite output file")
	return cmd
}
```

Register both on Root.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: import and export dotenv and json"
```

---

### Task 11: `hush run`

**Files:**
- Create: `internal/cli/run.go`, `internal/cli/run_test.go`
- Modify: `internal/cli/app.go`

**Interfaces:**
- Consumes: `open`, `doc.SecretMap`, `run.Overlay`, `a.Exec`, `a.Environ`
- Produces: `run` command with `SetInterspersed(false)`, `cobra.MinimumNArgs(1)` after flag parse. Does not read `.env` from disk. Overlay secrets win.

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"strings"
	"testing"
)

func TestRunOverlaysAndExec(t *testing.T) {
	dir := t.TempDir()
	var gotArgv, gotEnv []string
	app, _, _, _ := newTestApp(t, dir)
	app.Environ = func() []string { return []string{"PATH=/bin", "FOO=parent"} }
	app.Exec = func(argv, env []string) error {
		gotArgv = append([]string{}, argv...)
		gotEnv = append([]string{}, env...)
		return nil
	}
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=from-hush")
	_ = runApp(t, app, "set", "BAR=baz")
	if err := runApp(t, app, "run", "npm", "start"); err != nil {
		t.Fatal(err)
	}
	if len(gotArgv) != 2 || gotArgv[0] != "npm" || gotArgv[1] != "start" {
		t.Fatalf("argv %v", gotArgv)
	}
	m := map[string]string{}
	for _, kv := range gotEnv {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	if m["FOO"] != "from-hush" || m["BAR"] != "baz" || m["PATH"] != "/bin" {
		t.Fatalf("%v", m)
	}
}

func TestRunRequiresCommand(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "run")
	if err == nil || !strings.Contains(err.Error(), "hush run -- npm start") {
		t.Fatalf("%v", err)
	}
}

func TestRunEnvFlag(t *testing.T) {
	dir := t.TempDir()
	var gotEnv []string
	app, _, _, _ := newTestApp(t, dir)
	app.Exec = func(argv, env []string) error {
		gotEnv = env
		return nil
	}
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "env", "new", "staging")
	_ = runApp(t, app, "set", "--env", "staging", "ONLY=yes")
	if err := runApp(t, app, "run", "--env", "staging", "--", "true"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, kv := range gotEnv {
		if kv == "ONLY=yes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", gotEnv)
	}
}
```

`set --env staging` requires `set` to honor the persistent `--env` flag (already via `open`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestRun`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *App) runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [--] <command> [args...]",
		Short: "Run a command with secrets in the environment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return run.ErrNoCommand
			}
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			secrets, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			env := run.Overlay(a.Environ(), secrets)
			return a.Exec(args, env)
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}
```

If cobra still errors on `run` with no args before RunE, set `Args: cobra.ArbitraryArgs` and check `len(args)==0` in RunE (as above). Register on Root.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: inject secrets into child processes with hush run"
```

---

### Task 12: `key backup` and `key restore`

**Files:**
- Create: `internal/cli/key.go`, `internal/cli/key_test.go`
- Modify: `internal/cli/app.go`

**Interfaces:**
- Consumes: `keyring.FormatKey`, `ParseKey`, `Resolve`, `store.Load`
- Produces:
  - `hush key backup` — key string + newline on **stdout**; warning on **stderr**; never skip the warning
  - `hush key restore <key>` — parse, decrypt first, then Set keychain unless `HUSH_KEY` is set (then warn `HUSH_KEY is set; not writing keychain` and exit 0 if decrypt worked)

- [ ] **Step 1: Write the failing tests**

```go
package cli

import (
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
)

func TestKeyBackupRestore(t *testing.T) {
	dir := t.TempDir()
	app, out, errb, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init", "--name", "api")
	out.Reset()
	errb.Reset()
	if err := runApp(t, app, "key", "backup"); err != nil {
		t.Fatal(err)
	}
	key := strings.TrimSpace(out.String())
	if !strings.HasPrefix(key, "hush_key_v1_") {
		t.Fatalf("stdout %q", out.String())
	}
	if !strings.Contains(errb.String(), "Anyone with it can decrypt") {
		t.Fatalf("stderr %s", errb.String())
	}
	p, _ := project.Find(dir)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", "")
	if err := runApp(t, app, "status"); err == nil {
		t.Fatal("expected missing key")
	}
	if err := runApp(t, app, "key", "restore", key); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRestoreWrongKey(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	old, _ := ring.Get("hush", p.Config.ProjectID)
	raw := make([]byte, 32)
	raw[0] = 9
	bad, _ := keyring.FormatKey(raw)
	if err := runApp(t, app, "key", "restore", bad); err == nil {
		t.Fatal("expected fail")
	}
	got, _ := ring.Get("hush", p.Config.ProjectID)
	if got != old {
		t.Fatal("keychain mutated on failed restore")
	}
}

func TestKeyRestoreDoesNotWriteWhenHUSH_KEYSet(t *testing.T) {
	dir := t.TempDir()
	app, _, errb, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	s, _ := ring.Get("hush", p.Config.ProjectID)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", s)
	if err := runApp(t, app, "key", "restore", s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "HUSH_KEY is set; not writing keychain") {
		t.Fatalf("%s", errb.String())
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); err == nil {
		t.Fatal("wrote keychain despite HUSH_KEY")
	}
}
```

`TestKeyBackupRestore` deletes the memory key then expects `status` to fail. `Resolve` with empty HUSH_KEY and missing ring entry returns `ErrNoKey`. After restore, status works.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestKey`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`internal/cli/key.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

func (a *App) keyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "key", Short: "Backup or restore the project key"}
	cmd.AddCommand(a.keyBackupCmd(), a.keyRestoreCmd())
	return cmd
}

func (a *App) keyBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "backup",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, key, _, err := a.loadStore()
			if err != nil {
				return err
			}
			s, err := keyring.FormatKey(key)
			if err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, s)
			fmt.Fprintln(a.Stderr, "This is the project key. Anyone with it can decrypt .hush/store.")
			return nil
		},
	}
}

func (a *App) keyRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "restore <key>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			raw, err := keyring.ParseKey(args[0])
			if err != nil {
				return err
			}
			if _, err := store.Load(project.StorePath(p.Root), raw); err != nil {
				return err
			}
			if os.Getenv("HUSH_KEY") != "" {
				fmt.Fprintln(a.Stderr, "HUSH_KEY is set; not writing keychain")
				return nil
			}
			s, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, p.Config.ProjectID, s); err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, "Key restored. Store decrypts OK.")
			return nil
		},
	}
}
```

Register `a.keyCmd()` on Root. The restore success message is required when the keychain is written; when `HUSH_KEY` is set, only the stderr warning is required (exit 0).

- [ ] **Step 4: Run tests**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: backup and restore the project key"
```

---

### Task 13: README, CI, version flag

**Files:**
- Create: `README.md`, `.github/workflows/test.yml`
- Modify: `internal/cli/app.go` (`--version`), `internal/cli/version.go`

**Interfaces:**
- Produces: `hush --version` / `hush version` prints `hush 0.1.0\n`. README quickstart matches the spec success transcript (commands, not exact UUIDs).

- [ ] **Step 1: Write the failing test**

```go
package cli

import "strings"
import "testing"

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "--version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hush 0.1.0") {
		t.Fatalf("%s", out.String())
	}
}
```

Cobra’s default `--version` prints to stdout only if `cmd.Version` is set. Set `cmd.Version = "0.1.0"` and `cmd.SetVersionTemplate("hush {{.Version}}\n")`. Route version to `a.Stdout` via `cmd.SetOut(a.Stdout)` in `Root()`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -count=1 -run TestVersion`

Expected: FAIL (empty version).

- [ ] **Step 3: Implement version, README, workflow**

In `Root()`:

```go
cmd.Version = "0.1.0"
cmd.SetVersionTemplate("hush {{.Version}}\n")
cmd.SetOut(a.Stdout)
cmd.SetErr(a.Stderr)
```

`.github/workflows/test.yml`:

```yaml
name: test
on:
  push:
  pull_request:
jobs:
  test:
    timeout-minutes: 15
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go vet ./...
      - if: runner.os != 'Windows'
        run: go test ./...
      - if: runner.os == 'Windows'
        run: go build ./...
      - if: runner.os != 'Windows'
        run: go test -race ./...
```

`README.md`:

```markdown
# hush

Local-first secrets for people who already have a `.env`.

## Install

```bash
go install github.com/cfaulkingham/hush/cmd/hush@latest
```

Or from this repo:

```bash
go build -o hush ./cmd/hush
```

## Quickstart

```bash
hush init
hush import .env
hush ls
hush run -- npm start
```

Secrets live in gitignored `.hush/store` (XChaCha20-Poly1305). The project key sits in your OS keychain. CI:

```bash
HUSH_KEY=hush_key_v1_... hush run -- pytest   # key from `hush key backup`
```

## Commands

init, status, import, export, use, env ls, env new, set, get, ls, rm, run, key backup, key restore.

See `docs/superpowers/specs/2026-08-22-hush-core-cli-design.md`.
```

- [ ] **Step 4: Run tests**

Run: `go test ./... -count=1`

Expected: PASS. Also `go build -o /tmp/hush ./cmd/hush` succeeds.

Manual smoke in a temp dir (not committed): `./hush init && ./hush set FOO=bar && ./hush run -- /usr/bin/env | grep '^FOO='`

- [ ] **Step 5: Commit**

```bash
git add README.md .github/workflows/test.yml internal/cli/
git commit -m "docs: add README, version flag, and test workflow"
```

---

## Self-review vs spec

| Spec requirement | Task |
|------------------|------|
| HUSH1 + XChaCha20-Poly1305 + AAD magic | 1 |
| Atomic 0600 write, symlink refuse | 2 |
| Key/env/value validation, 64 KiB | 2 |
| Keychain + `hush_key_v1_` + HUSH_KEY | 3 |
| Walk-up `.hush/store`, config.json | 4 |
| godotenv import/export round-trip | 5, 10 |
| Overlay + unix Exec / windows wait | 6, 11 |
| init, gitignore, nested cwd init | 7 |
| status, use, env ls/new | 8 |
| set/get/ls/rm, no accidental reveal | 9 |
| import merge/overwrite/abort | 10 |
| run does not read `.env` | 11 |
| key backup/restore | 12 |
| README, MIT (task 1), CI | 1, 13 |
| `--plain` / NO_COLOR | 7 |
| Exit 0/1/2 | 7 |
| Out of scope (network, doctor, inheritance) | not planned |

No remaining spec gaps. Do not add `hush doctor`, env copy, or network in these tasks.
