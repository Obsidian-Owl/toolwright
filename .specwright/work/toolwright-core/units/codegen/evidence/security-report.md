# Security Gate Evidence

**Timestamp**: 2026-03-04
**Work Unit**: codegen

## Verdict: PASS (4 WARN, 1 INFO)

## Phase 1 — Detection

No BLOCK findings. No secrets, API keys, tokens, passwords, or private keys in source files.

## Phase 2 — Analysis

### WARN-1: Shell injection in generated scaffold via `sh -c`
- **Location**: cli_go.go:534
- **What**: Generated Go CLI template embeds tool name into `sh -c` argument — shell metacharacters in tool names could inject commands
- **Mitigating factors**: Manifest is developer-authored, generated code is reviewed before use
- **Recommendation**: Use `exec.CommandContext` with argument list instead of `sh -c`

### WARN-2: TypeScript code injection via unsanitized tool names
- **Location**: mcp_typescript.go:308, 349
- **What**: Tool names interpolated directly into TS imports/identifiers — special characters could break syntax or inject code
- **Mitigating factors**: Same as WARN-1
- **Recommendation**: Validate tool names against `^[a-z][a-z0-9_-]*$` in generator

### WARN-3: No path traversal guard on file write
- **Location**: engine.go:132
- **What**: `filepath.Join(opts.OutputDir, f.Path)` without `..` validation
- **Mitigating factors**: Only internal generators exist currently, all produce safe paths
- **Recommendation**: Add path containment check after filepath.Join

### WARN-4: .gitignore missing .env exclusion
- **Location**: .gitignore (project-level, pre-existing)
- **What**: No `.env*`, `*.pem`, `*.key` patterns
- **Recommendation**: Add patterns to .gitignore

### INFO-1: text/template usage is correct
- **Location**: cli_go.go:8, mcp_typescript.go:7
- **What**: text/template (not html/template) is correct for source code generation

## AC-13 / Constitution 23a, 25: No Secrets in Generated Code

**PASS** — Auth is handled exclusively through env vars and CLI flags. No literal secrets in any template. Test coverage validates this explicitly.
