# 09 - TOOL REGISTRY AND SANDBOXING

## 1. Built-in Core Tools
CITEOAI ships with a focused set of low-overhead native tools:

1. `read_file`:
   - Arguments: `path` (string), `start_line` (int), `end_line` (int).
   - Bounds: Caps individual file reads to 50KB or 800 lines to prevent memory exhaustion.

2. `write_file`:
   - Arguments: `path` (string), `content` (string), `overwrite` (bool).
   - Atomic writes via temporary files and rename operations.

3. `edit_file`:
   - Arguments: `path` (string), `target` (string), `replacement` (string).
   - Exact string substitution with whitespace preservation.

4. `run_command`:
   - Arguments: `command` (string), `timeout_seconds` (int), `is_background` (bool).
   - Executes via platform shell (`powershell.exe` on Windows, `/bin/sh` on Unix).
   - Streaming stdout/stderr reader with rolling circular buffer (max 64KB log).

5. `list_directory`:
   - Arguments: `path` (string), `recursive` (bool), `max_depth` (int).
   - Filters out `.git`, `node_modules`, `vendor` automatically.

## 2. Sandboxing & Path Confinement
- **Allowed Scope**: Defaults to current working directory and subdirectories.
- **Strictly Denied Paths**:
  - Windows: `C:\Windows\System32\config`, `$HOME\.ssh`, `$HOME\.gnupg`, `$HOME\.citeoai\credentials.json`
  - Linux: `/etc/shadow`, `/root`, `~/.ssh`
- Path traversal attacks (`../..`) are intercepted via canonical filepath secure-join algorithms before execution.
