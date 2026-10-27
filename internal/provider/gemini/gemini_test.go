package gemini

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

func TestGeminiStreamGenerateContent_TextChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("expected alt=sse, got %s", r.URL.Query().Get("alt"))
		}
		if r.URL.Query().Get("key") != "test-api-key" {
			t.Errorf("expected key=test-api-key, got %s", r.URL.Query().Get("key"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello, \"}]}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"world!\"}]}}]}\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-api-key", ModelGemini36Flash, WithBaseURL(server.URL), WithHTTPClient(server.Client()))

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

	expected := "Hello, world!"
	if textBuilder.String() != expected {
		t.Errorf("expected %q, got %q", expected, textBuilder.String())
	}
}

func TestGeminiStreamGenerateContent_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"read_file\",\"args\":{\"path\":\"main.go\"}}}]}}]}\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-key", ModelGemini38Flash, WithBaseURL(server.URL), WithHTTPClient(server.Client()))

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

	if receivedToolName != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", receivedToolName)
	}

	var argsMap map[string]string
	if err := json.Unmarshal([]byte(receivedArgs), &argsMap); err != nil {
		t.Fatalf("failed to parse tool args: %v", err)
	}
	if argsMap["path"] != "main.go" {
		t.Errorf("expected path 'main.go', got %q", argsMap["path"])
	}
}

func TestGeminiGenerateContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Execution \"}]}}]}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"complete.\"}]}}]}\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	prov := New("test-key", ModelGemini36Flash, WithBaseURL(server.URL), WithHTTPClient(server.Client()))

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

	expectedText := "Execution complete."
	if msg.Parts[0].Text != expectedText {
		t.Errorf("expected %q, got %q", expectedText, msg.Parts[0].Text)
	}
}

func TestGeminiToolConversion(t *testing.T) {
	tools := []interfaces.ToolDefinition{
		{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: map[string]interfaces.ToolParameter{
				"path": {
					Type:        "string",
					Description: "Path of the file",
					Required:    true,
				},
				"content": {
					Type:        "string",
					Description: "Content to write",
					Required:    true,
				},
				"overwrite": {
					Type:        "boolean",
					Description: "Overwrite existing file",
					Required:    false,
				},
			},
		},
	}

	geminiTools := convertTools(tools)
	if len(geminiTools) != 1 {
		t.Fatalf("expected 1 tool declaration set, got %d", len(geminiTools))
	}
	if len(geminiTools[0].FunctionDeclarations) != 1 {
		t.Fatalf("expected 1 function declaration, got %d", len(geminiTools[0].FunctionDeclarations))
	}

	decl := geminiTools[0].FunctionDeclarations[0]
	if decl.Name != "write_file" {
		t.Errorf("expected name 'write_file', got %q", decl.Name)
	}
	if decl.Parameters.Type != "OBJECT" {
		t.Errorf("expected parameter type OBJECT, got %q", decl.Parameters.Type)
	}
	if len(decl.Parameters.Required) != 2 {
		t.Errorf("expected 2 required parameters, got %d", len(decl.Parameters.Required))
	}
	if decl.Parameters.Properties["overwrite"].Type != "BOOLEAN" {
		t.Errorf("expected overwrite type BOOLEAN, got %q", decl.Parameters.Properties["overwrite"].Type)
	}
}

func TestGeminiErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":{"code":400,"message":"Invalid request"}}`)
	}))
	defer server.Close()

	prov := New("test-key", ModelGemini36Flash, WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	_, err := prov.GenerateStream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error to contain 400, got %v", err)
	}
}

func TestGeminiContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	prov := New("test-key", ModelGemini36Flash, WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := prov.GenerateStream(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestGeminiProviderIdentity(t *testing.T) {
	prov := New("key", ModelGemini38Flash)
	if prov.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %s", prov.Name())
	}
	if prov.Model() != ModelGemini38Flash {
		t.Errorf("expected model %s, got %s", ModelGemini38Flash, prov.Model())
	}
}
