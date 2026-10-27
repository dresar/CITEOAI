package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"citeoai/internal/interfaces"
)

func TestOpenAIStreamGenerateContent_TextChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello, \"}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"OpenAI!\"}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-api-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	messages := []interfaces.Message{
		{
			Role:  interfaces.RoleUser,
			Parts: []interfaces.ContentPart{{Text: "Hi"}},
		},
	}

	stream, err := prov.GenerateStream(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var textBuilder strings.Builder
	var doneReceived bool

	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.DeltaText != "" {
			textBuilder.WriteString(chunk.DeltaText)
		}
		if chunk.Done {
			doneReceived = true
		}
	}

	if !doneReceived {
		t.Errorf("expected done chunk")
	}

	expected := "Hello, OpenAI!"
	if textBuilder.String() != expected {
		t.Errorf("expected %q, got %q", expected, textBuilder.String())
	}
}

func TestOpenAIStreamGenerateContent_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"call_123\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"path\\\":\\\"main.go\\\"}\"}}]}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	stream, err := prov.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var receivedToolName string
	var receivedCallID string
	var receivedArgs string

	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.ToolName != "" {
			receivedToolName = chunk.ToolName
			receivedCallID = chunk.ToolCallID
			receivedArgs = chunk.ToolArgsJSON
		}
	}

	if receivedToolName != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", receivedToolName)
	}
	if receivedCallID != "call_123" {
		t.Errorf("expected call ID 'call_123', got %q", receivedCallID)
	}

	var argsMap map[string]string
	if err := json.Unmarshal([]byte(receivedArgs), &argsMap); err != nil {
		t.Fatalf("failed to parse tool args: %v", err)
	}
	if argsMap["path"] != "main.go" {
		t.Errorf("expected path 'main.go', got %q", argsMap["path"])
	}
}

func TestOpenAIStreamGenerateContent_FunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"function_call\":{\"name\":\"run_command\",\"arguments\":\"{\\\"command\\\":\\\"ls\\\"}\"}}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	stream, err := prov.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var receivedToolName string
	var receivedArgs string

	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.ToolName != "" {
			receivedToolName = chunk.ToolName
			receivedArgs = chunk.ToolArgsJSON
		}
	}

	if receivedToolName != "run_command" {
		t.Errorf("expected tool name 'run_command', got %q", receivedToolName)
	}
	if !strings.Contains(receivedArgs, "ls") {
		t.Errorf("expected args to contain 'ls', got %q", receivedArgs)
	}
}

func TestOpenAIGenerateContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Task \"}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"finished.\"}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	msg, err := prov.GenerateContent(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Role != interfaces.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msg.Role)
	}

	if len(msg.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(msg.Parts))
	}

	expectedText := "Task finished."
	if msg.Parts[0].Text != expectedText {
		t.Errorf("expected %q, got %q", expectedText, msg.Parts[0].Text)
	}
}

func TestOpenAIToolConversion(t *testing.T) {
	tools := []interfaces.ToolDefinition{
		{
			Name:        "search",
			Description: "Search internet",
			Parameters: map[string]interfaces.ToolParameter{
				"query": {
					Type:        "string",
					Description: "Search query",
					Required:    true,
				},
				"limit": {
					Type:        "integer",
					Description: "Result limit",
					Required:    false,
				},
			},
		},
	}

	openAITools := convertTools(tools)
	if len(openAITools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(openAITools))
	}

	tool := openAITools[0]
	if tool.Type != "function" {
		t.Errorf("expected tool type 'function', got %q", tool.Type)
	}
	if tool.Function.Name != "search" {
		t.Errorf("expected function name 'search', got %q", tool.Function.Name)
	}
	if tool.Function.Parameters.Type != "object" {
		t.Errorf("expected parameter type 'object', got %q", tool.Function.Parameters.Type)
	}
	if len(tool.Function.Parameters.Required) != 1 || tool.Function.Parameters.Required[0] != "query" {
		t.Errorf("unexpected required fields: %v", tool.Function.Parameters.Required)
	}
}

func TestOpenAILocalOllamaEndpoint(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	ollamaBase := server.URL + "/v1"
	prov := New("", "llama3.2", WithBaseURL(ollamaBase), WithName("ollama"), WithHTTPClient(server.Client()))

	if prov.Name() != "ollama" {
		t.Errorf("expected provider name 'ollama', got %s", prov.Name())
	}

	_, err := prov.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected path '/v1/chat/completions', got %q", capturedPath)
	}
}

func TestOpenAIErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer server.Close()

	prov := New("bad-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	_, err := prov.GenerateStream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain 401, got %v", err)
	}
}

func TestOpenAIContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	prov := New("test-key", "gpt-4o", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := prov.GenerateStream(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestOpenAIProviderIdentity(t *testing.T) {
	prov := New("key", "gpt-4o")
	if prov.Name() != "openai" {
		t.Errorf("expected name 'openai', got %s", prov.Name())
	}
	if prov.Model() != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %s", prov.Model())
	}
}
