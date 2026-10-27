# 18 - IMPLEMENTATION PROMPT TEMPLATES FOR AI BUILDERS

## 1. Master Supervisor Prompt Template
When delegating module creation to Claude Code CLI, Aider, or subagents, use this standardized prompt:

```text
You are an elite systems engineer implementing the CITEOAI autonomous engine.
Read AGENTS.md, CLAUDE.md, and docs/prd/ before modifying code.

CRITICAL INVARIANTS:
1. STRICT ZERO COMMENTS (nokomen): Never write any single-line (//) or multi-line (/* */) comments. The code must be 100% self-documenting.
2. ULTRA-LIGHTWEIGHT: Keep memory footprint strictly under 100MB RAM. Avoid excessive heap allocations.
3. MODULAR ARCHITECTURE: Create small, focused packages in internal/.
4. NO HARDCODED SECRETS: All credentials must be read from environment variables or ~/.citeoai/config.json.

TASK:
[Describe specific package to build, e.g., Implement internal/config with BOM stripping]
```

## 2. Verification Protocol After Every Agent Step
1. Check for comments: Run `git diff` and verify zero comment tokens exist in added lines.
2. Verify build: Run `go vet ./...` and `go test ./...`.
3. Verify memory: Ensure test allocations are under target limits.
