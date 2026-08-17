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

# Read names fresh for every completion so newly created and removed
# worktrees are reflected without restarting the shell.
_wt_worktree_names() {
  local line
  for line in "${(@f)$(command git worktree list --porcelain 2>/dev/null)}"; do
    if [[ "$line" == "branch refs/heads/"* ]]; then
      print -r -- "${line#branch refs/heads/}"
    fi
  done
}

_wt() {
  local -a commands reply
  commands=(
    'create:create and enter a worktree'
    'c:create and enter a worktree'
    'switch:enter an existing worktree'
    's:enter an existing worktree'
    'rm:remove a worktree'
    'ls:list worktrees'
    'done:merge locally and tear down'
    'ship:sync, merge, push, and tear down'
    'persist:toggle persistence'
    'skill:print or export the agent skill'
    'prompt:print the project-setup prompt'
    'init:scaffold project files'
    'shellenv:print shell integration'
    'help:show help'
  )

  if (( CURRENT == 2 )); then
    _describe -t commands 'wt command' commands
    return
  fi

  case "${words[2]}" in
    switch|s)
      if (( CURRENT == 3 )); then
        reply=("${(@f)$(_wt_worktree_names)}")
        _describe -t worktrees 'worktree' reply
      fi
      ;;
    rm)
      if [[ "${words[CURRENT]}" == -* ]]; then
        local -a options=(
          '--force:discard uncommitted changes'
          '--help:show help'
        )
        _describe -t options 'option' options
      else
        reply=("${(@f)$(_wt_worktree_names)}")
        _describe -t worktrees 'worktree' reply
      fi
      ;;
  esac
}

if (( ! $+functions[compdef] )); then
  autoload -Uz compinit && compinit
fi
compdef _wt wt
