# hush — core CLI + .env import

Date: 2026-08-22
Status: approved design (pending spec review)
Scope: first product slice of an open-source secrets/config manager

This spec covers only the local-first Go CLI. SDKs, Vercel/GitHub sync, the web dashboard, and the paid cloud are later slices and are out of scope here.

## Summary

`hush` is a DX-obsessed local secrets manager. A project keeps one gitignored encrypted blob in `.hush/store`. A 256-bit project key lives in the OS keychain (or `HUSH_KEY` in CI). Named environments (`development`, `staging`, `production`, plus custom names) each hold a full secret map with no inheritance. The daily loop is: import a `.env`, list/get/set secrets, `hush run -- <cmd>` so secrets never need to touch disk as plaintext.

## Goals

- A developer can go from an existing `.env` to `hush run -- npm start` in under a minute, without creating an account.
- Secret values never appear in command output unless the user asked (`get`, `ls --values`, `export`, `key backup`).
- The store is a versioned, documented file format so a future cloud can sync the same blob.
- The CLI is a single static Go binary with no runtime.

## Non-goals (this slice)

- Network I/O, accounts, teams, or a hosted API
- SDKs, GitHub Actions sync, Vercel env sync
- Environment inheritance / branching configs
- `hush doctor`, env copy, secret rename, env diff
- Age/SOPS interoperability
- A TUI or interactive init wizard
- Telemetry
- Writing `.env` as a side effect of `run` or `set`

## Users and success

Primary user: a solo or small-team developer who already has `.env` files and wants a safer local workflow before any cloud.

Success looks like:

```text
$ hush init
Initialized hush project "api" (gitignored .hush/).
Key stored in keychain. Next: hush import .env

$ hush import .env
Imported 14 keys into development (0 skipped).

$ hush ls
development  14 secrets  ● active
  DATABASE_URL
  REDIS_URL
  ...

$ hush run -- npm start
# process env contains the secrets; no .env written
```

CI success: `HUSH_KEY=hush_key_v1_<hex> hush run -- pytest` decrypts the same store without a keychain.

## Architecture

```text
cmd/hush                cobra entrypoint
internal/cli            command wiring, flags, human output
internal/project        walk up from cwd to find .hush/
internal/store          JSON schema, encrypt/decrypt, atomic write
internal/keyring        OS keychain + HUSH_KEY overlay
internal/dotenv         .env parse (godotenv) + round-trip serialize
internal/run            merge env, exec/replace process
internal/ui             lipgloss helpers, NO_COLOR / non-TTY
```

Data flow:

1. Resolve project: walk parents of cwd until `.hush/store` exists. If none, fail with `no hush project (run hush init)`.
2. Resolve key: read `project_id` from plaintext `.hush/config.json`. If `HUSH_KEY` is set, use it; otherwise OS keychain service `hush`, account = `project_id`. After decrypt, the blob’s `project_id` must equal `config.json`.
3. Decrypt `.hush/store` with XChaCha20-Poly1305.
4. Resolve environment: `--env` flag if present, else `config.json` `active_env`.
5. Mutating commands write a new ciphertext (fresh nonce) via temp file + rename, mode `0600`.
6. Read-modify-write commands hold an advisory `.hush/store.lock` for the complete transaction so concurrent invocations cannot lose updates.

### On-disk layout

```text
.hush/config.json   plaintext, 0644
.hush/store         binary, 0600, gitignored
.hush/store.lock    advisory transaction lock, 0600, gitignored
```

`hush init` appends `.hush/` to `.gitignore` (creates the file if missing; does not duplicate an existing `.hush/` line).

### `config.json`

```json
{
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "active_env": "development"
}
```

No secret values. `project_id` is a UUID v4 string.

### Store file (binary)

| Offset | Size | Field |
|--------|------|--------|
| 0 | 5 | Magic `HUSH1` |
| 5 | 24 | XChaCha20-Poly1305 nonce |
| 29 | remaining | ciphertext (AEAD tag appended by the cipher) |

Key: 32 random bytes from `crypto/rand`.

Cipher: `golang.org/x/crypto/chacha20poly1305` XChaCha20-Poly1305. Additional data: the 5-byte magic, so a truncated/swapped header fails closed.

Unknown magic or version → error `unrecognized hush store (magic …). This CLI reads HUSH1 only.`

### Decrypted JSON

```json
{
  "version": 1,
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "api",
  "created_at": "2026-08-22T12:00:00Z",
  "updated_at": "2026-08-22T12:05:00Z",
  "environments": {
    "development": {
      "updated_at": "2026-08-22T12:05:00Z",
      "secrets": {
        "DATABASE_URL": {
          "value": "postgres://localhost/api",
          "updated_at": "2026-08-22T12:05:00Z"
        }
      }
    }
  }
}
```

Rules:

- `version` must be `1`. Any other value → error naming the version, no write.
- `project_id` must equal `config.json` `project_id`.
- Environment names: `^[A-Za-z][A-Za-z0-9_-]*$`, 1–64 chars. Default created by init: `development`.
- Secret keys: `^[A-Za-z_][A-Za-z0-9_]*$`, 1–256 chars (POSIX-ish env names). `HUSH_KEY` is reserved, case-insensitively.
- Secret values: UTF-8 text, 0–65536 bytes. Binary is rejected.
- JSON object keys are written sorted (environments, then secrets) so snapshots in tests are stable. Nonces are still random, so ciphertext always changes.

### Keychain and CI

Interface (so tests inject memory):

```text
Get(service, account) (secret, error)
Set(service, account, secret) error
Delete(service, account) error
```

Production backend: `github.com/zalando/go-keyring`.

Key string format: `hush_key_v1_` + hex(32 bytes), lowercase. Example: `hush_key_v1_` + 64 hex chars.

Resolution order:

1. If `HUSH_KEY` is set (non-empty), use it. Do not consult the keychain.
2. Else keychain `service=hush`, `account=project_id`.
3. Else error `no project key. Restore with hush key restore <key>, or set HUSH_KEY.`

`HUSH_KEY` winning over the keychain is intentional (CI and “try this backup key”).

Never print `HUSH_KEY` or the keychain secret, including in errors. Decrypt failure message: `could not decrypt .hush/store (wrong key or corrupted file).`

Platforms: macOS Keychain, Windows Credential Manager, Linux Secret Service via libsecret. Headless Linux CI is expected to use `HUSH_KEY`, not a session keyring.

## Command surface

Global flags:

- `--env name` — override active environment for this invocation (not persisted).
- `--plain` — disable color even on a TTY.
- `--help`, `--version`

Color is on when stdout is a TTY and `NO_COLOR` is unset and `--plain` is absent. Errors go to stderr; they still use color under the same rule applied to stderr’s TTY.

Cobra + lipgloss. Quiet one-line successes. No banners, no telemetry, no auto-update.

### `hush init [--name <name>]`

- Refuses if `.hush/store` already exists in cwd (not parents): `already a hush project. See hush status.`
- `name` defaults to the current directory’s base name.
- Creates `.hush/`, writes `config.json` with a new UUID and `active_env=development`.
- Refuses while `HUSH_KEY` is set so the freshly generated key cannot be shadowed on the next command.
- Generates 32-byte key, stores it in the keychain under that UUID.
- Writes an encrypted store with one empty environment `development`.
- Ensures `.gitignore` contains `.hush/`.
- Prints the success blurb in Summary.

Does not walk up: init is always cwd. A nested init is allowed and becomes the project for that subtree (walk-up finds the nearest `.hush/store`).

### `hush status`

Prints:

```text
project:  api
id:       550e8400-e29b-41d4-a716-446655440000
env:      development
secrets:  14
store:    .hush/store
key:      keychain
```

`key` is one of `keychain` | `HUSH_KEY`. Never the secret. Decrypts to prove the key works; on failure the `store` line says `decrypt failed` and exit code is 1.

### `hush import <file> [--overwrite]`

- Parses `<file>` with `github.com/joho/godotenv`. Last duplicate key in the file wins (godotenv default).
- Target environment: `--env` or active env. The environment must already exist.
- Merge: new keys inserted; existing keys skipped unless `--overwrite`.
- If any imported key fails secret-key validation, abort with no writes. Report the first bad key.
- If any value exceeds 64 KiB, abort with no writes.
- Missing file: exit 1, `cannot read <path>: …`.
- Summary: `Imported 14 keys into development (3 skipped).` With `--overwrite`, skipped is replaced by `N overwritten`.

### `hush export [--format dotenv|json] [-o file]`

- Default format `dotenv`, default destination stdout.
- `json`: pretty 2-space object of string→string, keys sorted.
- `dotenv`: round-trip through our serializer so `godotenv` can parse it back, including values with spaces, `#`, quotes, and newlines.
- `-o file` writes with mode `0600`. Refuses to overwrite an existing file unless `--overwrite`.
- Empty env: empty stdout / empty JSON object `{}`, exit 0.

### `hush use <env>`

- Persists `active_env` in `config.json`.
- Unknown env: exit 1, `environment staging not found. Create it with: hush env new staging`.
- Success: `Now using staging (8 secrets).`

### `hush env ls`

```text
  development  14 secrets  ●
  staging       8 secrets
  production    8 secrets
```

Active env marked. Sorted by name.

### `hush env new <name> [--use]`

- Creates an empty environment. Duplicate name: exit 1, `environment staging already exists`.
- Does not switch unless `--use`.
- Success: `Created environment staging.` plus `Now using staging.` when `--use`.

### `hush set <KEY=VALUE | KEY>`

Three input forms, in this priority:

1. `hush set KEY=VALUE` — split on the first `=`. Value may contain further `=`.
2. `hush set KEY --from-stdin` — read stdin fully (trim a single trailing newline).
3. `hush set KEY` — prompt on a TTY with hidden input (`golang.org/x/term`). If stdin is not a TTY: exit 1, `no value: pass KEY=VALUE, --from-stdin, or run on a TTY`.

Overwrites an existing key. Success: `Set DATABASE_URL in development.` (no value).

### `hush get <KEY>`

- Prints the raw value to stdout followed by a single newline. No key name, no quotes.
- Missing: exit 1, stderr `secret DATABASE_URL not found in development`.

### `hush ls [--values]`

Without `--values`, keys only, sorted, one per line, indented under a header line `development  14 secrets  ● active`.

With `--values`, `KEY` then two spaces then value. Values may be long; do not truncate. This is an explicit reveal.

### `hush rm <KEY>`

- Missing: exit 1, `secret DATABASE_URL not found in development`.
- Success: `Removed DATABASE_URL from development.`

### `hush run [--] <command> [args…]`

- Requires at least one command token. Example in the error: `hush run -- npm start`.
- Loads secrets for the resolved env, copies `os.Environ()`, overlays secrets (secrets win on name conflict).
- Removes `HUSH_KEY` from the child environment; the decrypt key is never inherited by the launched command.
- Does not read `.env` from disk.
- Unix: `syscall.Exec` so hush is replaced (signals and exit code are the child’s).
- Windows: start the process, forward stdin/stdout/stderr, wait, exit with the child’s code.
- `hush run --env production -- ./server` works. `--` is optional unless the command looks like a hush flag.

### `hush key backup`

- Writes the key string (`hush_key_v1_` + hex) to stdout with a trailing newline.
- Stderr: `This is the project key. Anyone with it can decrypt .hush/store.`
- Exit 0.

### `hush key restore <key>`

- Validates the `hush_key_v1_` + 64 hex format (case-insensitive hex).
- Attempts decrypt with the new key before persisting. On failure, does not write the keychain, exit 1.
- On success, writes the keychain (unless `HUSH_KEY` is set — then warn `HUSH_KEY is set; not writing keychain` and exit 0 if decrypt worked).
- Success: `Key restored. Store decrypts OK.`

## Safety and errors

Exit codes:

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | user/usage/project error (not found, validation, decrypt) |
| 2 | unexpected I/O or keychain backend failure |

Rules:

- `ls` (without `--values`), `status`, `init`, `use`, `env ls`, `env new`, `set`, `rm` never print secret values.
- Debug-style wrapping of errors must not include plaintext values or keys.
- Store writes: write temp in `.hush/` (`store.tmp.<random>`), `fsync`, `chmod 0600`, rename over `store`. On any failure, leave the previous `store` intact and remove the temp.
- Do not follow symlinks for `.hush/store` writes. If `store` is a symlink, refuse (`refusing to write through symlink .hush/store`).
- Init refuses to run if cwd `.hush` exists as a non-directory.

## Testing

`go test ./...` is the gate. No live OS keychain in unit tests.

- `internal/store`: round-trip; wrong key; truncated file; bad magic; JSON version ≠ 1; project_id mismatch; 64 KiB+1 value rejected; atomic write leaves old file on injected failure.
- `internal/dotenv`: fixtures under `testdata/dotenv/` covering comments, `export`, single/double quotes, blank values, unicode, multiline double-quoted, duplicate keys. Export → import is identity on keys and values.
- `internal/keyring`: in-memory backend; `HUSH_KEY` overrides memory.
- `internal/project`: find nearest `.hush/store` walking up; none at root.
- `internal/cli`: temp directories, cobra `ExecuteC`. Cover init, import, ls (assert values absent), get, set, rm, use, env new, export, run (child that prints one injected var), key backup/restore.
- `internal/run`: overlay wins over parent env; missing command errors.

GitHub Actions: vet, unit tests, and the race detector on Linux and macOS, plus vet and a cross-build on Windows.

## Repo layout

```text
cmd/hush/main.go
internal/cli/
internal/project/
internal/store/
internal/keyring/
internal/dotenv/
internal/run/
internal/ui/
testdata/dotenv/
go.mod                  module github.com/cfaulkingham/hush, go 1.23
README.md               install via `go install` / `go build`, quickstart matching this spec
LICENSE                 MIT
.gitignore
.github/workflows/test.yml
docs/superpowers/specs/2026-08-22-hush-core-cli-design.md
```

Package boundaries: `cli` depends on the others; `store` does not depend on `cli` or `keyring`; `keyring` does not depend on `store`. `run` takes a `map[string]string`, not a store.

## Later slices (not this spec)

Order after this CLI exists:

1. Self-hostable API that syncs the same `HUSH1` blob
2. TypeScript SDK
3. GitHub Actions and Vercel env push
4. Web dashboard
5. Paid hosted cloud (auth, billing, multi-tenant)

The blob format and key string are the contract those slices will reuse. Do not break `HUSH1` without a `HUSH2` reader/writer pair.

## Key decisions

| Decision | Choice | Why |
|----------|--------|-----|
| First slice | Core CLI + .env, no network | Platform is too large for one spec; this is the foundation |
| Store location | Per-project `.hush/` gitignored | Matches the `.env` mental model; cloud later syncs the project |
| Unlock | OS keychain, random 32-byte key | Daily DX is biometric/OS; CI uses `HUSH_KEY` |
| Environments | Named, no inheritance | Simple, maps to one `.env` per env |
| Language | Go, static binary | Category-standard (Doppler, Infisical, gh) |
| File layout | Single encrypted blob | Simplest backup and future sync document |
| Cipher | XChaCha20-Poly1305, magic `HUSH1` | Modern AEAD, versioned, no age CLI dependency |
| Parser | godotenv | Do not invent a `.env` grammar |
| `run` | Unix exec replace | Child owns signals and exit code |
| Reveal | Opt-in only | `ls` is safe to screenshot |

## Open questions

None. Name (`hush`), format (`HUSH1`), and v1 command set are decided.
