package interfaces

import "context"

type Tool interface {
	Name() string
	Description() string
	Definition() ToolDefinition
	Execute(ctx context.Context, argsJSON string) (string, error)
}
