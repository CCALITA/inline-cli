#!/bin/sh
# inline-cli installer
# Usage: curl -sSL https://raw.githubusercontent.com/CCALITA/inline-cli/main/scripts/install.sh | sh

set -e

REPO="CCALITA/inline-cli"
INSTALL_DIR="${INLINE_CLI_INSTALL_DIR:-$HOME/.local/bin}"
MARKER_START="# >>> inline-cli >>>"
MARKER_END="# <<< inline-cli <<<"

# ── Helpers ────────────────────────────────────────────────────────────

info() { printf "\033[32m%s\033[0m\n" "$1"; }
warn() { printf "\033[33m%s\033[0m\n" "$1"; }
error() { printf "\033[31merror: %s\033[0m\n" "$1" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       error "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)             error "unsupported architecture: $(uname -m)" ;;
  esac
}

detect_shell() {
  basename "${SHELL:-/bin/sh}"
}

shell_config_file() {
  case "$1" in
    zsh)  echo "${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash) echo "$HOME/.bashrc" ;;
    *)    echo "" ;;
  esac
}

# ── Download ───────────────────────────────────────────────────────────

download_binary() {
  local os="$1" arch="$2"

  # Get latest release tag.
  local latest
  latest=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$latest" ]; then
    error "failed to fetch latest release. Check https://github.com/${REPO}/releases"
  fi

  local filename="inline-cli_${latest#v}_${os}_${arch}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${latest}/${filename}"

  info "downloading inline-cli ${latest} for ${os}/${arch}..."

  mkdir -p "$INSTALL_DIR"

  if command -v curl >/dev/null 2>&1; then
    curl -sSL "$url" | tar xz -C "$INSTALL_DIR" inline-cli
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url" | tar xz -C "$INSTALL_DIR" inline-cli
  else
    error "curl or wget is required"
  fi

  chmod +x "$INSTALL_DIR/inline-cli"
  info "installed to $INSTALL_DIR/inline-cli"
}

# ── Shell integration ──────────────────────────────────────────────────

install_shell_integration() {
  local shell="$1"
  local config_file
  config_file=$(shell_config_file "$shell")

  if [ -z "$config_file" ]; then
    warn "unknown shell: $shell — skipping shell integration"
    return
  fi

  case "$shell" in
    zsh|bash) ;;
    *)
      warn "shell '$shell' is not yet supported — skipping shell integration (supported: zsh, bash)"
      return
      ;;
  esac

  # Create config file if it doesn't exist.
  touch "$config_file"

  # Remove existing integration (idempotent).
  if grep -q "$MARKER_START" "$config_file" 2>/dev/null; then
    # Use a temp file to avoid sed -i portability issues.
    local tmpfile
    tmpfile=$(mktemp)
    awk "/$MARKER_START/{skip=1} /$MARKER_END/{skip=0; next} !skip" "$config_file" > "$tmpfile"
    mv "$tmpfile" "$config_file"
  fi

  # Add integration.
  cat >> "$config_file" << EOF
$MARKER_START
eval "\$($INSTALL_DIR/inline-cli init $shell)"
$MARKER_END
EOF

  info "added shell integration to $config_file"
}

# ── Ensure PATH ────────────────────────────────────────────────────────

ensure_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return ;;
  esac

  local shell
  shell=$(detect_shell)
  local config_file
  config_file=$(shell_config_file "$shell")

  if [ -n "$config_file" ] && ! grep -q "export PATH=.*${INSTALL_DIR}" "$config_file" 2>/dev/null; then
    echo "export PATH=\"${INSTALL_DIR}:\$PATH\"" >> "$config_file"
    info "added $INSTALL_DIR to PATH in $config_file"
  fi
}

# ── Main ───────────────────────────────────────────────────────────────

main() {
  local os arch shell

  os=$(detect_os)
  arch=$(detect_arch)
  shell=$(detect_shell)

  info "inline-cli installer"
  info "OS: $os, Arch: $arch, Shell: $shell"
  echo ""

  download_binary "$os" "$arch"
  ensure_path
  install_shell_integration "$shell"

  echo ""
  info "installation complete!"
  echo ""

  # Run interactive setup if stdin is a terminal.
  if [ -t 0 ]; then
    "$INSTALL_DIR/inline-cli" setup
  else
    echo "Run 'inline-cli setup' to configure your backend."
  fi

  echo ""
  echo "Next steps:"
  echo "  1. Restart your shell: exec \$SHELL"
  echo "  2. Type something and press Ctrl+J (or Shift+Enter in supported terminals)"
  echo ""
}

main "$@"
