# Spec: Unit 1 — manifest

## Acceptance Criteria

### AC-1: YAML manifest parses into Toolkit struct
- Parse the full example manifest from spec §2.2 (two tools, toolkit-level auth, generate config)
- Resulting `Toolkit` has correct `APIVersion`, `Kind`, `Metadata`, `Tools`, `Auth`, `Generate` fields
- All nested types (`Arg`, `Flag`, `Output`, `Example`, `ExitCodes`) populated correctly

### AC-2: Auth string shorthand unmarshals correctly
- `auth: none` (YAML string) parses to `Auth{Type: "none"}`
- `auth: {type: oauth2, provider_url: ..., scopes: [...], token_env: ..., token_flag: ...}` parses to full `Auth` struct
- Tool-level auth overrides toolkit-level auth
- Missing auth at both levels defaults to `Auth{Type: "none"}` via `ResolvedAuth()`

### AC-3: Auth with manual endpoints parses correctly
- `auth.endpoints.authorization`, `auth.endpoints.token`, `auth.endpoints.jwks` populate `Endpoints` struct
- Missing `endpoints` field results in `nil` Endpoints pointer

### AC-4: Validation catches invalid manifests
- Missing `metadata.name` → error at path `metadata.name`
- Name with uppercase → error (rule: `^[a-z0-9-]+$`)
- Invalid SemVer version (e.g., `1.0`) → error at path `metadata.version`
- Description over 200 chars → error
- Duplicate tool names → error
- Duplicate arg names within a tool → error
- Duplicate flag names within a tool → error

### AC-5: Validation catches invalid auth config
- `auth.type: token` without `token_env` → error
- `auth.type: token` without `token_flag` → error
- `auth.type: oauth2` without `provider_url` → error
- `auth.type: oauth2` without `scopes` → error
- `provider_url` with HTTP (not HTTPS) → error
- `auth.type: none` with no other fields → passes

### AC-6: Validation catches type mismatches
- Flag with `type: int` and `default: "abc"` → error
- Flag with `type: bool` and `enum: [yes, no]` → error (enum values must match type)
- Flag with `type: string` and `default: hello` → passes
- Flag with `type: int` and `enum: [1, 2, 3]` → passes (when provided as integers)

### AC-7: Parser rejects malformed YAML
- Empty input → error
- Valid YAML but missing `apiVersion` → error
- Valid YAML but wrong `kind` (not "Toolkit") → error
- Non-YAML input (binary data) → error

### AC-8: ValidationError has path, message, and rule
- Each error includes `Path` (e.g., `tools[0].args[1].name`), `Message` (human-readable), `Rule` (machine identifier like `name-format`, `unique-tool-name`, `semver`)
- Multiple errors returned in a single `Validate()` call (not fail-fast)

### AC-9: JSON Schema validator validates JSON against schema
- Valid JSON matching a schema → no error
- JSON missing a required field → error with path
- JSON with wrong type → error
- Schema loaded from `embed.FS`

### AC-10: Schema validator handles missing schema gracefully
- Request to validate against non-existent schema path → error (not panic)

### AC-11: Toolkit struct round-trips through YAML
- Parse a manifest, marshal back to YAML, parse again — result matches original
- Auth string shorthand is NOT preserved (marshals to object form) — this is acceptable

### AC-12: Project compiles and tests pass
- `go build ./...` succeeds
- `go test ./internal/manifest/... ./internal/schema/...` passes
- `go vet ./...` clean
