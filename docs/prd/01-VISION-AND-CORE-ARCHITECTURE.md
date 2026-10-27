# 01 - VISION AND CORE ARCHITECTURE

## 1. Vision Statement
CITEOAI is an autonomous, ultra-lightweight, high-autonomy AI agent and CLI orchestrator engineered in Go. It empowers developers and researchers with multi-agent orchestration, interactive terminal execution, persistent memory recall, tool sandboxing, and code synthesis while maintaining an exceptionally tiny memory footprint (strictly under 100 MB RAM at runtime, targeting 40-70 MB).

## 2. Core Architectural Principles
1. **Lightweight & Fast**: Static single-binary compilation (`CGO_ENABLED=0`), zero garbage collection thrashing, low allocation streaming pipelines.
2. **CLI-First Paradigm**: No heavy external daemon or remote server dependencies required for core functionality. Full power in the terminal.
3. **Modular Package Topology**: Clean separation of concerns into small, single-responsibility Go packages.
4. **Resilient LLM Routing**: Native streaming support for Gemini (3.6 Flash, 3.8 Flash), OpenAI-compatible gateways, and local Ollama nodes with intelligent fallback.
5. **Self-Documenting Codebase**: Zero inline comments (`nokomen`). Pure code clarity via semantic variable and function naming.

## 3. Package Topology Map
```
cmd/
  citeoai/
    main.go                   # Single binary entry point and CLI dispatcher
internal/
  agent/                      # Core agent runtime and ADK orchestrator
  agentrun/                   # Execution loop, concurrency management, cancellation
  auth/                       # Local Argon2id password management and JWT
  cli/                        # Subcommand handlers (init, doctor, session, etc.)
  config/                     # Hierarchical JSON and environment variable loader
  context/                    # Context compaction and instruction builders
  finops/                     # Token usage tracker and cost estimation ledger
  hooks/                      # Pre/post tool execution hooks and slash command handlers
  interfaces/                 # Domain interfaces (Provider, Tool, Store, Logger)
  memory/                     # Persistent memory notes journal and semantic search
  permissions/                # Interactive approval gates and security boundary
  sandbox/                    # Path confinement and command jail
  session/                    # Conversation session store and durable snapshot engine
  skill/                      # Markdown skill discovery, parsing, and execution
  tracing/                    # Minimalist OTLP tracing hooks
  tui/                        # Charmbracelet Bubble Tea split-pane interface
  util/                       # Zero-allocation utilities, string algorithms, file ops
  version/                    # Build metadata and version stamping
```

## 4. Target Performance Metrics
- **Binary Size**: < 30 MB (stripped).
- **Idle Memory (RSS)**: < 35 MB.
- **Active Agent Execution RSS**: < 75 MB.
- **Peak Load RSS**: Strictly < 100 MB.
- **Cold Startup Time**: < 80 milliseconds.
