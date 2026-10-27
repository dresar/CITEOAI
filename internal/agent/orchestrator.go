package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"citeoai/internal/config"
	"citeoai/internal/interfaces"
	"citeoai/internal/memory"
	"citeoai/internal/skill"
	"citeoai/internal/tools"
)

var (
	ErrMaxTurnsExceeded     = errors.New("maximum turn limit reached (25)")
	ErrDuplicateToolFailure = errors.New("duplicate tool failure loop detected")
	ErrNilProvider          = errors.New("provider cannot be nil")
)

const (
	MaxTurnsLimit          = 25
	MaxDuplicateErrorCount = 3
	MaxMemoryNotesLimit    = 15
	DefaultMaxMemoryChars  = 3000
)

type SecurityApprover func(toolName, argsJSON string) bool

type AgentOrchestrator struct {
	cfg              *config.Config
	prov             interfaces.Provider
	tools            *tools.ToolRegistry
	mem              *memory.MemoryStore
	skills           []skill.Skill
	securityApprover SecurityApprover
}

func NewOrchestrator(cfg *config.Config, prov interfaces.Provider, tools *tools.ToolRegistry, mem *memory.MemoryStore, skills []skill.Skill) *AgentOrchestrator {
	return &AgentOrchestrator{
		cfg:    cfg,
		prov:   prov,
		tools:  tools,
		mem:    mem,
		skills: skills,
	}
}

func (o *AgentOrchestrator) SetSecurityApprover(approver SecurityApprover) {
	o.securityApprover = approver
}

func (o *AgentOrchestrator) Config() *config.Config {
	return o.cfg
}

func (o *AgentOrchestrator) Provider() interfaces.Provider {
	return o.prov
}

func (o *AgentOrchestrator) Tools() *tools.ToolRegistry {
	return o.tools
}

func (o *AgentOrchestrator) Memory() *memory.MemoryStore {
	return o.mem
}

func (o *AgentOrchestrator) Skills() []skill.Skill {
	return o.skills
}

func (o *AgentOrchestrator) assembleSystemInstruction(prompt string) string {
	base := ""
	if o.cfg != nil && o.cfg.SystemInstruction != "" {
		base = o.cfg.SystemInstruction
	} else {
		base = "You are CITEOAI, an ultra-fast, autonomous coding and research agent."
	}

	var sb strings.Builder
	sb.WriteString(base)

	if o.mem != nil {
		maxNotes := MaxMemoryNotesLimit
		maxChars := DefaultMaxMemoryChars
		if o.cfg != nil {
			if o.cfg.Memory.MaxNotes > 0 {
				maxNotes = o.cfg.Memory.MaxNotes
			}
			if o.cfg.Memory.MaxPromptChars > 0 {
				maxChars = o.cfg.Memory.MaxPromptChars
			}
		}

		notes := o.mem.Search(prompt, maxNotes)
		if len(notes) > 0 {
			sb.WriteString("\n\n# Persistent Memories & User Preferences\n")
			currentChars := 0
			for _, note := range notes {
				entry := fmt.Sprintf("- [%s]: %s\n", note.Category, note.Content)
				if currentChars+len(entry) > maxChars {
					break
				}
				sb.WriteString(entry)
				currentChars += len(entry)
			}
		}
	}

	if len(o.skills) > 0 {
		matched := skill.MatchSkills(o.skills, prompt)
		if len(matched) > 0 {
			sb.WriteString("\n\n# Active Skills\n")
			for _, s := range matched {
				sb.WriteString(fmt.Sprintf("## Skill: %s\n%s\n\n", s.Name, s.Body))
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

func isTier3Command(toolName, argsJSON string) (bool, string) {
	if toolName != "run_command" {
		return false, ""
	}

	var parsed struct {
		Command string `json:"command"`
	}
	cmd := argsJSON
	if err := json.Unmarshal([]byte(argsJSON), &parsed); err == nil && parsed.Command != "" {
		cmd = parsed.Command
	}

	cmdLower := strings.ToLower(strings.TrimSpace(cmd))

	dangerousSubstrings := []struct {
		pattern string
		reason  string
	}{
		{"rm -rf", "recursive file deletion"},
		{"rm -r ", "recursive file deletion"},
		{"rm -fr", "recursive file deletion"},
		{"git reset --hard", "destructive git reset"},
		{"git clean -f", "destructive git clean"},
		{"drop table", "destructive database table drop"},
		{"drop database", "destructive database drop"},
		{"mkfs", "disk format operation"},
		{"dd if=", "raw disk write operation"},
		{":(){ :|:& };:", "fork bomb"},
		{"shutdown", "system shutdown"},
		{"reboot", "system reboot"},
		{"init 0", "system halt"},
	}

	for _, d := range dangerousSubstrings {
		if strings.Contains(cmdLower, d.pattern) {
			return true, d.reason
		}
	}

	if (strings.Contains(cmdLower, "curl ") || strings.Contains(cmdLower, "wget ")) &&
		(strings.Contains(cmdLower, "| sh") || strings.Contains(cmdLower, "| bash") || strings.Contains(cmdLower, "|sh") || strings.Contains(cmdLower, "|bash")) {
		return true, "remote script piping to shell"
	}

	return false, ""
}

func (o *AgentOrchestrator) RunTurn(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error) {
	if o.prov == nil {
		return "", ErrNilProvider
	}

	systemInstruction := o.assembleSystemInstruction(prompt)

	messages := []interfaces.Message{
		{
			Role: interfaces.RoleSystem,
			Parts: []interfaces.ContentPart{
				{Text: systemInstruction},
			},
		},
		{
			Role: interfaces.RoleUser,
			Parts: []interfaces.ContentPart{
				{Text: prompt},
			},
		},
	}

	var toolDefs []interfaces.ToolDefinition
	if o.tools != nil {
		toolDefs = o.tools.Definitions()
	}

	var accumulatedFinalText strings.Builder
	var lastFailedTool string
	var lastFailedArgs string
	var lastFailedErr string
	failedCount := 0

	for turn := 0; turn < MaxTurnsLimit; turn++ {
		select {
		case <-ctx.Done():
			return accumulatedFinalText.String(), ctx.Err()
		default:
		}

		ch, err := o.prov.GenerateStream(ctx, messages, toolDefs)
		if err != nil {
			return accumulatedFinalText.String(), err
		}

		var turnText strings.Builder
		toolCallsMap := make(map[string]*interfaces.ContentPart)
		var callOrder []string
		var currentCallID string

		for chunk := range ch {
			if chunk.Error != nil {
				return accumulatedFinalText.String(), chunk.Error
			}

			if chunk.DeltaText != "" {
				turnText.WriteString(chunk.DeltaText)
				accumulatedFinalText.WriteString(chunk.DeltaText)
				if streamCallback != nil {
					streamCallback(chunk.DeltaText)
				}
			}

			if chunk.ToolCallID != "" || chunk.ToolName != "" || chunk.ToolArgsJSON != "" {
				id := chunk.ToolCallID
				if id != "" {
					currentCallID = id
				} else if currentCallID != "" {
					id = currentCallID
				} else {
					id = chunk.ToolName
					currentCallID = id
				}

				part, exists := toolCallsMap[id]
				if !exists {
					part = &interfaces.ContentPart{
						ToolCallID: id,
						ToolName:   chunk.ToolName,
					}
					toolCallsMap[id] = part
					callOrder = append(callOrder, id)
				}
				if chunk.ToolName != "" {
					part.ToolName = chunk.ToolName
				}
				part.ToolArgs += chunk.ToolArgsJSON
			}
		}

		if len(callOrder) == 0 {
			return accumulatedFinalText.String(), nil
		}

		var assistantParts []interfaces.ContentPart
		if turnText.Len() > 0 {
			assistantParts = append(assistantParts, interfaces.ContentPart{Text: turnText.String()})
		}
		for _, id := range callOrder {
			assistantParts = append(assistantParts, *toolCallsMap[id])
		}
		messages = append(messages, interfaces.Message{
			Role:  interfaces.RoleAssistant,
			Parts: assistantParts,
		})

		for _, id := range callOrder {
			call := toolCallsMap[id]
			toolName := call.ToolName
			toolArgs := call.ToolArgs
			toolCallID := call.ToolCallID

			if logCallback != nil {
				logCallback(fmt.Sprintf("tool request: %s", toolName))
			}

			isTier3, reason := isTier3Command(toolName, toolArgs)
			var toolResult string
			var executionErr error

			if isTier3 {
				allowed := false
				if o.securityApprover != nil {
					allowed = o.securityApprover(toolName, toolArgs)
				}
				if !allowed {
					if logCallback != nil {
						logCallback(fmt.Sprintf("security gate denied: %s (%s)", toolName, reason))
					}
					toolResult = "User rejected command execution: permission denied."
				} else {
					if o.tools != nil {
						toolResult, executionErr = o.tools.Execute(ctx, toolName, toolArgs)
					} else {
						executionErr = errors.New("tool registry not initialized")
					}
				}
			} else {
				if o.tools != nil {
					toolResult, executionErr = o.tools.Execute(ctx, toolName, toolArgs)
				} else {
					executionErr = errors.New("tool registry not initialized")
				}
			}

			if executionErr != nil {
				errStr := executionErr.Error()
				toolResult = fmt.Sprintf("Error: %s", errStr)

				if toolName == lastFailedTool && toolArgs == lastFailedArgs && errStr == lastFailedErr {
					failedCount++
				} else {
					lastFailedTool = toolName
					lastFailedArgs = toolArgs
					lastFailedErr = errStr
					failedCount = 1
				}

				if failedCount >= MaxDuplicateErrorCount {
					if logCallback != nil {
						logCallback(fmt.Sprintf("duplicate failure guard triggered: %s", toolName))
					}
					return accumulatedFinalText.String(), fmt.Errorf("%w: tool %s failed %d consecutive times with: %s", ErrDuplicateToolFailure, toolName, MaxDuplicateErrorCount, errStr)
				}
			} else if !isTier3 || toolResult != "User rejected command execution: permission denied." {
				failedCount = 0
				lastFailedTool = ""
				lastFailedArgs = ""
				lastFailedErr = ""
			}

			if logCallback != nil {
				logCallback(fmt.Sprintf("tool executed: %s", toolName))
			}

			messages = append(messages, interfaces.Message{
				Role: interfaces.RoleTool,
				Parts: []interfaces.ContentPart{
					{
						ToolCallID: toolCallID,
						ToolName:   toolName,
						ToolResult: toolResult,
					},
				},
			})
		}
	}

	return accumulatedFinalText.String(), ErrMaxTurnsExceeded
}
