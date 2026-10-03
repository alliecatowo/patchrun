# Agent Test Policy

## Test Pyramid
- Unit: parsing, policy logic, execution primitives
- Integration: app workflow in disposable git repos
- Interactive acceptance: PTY and terminal lifecycle scenarios

## Release Gate
Block merge if:
- behavior changed but no regression test
- PTY changes without interactive-path assertions
- verification commands not reported
