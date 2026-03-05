# Spec: Unit 4 — wiring

## Acceptance Criteria

### AC-1: Manifest JSON Schema validates toolwright.yaml
- `schemas/toolwright.schema.json` exists and is valid JSON Schema (draft 2020-12)
- Schema requires `apiVersion`, `kind`, `metadata`, `tools`
- Schema validates metadata name pattern `[a-z0-9-]+`
- Schema validates auth type enum (none, token, oauth2)
- Schema validates tool objects (name, description, entrypoint required)
- A valid toolwright.yaml passes schema validation
- An invalid manifest (missing required fields) fails schema validation

### AC-2: Exit code 2 for usage errors
- Missing positional arg on `run` → exit 2
- Missing positional arg on `login` → exit 2
- Missing positional arg on `init` → exit 2
- `errors.As(err, &cli.UsageError{})` returns true for these errors

### AC-3: Exit code 3 for IO errors
- Missing manifest file → exit 3
- Manifest file permission denied → exit 3
- `errors.As(err, &cli.IOError{})` returns true for these errors

### AC-4: Exit code 1 for general errors
- Validation failure → exit 1
- Tool not found in manifest → exit 1
- Auth resolution failure → exit 1
- Default: any error not classified as usage or IO → exit 1

### AC-5: --debug writes timestamped diagnostics to stderr
- `toolwright validate --debug` → stderr contains `[DEBUG <timestamp>]` lines
- `toolwright run --debug <tool>` → stderr contains debug lines for manifest load and auth resolution
- Debug output never appears on stdout
- Without `--debug`, no debug output

### AC-6: Debug covers key diagnostic points
- Manifest loading: debug line with file path
- Auth resolution: debug line with tool name and auth type
- Tool execution: debug line with entrypoint
- Test parsing: debug line with test directory
- Each command has at least one debug line

### AC-7: wire.go wires real implementations
- `BuildRootCommand()` creates init with real scaffolder (not nil)
- `BuildRootCommand()` creates init with real wizard (not nil)
- `BuildRootCommand()` creates generate manifest with real generator (not nil)
- All commands are functional without nil-panics

### AC-8: Nil-guards removed
- No `if cfg.Wizard == nil` guard in init.go
- No `if cfg.Scaffolder == nil` guard in init.go
- No `if cfg.Generator == nil` guard in generate_manifest.go
- These are replaced by real implementations, not guarded

### AC-9: WizardResult.Name field removed
- `WizardResult` struct no longer has a `Name` field
- All references to `WizardResult.Name` in test code are removed or updated
- The wizard does not set or return a name value

### AC-10: Schema embedded via embed.FS
- `schemas/toolwright.schema.json` is accessible via `toolwright.Schemas` embed.FS
- Schema file can be read at path `schemas/toolwright.schema.json` from the FS
- `schemas/.gitkeep` is removed

### AC-11: Binary builds and all tests pass
- `go build ./...` succeeds with new imports in wire.go
- `go test ./...` passes (all packages including new ones)
- `go vet ./...` clean
- No import cycles between internal packages
