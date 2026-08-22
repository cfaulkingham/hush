# hush

Local-first secrets for people who already have a `.env`.

## Install

```bash
go install hush/cmd/hush@latest
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
export HUSH_KEY=hush_key_v1_...   # from `hush key backup`
hush run -- pytest
```

## Commands

init, status, import, export, use, env ls, env new, set, get, ls, rm, run, key backup, key restore.

See `docs/superpowers/specs/2026-08-22-hush-core-cli-design.md`.
