# Toolwright Charter

## What Is This Project?

Toolwright is a CLI-first tool development framework. Developers define AI agent tools as standard CLIs using a YAML manifest (`toolwright.yaml`). Toolwright validates, tests, and generates distributable packages from that manifest — both standalone CLI binaries and MCP servers.

## Who Uses It?

Developers building tools for AI agents. The primary consumers are:
- Tool authors who want their tools usable from any agent with a shell
- Teams distributing tools as both CLIs and MCP servers from a single definition
- Organizations that need auth-aware tool distribution without implementing auth

## What Problem Does It Solve?

MCP tool definitions consume 200+ tokens each in agent context windows. Five servers routinely burn 55K+ tokens before a conversation starts. Toolwright eliminates this with progressive discovery: `list` → `describe` → `run`. CLI tools cost zero tokens until invoked.

## Architectural Invariants

1. **CLI-first.** Every tool works via bash. MCP is a generated transport, not a development paradigm.
2. **Manifest is the single source of truth.** `toolwright.yaml` defines everything. CLIs, MCP servers, and schemas are compile targets.
3. **Generate, don't hand-code.** Distribution artifacts are generated from the manifest. One definition, many targets.
4. **Auth-aware, not auth-implementing.** Toolwright plumbs tokens. It never implements identity providers or authorization servers.
5. **Self-contained binary.** Single Go binary, no runtime dependencies, embedded templates and schemas.

## Foundational Technologies

| Technology | Role | Non-negotiable |
|-----------|------|---------------|
| Go 1.23+ | Implementation language | Yes |
| Cobra | CLI framework | Yes |
| Bubble Tea | TUI wizard | Yes |
| `go:embed` | Template/schema embedding | Yes |
| YAML | Manifest format | Yes |
| JSON Schema | Parameter/output validation | Yes |

## Project Boundaries

- Toolwright does NOT implement identity providers, authorization servers, or token issuance.
- Toolwright does NOT run as a daemon or long-lived service.
- Toolwright does NOT manage infrastructure or deployment targets.
- Toolwright does NOT require CGO.
