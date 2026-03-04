# Wiring Gate: Scaffold Unit

**Date**: 2026-03-05
**Verdict**: WARN

## Summary

No blocking issues for this unit. Two INFO findings are expected deferred wiring (pattern P3).
One WARN: `internal/scaffold` imports `internal/cli` for type definitions — will create a circular
import when the wiring unit adds `scaffold.New()` to `internal/cli/wire.go`.

## Checks

| # | Check | Status | Evidence |
|---|-------|--------|----------|
| 1 | `InitTemplates` consumed outside tests | INFO | Only in `integration_test.go`. `wire.go` has `Scaffolder: nil`. Expected per P3. |
| 2 | `scaffold.New()` consumed outside tests | INFO | Only in test files. Expected per P3 (wiring unit will wire). |
| 3 | Orphaned template files | PASS | All 11 files embedded via `//go:embed all:templates/init`. Accessibility verified by integration tests. |
| 4 | Architecture layer direction | WARN | `internal/scaffold` → `internal/cli` (for types) — inverts Rule 15 direction. |
| 5 | Circular imports (today) | PASS | No cycle exists today. `internal/cli/wire.go` does not import `internal/scaffold`. |

## Key Finding: Latent Circular Import

When the wiring unit adds `scaffold.New(toolwright.InitTemplates)` to `internal/cli/wire.go`,
it will import `internal/scaffold`, creating:

```
internal/cli → internal/scaffold → internal/cli
```

**Resolution required before wiring unit**:
Move `ScaffoldOptions` and `ScaffoldResult` from `internal/cli/init.go` into
`internal/scaffold/scaffold.go`. The `scaffolder` interface in `cli/init.go` would then
reference `scaffold.ScaffoldOptions`/`scaffold.ScaffoldResult`, restoring the correct
CLI → domain direction.

## Verdict: WARN

Fix the type location before the wiring unit. No action needed in this unit.
