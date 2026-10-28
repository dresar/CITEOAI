package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type TurnRunner interface {
	RunTurn(ctx context.Context, prompt string, streamCallback func(token string), logCallback func(entry string)) (string, error)
}

type ServerConfig struct {
	Port    int
	DistDir string
}

type ClientMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Model     string `json:"model,omitempty"`
	Project   string `json:"project,omitempty"`
}

type ServerEvent struct {
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	Delta     string    `json:"delta,omitempty"`
	Name      string    `json:"name,omitempty"`
	Args      string    `json:"args,omitempty"`
	Output    string    `json:"output,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	FinalText string    `json:"final_text,omitempty"`
	Error     string    `json:"error,omitempty"`
	Projects  []string  `json:"projects,omitempty"`
	Sessions  []Session `json:"sessions,omitempty"`
}

type fileResponse struct {
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

type Session struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at"`
}

type Server struct {
	port       int
	distDir    string
	runner     TurnRunner
	projects   []string
	sessions   []Session
	mu         sync.RWMutex
	httpServer *http.Server
	listener   net.Listener
}

func NewServer(cfg ServerConfig, runner TurnRunner) *Server {
	port := cfg.Port
	if port <= 0 {
		port = 8088
	}

	distDir := cfg.DistDir
	if distDir == "" {
		distDir = "/opt/data/home/ui-citeoai/dist"
	}

	defaultProjects := []string{
		"CITEOAI",
	}

	defaultSessions := []Session{
		{
			ID:        "sess_default",
			Project:   "CITEOAI",
			Title:     "Main Session",
			UpdatedAt: "now",
		},
	}

	return &Server{
		port:     port,
		distDir:  distDir,
		runner:   runner,
		projects: defaultProjects,
		sessions: defaultSessions,
	}
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) DistDir() string {
	return s.distDir
}

func (s *Server) SetRunner(runner TurnRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = runner
}

func (s *Server) ListProjects() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.projects))
	copy(res, s.projects)
	return res
}

func (s *Server) AddProject(project string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects = append(s.projects, project)
}

func (s *Server) ListSessions() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]Session, len(s.sessions))
	copy(res, s.sessions)
	return res
}

func (s *Server) AddSession(sess Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = append([]Session{sess}, s.sessions...)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	wsHandler := websocket.Server{
		Handshake: func(cfg *websocket.Config, req *http.Request) error {
			return nil
		},
		Handler: websocket.Handler(s.handleWebSocket),
	}

	mux.Handle("/ws", wsHandler)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/projects", s.handleProjects)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/", s.handleStaticOrFallback)

	return mux
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}
	s.mu.Unlock()

	return s.httpServer.ListenAndServe()
}

func (s *Server) StartOnListener(l net.Listener) error {
	s.mu.Lock()
	s.listener = l
	s.httpServer = &http.Server{
		Handler: s.Handler(),
	}
	s.mu.Unlock()

	return s.httpServer.Serve(l)
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	projects := s.ListProjects()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := struct {
		Status   string   `json:"status"`
		Projects []string `json:"projects"`
	}{
		Status:   "ok",
		Projects: projects,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	sessions := s.ListSessions()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := struct {
		Status   string    `json:"status"`
		Sessions []Session `json:"sessions"`
	}{
		Status:   "ok",
		Sessions: sessions,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(fileResponse{
			Status: "error",
			Error:  "file not found",
		})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(fileResponse{
			Status: "error",
			Error:  "file not found",
		})
		return
	}

	if realWd, err := filepath.EvalSymlinks(wd); err == nil {
		wd = realWd
	}

	cleanedPath := filepath.Clean(filePath)
	var targetPath string
	if filepath.IsAbs(cleanedPath) {
		targetPath = cleanedPath
	} else {
		targetPath = filepath.Join(wd, cleanedPath)
	}

	rel, err := filepath.Rel(wd, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(fileResponse{
			Status: "error",
			Error:  "file not found",
		})
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(fileResponse{
			Status: "error",
			Error:  "file not found",
		})
		return
	}

	if realTarget, err := filepath.EvalSymlinks(targetPath); err == nil {
		realRel, relErr := filepath.Rel(wd, realTarget)
		if relErr != nil || strings.HasPrefix(realRel, "..") || realRel == ".." {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(fileResponse{
				Status: "error",
				Error:  "file not found",
			})
			return
		}
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(fileResponse{
			Status: "error",
			Error:  "file not found",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	resp := fileResponse{
		Status:  "ok",
		Path:    filePath,
		Content: string(data),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStaticOrFallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	dist := s.distDir
	if dist == "" {
		s.serveFallbackHTML(w, r)
		return
	}

	stat, err := os.Stat(dist)
	if err != nil || !stat.IsDir() {
		s.serveFallbackHTML(w, r)
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	target := filepath.Join(dist, cleanPath)

	fileInfo, err := os.Stat(target)
	if err == nil && !fileInfo.IsDir() {
		http.ServeFile(w, r, target)
		return
	}

	indexPath := filepath.Join(dist, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	s.serveFallbackHTML(w, r)
}

func (s *Server) serveFallbackHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fallbackHTML))
}

func (s *Server) handleWebSocket(ws *websocket.Conn) {
	defer ws.Close()

	var sendMu sync.Mutex
	sendEvent := func(evt ServerEvent) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return websocket.JSON.Send(ws, evt)
	}

	for {
		var msg ClientMessage
		err := websocket.JSON.Receive(ws, &msg)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}

		switch msg.Type {
		case "list_projects":
			_ = sendEvent(ServerEvent{
				Type:     "projects",
				Projects: s.ListProjects(),
			})

		case "list_sessions":
			_ = sendEvent(ServerEvent{
				Type:     "sessions",
				Sessions: s.ListSessions(),
			})

		case "new_session":
			proj := msg.Project
			if proj == "" {
				proj = "CITEOAI"
			}
			newSess := Session{
				ID:        fmt.Sprintf("sess_%d", time.Now().UnixNano()),
				Project:   proj,
				Title:     "New Session",
				UpdatedAt: "now",
			}
			s.AddSession(newSess)
			_ = sendEvent(ServerEvent{
				Type:     "session_created",
				Sessions: []Session{newSess},
			})

		case "prompt":
			promptText := msg.Content
			if promptText == "" {
				promptText = msg.Prompt
			}
			s.executePrompt(ws.Request().Context(), msg.SessionID, promptText, sendEvent)
		}
	}
}

func (s *Server) executePrompt(ctx context.Context, sessionID string, promptText string, sendEvent func(ServerEvent) error) {
	s.mu.RLock()
	runner := s.runner
	s.mu.RUnlock()

	if runner == nil {
		_ = sendEvent(ServerEvent{
			Type:      "done",
			SessionID: sessionID,
			FinalText: "Error: runner not configured",
			Error:     "runner not configured",
		})
		return
	}

	var inThought bool
	var thoughtMu sync.Mutex

	streamCallback := func(token string) {
		thoughtMu.Lock()
		defer thoughtMu.Unlock()

		if strings.HasPrefix(token, "thought:") {
			_ = sendEvent(ServerEvent{
				Type:      "thought",
				SessionID: sessionID,
				Delta:     strings.TrimPrefix(token, "thought:"),
			})
			return
		}

		remaining := token
		for len(remaining) > 0 {
			if inThought {
				closeIdx := strings.Index(remaining, "</thought>")
				altCloseIdx := strings.Index(remaining, "</think>")
				endIdx := closeIdx
				tagLen := len("</thought>")
				if endIdx == -1 || (altCloseIdx != -1 && altCloseIdx < endIdx) {
					endIdx = altCloseIdx
					tagLen = len("</think>")
				}

				if endIdx != -1 {
					thoughtChunk := remaining[:endIdx]
					if len(thoughtChunk) > 0 {
						_ = sendEvent(ServerEvent{
							Type:      "thought",
							SessionID: sessionID,
							Delta:     thoughtChunk,
						})
					}
					inThought = false
					remaining = remaining[endIdx+tagLen:]
				} else {
					_ = sendEvent(ServerEvent{
						Type:      "thought",
						SessionID: sessionID,
						Delta:     remaining,
					})
					remaining = ""
				}
			} else {
				openIdx := strings.Index(remaining, "<thought>")
				altOpenIdx := strings.Index(remaining, "<think>")
				startIdx := openIdx
				tagLen := len("<thought>")
				if startIdx == -1 || (altOpenIdx != -1 && altOpenIdx < startIdx) {
					startIdx = altOpenIdx
					tagLen = len("<think>")
				}

				if startIdx != -1 {
					textChunk := remaining[:startIdx]
					if len(textChunk) > 0 {
						_ = sendEvent(ServerEvent{
							Type:      "token",
							SessionID: sessionID,
							Delta:     textChunk,
						})
					}
					inThought = true
					remaining = remaining[startIdx+tagLen:]
				} else {
					_ = sendEvent(ServerEvent{
						Type:      "token",
						SessionID: sessionID,
						Delta:     remaining,
					})
					remaining = ""
				}
			}
		}
	}

	logCallback := func(entry string) {
		_ = sendEvent(ServerEvent{
			Type:      "tool_log",
			SessionID: sessionID,
			Output:    entry,
		})

		trimmed := strings.TrimSpace(entry)
		if strings.HasPrefix(trimmed, "tool request: ") {
			payload := strings.TrimPrefix(trimmed, "tool request: ")
			name, args := parseToolNameAndArgs(payload)
			_ = sendEvent(ServerEvent{
				Type:      "tool_start",
				SessionID: sessionID,
				Name:      name,
				Args:      args,
			})
		} else if strings.HasPrefix(trimmed, "tool_start: ") {
			payload := strings.TrimPrefix(trimmed, "tool_start: ")
			name, args := parseToolNameAndArgs(payload)
			_ = sendEvent(ServerEvent{
				Type:      "tool_start",
				SessionID: sessionID,
				Name:      name,
				Args:      args,
			})
		} else if strings.HasPrefix(trimmed, "tool executed: ") {
			payload := strings.TrimPrefix(trimmed, "tool executed: ")
			name := strings.TrimSpace(strings.Split(payload, " ")[0])
			success := true
			_ = sendEvent(ServerEvent{
				Type:      "tool_end",
				SessionID: sessionID,
				Name:      name,
				Success:   &success,
			})
		} else if strings.HasPrefix(trimmed, "tool_end: ") {
			payload := strings.TrimPrefix(trimmed, "tool_end: ")
			parts := strings.Fields(payload)
			name := parts[0]
			success := true
			if len(parts) > 1 && (parts[1] == "false" || parts[1] == "failed" || parts[1] == "error") {
				success = false
			}
			_ = sendEvent(ServerEvent{
				Type:      "tool_end",
				SessionID: sessionID,
				Name:      name,
				Success:   &success,
			})
		} else if strings.HasPrefix(trimmed, "security gate denied: ") {
			payload := strings.TrimPrefix(trimmed, "security gate denied: ")
			name := strings.TrimSpace(strings.Split(payload, " ")[0])
			success := false
			_ = sendEvent(ServerEvent{
				Type:      "tool_end",
				SessionID: sessionID,
				Name:      name,
				Success:   &success,
			})
		} else if strings.HasPrefix(trimmed, "duplicate failure guard triggered: ") {
			payload := strings.TrimPrefix(trimmed, "duplicate failure guard triggered: ")
			name := strings.TrimSpace(strings.Split(payload, " ")[0])
			success := false
			_ = sendEvent(ServerEvent{
				Type:      "tool_end",
				SessionID: sessionID,
				Name:      name,
				Success:   &success,
			})
		} else if strings.HasPrefix(trimmed, "thought: ") {
			_ = sendEvent(ServerEvent{
				Type:      "thought",
				SessionID: sessionID,
				Delta:     strings.TrimPrefix(trimmed, "thought: "),
			})
		}
	}

	result, err := runner.RunTurn(ctx, promptText, streamCallback, logCallback)
	if err != nil {
		_ = sendEvent(ServerEvent{
			Type:      "done",
			SessionID: sessionID,
			FinalText: result,
			Error:     err.Error(),
		})
		return
	}

	_ = sendEvent(ServerEvent{
		Type:      "done",
		SessionID: sessionID,
		FinalText: result,
	})
}

func parseToolNameAndArgs(input string) (string, string) {
	trimmed := strings.TrimSpace(input)
	firstSpace := strings.Index(trimmed, " ")
	if firstSpace == -1 {
		return trimmed, ""
	}
	return trimmed[:firstSpace], strings.TrimSpace(trimmed[firstSpace+1:])
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>CITEOAI</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background-color: #0e0e11;
  color: #f4f4f5;
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  padding: 16px;
  line-height: 1.5;
}
.card {
  background: #18181c;
  border: 1px solid #27272a;
  border-radius: 4px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.4);
  padding: 16px;
  margin-bottom: 16px;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.title { font-size: 16px; font-weight: 700; letter-spacing: -0.02em; }
.badge {
  background: #1e1e24;
  border: 1px solid #27272a;
  border-radius: 4px;
  padding: 2px 8px;
  font-size: 12px;
  color: #10b981;
}
.badge-warn { color: #f59e0b; }
.logbox {
  background: #121215;
  border: 1px solid #27272a;
  border-radius: 4px;
  padding: 12px;
  height: 260px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 13px;
  color: #a1a1aa;
  white-space: pre-wrap;
}
.input-row {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
input[type="text"] {
  flex: 1;
  background: #121215;
  border: 1px solid #27272a;
  border-radius: 4px;
  padding: 8px 12px;
  color: #f4f4f5;
  font-size: 14px;
  outline: none;
}
input[type="text"]:focus {
  border-color: #6366f1;
}
button {
  background: #6366f1;
  color: #ffffff;
  border: none;
  border-radius: 4px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
button:hover { background: #4f46e5; }
.btn-secondary {
  background: #1e1e24;
  border: 1px solid #27272a;
  color: #f4f4f5;
}
.btn-secondary:hover { background: #27272a; }
.actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
</style>
</head>
<body>
<div class="card">
  <div class="header">
    <div class="title">CITEOAI Control</div>
    <div class="badge" id="statusBadge">Active</div>
  </div>
  <p style="font-size: 13px; color: #71717a; margin-top: 6px;">Fallback Interface</p>
</div>
<div class="card">
  <div class="logbox" id="outputBox">Ready.</div>
  <div class="input-row">
    <input type="text" id="promptInput" placeholder="Enter prompt" />
    <button id="sendBtn">Send</button>
  </div>
  <div class="actions">
    <button class="btn-secondary" id="projBtn">List Projects</button>
    <button class="btn-secondary" id="sessBtn">List Sessions</button>
    <button class="btn-secondary" id="clearBtn">Clear</button>
  </div>
</div>
<script>
const out = document.getElementById("outputBox");
const badge = document.getElementById("statusBadge");
const promptIn = document.getElementById("promptInput");
const sendBtn = document.getElementById("sendBtn");
const projBtn = document.getElementById("projBtn");
const sessBtn = document.getElementById("sessBtn");
const clearBtn = document.getElementById("clearBtn");

const loc = window.location;
const wsProtocol = loc.protocol === "https:" ? "wss:" : "ws:";
const wsUrl = wsProtocol + "//" + loc.host + "/ws";
let ws = null;

function connect() {
  ws = new WebSocket(wsUrl);
  ws.onopen = () => {
    badge.textContent = "Online";
    badge.className = "badge";
  };
  ws.onclose = () => {
    badge.textContent = "Offline";
    badge.className = "badge badge-warn";
    setTimeout(connect, 3000);
  };
  ws.onmessage = (evt) => {
    try {
      const data = JSON.parse(evt.data);
      if (data.type === "token") {
        out.textContent += data.delta || "";
      } else if (data.type === "thought") {
        out.textContent += "\n[Thought] " + (data.delta || "");
      } else if (data.type === "tool_start") {
        out.textContent += "\n[Tool Start] " + data.name + " " + (data.args || "");
      } else if (data.type === "tool_log") {
        out.textContent += "\n[Tool Log] " + data.output;
      } else if (data.type === "tool_end") {
        out.textContent += "\n[Tool End] " + data.name + " (success: " + data.success + ")";
      } else if (data.type === "done") {
        out.textContent += "\n[Done]\n";
      } else if (data.type === "projects") {
        out.textContent += "\n[Projects] " + JSON.stringify(data.projects);
      } else if (data.type === "sessions") {
        out.textContent += "\n[Sessions] " + JSON.stringify(data.sessions);
      }
      out.scrollTop = out.scrollHeight;
    } catch (e) {
      out.textContent += "\n" + evt.data;
    }
  };
}

sendBtn.onclick = () => {
  const txt = promptIn.value.trim();
  if (!txt || !ws || ws.readyState !== WebSocket.OPEN) return;
  out.textContent += "\n> " + txt + "\n";
  ws.send(JSON.stringify({ type: "prompt", content: txt }));
  promptIn.value = "";
};

promptIn.onkeydown = (e) => {
  if (e.key === "Enter") sendBtn.click();
};

projBtn.onclick = () => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "list_projects" }));
  }
};

sessBtn.onclick = () => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "list_sessions" }));
  }
};

clearBtn.onclick = () => {
  out.textContent = "";
};

connect();
</script>
</body>
</html>`
