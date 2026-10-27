# 13 - SECURITY PERMISSIONS AND APPROVAL GATES

## 1. Permission Matrix
CITEOAI categorizes all tool invocations into distinct risk tiers:

| Tier | Category | Examples | Default Action |
| :--- | :--- | :--- | :--- |
| **0** | Read-Only | `read_file`, `list_directory`, `inspect_code` | Auto-approved |
| **1** | Safe Mutation | `write_file` (within git tracked workspace) | Auto-approved (logged) |
| **2** | System Command | `go build`, `git status`, `npm test` | Auto-approved if in whitelist |
| **3** | High-Risk Mutation | `rm -rf`, `git reset --hard`, `drop table`, `curl | sh` | Interactive Prompt Required |
| **4** | Out-of-Bounds | Access to `~/.ssh`, `/etc/`, credentials | Strictly Denied |

## 2. Interactive Approval Prompt
When a Tier 3 tool call is requested:
```
[SECURITY GATE]
Tool: run_command
Command: git reset --hard HEAD~1
Risk: Destructive workspace mutation.

Allow execution? [y/N/always]: 
```
If rejected, the agent receives an immediate error string: `User rejected command execution: permission denied.`
