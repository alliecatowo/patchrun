# Interactive Playbook

Use for commands like `mise install` that may require trust prompts.

Required checks:
- Prompt appears and accepts input.
- Arrow/control sequences do not leak as raw garbage.
- On child completion, patchrun returns immediately and prints final status.
- Non-interactive mode fails predictably with guidance.
