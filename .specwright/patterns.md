# Toolwright Patterns

Reusable patterns discovered during development. Referenced from auto-memory.

## P1: Merge tightly coupled tasks
**Source:** manifest unit (tasks 2+3)
**When:** Two planned tasks share a test surface (e.g., types + parser — can't test parsing without types).
**Pattern:** Let the tester write tests covering both tasks in one file. The executor implements them together. Plan fine-grained tasks but merge at build time when coupling is obvious.

## P2: Defer embedded assets until consumed
**Source:** manifest unit (task 6 deferred)
**When:** A planned embedded asset (JSON Schema, template, etc.) has no production consumer yet.
**Pattern:** Use `fstest.MapFS` in tests with inline content. Create the real file only when a CLI command or runtime consumer needs it. Avoids orphaned files and premature asset design.

## P3: Foundation unit wiring INFO is expected
**Source:** manifest unit (wiring gate)
**When:** Building bottom-up (leaf packages first), early units export symbols not yet consumed by production code.
**Pattern:** Wiring gate INFO findings about unused exports are normal for foundation units. They resolve as downstream units consume the exports. Only escalate if the export still has no consumer after the final unit ships.

## P4: Config struct for testable function signatures
**Source:** auth unit (Login, Refresh, Resolver all changed from plan)
**When:** A function needs HTTPClient injection, timeouts, store overrides, or other test hooks — especially when 3+ optional parameters are needed.
**Pattern:** Use a config struct with optional fields (HTTPClient, ListenAddr, Timeout, Store). Set defaults inside the function, not the caller. Extensible without API breaks, and callers only set what they care about.

## P5: Fix WARNs before shipping
**Source:** auth unit (8 WARNs resolved post-verify in one commit)
**When:** Verification produces WARN-level gate findings.
**Pattern:** Fix all WARNs immediately after verify, before shipping. Post-verify is the cheapest time — context is fresh, tests pass, and each fix is typically 1-5 lines. Deferred WARNs accumulate into tech debt.

## P6: Size adversarial test inputs for the code path, not the parser
**Source:** runner unit (manifest TestParse_DoesNotPanic OOM)
**When:** Writing fuzz-style or adversarial tests with repeated/large inputs fed to a parser (YAML, JSON, etc.).
**Pattern:** Parsers can amplify input size non-linearly in memory (50KB YAML → 17.4GB peak alloc). Size inputs to exercise the target code path (e.g., "doesn't panic"), not to stress the parser. Profile memory with `go test -memprofile` before committing large-input tests. 100 repetitions usually suffices where 10000 was used.

## P7: Pre-push hooks must mirror CI
**Source:** runner unit (4 CI failures caught too late)
**When:** CI pipeline has build, test, lint, or security checks.
**Pattern:** Pre-push hooks should run the same checks as CI. When a CI step is added or changed, update `.githooks/pre-push` to match. This eliminates the push-wait-fail-fix-push loop. Pre-commit stays fast (format + vet + lint on changed files); pre-push does the full suite.

## P8: Check stdlib package name collisions during planning
**Source:** testing unit (renamed `internal/testing/` → `internal/tooltest/`)
**When:** Naming an internal package during design or planning.
**Pattern:** Before committing to a package name, check if it collides with a Go stdlib package (e.g., `testing`, `net`, `sync`). Collisions force awkward import aliases or rename refactors mid-build. Use `go doc <name>` to verify the name is free. Prefer descriptive prefixes (e.g., `tooltest` instead of `testing`).

## P9: Interface wrapping enables mock injection
**Source:** testing unit (ToolExecutor interface for runner)
**When:** A package depends on an external subsystem (process execution, HTTP, file I/O) that would make tests slow, flaky, or environment-dependent.
**Pattern:** Define a one-method interface at the consumer site (e.g., `ToolExecutor` in tooltest, not in runner). The real implementation satisfies it naturally; tests inject a mock struct. Pre-allocate result slices indexed by position for parallel test execution with deterministic ordering.
