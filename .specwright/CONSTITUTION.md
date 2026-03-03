# Toolwright Constitution

## Code Standards

1. All code passes `gofmt` and `golangci-lint` with no warnings.
2. No global mutable state. No `init()` functions.
3. Public functions accept interfaces, return concrete structs.
4. All errors are wrapped with context using `fmt.Errorf("...: %w", err)`.
5. No bare `panic()` outside of test code.
6. No CGO. Pure Go only.
7. All embedded resources use `go:embed`. No runtime file reads for templates or schemas.

## Testing

8. Tests are written before implementation (TDD). Red → Green → Refactor.
9. Table-driven tests for functions with multiple input/output combinations.
10. Test files live alongside the code they test (`foo_test.go` next to `foo.go`).
11. Use `testify` for assertions and `go-cmp` for struct comparison.
12. No test helpers that swallow errors silently — `t.Fatal` on unexpected failures.
13. Integration tests that touch the filesystem use `t.TempDir()`.
14a. When 2+ test files need the same helper, put it in `testhelpers_test.go` in that package immediately. Don't let duplicate helpers accumulate across files.

## Architecture

14. Internal packages under `internal/` — nothing exported from the module root except `embed.go`.
15. CLI commands in `internal/cli/` are thin — they parse flags and delegate to domain packages.
16. Each domain package (`manifest`, `schema`, `auth`, `runner`, `codegen`, `testing`) has a single clear responsibility.
17. No circular dependencies between internal packages.
17a. Components reading embedded files accept `fs.FS` (not `embed.FS`) for testability with `fstest.MapFS`.

## Output & UX

18. Structured JSON to stdout, diagnostics to stderr. Never mix.
19. All commands support `--json` for machine-readable output.
20. All destructive commands support `--dry-run`.
21. Respect `NO_COLOR` and `CI=true` environment variables.
22. Error messages state what happened, why, and how to fix it.

## Security

23. Tokens are never logged, printed, or included in error output.
24. Auth tokens are passed to entrypoints via CLI flags, not environment variables.
25. No secrets in generated code or templates.
26. Auth-adjacent code applies defense-in-depth: bind listeners to 127.0.0.1, limit I/O reads (`io.LimitReader`), enforce HTTPS on provider URLs, use Fstat on open fds (not stat-then-read), sanitize paths with `filepath.Clean`.

## Git

26. Conventional commits: `{type}({scope}): {description}`.
27. One logical change per commit.
28. All changes go through pull requests.
