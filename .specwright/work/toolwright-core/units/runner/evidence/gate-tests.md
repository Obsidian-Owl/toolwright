# Gate: Tests — PASS

- `go test ./... -count=1` — all packages pass
  - internal/auth: 0.922s
  - internal/manifest: 42.422s
  - internal/runner: 1.946s
  - internal/schema: 0.016s
- `go test ./internal/runner/... -count=1 -race` — pass (2.963s), no data races
- Runner test count: 73 test functions, 92 with subtests
