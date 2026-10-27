# 08 - SKILL ENGINE AND EXTENSIBILITY

## 1. Skill Discovery Roots
CITEOAI dynamically discovers skills in the following locations:
1. Workspace root: `./.citeoai/skills/*/SKILL.md` or `./skills/*/SKILL.md`
2. User home: `~/.citeoai/skills/*/SKILL.md`
3. Antigravity/Claude shared compatibility paths: `~/.claude/skills/*/SKILL.md`

## 2. Skill Schema (`SKILL.md`)
```markdown
---
name: go-build-verification
description: Automatically verifies and compiles Go packages with race detector and vet.
triggers:
  - build
  - compile
  - test
---

# Go Build Verification Skill

## Execution Rules
Run `go vet ./...` and `go test -race ./...` before asserting task completion.
```

## 3. Skill Lifecycle & Sandboxing
- **Parser**: Lightweight YAML frontmatter extractor that avoids external reflection libraries.
- **Validation**:
  - Name: lowercase alphanumeric with single hyphens (`^[a-z0-9]+(-[a-z0-9]+)*$`).
  - Description: strictly < 1024 characters.
  - Body: recommended < 500 lines.
- **Dynamic Skill Injection**: Only skills matching the active user prompt or session context are injected into the model prompt to minimize token burn.
