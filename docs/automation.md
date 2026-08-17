---
layout: default
title: Automation
description: Use wt with coding agents and lifecycle handlers.
permalink: /automation/
---

# Automation

`wt` ships its own agent instructions. Print them on demand or export them to
an agent's skills directory:

```sh
wt skill
wt skill --export ~/.agents/skills
```

Add “To manage worktrees, run `wt skill` and follow it” to `AGENTS.md` or
`CLAUDE.md`. `wt init` can add that pointer, while `wt prompt` prints a prompt
for configuring project hooks.

Create a worktree and launch an agent immediately:

```sh
wt create fix-checkout -- pi
```

## Lifecycle events

Set `WT_EVENT_HANDLER` to an executable path or command name. After a
successful ephemeral removal by `done`, `ship`, or `rm`, the handler receives
one JSON object on stdin:

```json
{
  "version": 1,
  "event": "worktree.removed",
  "operation": "ship",
  "repository": {
    "main_checkout": "/code/project",
    "trunk": "main",
    "trunk_worktree": "/code/project"
  },
  "worktree": {
    "name": "fix-checkout",
    "branch": "fix-checkout",
    "path": "/code/project/.wt/worktrees/fix-checkout"
  }
}
```

The handler runs synchronously from the surviving trunk worktree. Its failure
is reported but never changes the successful worktree operation. Run
`wt help events` for the complete contract.
