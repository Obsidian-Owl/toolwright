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

## P10: PR review catches generated code semantic bugs that gates miss
**Source:** codegen unit (template correctness) + tui unit (lineTracker unconditional apply) + generate unit (missing os.WriteFile)
**When:** Shipping code through automated quality gates.
**Pattern:** Automated gates verify structural correctness (builds, tests pass, interfaces satisfied, no leaks) but miss "does the code do what it says" gaps. Both the tui and generate PRs had real bugs that all 5 gates passed. Treat PR review as a required gate for semantic correctness, especially for template-generated or scaffold code.

## P11: Inline text/template for small generators
**Source:** codegen unit
**When:** Building code generators that produce fewer than ~20 files from templates.
**Pattern:** Use Go's `text/template` with templates as string constants. No build tool dependencies, no embedded FS needed at this scale. Templates are colocated with generator logic for easy maintenance.

## P12: Sanitise *url.Error to prevent credential leakage in query-param auth
**Source:** generate unit (Gemini provider — security gate BLOCK)
**When:** An HTTP client uses query-parameter authentication (e.g., `?key=...`) and the request can fail at the transport level (DNS, TLS, timeout).
**Pattern:** Go's `http.Client.Do` wraps transport failures in `*url.Error` whose `.Error()` includes the full URL — including query params. Strip the URL from `*url.Error` using a custom error type that preserves only Op + cause. Apply to both `Do()` and `NewRequestWithContext()` error paths. Test with `http.Hijacker` to force transport errors and assert `NotContains(err.Error(), secretKey)`.

## P13: Check HTTP status before reading response body
**Source:** generate unit (PR review — all 3 providers)
**When:** Making HTTP requests where non-200 responses don't need body parsing.
**Pattern:** Check `resp.StatusCode` before `io.ReadAll`. On error status, drain the body with `io.Copy(io.Discard, resp.Body)` for connection reuse, then return a status-only error. This avoids allocating memory for error bodies and prevents accidentally including response content in error messages.

## P14: Test the write path, not just the dry-run path
**Source:** generate unit (PR review CRITICAL — missing os.WriteFile)
**When:** A command has both dry-run (stdout) and write-to-disk modes.
**Pattern:** If all tests use `--dry-run`, the actual file-write path is untested. Add at least one test per output mode that asserts the side effect (file exists, correct content). Use `t.TempDir()` for hermetic filesystem tests. This caught a critical bug where "written to X" was printed but `os.WriteFile` was never called.

## P15: Non-dry-run tests must use temp directories
**Source:** generate unit (cascade from P13 fix)
**When:** Tests exercise a code path that writes files to disk.
**Pattern:** Always use `t.TempDir()` + `filepath.Join` for output paths in non-dry-run tests. Writing to the working directory pollutes the repo, can cause cross-test interference, and may fail in CI sandboxes. When fixing a missing write, audit all existing non-dry-run tests for hardcoded paths.

## P16: Error wrapper types should not duplicate caller context
**Source:** generate unit (PR review — doubled provider prefix)
**When:** Building custom error types that wrap underlying errors, called from sites that add their own context via `fmt.Errorf`.
**Pattern:** An error wrapper should contain only its own context (e.g., `op + ": " + cause`). If callers already wrap with `fmt.Errorf("provider: action: %w", ...)`, including the provider name in the wrapper's `Error()` produces doubled prefixes like `"gemini: send request: gemini: Post: <cause>"`. Let the caller supply outer context.
