package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"citeoai/internal/interfaces"
)

const (
	DefaultBaseURL       = "https://generativelanguage.googleapis.com"
	ModelGemini36Flash   = "gemini-3.6-flash"
	ModelGemini38Flash   = "gemini-3.8-flash"
	ModelGemini35Flash   = "gemini-3.5-flash"
	ModelGeminiFlashLite = "gemini-flash-lite-latest"
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

type Provider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

var _ interfaces.Provider = (*Provider)(nil)

func New(apiKey, model string, opts ...Option) *Provider {
	if model == "" {
		model = ModelGemini36Flash
	}
	p := &Provider{
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
	return "gemini"
}

func (p *Provider) Model() string {
	return p.model
}

func (p *Provider) endpointURL() string {
	base := strings.TrimRight(p.baseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", base, p.model)
	if p.apiKey != "" {
		endpoint += "&key=" + url.QueryEscape(p.apiKey)
	}
	return endpoint
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDecl struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Parameters  *geminiSchemaObj `json:"parameters,omitempty"`
}

type geminiSchemaObj struct {
	Type       string                      `json:"type"`
	Properties map[string]geminiSchemaProp `json:"properties,omitempty"`
	Required   []string                    `json:"required,omitempty"`
}

type geminiSchemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	Tools             []geminiTool             `json:"tools,omitempty"`
}

type geminiResponseChunk struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content      geminiCandidateContent `json:"content"`
	FinishReason string                 `json:"finishReason,omitempty"`
}

type geminiCandidateContent struct {
	Parts []geminiCandidatePart `json:"parts"`
	Role  string                `json:"role,omitempty"`
}

type geminiCandidatePart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func convertMessages(messages []interfaces.Message) ([]geminiContent, *geminiSystemInstruction) {
	var contents []geminiContent
	var systemParts []geminiPart

	for _, msg := range messages {
		switch msg.Role {
		case interfaces.RoleSystem:
			for _, part := range msg.Parts {
				if part.Text != "" {
					systemParts = append(systemParts, geminiPart{Text: part.Text})
				}
			}
		case interfaces.RoleUser:
			var parts []geminiPart
			for _, part := range msg.Parts {
				if part.Text != "" {
					parts = append(parts, geminiPart{Text: part.Text})
				}
			}
			if len(parts) > 0 {
				contents = append(contents, geminiContent{
					Role:  "user",
					Parts: parts,
				})
			}
		case interfaces.RoleAssistant:
			var parts []geminiPart
			for _, part := range msg.Parts {
				if part.Text != "" {
					parts = append(parts, geminiPart{Text: part.Text})
				}
				if part.ToolName != "" {
					argsMap := make(map[string]any)
					if part.ToolArgs != "" {
						_ = json.Unmarshal([]byte(part.ToolArgs), &argsMap)
					}
					parts = append(parts, geminiPart{
						FunctionCall: &geminiFunctionCall{
							Name: part.ToolName,
							Args: argsMap,
						},
					})
				}
			}
			if len(parts) > 0 {
				contents = append(contents, geminiContent{
					Role:  "model",
					Parts: parts,
				})
			}
		case interfaces.RoleTool:
			var parts []geminiPart
			for _, part := range msg.Parts {
				respMap := make(map[string]any)
				if part.ToolResult != "" {
					var raw any
					if err := json.Unmarshal([]byte(part.ToolResult), &raw); err == nil {
						respMap["content"] = raw
					} else {
						respMap["content"] = part.ToolResult
					}
				}
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResp{
						Name:     part.ToolName,
						Response: respMap,
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, geminiContent{
					Role:  "user",
					Parts: parts,
				})
			}
		}
	}

	var sysInstruction *geminiSystemInstruction
	if len(systemParts) > 0 {
		sysInstruction = &geminiSystemInstruction{Parts: systemParts}
	}

	return contents, sysInstruction
}

func convertTools(tools []interfaces.ToolDefinition) []geminiTool {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]geminiFunctionDecl, 0, len(tools))
	for _, tool := range tools {
		decl := geminiFunctionDecl{
			Name:        tool.Name,
			Description: tool.Description,
		}
		if len(tool.Parameters) > 0 {
			props := make(map[string]geminiSchemaProp, len(tool.Parameters))
			var required []string
			for paramName, param := range tool.Parameters {
				pType := strings.ToUpper(param.Type)
				if pType == "" {
					pType = "STRING"
				}
				props[paramName] = geminiSchemaProp{
					Type:        pType,
					Description: param.Description,
				}
				if param.Required {
					required = append(required, paramName)
				}
			}
			sort.Strings(required)
			decl.Parameters = &geminiSchemaObj{
				Type:       "OBJECT",
				Properties: props,
				Required:   required,
			}
		}
		decls = append(decls, decl)
	}
	return []geminiTool{{FunctionDeclarations: decls}}
}

func (p *Provider) GenerateStream(ctx context.Context, messages []interfaces.Message, tools []interfaces.ToolDefinition) (<-chan interfaces.StreamChunk, error) {
	contents, sysInstruction := convertMessages(messages)
	reqBody := geminiRequest{
		Contents:          contents,
		SystemInstruction: sysInstruction,
		Tools:             convertTools(tools),
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
			req.Header.Set("x-goog-api-key", p.apiKey)
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
		return nil, fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(errBytes)))
	}

	out := make(chan interfaces.StreamChunk, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		reader := bufio.NewReaderSize(resp.Body, 32*1024)
		var callCounter int

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
					if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
						var chunk geminiResponseChunk
						if parseErr := json.Unmarshal(payload, &chunk); parseErr != nil {
							var errResp geminiErrorResponse
							if json.Unmarshal(payload, &errResp) == nil && errResp.Error.Message != "" {
								out <- interfaces.StreamChunk{Error: fmt.Errorf("gemini stream error: %s", errResp.Error.Message)}
								return
							}
						} else {
							for _, cand := range chunk.Candidates {
								for _, part := range cand.Content.Parts {
									if part.Text != "" {
										out <- interfaces.StreamChunk{DeltaText: part.Text}
									}
									if part.FunctionCall != nil {
										callCounter++
										callID := fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, callCounter)
										argsJSON := "{}"
										if len(part.FunctionCall.Args) > 0 {
											if b, mErr := json.Marshal(part.FunctionCall.Args); mErr == nil {
												argsJSON = string(b)
											}
										}
										out <- interfaces.StreamChunk{
											ToolCallID:   callID,
											ToolName:     part.FunctionCall.Name,
											ToolArgsJSON: argsJSON,
										}
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
