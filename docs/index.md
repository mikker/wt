---
layout: default
title: Documentation
description: Ephemeral Git worktrees without the plumbing.
permalink: /
---

# Worktrees without the ceremony.

> **TL;DR:** `wt` creates a worktree for each task, moves you into it, and
> merges or ships it back to trunk when you are done.

<pre><code>$ wt create blue-button
$EDITOR .
$ wt ship <span class="cursor">█</span>
rebased blue-button onto main
merged blue-button into main
removed blue-button
pushed main</code></pre>

Latest release: **{{ site.version }}**

```sh
brew install mikker/tap/wt
```

Enable directory switching in zsh:

```sh
echo 'eval "$(wt shellenv zsh)"' >> ~/.zshrc
exec zsh
```

Then create a worktree from any Git repository:

```sh
wt create my-change
```

Worktrees live beside the main checkout at
`../<project>-worktrees/<name>`. They are ephemeral by default: `wt done`
merges locally and removes the worktree; `wt ship` also syncs and pushes.

## Start here

- [Commands and options](commands.md)
- [Project hooks and persistence](projects.md)
- [Agent and lifecycle automation](automation.md)
- [Source](https://github.com/mikker/wt)
