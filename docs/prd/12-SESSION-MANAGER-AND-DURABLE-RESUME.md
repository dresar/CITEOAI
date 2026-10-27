# 12 - SESSION MANAGER AND DURABLE RESUME

## 1. Objectives
Enable developers to pause, crash-recover, and resume long-running agent trajectories without loss of tool state or context.

## 2. Session Directory Structure
```
~/.citeoai/sessions/
├── active.id                        # Pointer to currently active session ID
├── sess_01HQX99AAA/
│   ├── metadata.json                # Created timestamp, model, total turns
│   ├── transcript.jsonl             # Sequential append-only event stream
│   └── snapshots/
│       ├── turn_001.json            # Periodic context snapshot for fast rollback
│       └── turn_005.json
```

## 3. Crash Recovery Protocol
On startup, CITEOAI inspects `active.id`. If a preceding run terminated abnormally (SIGKILL or crash), it reads the last valid snapshot from `transcript.jsonl` and prompts the user:
```
Session 'sess_01HQX99AAA' recovered from turn 14.
Resume? [Y/n]
```
Resume loads only the compacted state, keeping memory overhead minimal.
