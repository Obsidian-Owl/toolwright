# Context: Unit 3 — runner

## Purpose

Execute tool entrypoints as child processes. Map manifest arguments and flags to CLI invocations, inject auth tokens via the declared `token_flag`, enforce timeouts, and capture stdout/stderr separately.

## Key Spec Sections

- §2.7: Invocation mapping (`toolwright run` → entrypoint translation)
- §3.3: `toolwright run` command behavior
- §4.5: Process execution (os/exec, process groups, timeout, token injection)

## Files to Create

```
internal/runner/
├── executor.go          # Process execution with token injection
├── executor_test.go
├── output.go            # Result type, output capture
└── output_test.go
```

## Dependencies

- `internal/manifest` (Unit 1) — `Tool`, `Arg`, `Flag` types
- `internal/auth` (Unit 2) — `Resolver` for token resolution (used by CLI layer, not runner directly)

Note: The runner does NOT call auth.Resolver itself. It receives the resolved token as a parameter. Auth resolution is the CLI layer's responsibility (Unit 6). The runner just executes.

## Invocation Mapping

```
toolwright run <tool> [positional-args...] [--flags...]
  → <entrypoint> [positional-args...] [--flag value...] [--token-flag <token>]
```

- Positional args mapped by order to `tool.Args`
- Flags mapped by name to `tool.Flags`
- Auth token appended as `--{token_flag} <token>` (if auth != none and token provided)
- For `auth: none` or no token: no token flag appended

## Process Execution

- `os/exec.CommandContext` with timeout context
- Process group IDs for clean termination (`syscall.SysProcAttr{Setpgid: true}`)
- stdout and stderr captured separately into buffers
- Working directory is the project root (where toolwright.yaml lives)
- Exit code extracted from process state

## Gotchas

1. **Token via flag, never env** — Constitution rule 24. Token is appended as a CLI flag to the entrypoint, not passed as an environment variable to the child process.
2. **Process group termination** — on timeout, kill the entire process group (not just the parent), to avoid orphaned children.
3. **Exit code extraction** — on Unix, extract from `ProcessState.ExitCode()`. If process was killed by signal, return a distinct error.
4. **Output validation** — runner does NOT validate output against schemas. That's the testing framework's job (Unit 4). Runner just captures raw bytes.
