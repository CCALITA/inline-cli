#!/usr/bin/env zsh
# inline-cli shell integration for zsh
# Source this via: eval "$(inline-cli init zsh)"

# Binary path (substituted at init time).
INLINE_CLI_BIN="{{INLINE_CLI_BIN}}"

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

  # Reset prompt after output.
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
}

autoload -Uz add-zsh-hook
add-zsh-hook chpwd _inline_cli_chpwd

# ── Prompt indicator ──────────────────────────────────────────────────

# For powerlevel10k: Define a custom prompt segment.
# Add 'inline_cli' to POWERLEVEL9K_LEFT_PROMPT_ELEMENTS or RIGHT.
prompt_inline_cli() {
  if [[ -S "${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}" ]]; then
    p10k segment -f green -t "●"
  fi
}

# For generic prompts: function that returns indicator string.
inline_cli_prompt_info() {
  if [[ -S "${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}" ]]; then
    echo "%F{green}●%f "
  fi
}

# Auto-inject indicator into the prompt unless opted out.
# Set INLINE_CLI_NO_PROMPT=1 to disable, or use the p10k segment above.
if [[ -z "$INLINE_CLI_NO_PROMPT" ]]; then
  setopt PROMPT_SUBST 2>/dev/null
  PROMPT='$(inline_cli_prompt_info)'"${PROMPT}"
fi

# ── Auto-start daemon ─────────────────────────────────────────────────

if [[ ! -S "${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}" ]]; then
  "$INLINE_CLI_BIN" daemon start 2>/dev/null &!
fi
