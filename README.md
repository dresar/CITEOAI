# CITEOAI

> High-autonomy, ultra-lightweight (<100 MB RAM) AI agent engine and CLI orchestrator built in Go.

## Key Features
- **Ultra-Lightweight**: Target resident set size < 100 MB RAM (optimal 40-70 MB).
- **CLI-First**: Native terminal orchestration, Bubble Tea split-pane TUI.
- **Provider Agnostic**: Native support for Gemini 3.6/3.8 Flash, OpenAI-compatible APIs, and local Ollama nodes.
- **Self-Documenting Codebase**: 100% Zero-comment standard (`nokomen`).
- **Sandboxed Execution**: Path confinement, safe command execution, and interactive permission gates.
- **Persistent Memory**: Low-overhead append-only journal with BM25 keyword recall.
- **Extensible Skills**: Markdown-based skill engine (`SKILL.md`).

## Architectural Specification & PRD
The full design and implementation blueprint is documented in 20 sequential PRD files inside `docs/prd/`:
- `01-VISION-AND-CORE-ARCHITECTURE.md`
- `02-SYSTEM-STANDARDS-AND-CONSTRAINTS.md`
- `03-CLI-SPECIFICATION-AND-COMMAND-TREE.md`
- `04-CONFIG-AND-ENVIRONMENT-SYSTEM.md`
- `05-LLM-PROVIDER-ENGINE.md`
- `06-AGENT-ORCHESTRATOR-AND-EVENT-LOOP.md`
- `07-MEMORY-AND-PERSISTENCE-SYSTEM.md`
- `08-SKILL-ENGINE-AND-EXTENSIBILITY.md`
- `09-TOOL-REGISTRY-AND-SANDBOXING.md`
- `10-FINOPS-LEDGER-AND-TOKEN-ACCOUNTING.md`
- `11-TUI-SPECIFICATION-BUBBLETEA.md`
- `12-SESSION-MANAGER-AND-DURABLE-RESUME.md`
- `13-SECURITY-PERMISSIONS-AND-APPROVAL-GATES.md`
- `14-HOOKS-AND-LIFECYCLE-PIPELINE.md`
- `15-UI-UX-CARD-DESIGN-SYSTEM.md`
- `16-BENCHMARK-AND-MEMORY-PROFILING.md`
- `17-VERSIONING-AND-RELEASE-ENGINEERING.md`
- `18-IMPLEMENTATION-PROMPT-TEMPLATES.md`
- `19-TESTING-AND-VERIFICATION-SUITE.md`
- `20-MASTER-EXECUTION-ROADMAP.md`

## Instructions for AI Agents
Autonomous agents (Claude Code CLI, Aider, etc.) building this project must strictly consult `AGENTS.md` and `CLAUDE.md`.
