# 05 - LLM PROVIDER ENGINE

## 1. Provider Architecture
CITEOAI defines a clean, lightweight provider interface allowing zero-overhead model switching:

```go
package interfaces

import (
	"context"
	"io"
)

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
```

## 2. Gemini Provider Implementation
- **Target Endpoint**: `https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse&key={apiKey}`
- **Supported Models**: `gemini-3.6-flash`, `gemini-3.8-flash`, `gemini-3.5-flash`, `gemini-flash-lite-latest`.
- **SSE Stream Decoder**: Custom zero-allocation reader parsing Server-Sent Events (`data: {...}`) line-by-line using a small reusable buffer.
- **Function Calling**: Formatted using standard OpenAPI 3.0 schema definitions.

## 3. OpenAI-Compatible & Ollama Provider
- **Target Endpoint**: `{base_url}/chat/completions`
- **Supported Gateways**: Official OpenAI, Groq, OpenRouter, ByNara Router, local Ollama nodes (`http://localhost:11434/v1`).
- **SSE Stream Decoder**: Parses `data: {"choices":[{"delta":{"content":...}}]}` with immediate token delivery to channel.

## 4. Resilience & Error Circuit Breaker
- **Automatic Retry**: Exponential backoff with jitter on HTTP 429 (Rate Limit) and 503 (High Demand).
- **Graceful Fallback**: If a model returns 404 (deprecated), the engine reports clear diagnostics with actionable next steps.
