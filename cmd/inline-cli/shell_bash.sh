#!/usr/bin/env bash
# inline-cli shell integration for bash
# Source this via: eval "$(inline-cli init bash)"

# Binary path (substituted at init time).
INLINE_CLI_BIN="{{INLINE_CLI_BIN}}"

# ── Readline function ─────────────────────────────────────────────────

_inline_cli_query() {
  local prompt_text="$READLINE_LINE"
  if [[ -z "$prompt_text" ]]; then
    return
  fi

  # Clear the current line.
  READLINE_LINE=""
  READLINE_POINT=0

  # Run the query. Output goes directly to terminal.
  "$INLINE_CLI_BIN" query --dir "$PWD" --prompt "$prompt_text" </dev/tty
}

# ── Keybinding ─────────────────────────────────────────────────────────
#
# Ctrl+J (\C-j) is the primary binding — works in every terminal.
#
# Shift+Enter setup:
#   Configure your terminal to send \n (LF) for Shift+Enter.
#   Regular Enter sends \r (CR) → accept-line. Shift+Enter sends \n → our function.
#
#   Ghostty:  keybind = shift+enter=text:\n
#   kitty:    map shift+enter send_text all \x0a
#   WezTerm:  { key = "Enter", mods = "SHIFT", action = SendString("\x0a") }
#   iTerm2:   Keys > Key Mappings > Shift+Return → Send Hex Code 0a
#
# Bash's bind cannot capture raw CSI sequences the way zsh's bindkey can,
# so Ctrl+J is the reliable binding. Terminals that remap Shift+Enter to
# \n (0x0a = Ctrl+J) will trigger this automatically.

bind -x '"\C-j": _inline_cli_query'

# ── Directory change hook ──────────────────────────────────────────────

_inline_cli_prev_pwd="$PWD"

_inline_cli_chpwd() {
  if [[ "$PWD" != "$_inline_cli_prev_pwd" ]]; then
    "$INLINE_CLI_BIN" stop-session --dir "$_inline_cli_prev_pwd" 2>/dev/null &
    _inline_cli_prev_pwd="$PWD"
  fi
}

# Prepend to PROMPT_COMMAND so we don't clobber existing hooks.
if [[ -z "$PROMPT_COMMAND" ]]; then
  PROMPT_COMMAND="_inline_cli_chpwd"
else
  PROMPT_COMMAND="_inline_cli_chpwd;$PROMPT_COMMAND"
fi

# ── Prompt indicator ──────────────────────────────────────────────────

# Outputs the indicator with readline-safe markers (\001/\002 are the raw
# bytes that \[/\] expand to — needed because \[/\] don't work inside
# command substitution).
_inline_cli_indicator() {
  local sock="${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}"
  if [[ -S "$sock" ]]; then
    printf '\001\033[32m\002●\001\033[0m\002 '
  fi
}

# One-time prepend: purely additive, never snapshots or overwrites PS1.
# Set INLINE_CLI_NO_PROMPT=1 to disable.
if [[ -z "$INLINE_CLI_NO_PROMPT" ]]; then
  PS1='$(_inline_cli_indicator)'"${PS1}"
fi

# ── Auto-start daemon ─────────────────────────────────────────────────

_inline_cli_sock="${INLINE_CLI_SOCKET:-/tmp/inline-cli-$(id -u).sock}"
if [[ ! -S "$_inline_cli_sock" ]]; then
  "$INLINE_CLI_BIN" daemon start 2>/dev/null &
  disown 2>/dev/null
fi
unset _inline_cli_sock
