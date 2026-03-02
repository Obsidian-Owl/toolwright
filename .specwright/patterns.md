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
