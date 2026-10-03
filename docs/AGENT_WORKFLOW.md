# Agent Contribution Workflow

1. Open issue with reproduction and expected behavior.
2. Create plan + risk notes.
3. Implement smallest viable patch.
4. Pass required verification commands.
5. Include evidence section in PR:
   - commands run
   - tests added/updated
   - before/after behavior
6. Human review for high-risk areas:
   - PTY/input lifecycle
   - git/worktree mutation semantics
