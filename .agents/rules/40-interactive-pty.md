# Interactive and PTY Rules

## Risks to cover
- Child process exits but parent hangs.
- Stdin remains captured after child exit.
- Terminal mode not restored.
- Arrow/control-key input not passed through.

## Required Coverage for PTY Changes
- Unit: PTY run returns promptly after child exit.
- Integration: interactive child path with PTY enabled.
- Integration: verify process returns to caller and emits final summary.
- Integration: non-interactive mode remains deterministic and non-blocking.

## Diagnostics
When child exits non-zero in PTY mode, always surface underlying error cause.
