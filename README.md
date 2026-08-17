# wt

Ephemeral Git worktrees: create, switch, merge, ship, and remove them without the plumbing.

```sh
brew install mikker/tap/wt
echo 'eval "$(wt shellenv zsh)"' >> ~/.zshrc
```

Run `wt create my-change`, then `wt done` to merge locally or `wt ship` to sync and push.

[Documentation](https://wt.fut.sh)
