<p align="center">
  <h1 align="center">inline-cli</h1>
  <p align="center">
    Ask Claude without leaving your command line.<br/>
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

```
~/project $ what does the --recursive flag do in rsync ⇧↵

────────────────────────────────────────────────────
The --recursive (-r) flag tells rsync to copy
directories and their contents recursively.
Without it, rsync only copies files in the
top-level source directory.

Note: -a (archive mode) implies -r, so if you're
already using -a you don't need -r separately.
────────────────────────────────────────────────────

~/project $
```

No context switching. No new window. The answer streams in below your prompt and you keep working.

## Install

```sh
curl -sSL https://raw.githubusercontent.com/CCALITA/inline-cli/main/scripts/install.sh | sh
```

Or build from source:

```sh
git clone https://github.com/CCALITA/inline-cli.git
cd inline-cli
make build
# Binary is at ./build/inline-cli
```

Then add to your `.zshrc`:

```sh
eval "$(inline-cli init zsh)"
```

Set your API key:

```sh
export ANTHROPIC_API_KEY=sk-ant-...
```

Restart your shell. Done.

## How it works

```
 ┌─────────────────┐     Unix socket     ┌──────────────┐     HTTPS/SSE     ┌───────────┐
 │  zsh (ZLE widget)│ ──────────────────> │  daemon      │ ─────────────────>│ Claude API│
 │  captures buffer │ <── NDJSON stream── │  per-dir     │ <── streaming ──  │           │
 │  renders output  │                     │  sessions    │                   │           │
 └─────────────────┘                     └──────────────┘                   └───────────┘
```

**Three pieces:**

1. **Shell integration** — A zsh widget captures your command-line buffer on <kbd>Shift</kbd>+<kbd>Enter</kbd> (or <kbd>Ctrl</kbd>+<kbd>J</kbd>) and pipes it to the CLI.
2. **Background daemon** — A long-lived Go process manages conversation sessions over a Unix domain socket. Sub-millisecond IPC. No cold start per query.
3. **Pluggable backend** — Talks to Claude via direct API, the `claude` CLI, or ACP. Streams responses as markdown to your terminal.

### Directory-scoped sessions

Every directory gets its own conversation. Ask a follow-up question in the same directory and the daemon remembers context. Change directories and the old session stops automatically.

```
~/project $ explain the auth middleware ⇧↵
  ... (streaming response) ...

~/project $ what about the rate limiter? ⇧↵
  ... (knows you're still talking about this project) ...

cd ~/other-project
  (previous session ends)

~/other-project $ how does the build work here? ⇧↵
  ... (fresh session, fresh context) ...
```

### Auto-start

The daemon starts itself on the first query. No setup, no `systemd`, no `launchd`. When your shell exits, it cleans up.

## Usage

The primary interface is the keybinding — type and press <kbd>Shift</kbd>+<kbd>Enter</kbd> or <kbd>Ctrl</kbd>+<kbd>J</kbd>. But you can also use the CLI directly:

```sh
# Direct query
inline-cli query --prompt "explain git rebase --onto"

# Check what's running
inline-cli status

# Manage the daemon
inline-cli daemon start
inline-cli daemon stop

# Raw output (no markdown rendering)
inline-cli query --prompt "list 5 unix commands" --raw
```

## Configuration

Config lives at `~/.inline-cli/config.toml`. All fields are optional — defaults are sensible.

```toml
# Backend: "api" (default), "cli", or "acp"
backend = "api"

# API backend settings
api_key = "sk-ant-..."                    # or set ANTHROPIC_API_KEY env var
model = "claude-sonnet-4-20250514"        # default model
api_base_url = ""                         # custom API endpoint (proxy, gateway)

# CLI backend settings (uses `claude` command)
cli_path = ""                             # auto-detected from PATH if empty

# Session settings
max_session_idle_minutes = 30
max_messages = 50
```

### Backends

| Backend             | Config                    | What it does                                                                       |
| ------------------- | ------------------------- | ---------------------------------------------------------------------------------- |
| **`api`** (default) | Needs `ANTHROPIC_API_KEY` | Direct HTTPS to Anthropic Messages API with SSE streaming                          |
| **`cli`**           | Needs `claude` in PATH    | Execs `claude -p <prompt>` and streams stdout. Uses your existing claude CLI auth. |
| **`acp`**           | —                         | Agent Communication Protocol (planned)                                             |

Switch backends via config file or env var:

```sh
# Use claude CLI instead of direct API
export INLINE_CLI_BACKEND=cli

# Or in config.toml
backend = "cli"
```

### Environment variables

| Variable                  | Purpose                                |
| ------------------------- | -------------------------------------- |
| `ANTHROPIC_API_KEY`       | API key (required for `api` backend)   |
| `INLINE_CLI_BACKEND`      | Backend selection: `api`, `cli`, `acp` |
| `INLINE_CLI_MODEL`        | Override model                         |
| `INLINE_CLI_SOCKET`       | Custom socket path                     |
| `INLINE_CLI_API_BASE_URL` | Custom API endpoint                    |
| `INLINE_CLI_CLI_PATH`     | Path to `claude` binary                |
| `INLINE_CLI_MAX_IDLE`     | Session idle timeout (minutes)         |

Precedence: env vars > config file > defaults.

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

## Powerlevel10k integration

inline-cli ships a p10k prompt segment. Add `inline_cli` to your prompt elements:

```zsh
# In ~/.p10k.zsh
typeset -g POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(
  ...
  inline_cli  # green dot when daemon is active
  ...
)
```

For other prompt themes, use the `inline_cli_prompt_info` function:

```zsh
PROMPT='$(inline_cli_prompt_info)'$PROMPT
```

## Architecture

```
inline-cli/
├── cmd/inline-cli/       # CLI entry point + embedded shell scripts
├── internal/
│   ├── backend/          # Backend interface + implementations (API, CLI, ACP)
│   ├── claude/           # Claude API client + SSE streaming parser
│   ├── config/           # Config loading (TOML + env vars)
│   ├── daemon/           # Daemon lifecycle + Unix socket server
│   ├── ipc/              # IPC protocol (NDJSON over Unix socket)
│   ├── render/           # Terminal renderer + markdown (glamour)
│   └── session/          # Directory-scoped session manager
├── shell/                # Shell integration scripts
└── scripts/              # Install/uninstall scripts
```

Single Go binary, ~14MB. No runtime dependencies.

## Requirements

- **Go 1.22+** (build only)
- **macOS** or **Linux**
- **zsh** (bash and fish support planned)
- One of: [Anthropic API key](https://console.anthropic.com/), `claude` CLI installed, or ACP endpoint

## Uninstall

```sh
inline-cli daemon stop
rm "$(which inline-cli)"
```

Remove the `# >>> inline-cli >>>` block from your `.zshrc`.

## License

MIT
