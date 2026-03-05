# Spec Gate: Scaffold Unit -- Evidence Report

**Date**: 2026-03-05
**Branch**: feat/scaffold
**Build**: PASS (`go build ./...` succeeds)
**Tests**: PASS (all 70 tests pass in `./internal/scaffold/...`)

---

## AC-1: Scaffolder creates spec-compliant directory structure

- **Status**: PASS
- **Implementation**:
  - `Scaffold(ctx, opts)` signature: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:128`
  - Directory created at `outputDir/opts.Name`: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:150`
  - Empty OutputDir resolved to cwd: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:136-142`
  - `ScaffoldResult.Dir` set to abs path: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:240`
  - `ScaffoldResult.Files` set to sorted relative paths: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:228-233`
  - Shared entries (toolwright.yaml, schemas/hello-output.json, tests/hello.test.yaml, README.md): `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:42-62`
  - Shell runtime adds bin/hello: `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:67-74`
- **Tests**:
  - `TestScaffold_Shell_CreatesExpectedFiles` at scaffold_test.go:224
  - `TestScaffold_ResultDir_MatchesExpectedPath` at scaffold_test.go:260
  - `TestScaffold_OutputDir_Empty_UsesCurrentDirectory` at scaffold_test.go:279
  - `TestScaffold_ResultFiles_AreRelativePaths` at scaffold_test.go:310
  - `TestScaffold_AllRuntimes_CreateSharedFiles` at scaffold_test.go:1165
  - `TestIntegration_Scaffold_AllRuntimes` at integration_test.go:89 (verifies exact file set for all 4 runtimes)

## AC-2: Shell runtime produces working entrypoint

- **Status**: PASS
- **Implementation**:
  - Template at `/home/dmccarthy/projects/toolwright/templates/init/shell/hello.sh.tmpl` starts with `#!/bin/bash`, outputs `{"message":"hello"}`
  - Executable mode set via `renderedFile.executable=true`: scaffold.go:72, written as 0755: scaffold.go:219-221
- **Tests**:
  - `TestScaffold_Shell_BinHello_IsExecutable` at scaffold_test.go:370
  - `TestScaffold_Shell_BinHello_HasBashShebang` at scaffold_test.go:384
  - `TestScaffold_Shell_BinHello_OutputsValidJSON` at scaffold_test.go:395
  - `TestIntegration_Scaffold_AllRuntimes/shell` at integration_test.go:96-100

## AC-3: Go runtime produces working entrypoint

- **Status**: PASS
- **Implementation**:
  - `bin/hello` wrapper template: `/home/dmccarthy/projects/toolwright/templates/init/go/hello.sh.tmpl` -- calls `exec go run ./src/hello/main.go`
  - `src/hello/main.go` template: `/home/dmccarthy/projects/toolwright/templates/init/go/main.go.tmpl` -- declares `package main`, outputs JSON `{"message":"hello"}`
  - Runtime entries: scaffold.go:76-86
- **Tests**:
  - `TestScaffold_Go_CreatesExpectedFiles` at scaffold_test.go:432
  - `TestScaffold_Go_BinHello_IsWrapper` at scaffold_test.go:453
  - `TestScaffold_Go_BinHello_IsExecutable` at scaffold_test.go:468
  - `TestScaffold_Go_MainGo_HasPackageMain` at scaffold_test.go:479
  - `TestScaffold_Go_MainGo_ImportsJSON` at scaffold_test.go:494
  - `TestIntegration_Go_MainFile` at integration_test.go:479

## AC-4: Python runtime produces working entrypoint

- **Status**: PASS
- **Implementation**:
  - Template: `/home/dmccarthy/projects/toolwright/templates/init/python/hello.py.tmpl` -- starts with `#!/usr/bin/env python3`, outputs `json.dumps({"message": "hello"})`
  - Runtime entries: scaffold.go:87-94
- **Tests**:
  - `TestScaffold_Python_BinHello_HasPython3Shebang` at scaffold_test.go:508
  - `TestScaffold_Python_BinHello_IsExecutable` at scaffold_test.go:519
  - `TestScaffold_Python_BinHello_OutputsJSONWithMessage` at scaffold_test.go:530
  - `TestIntegration_Python_Entrypoint` at integration_test.go:509

## AC-5: TypeScript runtime produces working entrypoint

- **Status**: PASS
- **Implementation**:
  - `bin/hello` wrapper: `/home/dmccarthy/projects/toolwright/templates/init/typescript/hello.sh.tmpl` -- `exec npx ts-node src/hello/index.ts`
  - `src/hello/index.ts`: `/home/dmccarthy/projects/toolwright/templates/init/typescript/hello.ts.tmpl` -- outputs `JSON.stringify({ message: "hello" })`
  - `package.json`: `/home/dmccarthy/projects/toolwright/templates/init/typescript/package.json.tmpl` -- includes typescript and ts-node deps
  - Runtime entries: scaffold.go:95-113
- **Tests**:
  - `TestScaffold_TypeScript_CreatesExpectedFiles` at scaffold_test.go:548
  - `TestScaffold_TypeScript_BinHello_IsExecutable` at scaffold_test.go:568
  - `TestScaffold_TypeScript_IndexTs_OutputsMessage` at scaffold_test.go:579
  - `TestScaffold_TypeScript_PackageJSON_IsValidJSON` at scaffold_test.go:591
  - `TestScaffold_TypeScript_PackageJSON_HasDependencies` at scaffold_test.go:605
  - `TestIntegration_TypeScript_PackageJSON` at integration_test.go:414
  - `TestIntegration_TypeScript_SourceFile` at integration_test.go:451

## AC-6: Generated manifest is valid

- **Status**: FAIL
- **Implementation**:
  - Manifest template at `/home/dmccarthy/projects/toolwright/templates/init/toolwright.yaml.tmpl` includes apiVersion, kind, metadata, tools with entrypoint `bin/hello`
  - Auth=none: no auth block (template conditional at line 14)
  - Auth=token: produces `auth.type: token` with `token_env` (line 15-18)
  - Auth=oauth2: produces `auth.type: oauth2` with `provider_url` and `scopes` (line 19-24)
  - **MISSING**: `client_id` field is not in the oauth2 template output (lines 19-24 of toolwright.yaml.tmpl). The manifest `Auth` struct in `/home/dmccarthy/projects/toolwright/internal/manifest/types.go:46-55` also has no `ClientID` field.
- **Tests**:
  - `TestScaffold_Manifest_ValidForAllRuntimes` at scaffold_test.go:625
  - `TestScaffold_Manifest_HasRequiredTopLevelFields` at scaffold_test.go:652
  - `TestScaffold_Manifest_ReferencesEntrypoint` at scaffold_test.go:673
  - `TestScaffold_Manifest_AuthNone_NoAuthBlock` at scaffold_test.go:726
  - `TestScaffold_Manifest_AuthToken_HasTokenFields` at scaffold_test.go:739
  - `TestScaffold_Manifest_AuthOAuth2_HasOAuthFields` at scaffold_test.go:756 -- checks type, provider_url, scopes but NOT client_id
  - `TestScaffold_Manifest_AuthVariants_StillPassValidation` at scaffold_test.go:775
  - `TestIntegration_Manifest_ParseAndValidate` at integration_test.go:200
  - `TestIntegration_Manifest_TokenAuth` at integration_test.go:255
  - `TestIntegration_Manifest_OAuth2Auth` at integration_test.go:284
- **Notes**: The spec says `auth=oauth2` must have `client_id`. Neither the manifest Auth struct nor the template includes it. The oauth2 test at scaffold_test.go:756 does not assert `client_id`. This is a spec compliance gap. The `client_id` may have been intentionally excluded from the manifest as a runtime-only concern (it is a configuration secret used in the auth package), but the spec explicitly requires it. FAIL for this sub-criterion.

## AC-7: Generated test scenario is valid

- **Status**: PASS
- **Implementation**:
  - Template: `/home/dmccarthy/projects/toolwright/templates/init/hello.test.yaml.tmpl` -- valid YAML with `tool: hello`, `steps:`, `assert:`, `message: hello`
- **Tests**:
  - `TestScaffold_TestYAML_IsValidYAML` at scaffold_test.go:811
  - `TestScaffold_TestYAML_NamesToolHello` at scaffold_test.go:824
  - `TestScaffold_TestYAML_AssertsMessageField` at scaffold_test.go:836
  - `TestIntegration_TestScenarioFile` at integration_test.go:539

## AC-8: Schema file describes hello output

- **Status**: PASS
- **Implementation**:
  - Static file: `/home/dmccarthy/projects/toolwright/templates/init/hello-output.schema.json` -- JSON Schema with `$schema`, `type: object`, `required: ["message"]`, `properties.message.type: string`
- **Tests**:
  - `TestScaffold_Schema_IsValidJSON` at scaffold_test.go:851
  - `TestScaffold_Schema_IsJSONSchema` at scaffold_test.go:862
  - `TestScaffold_Schema_RequiresMessageProperty` at scaffold_test.go:879
  - `TestIntegration_StaticFile_SchemaJSON_IsValidJSON` at integration_test.go:366

## AC-9: Existing directory is rejected

- **Status**: PASS
- **Implementation**:
  - Check at `/home/dmccarthy/projects/toolwright/internal/scaffold/scaffold.go:153-155` -- returns error if `os.Stat` succeeds on projectDir
  - Error includes path: `fmt.Errorf("directory %q already exists", projectDir)`
  - No files written because all rendering happens before any disk write (lines 173-202 render to memory, lines 210-226 write to disk)
- **Tests**:
  - `TestScaffold_ExistingDirectory_ReturnsError` at scaffold_test.go:908
  - `TestScaffold_ExistingDirectory_ErrorContainsPath` at scaffold_test.go:928
  - `TestScaffold_ExistingDirectory_NoPartialFiles` at scaffold_test.go:949
  - `TestScaffold_ExistingDirectory_EvenIfEmpty_StillRejected` at scaffold_test.go:977

## AC-10: Template rendering failures are atomic

- **Status**: PASS
- **Implementation**:
  - All templates rendered to memory first (lines 173-202), then written to disk (lines 210-226). If any render fails, the function returns before any disk writes.
  - Error messages include template path: e.g., `fmt.Errorf("parse template %q: %w", entry.tmplPath, err)` at line 188
- **Tests**:
  - `TestScaffold_BadTemplate_NoFilesWritten` at scaffold_test.go:1001 -- confirms project directory does not exist after template failure
  - `TestScaffold_BadTemplate_ErrorIdentifiesTemplate` at scaffold_test.go:1041 -- confirms error contains template reference

## AC-11: Scaffolder accepts fs.FS for templates

- **Status**: PASS
- **Implementation**:
  - `Scaffolder` struct field: `templates fs.FS` at scaffold.go:118
  - Constructor `New(templates fs.FS)` at scaffold.go:123
  - All unit tests use `fstest.MapFS` with inline template content (scaffold_test.go:1-220 fixture constants, used throughout)
- **Tests**:
  - `TestNew_AcceptsMapFS` at scaffold_test.go:1085
  - `TestNew_DifferentFS_DifferentBehavior` at scaffold_test.go:1098
  - Every unit test in scaffold_test.go uses `fstest.MapFS`

## AC-12: embed.go exports InitTemplates

- **Status**: PASS
- **Implementation**:
  - `/home/dmccarthy/projects/toolwright/embed.go:12` -- `//go:embed all:templates/init`
  - `/home/dmccarthy/projects/toolwright/embed.go:13` -- `var InitTemplates embed.FS`
  - Integration tests import `toolwright.InitTemplates` and pass to `New()`: integration_test.go:125
- **Tests**:
  - `TestIntegration_InitTemplates_IsAccessible` at integration_test.go:54
  - `TestIntegration_InitTemplates_ContainsAllTemplateFiles` at integration_test.go:61
  - `TestIntegration_EmbeddedTemplateFiles_HaveContent` at integration_test.go:575

---

## Summary

| # | Criterion | Status | Notes |
|---|-----------|--------|-------|
| AC-1 | Directory structure | PASS | All files created, paths correct |
| AC-2 | Shell entrypoint | PASS | Executable, shebang, JSON output |
| AC-3 | Go entrypoint | PASS | Wrapper + main.go |
| AC-4 | Python entrypoint | PASS | Python3 shebang, JSON output |
| AC-5 | TypeScript entrypoint | PASS | Wrapper + index.ts + package.json |
| AC-6 | Manifest validity | FAIL | Missing `client_id` in oauth2 auth block |
| AC-7 | Test scenario | PASS | Valid YAML, names tool, asserts message |
| AC-8 | Schema file | PASS | Valid JSON Schema, requires message string |
| AC-9 | Existing dir rejected | PASS | Error with path, no partial writes |
| AC-10 | Atomic rendering | PASS | Render-then-write, error identifies template |
| AC-11 | fs.FS acceptance | PASS | Constructor takes fs.FS, tests use MapFS |
| AC-12 | embed.go InitTemplates | PASS | Correct embed directive, accessible FS |

- **Total**: 12 criteria
- **Verified**: 11 PASS
- **Unverified**: 1 FAIL (AC-6 sub-criterion: oauth2 `client_id`)
- **Warnings**: 0 WARN
- **Verdict**: FAIL

### Required Fix for AC-6

The spec at line 37 states: "When auth=oauth2: manifest has `auth.type: oauth2` with `provider_url`, `client_id`, `scopes` fields."

The oauth2 template block in `/home/dmccarthy/projects/toolwright/templates/init/toolwright.yaml.tmpl` (lines 19-24) does not include `client_id`. The manifest `Auth` struct in `/home/dmccarthy/projects/toolwright/internal/manifest/types.go:46-55` does not define a `ClientID` field.

To PASS, one of the following must happen:
1. Add `client_id` to the Auth struct, template, and validation -- then update tests; OR
2. Amend the spec to remove `client_id` from the oauth2 requirements (if it is intentionally a runtime-only concern not stored in manifests).
