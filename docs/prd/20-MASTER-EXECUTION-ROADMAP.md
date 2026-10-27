# 20 - MASTER EXECUTION ROADMAP

## Phase 1: Foundation & Scaffolding (Day 1)
- [x] Create project repository structure and `.gitignore`.
- [x] Author all 20 PRD documents, `AGENTS.md`, and `CLAUDE.md`.
- [ ] Initialize `go.mod` (Go 1.27 / 1.26).
- [ ] Implement `internal/version` build stamping.
- [ ] Implement `internal/config` with zero-BOM handling and env fallback.

## Phase 2: Core Tooling & Sandboxing (Day 2)
- [ ] Implement `internal/sandbox` path confinement.
- [ ] Implement `internal/tools`: `read_file`, `write_file`, `edit_file`, `run_command`, `list_directory`.
- [ ] Add unit tests for sandbox traversal prevention.

## Phase 3: LLM Provider Engine (Day 3)
- [ ] Implement `internal/provider/gemini` with SSE streaming decoder (`gemini-3.6-flash`).
- [ ] Implement `internal/provider/openai` for OpenAI-compatible gateways.
- [ ] Add mock streaming tests and error retry circuit breakers.

## Phase 4: Agent Orchestrator & Event Loop (Day 4)
- [ ] Implement `internal/agent` execution loop and context assembler.
- [ ] Implement loop guards (max 25 turns, duplicate call interceptor).
- [ ] Implement interactive permission gates for Tier 3 mutations.

## Phase 5: Memory Journal & Skills Engine (Day 5)
- [ ] Implement `internal/memory` append-only JSONL journal and BM25 search.
- [ ] Implement `internal/skill` markdown scanner and validator.

## Phase 6: Bubble Tea TUI (Day 6)
- [ ] Implement `internal/tui` split-pane layout (transcript on left, tool logs on right).
- [ ] Wire streaming token handler to Glamour markdown viewport.

## Phase 7: Memory Profiling & Optimization (<100MB) (Day 7)
- [ ] Run benchmark suite with memory profiler.
- [ ] Eliminate unnecessary allocations, verify RSS stays under 60-80 MB under heavy load.

## Phase 8: GitHub Deployment & Release
- [ ] Run build verification.
- [ ] Conventional commit & auto push to GitHub repository `dresar/CITEOAI`.
