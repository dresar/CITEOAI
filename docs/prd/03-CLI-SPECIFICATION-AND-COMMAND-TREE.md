# 03 - CLI SPECIFICATION AND COMMAND TREE

## 1. Overview
CITEOAI is built CLI-first. All capabilities are exposed via clean command verbs and flags.

## 2. Command Tree
```
citeoai
├── [root]                     # Launch interactive TUI session
├── init                       # Setup wizard (interactive config & credentials creation)
├── doctor                     # Preflight system diagnostics (toolchain, config, API reachability)
├── run "<prompt>"             # Headless execution of a single task with streaming output
├── tui                        # Launch split-pane terminal UI explicitly
├── session                    # Manage persistent conversation sessions
│   ├── list                   # List all stored sessions
│   ├── resume <id>            # Resume session from saved state
│   ├── export <id>            # Export conversation transcript as JSON/Markdown
│   └── delete <id>            # Delete session data
├── skill                      # Manage markdown skills
│   ├── list                   # List discovered skills
│   ├── test <name>            # Validate skill syntax and schema
│   └── create <name>          # Scaffold a new skill template
├── memory                     # Agent persistent memory journal
│   ├── list                   # Show all recorded memory entries
│   ├── add "<content>"        # Manually add a memory note
│   └── forget <id>            # Delete a specific memory note
├── stats                      # FinOps token ledger and model execution statistics
├── version                    # Print version, commit hash, and build timestamp
└── serve                      # Lightweight headless JSON API server (for future integration)
```

## 3. Global Flags
- `--config, -c <path>`: Path to custom configuration file (default: `~/.citeoai/config.json`).
- `--model, -m <name>`: Override model name for the execution (e.g. `gemini-3.6-flash`).
- `--provider, -p <name>`: Override provider (e.g. `gemini`, `openai-compatible`, `ollama`).
- `--debug`: Enable verbose diagnostic logging to stderr.
- `--quiet, -q`: Silence progress spinners and non-essential output.
- `--no-color`: Disable ANSI color escapes.

## 4. Exit Code Contract
- `0`: Success.
- `1`: General application error or failed validation.
- `2`: Invalid CLI syntax or unknown flags.
- `130`: Process interrupted by user (SIGINT / Ctrl+C).
