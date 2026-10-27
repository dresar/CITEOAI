# CITEOAI MULTI-AGENT ARCHITECTURE AND CONTEXT CONTRACT

## 1. Non-Negotiable Invariants

1. **STRICT ZERO COMMENTS (`nokomen`)**:
   - Every single `.go` source file MUST contain **ZERO COMMENTS**.
   - No single-line comments (`//`), no multi-line comments (`/* */`), no docstrings, no section dividers, no commented-out code.
   - Code must be 100% self-documenting through precise semantic naming and concise function logic.
   - Any comment token detected in any `.go` file is an immediate build failure.

2. **MEMORY CAP: STRICTLY UNDER 100 MB RAM**:
   - Streaming decoders (SSE) must read line-by-line using bounded buffers (`bufio.Scanner` or `bufio.Reader`).
   - No unbounded in-memory file buffering. Read files strictly capped at 50 KB / 800 lines.
   - Tool execution circular logs capped at 64 KB.
   - Keep heap allocations minimal.

3. **CLI-FIRST & NATIVE GO**:
   - Pure native Go standard library where possible, plus Charmbracelet Bubble Tea & Lipgloss for TUI.
   - Zero heavy remote server dependencies.

4. **UI TEXT MAX 3 WORDS (`ui-ux-text`)**:
   - Any user-facing terminal status, prompt, error label, or badge must NOT exceed 3 words (target: 1-2 words).

5. **CARD UI SPECIFICATION (`card_ui`)**:
   - Terminal & graphic styling: border-radius 4px (`RoundedBorder()` in Lipgloss), light theme palette, 1px solid borders, clean contrast.

6. **ZERO HARDCODED CREDENTIALS (`env-secrets-management`)**:
   - Resolve credentials strictly via `config.Config` loaded from `~/.citeoai/config.json` with environment variable overrides (`CITEOAI_API_KEY`, `CITEOAI_PROVIDER`, etc.).

---

## 2. Package Topology & File Boundaries

### A. AGY02: Backend Core & Sandbox Owner
- **Worktree**: `/opt/data/home/CITEOAI-agy02`
- **Branch**: `work/agy02-core-tools-memory`
- **Files Owned**:
  - `internal/interfaces/tool.go`
  - `internal/sandbox/sandbox.go`
  - `internal/sandbox/sandbox_test.go`
  - `internal/tools/tools.go`
  - `internal/tools/tools_test.go`
  - `internal/memory/store.go`
  - `internal/memory/store_test.go`
  - `internal/skill/discover.go`
  - `internal/skill/discover_test.go`

### B. AGY03: LLM Streaming Providers Owner
- **Worktree**: `/opt/data/home/CITEOAI-agy03`
- **Branch**: `work/agy03-providers`
- **Files Owned**:
  - `internal/provider/gemini/gemini.go`
  - `internal/provider/gemini/gemini_test.go`
  - `internal/provider/openai/openai.go`
  - `internal/provider/openai/openai_test.go`

### C. AGY04: Terminal UI (Bubble Tea & Lipgloss) Owner
- **Worktree**: `/opt/data/home/CITEOAI-agy04`
- **Branch**: `work/agy04-tui`
- **Files Owned**:
  - `internal/tui/tui.go`
  - `internal/tui/styles.go`

### D. AGY05: Agent Orchestrator & CLI Main Integration Owner
- **Worktree**: `/opt/data/home/CITEOAI-agy05`
- **Branch**: `work/agy05-agent-cli`
- **Files Owned**:
  - `internal/agent/orchestrator.go`
  - `internal/agent/orchestrator_test.go`
  - `cmd/citeoai/main.go`
  - `tests/nokomen_test.go`

---

## 3. Detailed Struct and Interface Contracts

### 3.1 `internal/interfaces/tool.go`
```go
package interfaces

import "context"

type Tool interface {
	Name() string
	Description() string
	Definition() ToolDefinition
	Execute(ctx context.Context, argsJSON string) (string, error)
}
```

### 3.2 `internal/sandbox/sandbox.go`
- `type Sandbox struct` with `allowedPaths []string`, `deniedPaths []string`, `workspaceRoot string`
- `func NewSandbox(workspaceRoot string, allowedPaths, deniedPaths []string) *Sandbox`
- `func (s *Sandbox) ResolvePath(inputPath string) (string, error)`
- Validates canonical paths (`filepath.Clean`, `filepath.Abs`).
- Rejects directory traversal outside allowed roots (`ErrAccessDenied = errors.New("access denied")`).
- Blocks denied paths (e.g., `~/.ssh`, `/etc/shadow`, credentials).

### 3.3 `internal/tools/tools.go`
- `read_file`: args `{"path": "string", "start_line": int, "end_line": int}`. Cap output at 50KB or 800 lines.
- `write_file`: args `{"path": "string", "content": "string", "overwrite": bool}`. Atomic write via temporary file + rename.
- `edit_file`: args `{"path": "string", "target": "string", "replacement": "string"}`. Exact substring match and replacement.
- `run_command`: args `{"command": "string", "timeout_seconds": int, "is_background": bool}`. Platform shell (`/bin/sh -c` on linux, `powershell.exe -Command` on windows), rolling circular buffer up to 64KB log.
- `list_directory`: args `{"path": "string", "recursive": bool, "max_depth": int}`. Filters `.git`, `node_modules`, `vendor`.
- `type ToolRegistry struct` with `Register(t interfaces.Tool)`, `Get(name string) (interfaces.Tool, bool)`, `Definitions() []interfaces.ToolDefinition`, `Execute(ctx context.Context, name, argsJSON string) (string, error)`.
- `func NewDefaultRegistry(sb *sandbox.Sandbox) *ToolRegistry`.

### 3.4 `internal/memory/store.go`
- `type MemoryNote struct` with `ID`, `CreatedAt`, `UpdatedAt`, `Category`, `Tags`, `Content`, `SourceSessionID`, `Tombstoned`
- `type MemoryStore struct` backed by JSONL file (`~/.citeoai/memory/notes.jsonl`).
- `func Open(path string) (*MemoryStore, error)`
- `func (m *MemoryStore) AddNote(category string, tags []string, content, sessionID string) (*MemoryNote, error)`
- `func (m *MemoryStore) Search(query string, limit int) []MemoryNote` (BM25 or token-overlap scoring).
- `func (m *MemoryStore) Forget(id string) error` (tombstone marker).
- `func (m *MemoryStore) List() []MemoryNote`
- `func (m *MemoryStore) Compact() error`

### 3.5 `internal/skill/discover.go`
- `type Skill struct` with `Name`, `Description`, `Triggers []string`, `Body string`, `Path string`
- `func DiscoverSkills(roots []string) ([]Skill, error)`
- `func ParseSkillFile(filePath string) (*Skill, error)` (lightweight YAML frontmatter parser between `---`).
- Validates name format `^[a-z0-9]+(-[a-z0-9]+)*$`.
- `func MatchSkills(skills []Skill, query string) []Skill`

### 3.6 `internal/provider/gemini/gemini.go`
- Implements `interfaces.Provider` for Google Generative Language API.
- Endpoint: `https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse&key={apiKey}`
- Supports `gemini-3.6-flash`, `gemini-3.8-flash`, etc.
- Parses SSE stream chunks `data: {...}` line-by-line with bounded buffer.
- Emits `interfaces.StreamChunk` with `DeltaText` and function call invocations.

### 3.7 `internal/provider/openai/openai.go`
- Implements `interfaces.Provider` for OpenAI-compatible APIs and local Ollama (`/chat/completions`).
- Parses SSE `data: {"choices":[{"delta":{"content":...}}]}`.

### 3.8 `internal/agent/orchestrator.go`
- `type AgentOrchestrator struct`
- `func NewOrchestrator(cfg *config.Config, prov interfaces.Provider, tools *tools.ToolRegistry, mem *memory.MemoryStore, skills []skill.Skill) *AgentOrchestrator`
- Execution cycle:
  1. Assemble context: system instruction, matched skills, relevant memory notes, conversation history, tool definitions.
  2. Provider stream invocation.
  3. Interactive permission check for Tier 3 destructive tools (`rm -rf`, `git reset --hard`, `curl | sh`).
  4. Tool dispatch and result appending.
  5. Recursion guard: max 25 turns per command, duplicate tool call interceptor (3 identical calls with identical errors).

### 3.9 `internal/tui/tui.go`
- Split-pane terminal UI using Bubble Tea & Lipgloss:
  - Left pane: transcript & live token stream.
  - Right pane: tool execution logs.
  - Bottom bar: status (Model, RAM, Tokens, Session) + prompt input.
  - Strictly follows UI text rule: status badges <= 3 words (e.g., `Active`, `Token ready`, `Executing`).
  - Strict 4px rounded borders via Lipgloss `RoundedBorder()`.

### 3.10 `cmd/citeoai/main.go`
- Wire all subcommands:
  - `citeoai` -> launches interactive Bubble Tea TUI
  - `citeoai run "<prompt>"` -> runs headless execution
  - `citeoai doctor` -> diagnostics check
  - `citeoai memory list / add / forget` -> memory CLI commands
  - `citeoai skill list` -> skills CLI command
  - `citeoai version` -> prints version
