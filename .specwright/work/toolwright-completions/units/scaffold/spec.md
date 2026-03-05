# Spec: Unit 1 — scaffold

## Acceptance Criteria

### AC-1: Scaffolder creates spec-compliant directory structure
- `Scaffold(ctx, opts)` with runtime=shell creates: `toolwright.yaml`, `bin/hello`, `schemas/hello-output.json`, `tests/hello.test.yaml`, `README.md`
- Directory is created at `opts.OutputDir/opts.Name` (or `./opts.Name` when OutputDir is empty)
- `ScaffoldResult.Dir` contains the created directory path
- `ScaffoldResult.Files` contains relative paths of all created files

### AC-2: Shell runtime produces working entrypoint
- `bin/hello` is executable (mode 0755)
- `bin/hello` outputs valid JSON containing `"message":"hello"` and exits 0
- Content is a bash script with `#!/bin/bash` shebang

### AC-3: Go runtime produces working entrypoint
- `bin/hello` is executable bash wrapper calling `go run ./src/hello/main.go`
- `src/hello/main.go` exists and compiles
- Running `bin/hello` outputs JSON containing `"message":"hello"` and exits 0

### AC-4: Python runtime produces working entrypoint
- `bin/hello` is executable python script with `#!/usr/bin/env python3` shebang
- Outputs JSON containing `"message":"hello"` and exits 0

### AC-5: TypeScript runtime produces working entrypoint
- `bin/hello` is executable wrapper for the TypeScript source
- `src/hello/index.ts` exists
- `package.json` exists with appropriate dependencies
- Running `bin/hello` outputs JSON containing `"message":"hello"` and exits 0

### AC-6: Generated manifest is valid
- `toolwright.yaml` passes `manifest.Validate()` for all 4 runtimes
- Manifest includes `apiVersion: toolwright/v1`, `kind: Toolkit`, complete metadata
- Manifest references `bin/hello` as entrypoint
- When auth=none: no auth block in manifest
- When auth=token: manifest has `auth.type: token` with `token_env` field
- When auth=oauth2: manifest has `auth.type: oauth2` with `provider_url`, `client_id`, `scopes` fields

### AC-7: Generated test scenario is valid
- `tests/hello.test.yaml` is valid YAML
- Test asserts tool output contains `"message"`
- Test names the tool `hello`

### AC-8: Schema file describes hello output
- `schemas/hello-output.json` is valid JSON Schema
- Schema requires `message` property of type string

### AC-9: Existing directory is rejected
- `Scaffold(ctx, opts)` returns error when target directory already exists
- Error message includes the directory path
- No partial files are written

### AC-10: Template rendering failures are atomic
- If any template fails to render, no files are written to disk
- Error message identifies which template failed

### AC-11: Scaffolder accepts fs.FS for templates
- Constructor accepts `fs.FS` (not `embed.FS`)
- Tests use `fstest.MapFS` with inline template content
- Production code passes `toolwright.InitTemplates` from embed.go

### AC-12: embed.go exports InitTemplates
- `embed.go` contains `//go:embed all:templates/init` or equivalent directive
- `toolwright.InitTemplates` is accessible as `embed.FS`
- Template files are accessible via the FS at their relative paths
