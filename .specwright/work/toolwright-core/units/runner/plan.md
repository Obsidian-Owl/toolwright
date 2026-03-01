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
