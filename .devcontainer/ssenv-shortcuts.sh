#!/usr/bin/env bash
# Shell shortcuts for ssenv (source this file in interactive shells).

_ssenv_bin() {
  if [ -x /usr/local/bin/ssenv ]; then
    /usr/local/bin/ssenv "$@"
  else
    command ssenv "$@"
  fi
}

# Wrap ssenv so deleting the active env auto-resets back to original HOME first.
ssenv() {
  if [ "${1:-}" = "delete" ] && [ $# -ge 2 ]; then
    local target="$2"
    if [ -n "${SSENV_ACTIVE:-}" ] && [ "$target" = "$SSENV_ACTIVE" ]; then
      eval "$(_ssenv_bin --eval reset)"
      if [ -d /workspace ]; then
        cd /workspace
      fi
    fi
  fi
  _ssenv_bin "$@"
}

ssuse() {
  command ssuse "$@"
}

ssback() {
  if [ "${SSENV_ENTERED:-0}" = "1" ]; then
    exit
  fi

  if [ -n "${SSENV_ACTIVE:-}" ]; then
    eval "$(_ssenv_bin --eval reset)"
    return
  fi

  if [ -d /workspace ] && [ "${PWD:-}" != "/workspace" ]; then
    cd /workspace
    return
  fi

  echo "Already in base shell."
}

ssnew() {
  command ssnew "$@"
}

ssls() {
  command ssls "$@"
}

sshelp() {
  command sshelp "$@"
}

# Override bash built-in help
alias help='/workspace/.devcontainer/bin/help'

# Visual feedback for isolated environments
if [ -n "${SSENV_ACTIVE:-}" ]; then
  export PS1="(${SSENV_ACTIVE}) ${PS1:-}"
fi
