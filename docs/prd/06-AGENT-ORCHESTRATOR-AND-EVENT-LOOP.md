# 06 - AGENT ORCHESTRATOR AND EVENT LOOP

## 1. Orchestration Philosophy
CITEOAI adopts an event-driven autonomous execution loop. The agent operates in cycles of:
1. **Context Construction**: Assemble system prompts, skills, relevant memory notes, conversation history, and tool declarations.
2. **Model Invocation**: Stream reasoning tokens and tool-call proposals from the LLM provider.
3. **Tool Execution Gate**: Evaluate security policies (approval required vs auto-approved).
4. **Execution & Feedback**: Execute tool in sandbox and append tool-response messages back into the conversation context.
5. **Convergence / Termination**: Complete turn when the model emits final text without further tool calls.

## 2. Event Loop Lifecycle
```
User Prompt
    │
    ▼
[Context Assembly] ───► Memory + Skills + History
    │
    ▼
[Provider Stream] ─────► Token Stream (TUI renders live)
    │
    ├── Has Tool Calls?
    │       │
    │      YES ──► [Permission Gate] ──► [Sandbox Execution] ──┐
    │       │                                                  │
    │       └──────────────── Append Result ◄──────────────────┘
    │                               │
    │                               ▼
    │                     Loop back to Provider
    │
    NO ─────► [Turn Complete] ──► [Save State] ──► [Update FinOps]
```

## 3. Loop Guard & Infinite Recursion Prevention
- **Max Consecutive Tool Calls**: Default 25 turns per single user command.
- **Duplicate Tool Detection**: If the exact same tool and arguments are executed 3 times with identical errors, the loop breaks with an explicit request for human guidance.
- **Context Compaction**: When conversation history exceeds 80% of model context window, older tool results are truncated to their summary header.
