# CLAUDE.md - Rules and Instructions for Claude Code CLI

See `AGENTS.md` and `docs/prd/` for complete architectural rules.

## Core Directives
1. **NO COMMENTS IN CODE (`nokomen`)**: Never write `//` or `/* */` comments in any `.go`, `.ts`, `.vue`, `.js`, or `.css` files. Code must be purely self-documenting.
2. **ULTRA-LIGHTWEIGHT (<100MB RAM)**: Optimize all Go structs, buffers, and goroutines to keep memory footprint under 100MB.
3. **CLI FIRST**: Focus on terminal CLI commands and Bubble Tea TUI.
4. **UI TEXT MAX 3 WORDS (`ui-ux-text`)**: Keep any UI copy within 1 to 3 words max.
5. **CARD UI 4PX RADIUS (`card_ui`)**: When building Web UI components, strict 4px radius, light mode, dual shadows.
6. **NO HARDCODED SECRETS**: Read all keys from environment variables or `~/.citeoai/config.json`.
7. **20 PRD FILES**: Consult `docs/prd/01` through `docs/prd/20` before creating or editing modules.
