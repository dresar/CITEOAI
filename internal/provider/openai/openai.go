package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"citeoai/internal/interfaces"
)

const (
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4o"
)

type Option func(*Provider)

func WithBaseURL(baseURL string) Option {
	return func(p *Provider) {
		p.baseURL = baseURL
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) {
		p.client = client
	}
}

func WithName(name string) Option {
	return func(p *Provider) {
		p.name = name
	}
}

type Provider struct {
	name    string
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

var _ interfaces.Provider = (*Provider)(nil)

func New(apiKey, model string, opts ...Option) *Provider {
	if model == "" {
		model = DefaultModel
	}
	p := &Provider{
		name:    "openai",
		apiKey:  apiKey,
		model:   model,
		baseURL: DefaultBaseURL,
		client:  &http.Client{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "openai"
}

func (p *Provider) Model() string {
	return p.model
}

func (p *Provider) endpointURL() string {
	base := strings.TrimRight(p.baseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

type openAIMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCalls  []openAIToolCallReq `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

type openAIToolCallReq struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function openAIFunctionCallReq `json:"function"`
}

type openAIFunctionCallReq struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIFunctionDecl `json:"function"`
}

type openAIFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  openAISchemaObj `json:"parameters"`
}

type openAISchemaObj struct {
	Type       string                      `json:"type"`
	Properties map[string]openAISchemaProp `json:"properties"`
	Required   []string                    `json:"required,omitempty"`
}

type openAISchemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []openAITool    `json:"tools,omitempty"`
}

type openAIStreamResponse struct {
	ID      string         `json:"id"`
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Index        int         `json:"index"`
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason,omitempty"`
}

type openAIDelta struct {
	Role         string                `json:"role,omitempty"`
	Content      string                `json:"content,omitempty"`
	ToolCalls    []openAIToolCallDelta `json:"tool_calls,omitempty"`
	FunctionCall *openAIFunctionDelta  `json:"function_call,omitempty"`
}

type openAIToolCallDelta struct {
	Index    int                 `json:"index"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function openAIFunctionDelta `json:"function,omitempty"`
}

type openAIFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func convertMessages(messages []interfaces.Message) []openAIMessage {
	var result []openAIMessage
	for _, msg := range messages {
		switch msg.Role {
		case interfaces.RoleSystem:
			var texts []string
			for _, part := range msg.Parts {
				if part.Text != "" {
					texts = append(texts, part.Text)
				}
			}
			result = append(result, openAIMessage{
				Role:    "system",
				Content: strings.Join(texts, "\n"),
			})
		case interfaces.RoleUser:
			var texts []string
			for _, part := range msg.Parts {
				if part.Text != "" {
					texts = append(texts, part.Text)
				}
			}
			result = append(result, openAIMessage{
				Role:    "user",
				Content: strings.Join(texts, "\n"),
			})
		case interfaces.RoleAssistant:
			var texts []string
			var toolCalls []openAIToolCallReq
			for _, part := range msg.Parts {
				if part.Text != "" {
					texts = append(texts, part.Text)
				}
				if part.ToolName != "" {
					callID := part.ToolCallID
					if callID == "" {
						callID = fmt.Sprintf("call_%s", part.ToolName)
					}
					toolCalls = append(toolCalls, openAIToolCallReq{
						ID:   callID,
						Type: "function",
						Function: openAIFunctionCallReq{
							Name:      part.ToolName,
							Arguments: part.ToolArgs,
						},
					})
				}
			}
			result = append(result, openAIMessage{
				Role:      "assistant",
				Content:   strings.Join(texts, "\n"),
				ToolCalls: toolCalls,
			})
		case interfaces.RoleTool:
			for _, part := range msg.Parts {
				result = append(result, openAIMessage{
					Role:       "tool",
					ToolCallID: part.ToolCallID,
					Content:    part.ToolResult,
				})
			}
		}
	}
	return result
}

func convertTools(tools []interfaces.ToolDefinition) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		props := make(map[string]openAISchemaProp, len(tool.Parameters))
		var required []string
		for paramName, param := range tool.Parameters {
			paramType := strings.ToLower(param.Type)
			if paramType == "" {
				paramType = "string"
			}
			props[paramName] = openAISchemaProp{
				Type:        paramType,
				Description: param.Description,
			}
			if param.Required {
				required = append(required, paramName)
			}
		}
		sort.Strings(required)
		result = append(result, openAITool{
			Type: "function",
			Function: openAIFunctionDecl{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: openAISchemaObj{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
			},
		})
	}
	return result
}

func (p *Provider) GenerateStream(ctx context.Context, messages []interfaces.Message, tools []interfaces.ToolDefinition) (<-chan interfaces.StreamChunk, error) {
	reqBody := openAIRequest{
		Model:    p.model,
		Messages: convertMessages(messages),
		Stream:   true,
		Tools:    convertTools(tools),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	targetURL := p.endpointURL()
	var resp *http.Response
	maxRetries := 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}

		resp, err = p.client.Do(req)
		if err != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
					continue
				}
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			if attempt < maxRetries && ctx.Err() == nil {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
					continue
				}
			}
		}
		break
	}

	if resp.StatusCode != http.StatusOK {
		errBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(errBytes)))
	}

	out := make(chan interfaces.StreamChunk, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		reader := bufio.NewReaderSize(resp.Body, 32*1024)

		for {
			select {
			case <-ctx.Done():
				out <- interfaces.StreamChunk{Error: ctx.Err()}
				return
			default:
			}

			line, readErr := reader.ReadBytes('\n')
			if len(line) > 0 {
				line = bytes.TrimSpace(line)
				if bytes.HasPrefix(line, []byte("data:")) {
					payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
					if bytes.Equal(payload, []byte("[DONE]")) {
						break
					}
					if len(payload) > 0 {
						var streamResp openAIStreamResponse
						if parseErr := json.Unmarshal(payload, &streamResp); parseErr != nil {
							var errResp struct {
								Error openAIError `json:"error"`
							}
							if json.Unmarshal(payload, &errResp) == nil && errResp.Error.Message != "" {
								out <- interfaces.StreamChunk{Error: fmt.Errorf("openai stream error: %s", errResp.Error.Message)}
								return
							}
						} else {
							if streamResp.Error != nil {
								out <- interfaces.StreamChunk{Error: fmt.Errorf("openai error: %s", streamResp.Error.Message)}
								return
							}
							for _, choice := range streamResp.Choices {
								if choice.Delta.Content != "" {
									out <- interfaces.StreamChunk{DeltaText: choice.Delta.Content}
								}
								for _, tc := range choice.Delta.ToolCalls {
									out <- interfaces.StreamChunk{
										ToolCallID:   tc.ID,
										ToolName:     tc.Function.Name,
										ToolArgsJSON: tc.Function.Arguments,
									}
								}
								if choice.Delta.FunctionCall != nil {
									out <- interfaces.StreamChunk{
										ToolName:     choice.Delta.FunctionCall.Name,
										ToolArgsJSON: choice.Delta.FunctionCall.Arguments,
									}
								}
							}
						}
					}
				}
			}

			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					out <- interfaces.StreamChunk{Error: readErr}
				}
				break
			}
		}

		out <- interfaces.StreamChunk{Done: true}
	}()

	return out, nil
}

func (p *Provider) GenerateContent(ctx context.Context, messages []interfaces.Message, tools []interfaces.ToolDefinition) (*interfaces.Message, error) {
	ch, err := p.GenerateStream(ctx, messages, tools)
	if err != nil {
		return nil, err
	}
	return collectStream(ch)
}

func collectStream(ch <-chan interfaces.StreamChunk) (*interfaces.Message, error) {
	var textBuilder strings.Builder
	var currentCallID string
	toolCalls := make(map[string]*interfaces.ContentPart)
	var callOrder []string

	for chunk := range ch {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.DeltaText != "" {
			textBuilder.WriteString(chunk.DeltaText)
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

			part, exists := toolCalls[id]
			if !exists {
				part = &interfaces.ContentPart{
					ToolCallID: id,
					ToolName:   chunk.ToolName,
				}
				toolCalls[id] = part
				callOrder = append(callOrder, id)
			}
			if chunk.ToolName != "" {
				part.ToolName = chunk.ToolName
			}
			part.ToolArgs += chunk.ToolArgsJSON
		}
	}

	var parts []interfaces.ContentPart
	if textBuilder.Len() > 0 {
		parts = append(parts, interfaces.ContentPart{Text: textBuilder.String()})
	}
	for _, id := range callOrder {
		parts = append(parts, *toolCalls[id])
	}

	return &interfaces.Message{
		Role:  interfaces.RoleAssistant,
		Parts: parts,
	}, nil
}
