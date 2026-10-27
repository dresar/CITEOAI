# 14 - HOOKS AND LIFECYCLE PIPELINE

## 1. Lifecycle Hook Architecture
CITEOAI provides hooks for developers to inject automated actions around agent turns:

- `PreTurnHook`: Runs before user prompt is submitted to model. Allows secret redaction and prompt enrichment.
- `PreToolHook`: Runs before a tool executes. Allows static analysis and argument sanitization.
- `PostToolHook`: Runs after tool execution. Allows automatic linting or formatting.
- `PostTurnHook`: Runs after turn finishes. Triggers auto-commit, memory extraction, and telemetry.

## 2. Slash Commands
Built-in slash commands handled instantly without hitting the LLM:
- `/clear`: Clear terminal viewport.
- `/reset`: Start fresh conversation session.
- `/compact`: Force context window summarization.
- `/memory`: Inspect current active memory notes.
- `/stats`: Display current session FinOps metrics.
- `/help`: Display cheat sheet.
