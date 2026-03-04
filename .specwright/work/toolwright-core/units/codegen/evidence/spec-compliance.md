# Spec Compliance Gate Evidence

**Timestamp**: 2026-03-04
**Work Unit**: codegen

## Verdict: PASS (16/16 criteria verified)

## Compliance Matrix

| # | Criterion | Implementation | Test | Status |
|---|-----------|---------------|------|--------|
| AC-1 | Generator interface target-agnostic | engine.go:15-19, 68-71, 94-106 | TestEngine_Generate_DispatchesToRegisteredGenerator, ...UnknownMode, ...ListsAllAvailable | PASS |
| AC-2 | Go CLI valid project structure | cli_go.go:37-139 | TestGoCLI_AC2_MainGoPresent, ...RootGoPresent, ...PerTool, ...AuthResolver, ...GoMod, ...NoLoginGo | PASS |
| AC-3 | Go CLI login command for oauth2 | cli_go.go:128-139, 615-683 | TestGoCLI_AC3_LoginGoPresent_WhenOAuth2, ...ContainsPKCE, ...NotPresent_WhenOnlyToken | PASS |
| AC-4 | Go CLI list and describe subcommands | cli_go.go:390-466 | TestGoCLI_AC4_RootGoContainsListSubcommand, ...ListSupportsJSON, ...Describe | PASS |
| AC-5 | Go CLI per-tool args/flags mapping | cli_go.go:268-311, 468-543 | TestGoCLI_AC5_ToolCommandMapsPositionalArgs, ...MapsFlags, ...FlagTypes, ...Required, ...Enum | PASS |
| AC-6 | Go CLI output compiles | cli_go.go:545-552 | TestIntegration_GoCLI_Compiles, ...GoModValid, ...GoVetClean | PASS |
| AC-7 | TS MCP valid project structure | mcp_typescript.go:39-103 | TestTSMCP_AC7_IndexTSPresent, ...PerTool, ...Search, ...PackageJSON, ...TSConfig | PASS |
| AC-8 | TS MCP auth middleware for token | mcp_typescript.go:97-103, 529-581 | TestTSMCP_AC8_MiddlewareTSPresent, ...ValidatesBearerHeader, ...HandleInvalidToken | PASS |
| AC-9 | TS MCP PRM endpoint for oauth2 | mcp_typescript.go:106-113, 583-612 | TestTSMCP_AC9_MetadataTSPresent, ...WellKnownPath, ...Resource, ...AuthorizationServers, ...Absent_WhenStdioOnly | PASS |
| AC-10 | TS MCP search_tools meta-tool | mcp_typescript.go:62-68, 405-463 | TestTSMCP_AC10_SearchTSListsToolNames, ...Descriptions, ...ToolInterface, ...ProgressiveDiscovery | PASS |
| AC-11 | Type mapping correct | cli_go.go:233-244, mcp_typescript.go:181-189 | TestGoCLI_AC11_TypeMapping (table-driven), TestTSMCP_AC11_TypeMapping (table-driven) | PASS |
| AC-12 | .toolwright-generated marker file | engine.go:52, 81-92, 141-147 | TestEngine_Generate_CreatesMarkerFile, ...ContainsVersion, ...Timestamp, ...ExistingMarker_NoForce | PASS |
| AC-13 | No secrets in generated code | cli_go.go:527, mcp_typescript.go:371 | TestGoCLI_AC13_NoLiteralTokenValues, ...EnvVarsNotLiterals, TestTSMCP_AC13_NoLiteralTokenValues | PASS |
| AC-14 | Dry run outputs without writing | engine.go:81, 129-148 | TestEngine_Generate_DryRun_WritesNothingToDisk, ...ReturnsFileList, ...DoesNotWriteMarkerFile | PASS |
| AC-15 | Handles tools with no auth | cli_go.go:293, mcp_typescript.go:236 | TestGoCLI_AC15_NoAuthCodeForNoneAuthTool, ...MixedAuth, TestTSMCP_AC15_NoAuthCode, ...MixedAuth | PASS |
| AC-16 | Build and tests pass | N/A | go build (clean), go vet (clean), go test (218/218 pass) | PASS |
