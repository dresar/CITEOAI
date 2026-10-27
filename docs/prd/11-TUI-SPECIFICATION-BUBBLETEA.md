# 11 - TUI SPECIFICATION (BUBBLE TEA)

## 1. Interface Architecture
CITEOAI implements a high-performance terminal UI built with Charmbracelet Bubble Tea v2, Glamour (terminal Markdown rendering), and Lipgloss (styling).

## 2. Layout Structure
```
┌─────────────────────────────────── CITEOAI (v0.1.0) ───────────────────────────────────┐
│                                                                                       │
│  [LEFT PANE: Transcript & Conversation]           [RIGHT PANE: Tool Execution Logs]    │
│  User: build the auth handler                     > Running: go vet ./...              │
│                                                   > Exit: 0 (clean)                    │
│  CITEOAI: I will create the auth handler          > Running: go test -v ./...          │
│  using Argon2id with 0 comments.                  > PASS (12 tests)                    │
│                                                                                       │
│                                                                                       │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ Model: gemini-3.6-flash │ RAM: 58.2 MB │ Tokens: 1,420 │ Session: default             │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ > Enter prompt (Esc to cancel, Ctrl+C to quit)...                                     │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

## 3. Performance & Memory Budget in TUI
- Viewport lines capped to 2,000 in memory. Older lines are flushed to disk.
- Frame rate throttled to 60 FPS maximum during heavy streaming.
- Avoids terminal flickering by caching styled Lipgloss blocks.
- Total memory when TUI is running must remain under **60 MB RAM**.
