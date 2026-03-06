# Gate: tests
**Status**: PASS
**Timestamp**: 2026-03-06T01:15:00Z

## Command
```
go test ./... -v
```

## Results
```
ok  github.com/Obsidian-Owl/toolwright/internal/auth       0.903s
ok  github.com/Obsidian-Owl/toolwright/internal/cli        0.132s
ok  github.com/Obsidian-Owl/toolwright/internal/codegen    1.191s
ok  github.com/Obsidian-Owl/toolwright/internal/manifest   0.030s
ok  github.com/Obsidian-Owl/toolwright/internal/runner     4.069s
ok  github.com/Obsidian-Owl/toolwright/internal/scaffold   0.057s
ok  github.com/Obsidian-Owl/toolwright/internal/schema     (cached)
ok  github.com/Obsidian-Owl/toolwright/internal/tooltest   0.911s
ok  github.com/Obsidian-Owl/toolwright/internal/tui        0.111s
```

## tui package: 35 tests, all PASS
- Happy path (description, runtime, auth collection)
- Anti-hardcoding (multiple selection indices)
- All option ordering (4 runtimes × 3 auths)
- Name is always empty
- Accessible mode without TTY
- Abort: empty input, partial input, cancelled context
- `errors.Is(err, huh.ErrUserAborted)` verified
- Table-driven: 6 runtime/auth combinations

## Findings
None.
