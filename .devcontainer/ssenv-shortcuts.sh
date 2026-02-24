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

# Dynamic prompt prefix for isolated environments
_ssenv_prompt() {
  if [ -n "${SSENV_ACTIVE:-}" ]; then
    PS1="(${SSENV_ACTIVE}) ${_SSENV_ORIG_PS1:-\$ }"
  else
    PS1="${_SSENV_ORIG_PS1:-\$ }"
  fi
}

# Save original PS1 once (guard against re-source)
if [ -z "${_SSENV_ORIG_PS1+x}" ]; then
  _SSENV_ORIG_PS1="${PS1:-}"
fi

# Register prompt hook (append, don't clobber existing PROMPT_COMMAND)
if [[ "${PROMPT_COMMAND:-}" != *"_ssenv_prompt"* ]]; then
  PROMPT_COMMAND="_ssenv_prompt${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
