package interfaces

import "context"

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type ContentPart struct {
	Text       string
	ToolCallID string
	ToolName   string
	ToolArgs   string
	ToolResult string
}

type Message struct {
	Role  MessageRole
	Parts []ContentPart
}

type ToolParameter struct {
	Type        string
	Description string
	Required    bool
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]ToolParameter
}

type StreamChunk struct {
	DeltaText    string
	ToolCallID   string
	ToolName     string
	ToolArgsJSON string
	Done         bool
	Error        error
}

type Provider interface {
	Name() string
	GenerateStream(ctx context.Context, messages []Message, tools []ToolDefinition) (<-chan StreamChunk, error)
	GenerateContent(ctx context.Context, messages []Message, tools []ToolDefinition) (*Message, error)
}
