package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

type mockTurnRunner struct {
	runTurnFunc func(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error)
}

func (m *mockTurnRunner) RunTurn(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error) {
	if m.runTurnFunc != nil {
		return m.runTurnFunc(ctx, prompt, streamCallback, logCallback)
	}
	return "mock result", nil
}

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if parsed["status"] != "ok" {
		t.Fatalf("expected status ok, got: %s", parsed["status"])
	}
}

func TestProjectsEndpoint(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/projects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	var parsed struct {
		Status   string   `json:"status"`
		Projects []string `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("json decode error: %v", err)
	}

	if parsed.Status != "ok" {
		t.Fatalf("expected status ok, got: %s", parsed.Status)
	}

	expected := []string{"CITEOAI"}
	if len(parsed.Projects) != len(expected) {
		t.Fatalf("expected %d projects, got %d", len(expected), len(parsed.Projects))
	}
	for i, exp := range expected {
		if parsed.Projects[i] != exp {
			t.Errorf("project mismatch at %d: expected %s, got %s", i, exp, parsed.Projects[i])
		}
	}
}

func TestSessionsEndpoint(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	var parsed struct {
		Status   string    `json:"status"`
		Sessions []Session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("json decode error: %v", err)
	}

	if parsed.Status != "ok" {
		t.Fatalf("expected status ok, got: %s", parsed.Status)
	}
	if len(parsed.Sessions) != 1 {
		t.Fatalf("expected 1 session, got: %d", len(parsed.Sessions))
	}
	if parsed.Sessions[0].ID != "sess_default" || parsed.Sessions[0].Project != "CITEOAI" || parsed.Sessions[0].Title != "Main Session" {
		t.Fatalf("unexpected session content: %+v", parsed.Sessions[0])
	}
}

func TestStaticFallbackWhenDistMissing(t *testing.T) {
	srv := NewServer(ServerConfig{
		DistDir: filepath.Join(t.TempDir(), "non-existent-dir"),
	}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}

	if !strings.Contains(string(body), "CITEOAI Control") {
		t.Fatalf("expected fallback HTML with title CITEOAI Control")
	}
}

func TestStaticServingWhenDistExists(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexFile, []byte("<h1>Real UI Dist</h1>"), 0o600); err != nil {
		t.Fatalf("write index error: %v", err)
	}

	assetFile := filepath.Join(tmpDir, "main.js")
	if err := os.WriteFile(assetFile, []byte("console.log('citeoai');"), 0o600); err != nil {
		t.Fatalf("write asset error: %v", err)
	}

	srv := NewServer(ServerConfig{
		DistDir: tmpDir,
	}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	respIndex, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected index error: %v", err)
	}
	defer respIndex.Body.Close()
	bodyIndex, _ := io.ReadAll(respIndex.Body)
	if !strings.Contains(string(bodyIndex), "Real UI Dist") {
		t.Fatalf("expected real ui dist index content, got: %s", string(bodyIndex))
	}

	respAsset, err := http.Get(ts.URL + "/main.js")
	if err != nil {
		t.Fatalf("unexpected asset error: %v", err)
	}
	defer respAsset.Body.Close()
	bodyAsset, _ := io.ReadAll(respAsset.Body)
	if !strings.Contains(string(bodyAsset), "console.log('citeoai')") {
		t.Fatalf("expected asset content, got: %s", string(bodyAsset))
	}

	respSPA, err := http.Get(ts.URL + "/sessions/chat-123")
	if err != nil {
		t.Fatalf("unexpected spa route error: %v", err)
	}
	defer respSPA.Body.Close()
	bodySPA, _ := io.ReadAll(respSPA.Body)
	if !strings.Contains(string(bodySPA), "Real UI Dist") {
		t.Fatalf("expected spa fallback to index.html, got: %s", string(bodySPA))
	}
}

func TestWebSocketListProjects(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{Type: "list_projects"}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var resp ServerEvent
	if err := websocket.JSON.Receive(ws, &resp); err != nil {
		t.Fatalf("receive error: %v", err)
	}

	if resp.Type != "projects" {
		t.Fatalf("expected type projects, got: %s", resp.Type)
	}

	if len(resp.Projects) != 1 || resp.Projects[0] != "CITEOAI" {
		t.Fatalf("expected 1 project [CITEOAI], got: %v", resp.Projects)
	}
}

func TestWebSocketListSessions(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{Type: "list_sessions"}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var resp ServerEvent
	if err := websocket.JSON.Receive(ws, &resp); err != nil {
		t.Fatalf("receive error: %v", err)
	}

	if resp.Type != "sessions" {
		t.Fatalf("expected type sessions, got: %s", resp.Type)
	}

	if len(resp.Sessions) != 1 || resp.Sessions[0].ID != "sess_default" {
		t.Fatalf("expected 1 session [sess_default], got: %v", resp.Sessions)
	}
}

func TestWebSocketNewSession(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{
		Type:    "new_session",
		Project: "CITEOAI",
	}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var resp ServerEvent
	if err := websocket.JSON.Receive(ws, &resp); err != nil {
		t.Fatalf("receive error: %v", err)
	}

	if resp.Type != "session_created" {
		t.Fatalf("expected type session_created, got: %s", resp.Type)
	}

	if len(resp.Sessions) != 1 || resp.Sessions[0].Project != "CITEOAI" {
		t.Fatalf("unexpected new session payload: %+v", resp.Sessions)
	}
}

func TestWebSocketPromptExecution(t *testing.T) {
	runner := &mockTurnRunner{
		runTurnFunc: func(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error) {
			streamCallback("<thought>analyzing sandbox structure</thought>")
			streamCallback("Hello ")
			streamCallback("world!")
			logCallback("tool request: read_file /opt/main.go")
			logCallback("tool log: reading 50 lines")
			logCallback("tool executed: read_file")
			return "Task completed successfully", nil
		},
	}

	srv := NewServer(ServerConfig{}, runner)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{
		Type:    "prompt",
		Content: "test execution prompt",
	}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var receivedEvents []ServerEvent
	for {
		var evt ServerEvent
		if err := websocket.JSON.Receive(ws, &evt); err != nil {
			t.Fatalf("receive error: %v", err)
		}
		receivedEvents = append(receivedEvents, evt)
		if evt.Type == "done" {
			break
		}
	}

	var hasThought bool
	var hasToken bool
	var hasToolStart bool
	var hasToolLog bool
	var hasToolEnd bool
	var hasDone bool

	for _, e := range receivedEvents {
		switch e.Type {
		case "thought":
			hasThought = true
			if !strings.Contains(e.Delta, "analyzing sandbox") {
				t.Errorf("unexpected thought delta: %s", e.Delta)
			}
		case "token":
			hasToken = true
		case "tool_start":
			hasToolStart = true
			if e.Name != "read_file" {
				t.Errorf("unexpected tool start name: %s", e.Name)
			}
		case "tool_log":
			hasToolLog = true
		case "tool_end":
			hasToolEnd = true
			if e.Name != "read_file" || e.Success == nil || !*e.Success {
				t.Errorf("unexpected tool end payload: %+v", e)
			}
		case "done":
			hasDone = true
			if e.FinalText != "Task completed successfully" {
				t.Errorf("unexpected final text: %s", e.FinalText)
			}
		}
	}

	if !hasThought {
		t.Error("missing thought event")
	}
	if !hasToken {
		t.Error("missing token event")
	}
	if !hasToolStart {
		t.Error("missing tool_start event")
	}
	if !hasToolLog {
		t.Error("missing tool_log event")
	}
	if !hasToolEnd {
		t.Error("missing tool_end event")
	}
	if !hasDone {
		t.Error("missing done event")
	}
}

func TestWebSocketPromptDeniedAndFailure(t *testing.T) {
	runner := &mockTurnRunner{
		runTurnFunc: func(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error) {
			logCallback("tool request: run_command rm -rf /")
			logCallback("security gate denied: run_command (recursive file deletion)")
			return "command blocked", nil
		},
	}

	srv := NewServer(ServerConfig{}, runner)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{
		Type:    "prompt",
		Content: "delete system",
	}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var hasDeniedEnd bool
	for {
		var evt ServerEvent
		if err := websocket.JSON.Receive(ws, &evt); err != nil {
			t.Fatalf("receive error: %v", err)
		}
		if evt.Type == "tool_end" && evt.Name == "run_command" {
			if evt.Success != nil && !*evt.Success {
				hasDeniedEnd = true
			}
		}
		if evt.Type == "done" {
			break
		}
	}

	if !hasDeniedEnd {
		t.Errorf("expected tool_end event with success: false for security denial")
	}
}

func TestWebSocketPromptRunnerError(t *testing.T) {
	runner := &mockTurnRunner{
		runTurnFunc: func(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error) {
			return "", errors.New("provider timeout")
		},
	}

	srv := NewServer(ServerConfig{}, runner)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws://" + ts.Listener.Addr().String() + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer ws.Close()

	req := ClientMessage{
		Type:    "prompt",
		Content: "fail please",
	}
	if err := websocket.JSON.Send(ws, req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	var evt ServerEvent
	for {
		if err := websocket.JSON.Receive(ws, &evt); err != nil {
			t.Fatalf("receive error: %v", err)
		}
		if evt.Type == "done" {
			break
		}
	}

	if evt.Error != "provider timeout" {
		t.Errorf("expected provider timeout error, got: %s", evt.Error)
	}
}

func TestServerLifecycle(t *testing.T) {
	srv := NewServer(ServerConfig{Port: 0}, nil)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.StartOnListener(l)
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}

	serveErr := <-errCh
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		t.Fatalf("unexpected serve error: %v", serveErr)
	}
}

func TestServerConfigAndAccessors(t *testing.T) {
	srv := NewServer(ServerConfig{
		Port:    9099,
		DistDir: "/custom/dist",
	}, nil)

	if srv.Port() != 9099 {
		t.Errorf("expected port 9099, got %d", srv.Port())
	}
	if srv.DistDir() != "/custom/dist" {
		t.Errorf("expected distDir /custom/dist, got %s", srv.DistDir())
	}

	srv.AddProject("NewProject")
	projects := srv.ListProjects()
	if len(projects) != 2 || projects[1] != "NewProject" {
		t.Errorf("expected NewProject added, got: %v", projects)
	}

	newRunner := &mockTurnRunner{}
	srv.SetRunner(newRunner)
}

func TestFileEndpointSuccess(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/file?path=server.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	var parsed struct {
		Status  string `json:"status"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("json decode error: %v", err)
	}

	if parsed.Status != "ok" {
		t.Fatalf("expected status ok, got: %s", parsed.Status)
	}
	if parsed.Path != "server.go" {
		t.Fatalf("expected path server.go, got: %s", parsed.Path)
	}
	if !strings.Contains(parsed.Content, "package web") {
		t.Fatalf("expected content to contain package web, got: %s", parsed.Content)
	}
}

func TestFileEndpointNotFound(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/file?path=does_not_exist_file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", resp.StatusCode)
	}

	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("json decode error: %v", err)
	}

	if parsed.Status != "error" {
		t.Fatalf("expected status error, got: %s", parsed.Status)
	}
	if parsed.Error != "file not found" {
		t.Fatalf("expected error 'file not found', got: %s", parsed.Error)
	}
}

func TestFileEndpointPathTraversal(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	traversalPaths := []string{
		"../../../../etc/passwd",
		"/etc/passwd",
		"internal/../../../../etc/shadow",
		"..",
		".",
	}

	for _, p := range traversalPaths {
		resp, err := http.Get(ts.URL + "/api/file?path=" + p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			t.Fatalf("json decode error: %v", err)
		}
		resp.Body.Close()

		if parsed.Status != "error" {
			t.Fatalf("expected status error for path %s, got: %s", p, parsed.Status)
		}
		if parsed.Error != "file not found" {
			t.Fatalf("expected error 'file not found' for path %s, got: %s", p, parsed.Error)
		}
	}
}

func TestFileEndpointEmptyPath(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("json decode error: %v", err)
	}

	if parsed.Status != "error" || parsed.Error != "file not found" {
		t.Fatalf("unexpected response: %+v", parsed)
	}
}

func TestFileEndpointMethodNotAllowed(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/file?path=go.mod", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got: %d", resp.StatusCode)
	}
}
