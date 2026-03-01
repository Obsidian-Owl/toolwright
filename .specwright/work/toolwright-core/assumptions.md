# Assumptions

Status: 4/5 resolved

## Blocking

### A5: MCP TypeScript SDK supports deferred tool loading and search_tools
- **Category**: integration
- **Resolution**: reference
- **Status**: ACCEPTED
- **Impact**: If @modelcontextprotocol/sdk does not support `defer_loading` or `search_tools`, generated MCP servers cannot implement progressive discovery — the project's core value proposition.
- **Rationale**: The MCP SDK does not natively provide a `defer_loading` flag or `search_tools` meta-tool. However, the generated MCP server can implement `search_tools` as a custom tool that wraps the tool registry — this is an application-level pattern, not an SDK feature. The spec already describes this approach. Progressive discovery works via the generated `search_tools` tool regardless of SDK support. Risk accepted.

## Accepted

(none)

## Verified

### A1: go-keyring works without CGO on all target platforms
- **Category**: technical
- **Resolution**: reference
- **Status**: VERIFIED
- **Evidence**: go-keyring explicitly aims to simplify using statically linked binaries by avoiding C bindings. Linux uses Secret Service D-Bus interface (pure Go). macOS uses `/usr/bin/security` binary (no CGO). Windows uses syscall-based Credential Manager. See [go-keyring README](https://github.com/zalando/go-keyring).

### A2: golang.org/x/oauth2 supports PKCE natively
- **Category**: integration
- **Resolution**: reference
- **Status**: VERIFIED
- **Evidence**: x/oauth2 provides `GenerateVerifier()`, `S256ChallengeOption(verifier)`, and `VerifierOption(verifier)` functions. Usage: generate verifier, pass to `AuthCodeURL()` with `S256ChallengeOption`, pass to `Exchange()` with `VerifierOption`. See [x/oauth2 docs](https://pkg.go.dev/golang.org/x/oauth2) and [issue #59835](https://github.com/golang/go/issues/59835).

### A3: quicktemplate supports Go 1.23+
- **Category**: technical
- **Resolution**: reference
- **Status**: VERIFIED
- **Evidence**: quicktemplate's go.mod specifies `go 1.17` as minimum. Go maintains backward compatibility, so 1.23+ works. Latest release v1.7.0 available. See [quicktemplate repo](https://github.com/valyala/quicktemplate).

### A4: ohler55/ojg JSONPath covers required assertion expressions
- **Category**: technical
- **Resolution**: reference
- **Status**: VERIFIED
- **Evidence**: ojg implements full Goessner JSONPath spec including array indices (`$[0]`), wildcards (`$[*]`), nested properties, unions, slices, and negative indices. Supports programmatic expression building. See [ojg jp package](https://pkg.go.dev/github.com/ohler55/ojg/jp).
