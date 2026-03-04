# Security Gate: Scaffold Unit

**Date**: 2026-03-05
**Verdict**: WARN

## Summary

The scaffold unit passes all critical security checks. One WARN-level gap exists:
`opts.Name` is not sanitized against path traversal, violating Constitution rule 26.

## Checks Performed

| # | Check | Result | Notes |
|---|-------|--------|-------|
| 1 | Secret leakage | PASS | No tokens, passwords, or credentials in templates or error output |
| 2 | Path traversal | WARN | `opts.Name` used in `filepath.Join` without containment check |
| 3 | Template injection | PASS | Template source is embedded (developer-controlled), not user input |
| 4 | Command injection | PASS | All shell scripts use hardcoded commands; no user data in scripts |
| 5 | Unsafe permissions | PASS | Files 0644, executables 0755, directories 0755 |
| 6 | Constitution 23 (no token logging) | PASS | No token values in output or error messages |
| 7 | Constitution 26 (defense-in-depth) | WARN | No path containment validation on `opts.Name` |

## Findings

### WARN-1: No path traversal defense on opts.Name

- **File**: `internal/scaffold/scaffold.go:150`
- **Code**: `projectDir := filepath.Join(outputDir, opts.Name)`
- **Risk**: A name like `../../etc/shadow` resolves outside `outputDir`. While `filepath.Join`
  cleans `.` segments, it does not prevent ancestor traversal.
- **CLI layer**: `internal/cli/init.go:95` passes `args[0]` without validation.
- **Mitigation present**: Line 153 rejects existing directories, limiting exploitation.
- **Mitigation absent**: No containment check, no rejection of `/` or `..` components. No test covers path traversal.
- **Constitution 26** requires: "sanitize paths with `filepath.Clean`"
- **Recommended fix**: Validate `opts.Name` contains no path separators or `..`,
  OR verify the resolved path is a child of `outputDir`.

### INFO-1: Description rendered unquoted into YAML/JSON templates

- **Files**: `templates/init/toolwright.yaml.tmpl:6`, `templates/init/typescript/package.json.tmpl:3`
- **Risk**: Descriptions with YAML metacharacters or JSON quotes produce malformed output.
  Not a security risk (local files owned by user). Usability concern only.

## Evidence

- All 11 template files reviewed: no secrets, no dynamic shell interpolation
- File permissions verified by integration tests
- Auth template references env var *names* (`token_env`), not actual token values
