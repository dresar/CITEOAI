# 17 - VERSIONING AND RELEASE ENGINEERING

## 1. Semantic Versioning Protocol
CITEOAI follows strict semantic versioning (`vMAJOR.MINOR.PATCH`):
- `v0.1.0`: Core CLI foundation, config loader, Gemini 3.6 Flash provider, native tools, zero-comment enforcement.
- `v0.2.0`: Bubble Tea TUI split-pane interface, interactive approval gates.
- `v0.3.0`: Persistent memory journal, BM25 keyword recall.
- `v0.4.0`: Dynamic markdown skills engine (`SKILL.md`).
- `v0.5.0`: Durable session snapshots and rollback.
- `v1.0.0`: Production-ready release, fully verified under 100 MB RAM benchmark.

## 2. Build Stamping
Build metadata is injected via `-ldflags` during compilation:
```bash
go build -ldflags "-s -w \
  -X citeoai/internal/version.Version=v0.1.0 \
  -X citeoai/internal/version.Commit=$(git rev-parse --short HEAD) \
  -X citeoai/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o citeoai.exe ./cmd/citeoai/
```

## 3. Changelog Maintenance
Every feature, fix, or performance optimization must be recorded in `CHANGELOG.md` categorized under:
- `Added`
- `Changed`
- `Fixed`
- `Security`
- `Performance`
