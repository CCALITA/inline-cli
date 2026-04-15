<p align="center">
  <h1 align="center">inline-cli</h1>
  <p align="center">
    Lazist way to ask AI without leaving your command line.<br/>
    Type a question, press <kbd>Shift</kbd>+<kbd>Enter</kbd>, get a streaming answer — right where you are.
  </p>
</p>

<p align="center">
  <a href="#install">Install</a> &nbsp;&bull;&nbsp;
  <a href="#how-it-works">How it works</a> &nbsp;&bull;&nbsp;
  <a href="#configuration">Configuration</a> &nbsp;&bull;&nbsp;
  <a href="#supported-terminals">Supported terminals</a>
</p>

---

![inline-cli4](https://github.com/user-attachments/assets/be431ede-4b5f-4484-8547-8032633d0a92)

No context switching. No new window. The answer streams in below your prompt and you keep working.
Just press shift with enter to trigger claude response.

## Install

```sh
curl -sSL https://raw.githubusercontent.com/CCALITA/inline-cli/main/scripts/install.sh | sh
```

Or build from source:

```sh
git clone https://github.com/CCALITA/inline-cli.git
cd inline-cli
make build                                        # → ./build/inline-cli
cp ./build/inline-cli ~/.local/bin/inline-cli     # or anywhere in your PATH
```

Then set up your shell and backend:

```sh
# 1. Add shell integration (pick one)
echo 'eval "$(inline-cli init zsh)"'  >> ~/.zshrc    # zsh
echo 'eval "$(inline-cli init bash)"' >> ~/.bashrc   # bash

# 2. Choose a backend
inline-cli setup

# 3. Restart your shell
exec $SHELL
```

## How it works

```
 ┌─────────────────┐     Unix socket     ┌──────────────┐     HTTPS/SSE     ┌───────────┐
 │  shell widget    │ ──────────────────> │  daemon      │ ─────────────────>│ Claude API│
 │  captures buffer │ <── NDJSON stream── │  per-dir     │ <── streaming ──  │           │
 │  renders output  │                     │  sessions    │                   └───────────┘
 └─────────────────┘                     │              │        or
                                          │              │ ───> claude CLI
                                          │              │ ───> gemini CLI
                                          │              │ ───> opencode CLI
                                          └──────────────┘
```

**Three pieces:**

1. **Shell integration** — A zsh ZLE widget or bash readline binding captures your command-line buffer on <kbd>Shift</kbd>+<kbd>Enter</kbd> (or <kbd>Ctrl</kbd>+<kbd>J</kbd>) and pipes it to the CLI.
2. **Background daemon** — A long-lived Go process manages conversation sessions over a Unix domain socket. Sub-millisecond IPC. No cold start per query.
3. **Pluggable backend** — Talks to Claude via direct API, the `claude` CLI, Gemini CLI, or OpenCode CLI. Streams responses as markdown to your terminal.

### Directory-scoped sessions

Every directory gets its own conversation. Ask a follow-up question in the same directory and the daemon remembers context. Change directories and the old session stops automatically.

A session indicator appears in your prompt while a session is active:

```
👀 ~/project $ explain the auth middleware ⇧↵
  ... (streaming response) ...

👀 ~/project $ what about the rate limiter? ⇧↵
  ... (knows you're still talking about this project) ...

cd ~/other-project
  (previous session ends, 👀 disappears)

~/other-project $ how does the build work here? ⇧↵
  ... (fresh session, 👀 appears again) ...
```

The indicator auto-detects your prompt framework (Powerlevel10k, Starship, plain zsh/bash) and injects itself without overwriting your existing prompt. Set `INLINE_CLI_NO_PROMPT=1` to disable it.

### Auto-start

The daemon starts itself on the first query. No setup, no `systemd`, no `launchd`. When your shell exits, it cleans up.

## Usage

The primary interface is the keybinding — type and press <kbd>Shift</kbd>+<kbd>Enter</kbd> or <kbd>Ctrl</kbd>+<kbd>J</kbd>. But you can also use the CLI directly:

```sh
# Direct query
shift + enter
# Check what's running
inline-cli status

# Manage backends
inline-cli setup              # Interactive first-time setup
inline-cli backend list       # List backends with install status
inline-cli backend show       # Show current backend
inline-cli backend set gemini # Switch backend (auto-restarts daemon)

# Manage the daemon
inline-cli daemon start
inline-cli daemon stop
```

## Configuration

Config lives at `~/.inline-cli/config.toml`. All fields are optional — defaults are sensible.

```toml
# Backend: "api" (default), "claude", "gemini", "opencode"
backend = "api"

# API backend settings
api_key = "sk-ant-..."                    # or set ANTHROPIC_API_KEY env var
model = "claude-sonnet-4-20250514"        # default model
api_base_url = ""                         # custom API endpoint (proxy, gateway)

# CLI backend paths (auto-detected from PATH if empty)
cli_path = ""
gemini_path = ""
opencode_path = ""

# Session settings
max_session_idle_minutes = 30
max_messages = 50
```

### Backends

| Backend                | Config                    | What it does                                                                       |
| ---------------------- | ------------------------- | ---------------------------------------------------------------------------------- |
| **`api`** (default)    | Needs `ANTHROPIC_API_KEY` | Direct HTTPS to Anthropic Messages API with SSE streaming                          |
| **`claude`**           | Needs `claude` in PATH    | Execs `claude -p <prompt>` and streams stdout. Uses your existing Claude CLI auth. |
| **`gemini`**           | Needs `gemini` in PATH    | Execs `gemini -p <prompt> -o text` and streams stdout. Uses Gemini CLI auth.       |
| **`opencode`**         | Needs `opencode` in PATH  | Execs `opencode run --format json` and parses the JSON event stream.               |

Switch backends via the CLI or config file:

```sh
# Interactive setup (detects installed CLIs)
inline-cli setup

# Direct switch
inline-cli backend set gemini
```

### Environment variables

| Variable                    | Purpose                                          |
| --------------------------- | ------------------------------------------------ |
| `ANTHROPIC_API_KEY`         | API key (required for `api` backend)             |
| `INLINE_CLI_MODEL`          | Override model                                   |
| `INLINE_CLI_SOCKET`         | Custom socket path                               |
| `INLINE_CLI_API_BASE_URL`   | Custom API endpoint                              |
| `INLINE_CLI_CLI_PATH`       | Path to `claude` binary                          |
| `INLINE_CLI_GEMINI_PATH`    | Path to `gemini` binary                          |
| `INLINE_CLI_OPENCODE_PATH`  | Path to `opencode` binary                        |
| `INLINE_CLI_MAX_IDLE`       | Session idle timeout (minutes)                   |
| `INLINE_CLI_NO_PROMPT`      | Set to `1` to disable prompt indicator           |

Precedence: env vars > config file > defaults.

## Supported shells

| Shell    | Integration           | Keybinding                                                  |
| -------- | --------------------- | ----------------------------------------------------------- |
| **zsh**  | ZLE widget            | <kbd>Ctrl</kbd>+<kbd>J</kbd>, Shift+Enter (CSI u / xterm)  |
| **bash** | `bind -x` / readline  | <kbd>Ctrl</kbd>+<kbd>J</kbd>, Shift+Enter (terminal remap) |

## Supported terminals

### Shift+Enter works natively

inline-cli auto-detects your terminal and enables the right protocol:

**Kitty keyboard protocol** (CSI u):

| Terminal          | Status       |
| ----------------- | ------------ |
| **kitty**         | Full support |
| **WezTerm**       | Full support |
| **ghostty**       | Full support |
| **iTerm2** (3.5+) | Full support |
| **foot**          | Full support |

**xterm modifyOtherKeys** (CSI 27;2;13~):

| Terminal                              | Status       |
| ------------------------------------- | ------------ |
| **xterm**                             | Full support |
| **VTE-based** (GNOME Terminal, Tilix) | Full support |

Both protocols are bound automatically — no manual configuration needed.

### Fallback: Ctrl+J

Terminals that support neither protocol use <kbd>Ctrl</kbd>+<kbd>J</kbd>:

| Terminal                 | Keybinding                   |
| ------------------------ | ---------------------------- |
| **Terminal.app** (macOS) | <kbd>Ctrl</kbd>+<kbd>J</kbd> |
| **Alacritty**            | <kbd>Ctrl</kbd>+<kbd>J</kbd> |

> **tmux note:** Extended key sequences are stripped by default. <kbd>Ctrl</kbd>+<kbd>J</kbd> always works. For Shift+Enter, add `set -g extended-keys on` to your tmux config.

## Prompt indicator

A 👀 emoji appears in your prompt while a session is active for the current directory. It auto-detects your setup:

| Prompt framework     | How it works                                                |
| -------------------- | ----------------------------------------------------------- |
| **Powerlevel10k**    | Auto-registers `inline_cli` segment in right prompt         |
| **Starship**         | `precmd` hook prints indicator above the prompt             |
| **Plain zsh**        | Prepends `$(inline_cli_prompt_info)` to `PROMPT`            |
| **oh-my-zsh**        | Same as plain zsh                                           |
| **bash**             | Prepends `$(_inline_cli_indicator)` to `PS1`                |

Set `INLINE_CLI_NO_PROMPT=1` to disable auto-injection. The `inline_cli_prompt_info` function is still available for manual use.

## Cross-compilation

Build release binaries for all supported platforms:

```sh
make release
```

Produces `{linux,darwin}_{amd64,arm64}` tarballs in `./build/` with SHA-256 checksums.

### Make targets

| Target         | What it does                                        |
| -------------- | --------------------------------------------------- |
| `make build`   | Build binary → `./build/inline-cli`                 |
| `make test`    | Run all tests with `-race -cover`                   |
| `make lint`    | Run `go vet`                                        |
| `make clean`   | Remove `./build/`                                   |
| `make release` | Cross-compile + tarball + SHA-256 checksums          |

## Architecture

```
inline-cli/
├── cmd/inline-cli/       # CLI entry point + embedded shell scripts
├── internal/
│   ├── backend/          # Backend interface + implementations (API, Claude CLI, Gemini CLI, OpenCode CLI)
│   ├── claude/           # Claude API client + SSE streaming parser
│   ├── config/           # Config loading (TOML + env vars)
│   ├── daemon/           # Daemon lifecycle + Unix socket server
│   ├── ipc/              # IPC protocol (NDJSON over Unix socket)
│   ├── render/           # Terminal renderer + markdown (glamour)
│   └── session/          # Directory-scoped session manager
├── shell/                # Shell integration scripts (zsh, bash)
└── scripts/              # Install/uninstall scripts
```

Single Go binary, ~14MB. No runtime dependencies.

## Requirements

- **Go 1.26+** (build only)
- **macOS** or **Linux**
- **zsh** or **bash**
- One of: [Anthropic API key](https://console.anthropic.com/), `claude` CLI, `gemini` CLI, or `opencode` CLI

## Uninstall

```sh
inline-cli daemon stop
rm "$(which inline-cli)"
```

Remove the `# >>> inline-cli >>>` block from your `.zshrc` or `.bashrc`.

## License

MIT
