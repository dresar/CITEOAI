# CITEOAI AGENT INSTRUCTION & SUPERVISOR SPECIFICATION

## Project Identity
- **Name**: CITEOAI
- **Mission**: High-autonomy, ultra-lightweight (<100 MB RAM runtime), modular AI agent engine and CLI orchestrator built in Go.
- **Root Directory**: `C:\Users\NCN0C\Pictures\CITEOAI`
- **Owner**: Eka Syarif Maulana (dresar)

---

## Non-Negotiable Operational Rules for AI Builders (Claude Code CLI / Aider / Agents)

### 1. Strict Zero-Comment Rule (`nokomen`)
- **ABSOLUTE BAN**: Do NOT include any single-line (`//`), multi-line (`/* */`), docstrings, or decorative comments in ANY Go, CSS, TypeScript, JavaScript, HTML, or configuration code files.
- The code must be 100% self-documenting through crystal-clear variable naming, focused function scopes, and modular architecture.
- Any comment found in code is considered a build violation and must be stripped immediately.

### 2. UI/UX Text Standard (`ui-ux-text`)
- **STRICT WORD LIMIT**: Every user-facing UI label, button, toast, or badge must NOT exceed 3 words (target: 1-2 words).
- Never use AI filler, conversational chatter, or verbose explanatory paragraphs in the UI.
- Use progressive disclosure and floating info hints `(!)` for deep technical details.

### 3. Card UI Specification (`card_ui`)
- Every card corner radius must be **exactly 4px** (`border-radius: 4px`).
- Theme: Clean light theme with white or near-white surfaces (`#ffffff`), subtle neutral borders (`#e4e7ec`), and dark readable text (`#172033`).
- Dual shadows: Both an outer shadow and a subtle inner shadow are mandatory:
  ```css
  box-shadow: 0 2px 8px rgb(16 24 40 / 8%), inset 0 1px 2px rgb(255 255 255 / 85%), inset 0 -1px 2px rgb(16 24 40 / 4%);
  ```
- Accent styling: Restrained to small borders/edges (`border-inline-start: 3px solid #4f46e5`).

### 4. Memory Footprint Mission (< 100 MB RAM)
- Target resident set size (RSS): Strictly below **100 MB** under full load (optimal target: 40-70 MB).
- Avoid heavy runtime reflections, runaway heap allocations, and unnecessary memory-hungry dependencies.
- Compile static Go binaries (`CGO_ENABLED=0`, `-ldflags "-s -w"`).

### 5. CLI First (No Remote Yet)
- Focus 100% on native CLI commands, split-pane TUI (Bubble Tea), and command-line orchestration first before remote servers.

### 6. Secrets & Environment Management (`env-secrets-management`)
- NEVER hardcode API keys, tokens, or credentials into source files.
- All secrets are resolved via environment variables (`CITEOAI_API_KEY`, `CITEOAI_PROVIDER`) or `~/.citeoai/config.json`.
- `.env` files are strictly gitignored. Only `.env.example` is committed.

### 7. Documentation Roadmap
- Read and follow all 20 PRD specification files located in `docs/prd/` in numerical order from `01` to `20`.
