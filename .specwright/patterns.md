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
