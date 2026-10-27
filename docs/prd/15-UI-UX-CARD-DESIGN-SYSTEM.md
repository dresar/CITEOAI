# 15 - UI/UX CARD DESIGN SYSTEM

## 1. Scope
When future web interfaces, HTML dashboards, or visual sidecars are created for CITEOAI, they must strictly follow this design specification.

## 2. Card Design Rules (`card_ui`)
- **Strict 4px Corner Radius**: Every card corner radius must be **exactly 4px**. Never use 8px, 12px, 16px, 24px, or pill/capsule shapes on cards.
- **Light Theme Default**: White surfaces (`#ffffff`), subtle neutral borders (`#e4e7ec`), dark readable text (`#172033`).
- **Layered Dual Shadows**: Both outer and soft inner shadows are required:
  ```css
  :root {
    --card-bg: #ffffff;
    --card-text: #172033;
    --card-muted: #667085;
    --card-border: #e4e7ec;
    --card-accent: #4f46e5;
    --card-radius: 4px;
    --card-shadow:
      0 2px 8px rgb(16 24 40 / 8%),
      inset 0 1px 2px rgb(255 255 255 / 85%),
      inset 0 -1px 2px rgb(16 24 40 / 4%);
  }

  .card {
    box-sizing: border-box;
    border: 1px solid var(--card-border);
    border-radius: 4px;
    background: var(--card-bg);
    color: var(--card-text);
    box-shadow: var(--card-shadow);
    overflow: hidden;
  }

  .card--accent {
    border-inline-start: 3px solid var(--card-accent);
  }

  .card:hover {
    box-shadow:
      0 4px 12px rgb(16 24 40 / 11%),
      inset 0 1px 2px rgb(255 255 255 / 85%),
      inset 0 -1px 2px rgb(16 24 40 / 4%);
  }
  ```

## 3. Ultra-Minimal Copy Standard (`ui-ux-text`)
- All buttons, badges, labels, and titles must be **1 to 3 words maximum**.
- For technical parameters or guidance, use a compact circular exclamation icon `(!)` with a hover popover (maximum width 280px) that disappears automatically when the mouse leaves.
