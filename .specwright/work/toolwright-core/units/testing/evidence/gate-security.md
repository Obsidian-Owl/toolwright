# Gate: Security
**Status**: PASS
**Timestamp**: 2026-03-03T16:40:00Z

## Findings

### WARN-1: Auth tokens sourced from environment variables
- File: parser.go:53-62
- Token env var expansion (`${VAR}`) reads from env vars as source, but delivers via CLI flags to tool entrypoints
- Architecturally sound per constitution rule 24 (CLI flag delivery)
- Recommendation: document TOOLWRIGHT_TEST_TOKEN as CI convenience

### WARN-2: No explicit token scrubbing in error messages
- File: runner.go:99-105
- Built-in runner.Executor does not include tokens in errors
- ToolExecutor interface is internal, limiting blast radius
- Recommendation: add interface documentation about token exclusion

### WARN-3: No file size limit on YAML parsing
- File: parser.go:107
- os.ReadFile with no size limit (gosec suppressed with rationale)
- Low risk: controlled input paths from glob or caller
- Recommendation: add size check for consistency with runner's limitedWriter

### INFO findings (5)
- Token visible in process listing (by design per rule 24)
- No filepath.Clean on ParseTestFile path (internal API)
- Regex from user YAML (safe: Go RE2)
- JSONPath from user YAML (safe: bounded input)
- YAML bomb resistance (struct-typed unmarshal)

### Positive finding
- Token excluded from TestResult/TestReport data model — structurally impossible to leak through output formatters
