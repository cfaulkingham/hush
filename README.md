# hush

Local-first secrets for people who already have a `.env`.

## Install

```bash
go install github.com/cfaulkingham/hush/cmd/hush@latest
```

Or grab a signed-off release binary (macOS/Linux/Windows, `checksums.txt`) from
GitHub Releases. Or from this repo:

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

## Key handling

Keys and secrets are never read from command-line arguments (argv leaks into
process listings and shell history):

```bash
hush set KEY                 # hidden prompt
hush key backup --wrap       # key sealed with a passphrase; safe to paste anywhere
hush key restore < key.txt   # reads stdin, --file, or a hidden prompt; prompts for passphrase when sealed
hush key rotate              # re-encrypt .hush/store with a fresh key; old backups stop working
```

`hush export -o file` writes atomically with mode 0600, refuses symlinks, and
refuses paths git would track unless you pass `--force`.

## Automation

- `--json` on `status`, `ls`, `env ls`, and `version` for scripts and CI gates
  (`ls --json` carries names only unless you also pass `--values`).
- `hush run --require KEY1,KEY2` (or `--require-file .env.example`) fails
  before launching the command if required secrets are missing.
- `--dry-run` on `import`, `rm`, `env rm`, `deinit`, and `key rotate` previews
  the effect with **masked** values; destructive commands prompt on a terminal
  and take `--yes`/`--force` for scripts.

## Shell completion

```bash
# zsh (~/.zshrc)
source <(hush completion zsh)
# bash (~/.bashrc)
source <(hush completion bash)
```

`get`/`rm` complete secret names, `use`/`env`/`--env` complete environments.

## Commands

init, deinit, status, import, export, use, env ls, env new, env copy, env rename,
env rm, set, get, ls, rm, run, key backup, key restore, key rotate, version.

## Exit codes

| code | meaning |
|------|---------|
| 0 | success |
| 1 | runtime failure |
| 2 | usage error (flags, arguments, unknown command) |
| 3 | keychain backend failure |
| 126 | `hush run`: command found but not executable |
| 127 | `hush run`: command not found |

Exit codes of commands run via `hush run` pass through unchanged.

Walk through a real app in [`samples/shop-api`](samples/shop-api).

See `docs/superpowers/specs/2026-08-22-hush-core-cli-design.md`.
