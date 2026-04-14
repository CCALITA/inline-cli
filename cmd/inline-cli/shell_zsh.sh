#!/usr/bin/env zsh
# inline-cli shell integration for zsh
# Source this via: eval "$(inline-cli init zsh)"

# Binary path (substituted at init time).
INLINE_CLI_BIN="{{INLINE_CLI_BIN}}"

# ── Session state ─────────────────────────────────────────────────────
# Tracks which directory has an active session. Set on query, cleared on chpwd.

_INLINE_CLI_SESSION_DIR=""

# ── ZLE Widget ─────────────────────────────────────────────────────────

_inline_cli_widget() {
  local prompt_text="$BUFFER"
  if [[ -z "$prompt_text" ]]; then
    return
  fi

  # Clear the current buffer.
  BUFFER=""
  zle -R

  # Run the query. Output goes directly to terminal.
  "$INLINE_CLI_BIN" query --dir "$PWD" --prompt "$prompt_text" </dev/tty

  # Mark session active for this directory.
  _INLINE_CLI_SESSION_DIR="$PWD"

  # Update prompt indicator.
  # p10k caches prompt content — zle reset-prompt alone won't re-evaluate
  # segment functions. p10k display forces immediate re-evaluation.
  if (( $+functions[p10k] )); then
    p10k display '*/inline_cli'=show 2>/dev/null
  fi
  zle reset-prompt
}

zle -N _inline_cli_widget

# ── Keybinding ─────────────────────────────────────────────────────────
#
# Shift+Enter setup:
#   Configure your terminal to send \n (LF) for Shift+Enter.
#   Regular Enter sends \r (CR) → accept-line. Shift+Enter sends \n → our widget.
#
#   Ghostty:  keybind = shift+enter=text:\n
#   kitty:    map shift+enter send_text all \x0a
#   WezTerm:  { key = "Enter", mods = "SHIFT", action = SendString("\x0a") }
#   iTerm2:   Keys > Key Mappings > Shift+Return → Send Hex Code 0a
#
# Also bind CSI sequences for terminals that natively distinguish Shift+Enter:
#   \e[13;2u     kitty keyboard protocol
#   \e[27;2;13~  xterm modifyOtherKeys

bindkey '^J'           _inline_cli_widget
bindkey '\e[13;2u'     _inline_cli_widget
bindkey '\e[27;2;13~'  _inline_cli_widget

# ── Directory change hook ──────────────────────────────────────────────

_inline_cli_chpwd() {
  # Notify daemon to stop the session for the previous directory.
  if [[ -n "$OLDPWD" && "$OLDPWD" != "$PWD" ]]; then
    "$INLINE_CLI_BIN" stop-session --dir "$OLDPWD" 2>/dev/null &!
  fi
  # Session ended for the old dir; no session yet in the new dir.
  _INLINE_CLI_SESSION_DIR=""
}

autoload -Uz add-zsh-hook
add-zsh-hook chpwd _inline_cli_chpwd

# ── Prompt indicator ──────────────────────────────────────────────────
# Shows ● only when there is an active session for the current directory.

# Powerlevel10k custom segment (called by p10k if registered).
prompt_inline_cli() {
  if [[ "$_INLINE_CLI_SESSION_DIR" == "$PWD" ]]; then
    p10k segment -t "👀"
  fi
}

# Generic indicator for plain zsh / oh-my-zsh without p10k.
inline_cli_prompt_info() {
  if [[ "$_INLINE_CLI_SESSION_DIR" == "$PWD" ]]; then
    echo "👀 "
  fi
}

# Auto-inject indicator into the prompt unless opted out.
# Set INLINE_CLI_NO_PROMPT=1 to disable.
if [[ -z "$INLINE_CLI_NO_PROMPT" ]]; then
  if [[ -n "$POWERLEVEL9K_LEFT_PROMPT_ELEMENTS" ]]; then
    # Powerlevel10k: register our segment in the right prompt.
    # p10k calls prompt_inline_cli() on each render — no PROMPT clobbering.
    POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(inline_cli "${(@)POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS}")
  elif [[ -n "$STARSHIP_SHELL" ]]; then
    # Starship owns PROMPT entirely; use precmd to print the indicator
    # on a line above the prompt so it doesn't interfere.
    _inline_cli_starship_precmd() {
      if [[ "$_INLINE_CLI_SESSION_DIR" == "$PWD" ]]; then
        print -P "%F{green}● inline-cli%f"
      fi
    }
    add-zsh-hook precmd _inline_cli_starship_precmd
  else
    # Plain zsh / oh-my-zsh: prepend dynamic call to PROMPT.
    setopt PROMPT_SUBST 2>/dev/null
    PROMPT='$(inline_cli_prompt_info)'"${PROMPT}"
  fi
fi

# ── Auto-start daemon ─────────────────────────────────────────────────

if [[ ! -S "${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}" ]]; then
  "$INLINE_CLI_BIN" daemon start 2>/dev/null &!
fi
