# wt shell integration — eval "$(wt shellenv zsh)" in your .zshrc
#
# A subprocess can't change its parent shell's cwd or environment, so this
# wrapper shadows the `wt` binary with a function of the same name. It runs
# the real binary (`command wt`, bypassing this function) with WT_SHIM=1 and
# fd 3 pointed at a temp file. The binary writes zsh directives there
# (builtin cd, export/unset WORKTREE) instead of to stdout, one per line;
# this wrapper evals them after the binary exits. stdout and stderr pass
# through untouched, so interactive tools like fzf still work.
wt() {
  local tmp ret
  tmp=$(mktemp)
  WT_SHIM=1 command wt "$@" 3>"$tmp"
  ret=$?
  eval "$(<"$tmp")"
  rm -f "$tmp"
  return $ret
}
