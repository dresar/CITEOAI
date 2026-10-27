# 10 - FINOPS LEDGER AND TOKEN ACCOUNTING

## 1. Accounting Objective
CITEOAI tracks every model call, prompt token, completion token, and cost in a zero-dependency append-only ledger (`~/.citeoai/finops.jsonl`).

## 2. Ledger Record Schema
```json
{
  "timestamp": "2026-10-27T14:40:00Z",
  "session_id": "sess_123",
  "provider": "gemini",
  "model": "gemini-3.6-flash",
  "prompt_tokens": 1420,
  "completion_tokens": 312,
  "total_tokens": 1732,
  "estimated_cost_usd": 0.00035,
  "duration_ms": 840
}
```

## 3. Real-Time Budget Guards
- Configurable soft and hard limits:
  - `budget_session_usd`: Warning displayed in TUI when cost exceeds threshold.
  - `budget_daily_usd`: Halts execution and requires `--force` flag.

## 4. CLI Reporting (`citeoai stats`)
Outputs a clean ASCII table summary:
- Total sessions executed.
- Token consumption breakdown per provider and model.
- Total expenditure in USD.
