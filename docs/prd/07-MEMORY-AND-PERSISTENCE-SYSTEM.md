# 07 - MEMORY AND PERSISTENCE SYSTEM

## 1. Storage Architecture
CITEOAI avoids heavy vector databases by employing an ultra-fast, append-only JSONL journal backed by an in-memory inverted index and BM25/keyword scoring.

- **Storage Location**: `~/.citeoai/memory/notes.jsonl`
- **Memory Overhead**: < 5 MB RAM for 10,000 indexed memory notes.

## 2. Note Schema
```json
{
  "id": "mem_01HQX78ABZ...",
  "created_at": "2026-10-27T14:30:00Z",
  "updated_at": "2026-10-27T14:30:00Z",
  "category": "preference",
  "tags": ["go", "formatting", "nokomen"],
  "content": "User prefers zero comments in all generated Go code.",
  "source_session_id": "sess_default"
}
```

## 3. Operations
1. `AddNote(category, tags, content)`: Appends to journal and updates memory index.
2. `SearchNotes(query, limit)`: Ranks notes using token overlap and recency decay.
3. `ForgetNote(id)`: Marks note as tombstoned in the journal without rewriting entire file.
4. `CompactJournal()`: Purges tombstoned records during `citeoai doctor` or shutdown.

## 4. Context Injection Strategy
During turn assembly, top-K relevant notes (maximum 15 notes, capped at 3,000 characters) are injected into the agent's system instruction block:
```markdown
# Persistent Memories & User Preferences
- [preference]: User prefers zero comments in all generated Go code.
- [project]: Active workspace uses 4px border-radius on cards.
```
