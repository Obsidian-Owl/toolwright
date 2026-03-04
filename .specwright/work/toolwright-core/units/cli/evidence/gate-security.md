# Gate: Security

**Status**: PASS
**Timestamp**: 2026-03-04

## Findings

### WARN-1: validate.go:92 — HTTP HEAD to non-HTTPS provider_url not gated
- `http.Head(tk.Auth.ProviderURL)` runs even if manifest validation already flagged URL as non-HTTPS
- Mitigated: requires `--online` flag and manifest validation does report the error
- Low risk since the URL comes from a local user-authored manifest

### INFO-1: login.go:158 — Windows cmd injection in openBrowser
- `cmd /c start <url>` passes through Windows shell interpreter
- URL comes from manifest provider_url (user-authored), mitigated by HTTPS validation at manifest layer

### INFO-2: validate.go:72 — No filepath.Clean on entrypoint stat
- `os.Stat(tool.Entrypoint)` is read-only, manifest is local file
- Informational only

## Constitution Compliance
- Rule 23 (tokens never printed): PASS — token discarded with `_` in login, never in output
- Rule 23a (no token fields): PASS — no output struct contains token fields
- Rule 24 (auth via CLI flags): PASS — `--token` flag on run command
- Rule 25 (no secrets in templates): PASS — no template processing in CLI layer

## Summary
| Severity | Count |
|----------|-------|
| BLOCK | 0 |
| WARN | 1 |
| INFO | 2 |
