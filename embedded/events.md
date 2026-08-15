Lifecycle events

Set `WT_EVENT_HANDLER` to one executable path or name to receive lifecycle
events. The value is executed directly, never evaluated by a shell. An unset
or empty value disables events.

After a successful ephemeral `wt done`, `wt ship`, or `wt rm`, wt invokes the
handler synchronously from the surviving trunk worktree. For `wt rm` in the
unusual case where trunk is not checked out, it uses the surviving main
checkout instead and reports that fallback in `trunk_worktree`. The handler
inherits wt's environment and receives one JSON object followed by a newline
on stdin:

    {
      "version": 1,
      "event": "worktree.removed",
      "operation": "ship",
      "repository": {
        "main_checkout": "/absolute/main/checkout",
        "trunk": "main",
        "trunk_worktree": "/absolute/trunk/worktree"
      },
      "worktree": {
        "name": "feature-name",
        "branch": "feature-name",
        "path": "/absolute/removed/worktree"
      }
    }

`operation` is `done`, `ship`, or `rm`. No removal event is sent when an
operation fails or leaves a persistent worktree in place. For `ship`, the
event is sent only after the final trunk push succeeds.

The handler environment omits the internal `WT_SHIM` marker. `WORKTREE` is
unset when the handler runs in the main checkout, or set to the trunk branch
name when trunk is checked out in a linked worktree.

Handler stdout and stderr pass through normally. If the handler cannot start
or exits unsuccessfully, wt warns that the requested operation already
succeeded and still exits successfully; retrying a completed teardown is not
safe recovery.
