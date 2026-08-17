# wt shell integration — eval "$(wt shellenv bash)" in your .bashrc
#
# A subprocess can't change its parent shell's cwd or environment, so this
# wrapper shadows the `wt` binary with a function of the same name. It runs
# the real binary (`command wt`, bypassing this function) with WT_SHIM=1 and
# fd 3 pointed at a temp file. The binary writes shell directives there
# (builtin cd, export/unset WORKTREE) instead of to stdout, one per line;
# this wrapper evaluates them after the binary exits. stdout and stderr pass
# through untouched, so interactive tools like fzf still work.
wt() {
  local tmp ret
  tmp=$(mktemp)
  WT_SHIM=1 command wt "$@" 3>"$tmp"
  ret=$?
  source "$tmp"
  rm -f "$tmp"
  return $ret
}

# Read names fresh for every completion so newly created and removed
# worktrees are reflected without restarting the shell.
_wt_worktree_names() {
  local line
  while IFS= read -r line; do
    if [[ "$line" == "branch refs/heads/"* ]]; then
      printf '%s\n' "${line#branch refs/heads/}"
    fi
  done < <(command git worktree list --porcelain 2>/dev/null)
}

_wt() {
  local cur subcommand
  COMPREPLY=()
  cur=${COMP_WORDS[COMP_CWORD]}

  if (( COMP_CWORD == 1 )); then
    COMPREPLY=( $(compgen -W 'create c switch s rm ls done ship persist skill prompt init shellenv help' -- "$cur") )
    return
  fi

  subcommand=${COMP_WORDS[1]}
  case "$subcommand" in
    switch|s)
      if (( COMP_CWORD == 2 )); then
        COMPREPLY=( $(compgen -W "$(_wt_worktree_names)" -- "$cur") )
      fi
      ;;
    rm)
      if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W '--force --help' -- "$cur") )
      else
        COMPREPLY=( $(compgen -W "$(_wt_worktree_names)" -- "$cur") )
      fi
      ;;
  esac
}

complete -F _wt wt
