# 02 - SYSTEM STANDARDS AND CONSTRAINTS

## 1. Zero-Comment Code Standard (`nokomen`)
All source code files in CITEOAI (`.go`, `.ts`, `.vue`, `.js`, `.css`, `.json`) must contain **ZERO COMMENTS**.
- Forbidden: `//`, `/* ... */`, docstrings, JSDoc, section dividers, TODO/FIXME markers, AI watermarks.
- Requirements:
  - Meaningful, self-explanatory function names (`LoadUserConfiguration` instead of `Load`).
  - Clear boolean variables (`isSessionActive`, `hasValidApiKey`).
  - Small, single-purpose functions.
  - Whitespace used logically for separation instead of comment headers.

## 2. Ultra-Minimalist UI/UX Text Standard (`ui-ux-text`)
For any terminal outputs, prompts, and future web UI:
- **Maximum 3 words per label**: Buttons, titles, headers, badges, and statuses must never exceed 3 words.
  - Valid: `Save`, `Run task`, `Active agent`, `Connecting...`, `Token ready`.
  - Invalid: `Click here to save your settings`, `Manage your personal account preferences`.
- **Zero AI Filler**: Never output patronizing text or conversational bloat.
- **Progressive Disclosure**: Use floating popovers `(!)` for deep technical details rather than long inline paragraphs.

## 3. Card UI Standard (`card_ui`)
When Web UI or rich graphical cards are rendered:
- **Radius**: Exactly 4px (`border-radius: 4px`). No pills or rounded-full.
- **Theme**: Light theme default:
  - Background: `#ffffff`
  - Text: `#172033`
  - Border: `#e4e7ec`
  - Accent: `#4f46e5`
- **Dual Shadows**:
  ```css
  box-shadow: 0 2px 8px rgb(16 24 40 / 8%), inset 0 1px 2px rgb(255 255 255 / 85%), inset 0 -1px 2px rgb(16 24 40 / 4%);
  ```
- **Accent styling**: Restrained to edge indicators (`border-inline-start: 3px solid var(--card-accent)`).

## 4. Environment & Secrets Management (`env-secrets-management`)
- Strictly zero hardcoded keys or passwords in repository files.
- `.env` must be in `.gitignore`.
- Provide `.env.example` with blank values.
- Runtime reads configuration from `~/.citeoai/config.json` with environment variable overrides (`CITEOAI_API_KEY`, etc.).
