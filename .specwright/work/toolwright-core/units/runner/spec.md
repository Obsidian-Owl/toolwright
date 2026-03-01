# Spec: Unit 3 — runner

## Acceptance Criteria

### AC-1: Positional args mapped correctly
- Tool with `args: [{name: path, type: string, required: true}]` and input `["./src"]` → entrypoint called with `./src` as first arg
- Multiple positional args mapped in order

### AC-2: Flags mapped correctly
- Tool with flag `{name: severity, type: string}` and input `{"severity": "high"}` → entrypoint called with `--severity high`
- Multiple flags each become `--{name} {value}` pairs
- Bool flag with value `"true"` → `--{name}` (presence only, no value)
- Bool flag with value `"false"` → omitted entirely

### AC-3: Auth token injected as flag
- Token `"secret123"`, tool with `token_flag: "--api-key"` → entrypoint called with `--api-key secret123` appended
- Empty token → no token flag appended
- Token flag appears after all other args and flags

### AC-4: Token never passed via environment
- Child process environment does NOT contain the token value in any variable
- Token appears only in argv

### AC-5: stdout and stderr captured separately
- Entrypoint writes `{"ok":true}` to stdout and `debug info` to stderr → Result.Stdout contains JSON, Result.Stderr contains debug text
- Neither stream contaminates the other

### AC-6: Exit code captured correctly
- Entrypoint exits 0 → Result.ExitCode == 0
- Entrypoint exits 1 → Result.ExitCode == 1
- Entrypoint exits 2 → Result.ExitCode == 2

### AC-7: Timeout kills process and children
- Entrypoint that sleeps 10s with 1s timeout → context deadline exceeded error
- Entrypoint that spawns child processes → entire process group killed on timeout (no orphans)

### AC-8: Non-existent entrypoint returns error
- Tool with entrypoint `./nonexistent` → error (not panic) with message indicating the file doesn't exist

### AC-9: Non-executable entrypoint returns error
- Tool with entrypoint pointing to a non-executable file → permission denied error

### AC-10: Working directory set correctly
- Executor with `WorkDir: "/tmp/project"` → child process runs in `/tmp/project`

### AC-11: Duration captured
- Result.Duration reflects actual execution time (within reasonable tolerance)

### AC-12: Build and tests pass
- `go build ./...` succeeds
- `go test ./internal/runner/...` passes
- `go vet ./...` clean
