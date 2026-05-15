# Skill: interactive-pty-debug

Use for TTY/PTY/input bugs.

1. Confirm whether path is PTY or pipe.
2. Check stdin ownership and close semantics.
3. Check goroutine shutdown order on child exit.
4. Verify terminal restore behavior.
5. Add regression tests for hang/input/restore.
6. Capture timing expectations (must return promptly).
