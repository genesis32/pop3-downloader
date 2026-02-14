# Copilot Instructions for pop3-downloader

## Project Overview

Go CLI utility that downloads emails from a POP3S server into mbox format with Message-ID-based deduplication, optional server-side deletion, and mutt integration.

## Architecture

Single-package (`main`) app with three source files:

- **main.go** — CLI flags (`flag` package), TOML config loading, workflow orchestration via `run(Config)`, mutt launcher
- **pop3.go** — POP3S/POP3 connection (`github.com/knadh/go-pop3`), auth, message fetch/delete. `connectPOP3S` for production (TLS), `connectPOP3` for tests (plain)
- **mbox.go** — mbox read/write (`github.com/emersion/go-mbox`), `extractMessageID` for dedup, `getExistingMessageIDs` loads known IDs, `writeMbox` appends only new messages

Data flow: connect → fetch all → load existing Message-IDs from mbox → write new messages → delete from server (unless `-dryrun`).

## Critical Invariant

Messages MUST be written to the mbox file BEFORE deletion from the server. This ordering in `run()` prevents data loss. Never reorder steps 3 and 4 in the workflow.

## Build & Test

```bash
go build                # produces ./pop3-downloader
go test ./...           # run all tests
go test -v ./...        # verbose
go test -run TestName   # single test
```

## Testing Patterns

- **Table-driven tests** — see `TestExtractMessageID` in `mbox_test.go` for the pattern
- **Mock POP3 server** — `TestPOP3Server` in `pop3_test.go` is a full in-process POP3 server on localhost with configurable messages, auth, and deletion tracking via `server.Deleted()`
- **Test fixtures** — `testdata/sample_email_*.txt` files; loaded via `loadTestEmail(t, filename)` (returns string) or `loadTestEmailContent(t, filename)` (returns []byte)
- **Temp directories** — tests use `t.TempDir()` for mbox file isolation
- All tests use plain POP3 via `connectPOP3()` (not TLS) against the mock server

## Code Conventions

- **Error wrapping**: Always use `fmt.Errorf("context: %w", err)` with descriptive context prefix
- **Struct types**: `Config` (CLI config), `ConfigFile` (TOML), `MessageData` (ID + Content bytes)
- **File permissions**: mbox files created with `0600` — tested in `TestWriteMbox_FilePermissions`
- **Dedup logic**: Messages without a Message-ID header are always written (cannot deduplicate). Messages with a known Message-ID are skipped. See `writeMbox` in `mbox.go`
- **Deletion**: `deleteMessages` logs warnings for individual failures but returns only the first error

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/knadh/go-pop3` | POP3 client |
| `github.com/emersion/go-mbox` | mbox format read/write |
| `github.com/BurntSushi/toml` | Config file parsing |

## Configuration

Password is stored in `$HOME/.config/pop3-downloader-config.toml` (not CLI args, for security):
```toml
password = "your-password-here"
```
