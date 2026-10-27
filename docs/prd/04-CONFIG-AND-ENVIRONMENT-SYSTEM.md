# 04 - CONFIG AND ENVIRONMENT SYSTEM

## 1. Configuration Resolution Order
1. CLI command-line flags (`--config`, `--model`, `--provider`).
2. Environment variables (`CITEOAI_*`).
3. Local directory configuration (`./citeoai.json` or `./config.json`).
4. User home directory configuration (`~/.citeoai/config.json`).
5. Built-in defaults.

## 2. Directory Structure (`~/.citeoai`)
```
~/.citeoai/
├── config.json               # Master configuration file (0600 permissions)
├── credentials.json          # Admin Argon2id authentication credentials
├── sessions/                 # Serialized session state JSON files
├── memory/                   # Notes journal (notes.jsonl)
├── skills/                   # User-defined custom markdown skills
└── logs/                     # Truncated audit and execution traces
```

## 3. Configuration Schema (`config.json`)
```json
{
  "provider": "gemini",
  "model_name": "gemini-3.6-flash",
  "api_key": "",
  "base_url": "",
  "system_instruction": "You are CITEOAI, an ultra-fast, autonomous coding and research agent.",
  "max_tokens": 8192,
  "temperature": 0.7,
  "sandbox": {
    "mode": "restricted",
    "allowed_paths": [
      "."
    ],
    "denied_paths": [
      "~/.ssh",
      "~/.gnupg",
      "~/.citeoai/credentials.json"
    ]
  },
  "permissions": {
    "auto_approve_readonly": true,
    "require_approval_destructive": true
  },
  "memory": {
    "max_notes": 100,
    "max_prompt_chars": 3000
  }
}
```

## 4. Environment Variables Specification
- `CITEOAI_HOME`: Override the home directory path.
- `CITEOAI_PROVIDER`: Default LLM provider (`gemini`, `openai-compatible`, `ollama`).
- `CITEOAI_MODEL`: Model identifier (e.g. `gemini-3.6-flash`, `llama3.3`).
- `CITEOAI_API_KEY`: API key for the chosen provider.
- `CITEOAI_BASE_URL`: Custom base URL endpoint.
- `CITEOAI_SANDBOX_MODE`: `restricted` | `open` | `dry-run`.

## 5. UTF-8 BOM Handling
The JSON loader must automatically strip any UTF-8 Byte Order Mark (`\ufeff`) prior to unmarshaling, avoiding common cross-platform PowerShell encoding issues.
