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
# Shift+Enter triggers the inline-cli widget.
#
# How it works:
#   The kitty keyboard protocol is enabled via zle-line-init (below),
#   which makes modern terminals send \e[13;2u for Shift+Enter.
#   This works automatically in iTerm2 3.5+, kitty, WezTerm, Ghostty, foot.
#
#   For terminals using xterm's modifyOtherKeys, \e[27;2;13~ is also bound.
#
#   Ctrl+J (^J) is always available as a universal fallback.
#
# Legacy terminals (no protocol support):
#   Configure your terminal to send \n (0x0a) for Shift+Enter:
#     Ghostty:  keybind = shift+enter=text:\n
#     kitty:    map shift+enter send_text all \x0a
#     WezTerm:  { key = "Enter", mods = "SHIFT", action = SendString("\x0a") }
#     iTerm2:   Keys > Key Mappings > Shift+Return → Send Hex Code 0a
#   (These are fallbacks — the protocol approach above is preferred.)

bindkey '^J'           _inline_cli_widget
bindkey '\e[13;2u'     _inline_cli_widget
bindkey '\e[27;2;13~'  _inline_cli_widget

# ── Kitty keyboard protocol ───────────────────────────────────────────
#
# Enable the kitty keyboard protocol so terminals that support it
# (iTerm2 3.5+, kitty, WezTerm, foot, Ghostty) send CSI u sequences
# for modified keys. This makes Shift+Enter send \e[13;2u which we
# bind above.
#
# The protocol is pushed on zle-line-init and popped on zle-line-finish,
# so it only affects ZLE (line editing) — it never leaks into command
# execution or subshells.
#
# Safe chaining: if another plugin (oh-my-zsh, fzf, vi-mode, etc.)
# already defined zle-line-init, we wrap it rather than clobbering it.

_inline_cli_kitty_push() {
  printf '\e[>1u'
}

_inline_cli_kitty_pop() {
  printf '\e[<u'
}

# -- zle-line-init hook (chain-safe) --
if (( ! ${+functions[_inline_cli_zle-line-init]} )); then
  case "${widgets[zle-line-init]:-}" in
    builtin|"")
      _inline_cli_zle-line-init() {
        _inline_cli_kitty_push
      }
      ;;
    user:*)
      zle -A zle-line-init _inline_cli_orig_zle-line-init
      _inline_cli_zle-line-init() {
        _inline_cli_kitty_push
        zle _inline_cli_orig_zle-line-init -- "$@"
      }
      ;;
  esac
  zle -N zle-line-init _inline_cli_zle-line-init
fi

# -- zle-line-finish hook (chain-safe) --
if (( ! ${+functions[_inline_cli_zle-line-finish]} )); then
  case "${widgets[zle-line-finish]:-}" in
    builtin|"")
      _inline_cli_zle-line-finish() {
        _inline_cli_kitty_pop
      }
      ;;
    user:*)
      zle -A zle-line-finish _inline_cli_orig_zle-line-finish
      _inline_cli_zle-line-finish() {
        zle _inline_cli_orig_zle-line-finish -- "$@"
        _inline_cli_kitty_pop
      }
      ;;
  esac
  zle -N zle-line-finish _inline_cli_zle-line-finish
fi

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
# Shows 👀 only when there is an active session for the current directory.

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
        print -P "👀 %F{green}inline-cli%f"
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
