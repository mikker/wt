# wt — ephemeral git worktrees

A single Go binary that replaces the `gwt`/`gwrm` zsh functions and most of the
`regroup` agent skill. It spawns worktrees, runs project hooks, regroups work
back onto trunk, and tears worktrees down — optimized for worktrees that are
**ephemeral by default**: quick to spawn, quick to merge, quick to delete.
Every feature or bug-fix thread starts in a fresh worktree and disappears when
it lands.

Persistent worktrees (projects where setup is costly) remain a supported,
opt-in mode.

## Design principles

- **Zero config works.** No `.wt/` dir means plain `git worktree add`, cd in,
  work. Everything in `.wt/` is optional, including the directory itself.
- **The binary does plumbing, agents do judgment.** `wt done` runs the
  deterministic happy path and hard-stops with a clear message on anything
  requiring a decision (conflicts, dirty state). The agent skill resolves and
  re-runs. No interactive prompts in merge flows.
- **One source of truth.** Docs, the agent skill, and the project-setup prompt
  are embedded in the binary (`go:embed`). Skills and prompts reference the
  binary; they never duplicate its logic.
- **Never lose work silently.** Dirty checks before removal, `git branch -d`
  never `-D`, ff-only merges into trunk, no pushes except in `wt ship`.

## Layout conventions (unchanged from gwt)

- Worktrees live at `../<project>-worktrees/<name>` relative to the main
  checkout. E.g. `~/dev/tuna` → `~/dev/tuna-worktrees/login-fix`.
- Branch name == worktree name.
- Trunk is resolved as: `refs/remotes/origin/HEAD` if set, else local `main`,
  else local `master`. Error out otherwise.
- The trunk worktree is the worktree whose checked-out branch is trunk
  (usually the main checkout). Located via `git worktree list --porcelain`.

## Command surface

Creation and navigation are separate actions: `wt create <name>` always
creates, while `wt switch <name>` only enters an existing worktree. Worktree
names never collide with command names (`done`, `ls`, …).

### `wt create <name> [-- command [args...]]` — create and enter (alias: `wt c`)

1. Refuse if a worktree named `<name>` already exists, with guidance to use
   `wt switch <name>`.
2. Run `git worktree add ../<project>-worktrees/<name> -b <name> <trunk>`;
   if the branch already exists, add the worktree on the existing branch
   instead (current gwt behavior).
3. Run `.wt/create <base_dir>` if present and executable, from inside the new
   worktree, with the main checkout's absolute path as `$1`. A failing create
   hook reports the failure but leaves the worktree in place (you can inspect
   and fix; `wt rm` to abandon).
4. If `-- command [args...]` was supplied, run it from inside the worktree.
5. Print the worktree path for the shim to cd into.

Flags: `--persist` marks the worktree persistent at creation
(`git config branch.<name>.wt-persist true`).

### `wt switch <name>` — enter

Enter an existing worktree. Refuse if it does not exist, with guidance to use
`wt create <name>`; switching never creates anything.

### `wt switch` (no name) — pick

`git worktree list` piped through `fzf`, cd to the selection. Requires fzf on
PATH; degrade to a numbered list if absent.

Bare `wt` prints the general usage notes.

### `wt ls`

All worktrees with branch, dirty flag, ahead/behind trunk counts, and a
`persistent` marker. Human-readable; `--porcelain` later if needed.

### `wt done` — merge back and tear down (offline)

The ephemeral happy path. Never touches the network.

1. Preconditions (hard stop with a clear message if violated):
   - Run from a non-trunk worktree.
   - Worktree is clean (`git status --porcelain` empty, untracked included).
   - Trunk worktree is clean.
2. Rebase feature branch on local trunk. On conflict: stop, print state and
   next steps (`resolve → git rebase --continue → wt done`), exit non-zero.
3. `git -C <trunk_worktree> merge --ff-only <name>`.
4. Teardown (unless persistent — see below): run `.wt/destroy` if present,
   `git worktree remove`, `git branch -d <name>`, cd back to the trunk
   worktree via the shim.

Persistent mode (project `persistent = true`, or branch `wt-persist`, or
`--keep`): skip teardown, instead rebase the feature branch on the updated
trunk (the current regroup behavior) and stay in the worktree.

Flags: `--rm` / `--keep` override persistence config one-off.

### `wt ship` — done, bracketed by pull and push

Same as `done` with network sync:

1. `git fetch origin --prune`.
2. Fast-forward local trunk to `origin/<trunk>` (ff-only; stop if trunk has
   diverged from origin — that's a human decision).
3. Run the `done` flow (rebase, ff-merge, teardown/persist logic identical).
4. `git push origin <trunk>`.

The feature branch itself is never pushed — trunk is what ships. Same
`--rm`/`--keep` flags.

### `wt rm [name] [--force]` — abandon

Teardown without merging. Current gwrm behavior, kept interactive since it's a
human-initiated destructive action:

- No name + inside a worktree: remove the current one. No name + in the main
  checkout: fzf-pick. Never removes the trunk worktree.
- Dirty worktree: refuse, show the dirt, require `--force`.
- Confirm `[y/N]`, then `.wt/destroy` hook, `git worktree remove [--force]`,
  and `git branch -d` (branch deletion skipped if unmerged — report it,
  suggest `git branch -D` manually).
- If cwd was inside the removed worktree, shim cds back to the main checkout.

### `wt persist` — toggle

Flip `branch.<name>.wt-persist` for the current worktree. Prints new state.

### `wt skill` — print the wt agent skill

Prints the embedded wt skill to stdout. Agents don't get an installed
skill file; instead, agent docs (CLAUDE.md / AGENTS.md) carry a one-liner:
"To manage worktrees, run `wt skill` and follow it." No
install step, no staleness — the binary is the source of truth.

For those who want a file on disk anyway: `wt skill --export <skills-dir>`
writes `<skills-dir>/wt/SKILL.md`. The skill can then be invoked with requests
such as `/wt ship`, `/wt merge`, or `/wt create a new workspace`.

### `wt init` — set up a project

Interactive-ish, all optional pieces:

- Offer to scaffold `.wt/` with commented example `create`/`destroy` hooks and
  `config`.
- Offer to append the `wt skill` pointer line to CLAUDE.md / AGENTS.md.

### `wt prompt` — emit the project-setup prompt

Prints the embedded "prepare this project" prompt to stdout for pasting into
any agent. The prompt instructs the agent to:

1. Read `wt --help` / `wt help hooks` for the hook contract.
2. Inspect the project and write `.wt/create` so a fresh worktree is runnable:
   dependencies, secrets/keys copied from the base checkout (e.g.
   `config/master.key`), symlinks or redirects for shared state, anything else
   `git worktree add` doesn't give you.
3. Write `.wt/destroy` if teardown needs work (kill servers, drop DBs).
4. Prove it: `wt create tmp-<something>`, verify the app runs, `wt rm` it.
5. Consider `persistent = true` if create takes long enough to hurt.

### `wt shellenv` — emit the shim

`eval "$(wt shellenv zsh)"` in `.zshrc`. (Named `shellenv`, homebrew-style,
because `init` is taken by project setup.)

## The shell shim

A binary can't change the parent shell's cwd, so the direnv/zoxide pattern:

- `wt shellenv zsh` prints a small wrapper function `wt()` that calls the real
  binary with `WT_SHIM=1` set, captures a final `cd:<path>` line from a
  side-band (fd 3 or a distinguishable last stdout line — decide at
  implementation; fd 3 is cleaner), and `builtin cd`s to it.
- The shim exports `WORKTREE=<name>` when entering a worktree and unsets it
  when cd-ing back to the main checkout. This replaces the old `.gwt/env`
  hook, which is dropped entirely.
- Without the shim (bare binary, scripts, agents), every command still works;
  commands that would cd instead print the path so callers can cd themselves.
- zsh first; the shim is trivial enough that bash/fish are cheap follow-ups.

## Hooks — the `.wt/` directory (all optional)

Committed to the repo, so versioned and present in every worktree.

| File | When | Contract |
|---|---|---|
| `.wt/create` | after worktree add, cwd = new worktree | `$1` = absolute path of main checkout. Make the directory runnable. |
| `.wt/destroy` | before worktree remove, cwd = worktree | Cleanup. Failure warns but does not block removal. |
| `.wt/config` | read by the binary | `key = value` lines. Initially just `persistent = true`. |

Hooks run only if executable. `.gwt/` is not read — no migration; projects
rename the dir and delete `env`.

## Persistence resolution

Effective persistence for a worktree, first match wins:

1. `--rm` / `--keep` flag on `done`/`ship`
2. `git config branch.<name>.wt-persist` (set by `wt persist` / `--persist`)
3. `persistent = true` in `.wt/config`
4. default: ephemeral

## The wt skill

This is what `wt skill` prints:

```
Use the matching `wt` command to create, switch, list, remove, persist, merge,
or ship worktrees. Use `wt done` to merge locally and `wt ship` when asked to
sync with the remote.
If it stops on a rebase conflict: inspect, resolve, `git add`,
`git rebase --continue`, then re-run the same wt command.
If it stops on dirty state: commit or stash as appropriate, re-run.
Never non-ff merge into trunk. Never push unless the user asked (use ship).
```

Named `wt`, embedded in the binary, served by `wt skill`, exportable to a
specified skills directory with `wt skill --export <skills-dir>`. Agent docs
point at it with a one-liner instead of carrying a copy.

## Implementation

### Language & dependencies

Go, stdlib only (no cobra needed for this surface; hand-rolled dispatch is
fine and keeps the binary lean). Shells out to `git` and `fzf`. Docs, skill,
prompt, and shim templates in `embed.FS`.

### Project layout

```
wt/
  main.go            # dispatch, flag parsing
  git.go             # git helpers: run, worktree list parsing, trunk resolution
  create.go          # wt create
  switch.go          # wt switch and fzf pick
  done.go            # done + ship (ship = done with sync bracket)
  rm.go              # wt rm
  ls.go              # wt ls
  persist.go         # wt persist
  init.go            # wt init, wt prompt, wt skill
  shellenv.go        # wt shellenv
  hooks.go           # .wt/ discovery, config parsing, hook execution
  embedded/
    skill.md         # wt skill
    prompt.md        # project-setup prompt
    shim.zsh         # shell wrapper template
    hooks/           # example create/destroy/config for wt init
  *_test.go
```

### Milestones

1. **Core spawn/teardown**: git helpers, trunk resolution, `wt create`/`switch`,
   `wt rm`, hooks (`create`/`destroy`), `wt ls`. Usable without shim
   (prints paths).
2. **Shim**: `wt shellenv zsh`, cd side-band, `WORKTREE` export, fzf pick.
   At this point `wt` fully replaces `gwt`/`gwrm` daily.
3. **Regroup**: `wt done`, `wt ship`, persistence resolution, `wt persist`.
4. **Agent story**: embedded docs, `wt skill` (+ `--export`), `wt prompt`,
   `wt init`.
5. **Cutover**: rename `.gwt/` → `.wt/` in 10er/Tuna/nitro_kit, delete `env`
   hooks, retire the zsh functions, replace the regroup skill.

### Testing

Integration tests against real temp git repos (Go tests that `git init` a
fixture with a trunk, commits, an origin remote via a local bare repo). Cover:

- trunk resolution (origin/HEAD, main-only, master-only, neither)
- create: fresh branch, existing branch, existing worktree, create-hook
  failure
- done: happy path, dirty feature, dirty trunk, rebase conflict exit,
  persistent variants, branch-delete refusal when unmerged
- ship: ff of local trunk, diverged-trunk stop, push
- rm: dirty refusal, --force, destroy-hook failure is non-fatal
- config/persistence precedence

### Error message quality

Since agents consume these, every hard stop states: what was attempted, what
blocked it, and the exact commands to resolve and resume. Exit codes: 0 ok,
1 blocked (resolvable), 2 usage/environment error.

## Decisions log

- Single binary (Go) + eval'd shell shim; shim is the only non-binary piece.
- Ephemeral by default; persistence via config/flag for costly-setup projects.
- `done` = offline merge+teardown; `ship` = fetch/ff + done + push trunk.
  Feature branches never pushed.
- Hard stop on dirty state everywhere in merge flows; agent handles judgment.
- `.gwt/env` dropped; `WORKTREE` exported by the shim.
- `.wt/` entirely optional; no `.gwt/` back-compat.
- `git branch -d` only, never `-D`.
- wt never drops the trunk: deleting the trunk branch or removing the trunk
  worktree is refused in the low-level helpers (`deleteBranch`,
  `removeWorktree`), not just at command-level target resolution; `wt rm`
  refuses to remove anything when trunk can't even be resolved.
- No pushes outside `wt ship`; `ship` pushes trunk only.
- Creation and switching are explicit, separate commands so navigation can
  never create a worktree by mistake.
- Skills live inline in the binary: `wt skill` prints, agent docs carry a
  "run `wt skill`" pointer, `--export` for those who want a file. No install
  ceremony.
