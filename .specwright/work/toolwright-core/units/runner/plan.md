# Plan: Unit 3 — runner

## Task Breakdown

### Task 1: Result type
- File: `internal/runner/output.go`
- `Result` struct: `ExitCode int`, `Stdout []byte`, `Stderr []byte`, `Duration time.Duration`

### Task 2: Arg/flag mapping
- File: `internal/runner/executor.go`
- `buildArgs(tool manifest.Tool, positionalArgs []string, flags map[string]string, token string) []string`
- Maps positional args by order
- Maps flags as `--{name} {value}`
- Appends `--{token_flag} {token}` if token non-empty
- Bool flags: `--{name}` (no value) when true, omitted when false

### Task 3: Process execution
- File: `internal/runner/executor.go`
- `Executor` struct with `WorkDir string`
- `Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*Result, error)`
- `os/exec.CommandContext` with timeout
- Set `Setpgid: true` for process group
- Capture stdout/stderr into separate buffers
- Extract exit code from process state

### Task 4: Timeout and cleanup
- File: `internal/runner/executor.go`
- Context with timeout (configurable, default 30s)
- On context cancellation: kill process group (`syscall.Kill(-pid, syscall.SIGKILL)`)
- Return timeout-specific error

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `internal/runner/output.go` | Create | runner |
| `internal/runner/executor.go` | Create | runner |
| `internal/runner/executor_test.go` | Create | runner |
| `internal/runner/output_test.go` | Create | runner |

## As-Built Notes

### Plan deviations
- **Tasks 3+4 merged**: Process execution and timeout/cleanup were implemented together since they share the same `Executor.Run` method and test surface (pattern P1).
- **`BuildArgs` is exported**: Plan specified lowercase `buildArgs`, but it was made `BuildArgs` (exported) since the CLI layer (Unit 6) will need to call it for `--dry-run` support.
- **`exec.Command` instead of `exec.CommandContext`**: Go's `CommandContext` only kills the parent process PID, not the process group. Used `exec.Command` with a manual goroutine watching `ctx.Done()` that calls `syscall.Kill(-pid, SIGKILL)` to kill the entire process group.
- **Duration measures wait time**: `time.Now()` is captured just before `cmd.Wait()`, not before `cmd.Start()`. This excludes process startup overhead but provides consistent measurement of execution time.

### Implementation decisions
- Non-zero exit codes return `(*Result, nil)` — only true execution failures (can't start, timeout) return errors
- Token is passed exclusively via argv (`BuildArgs`); `cmd.Env` is never set
- Empty flag values are omitted (not passed as `--name ""`)
- Flags iterate `tool.Flags` slice (not map) for deterministic ordering

### Actual file paths
- `internal/runner/output.go` — Result struct (11 lines)
- `internal/runner/executor.go` — BuildArgs + Executor.Run (107 lines)
- `internal/runner/output_test.go` — 17 Result tests
- `internal/runner/executor_test.go` — 32 BuildArgs tests + 24 Executor tests (92 total with subtests)
