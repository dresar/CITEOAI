package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"citeoai/internal/interfaces"
	"citeoai/internal/sandbox"
)

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]interfaces.Tool
	sb    *sandbox.Sandbox
}

func NewToolRegistry(sb *sandbox.Sandbox) *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]interfaces.Tool),
		sb:    sb,
	}
}

func NewDefaultRegistry(sb *sandbox.Sandbox) *ToolRegistry {
	reg := NewToolRegistry(sb)
	reg.Register(newReadFileTool(sb))
	reg.Register(newWriteFileTool(sb))
	reg.Register(newEditFileTool(sb))
	reg.Register(newRunCommandTool(sb))
	reg.Register(newListDirectoryTool(sb))
	return reg
}

func (r *ToolRegistry) Sandbox() *sandbox.Sandbox {
	return r.sb
}

func (r *ToolRegistry) Register(t interfaces.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (interfaces.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) Definitions() []interfaces.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]interfaces.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

func (r *ToolRegistry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}

type readFileTool struct {
	sb *sandbox.Sandbox
}

func newReadFileTool(sb *sandbox.Sandbox) interfaces.Tool {
	return &readFileTool{sb: sb}
}

func (t *readFileTool) Name() string {
	return "read_file"
}

func (t *readFileTool) Description() string {
	return "Read file contents with bounds"
}

func (t *readFileTool) Definition() interfaces.ToolDefinition {
	return interfaces.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]interfaces.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Path to file",
				Required:    true,
			},
			"start_line": {
				Type:        "integer",
				Description: "Start line number (1-based)",
				Required:    false,
			},
			"end_line": {
				Type:        "integer",
				Description: "End line number (inclusive)",
				Required:    false,
			},
		},
	}
}

type readFileArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func (t *readFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args readFileArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	resolved, err := t.sb.ResolvePath(args.Path)
	if err != nil {
		return "", err
	}

	file, err := os.Open(resolved)
	if err != nil {
		return "", err
	}
	defer file.Close()

	startLine := args.StartLine
	if startLine < 1 {
		startLine = 1
	}

	maxBytes := 50 * 1024
	maxLines := 800

	var builder strings.Builder
	reader := bufio.NewReader(file)
	currentLine := 1
	linesRead := 0
	bytesRead := 0

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if currentLine >= startLine {
				if args.EndLine > 0 && currentLine > args.EndLine {
					break
				}
				if linesRead >= maxLines {
					break
				}

				lineBytes := len(line)
				if bytesRead+lineBytes > maxBytes {
					remaining := maxBytes - bytesRead
					if remaining > 0 {
						builder.WriteString(line[:remaining])
					}
					break
				}

				builder.WriteString(line)
				bytesRead += lineBytes
				linesRead++
			}
			currentLine++
		}
		if err != nil {
			break
		}
	}

	return builder.String(), nil
}

type writeFileTool struct {
	sb *sandbox.Sandbox
}

func newWriteFileTool(sb *sandbox.Sandbox) interfaces.Tool {
	return &writeFileTool{sb: sb}
}

func (t *writeFileTool) Name() string {
	return "write_file"
}

func (t *writeFileTool) Description() string {
	return "Write content to a file atomically"
}

func (t *writeFileTool) Definition() interfaces.ToolDefinition {
	return interfaces.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]interfaces.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Path to file",
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
	}
}

type writeFileArgs struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Overwrite bool   `json:"overwrite"`
}

func (t *writeFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args writeFileArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	resolved, err := t.sb.ResolvePath(args.Path)
	if err != nil {
		return "", err
	}

	if !args.Overwrite {
		if _, err := os.Stat(resolved); err == nil {
			return "", errors.New("file already exists and overwrite is false")
		}
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(dir, ".citeoai_write_*")
	if err != nil {
		return "", err
	}
	tmpName := tmpFile.Name()
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.WriteString(args.Content); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	if err := os.Rename(tmpName, resolved); err != nil {
		return "", err
	}
	cleanedUp = true

	return "file written successfully", nil
}

type editFileTool struct {
	sb *sandbox.Sandbox
}

func newEditFileTool(sb *sandbox.Sandbox) interfaces.Tool {
	return &editFileTool{sb: sb}
}

func (t *editFileTool) Name() string {
	return "edit_file"
}

func (t *editFileTool) Description() string {
	return "Edit a file via exact substring substitution"
}

func (t *editFileTool) Definition() interfaces.ToolDefinition {
	return interfaces.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]interfaces.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Path to file",
				Required:    true,
			},
			"target": {
				Type:        "string",
				Description: "Exact target substring",
				Required:    true,
			},
			"replacement": {
				Type:        "string",
				Description: "Replacement string",
				Required:    true,
			},
		},
	}
}

type editFileArgs struct {
	Path        string `json:"path"`
	Target      string `json:"target"`
	Replacement string `json:"replacement"`
}

func (t *editFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args editFileArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Target == "" {
		return "", errors.New("target cannot be empty")
	}

	resolved, err := t.sb.ResolvePath(args.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}

	content := string(data)
	if !strings.Contains(content, args.Target) {
		return "", errors.New("target string not found in file")
	}

	newContent := strings.ReplaceAll(content, args.Target, args.Replacement)

	dir := filepath.Dir(resolved)
	tmpFile, err := os.CreateTemp(dir, ".citeoai_edit_*")
	if err != nil {
		return "", err
	}
	tmpName := tmpFile.Name()
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.WriteString(newContent); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	if err := os.Rename(tmpName, resolved); err != nil {
		return "", err
	}
	cleanedUp = true

	return "file edited successfully", nil
}

type circularLogBuffer struct {
	mu       sync.Mutex
	capacity int
	data     []byte
}

func newCircularLogBuffer(capacity int) *circularLogBuffer {
	return &circularLogBuffer{
		capacity: capacity,
		data:     make([]byte, 0, capacity),
	}
}

func (b *circularLogBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n = len(p)
	if n >= b.capacity {
		b.data = make([]byte, b.capacity)
		copy(b.data, p[n-b.capacity:])
		return n, nil
	}

	needed := len(b.data) + n - b.capacity
	if needed > 0 {
		b.data = b.data[needed:]
	}
	b.data = append(b.data, p...)
	return n, nil
}

func (b *circularLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

type runCommandTool struct {
	sb *sandbox.Sandbox
}

func newRunCommandTool(sb *sandbox.Sandbox) interfaces.Tool {
	return &runCommandTool{sb: sb}
}

func (t *runCommandTool) Name() string {
	return "run_command"
}

func (t *runCommandTool) Description() string {
	return "Run shell command with timeout and circular buffer log"
}

func (t *runCommandTool) Definition() interfaces.ToolDefinition {
	return interfaces.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]interfaces.ToolParameter{
			"command": {
				Type:        "string",
				Description: "Shell command string",
				Required:    true,
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: "Execution timeout in seconds",
				Required:    false,
			},
			"is_background": {
				Type:        "boolean",
				Description: "Run asynchronously in background",
				Required:    false,
			},
		},
	}
}

type runCommandArgs struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	IsBackground   bool   `json:"is_background"`
}

func (t *runCommandTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args runCommandArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	cmdStr := strings.TrimSpace(args.Command)
	if cmdStr == "" {
		return "", errors.New("command cannot be empty")
	}

	timeoutSec := args.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(runCtx, "/bin/sh", "-c", cmdStr)
	}

	cmd.Dir = t.sb.WorkspaceRoot()

	if args.IsBackground {
		if err := cmd.Start(); err != nil {
			return "", err
		}
		return fmt.Sprintf("process started in background with PID %d", cmd.Process.Pid), nil
	}

	buf := newCircularLogBuffer(64 * 1024)
	cmd.Stdout = buf
	cmd.Stderr = buf

	err := cmd.Run()
	output := buf.String()
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return output, fmt.Errorf("command timed out after %d seconds: %w", timeoutSec, err)
		}
		return output, fmt.Errorf("command execution failed: %w", err)
	}

	return output, nil
}

type listDirectoryTool struct {
	sb *sandbox.Sandbox
}

func newListDirectoryTool(sb *sandbox.Sandbox) interfaces.Tool {
	return &listDirectoryTool{sb: sb}
}

func (t *listDirectoryTool) Name() string {
	return "list_directory"
}

func (t *listDirectoryTool) Description() string {
	return "List directory contents with automatic filters"
}

func (t *listDirectoryTool) Definition() interfaces.ToolDefinition {
	return interfaces.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]interfaces.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Directory path",
				Required:    false,
			},
			"recursive": {
				Type:        "boolean",
				Description: "Recursive listing",
				Required:    false,
			},
			"max_depth": {
				Type:        "integer",
				Description: "Maximum recursion depth",
				Required:    false,
			},
		},
	}
}

type listDirectoryArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	MaxDepth  int    `json:"max_depth"`
}

func (t *listDirectoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args listDirectoryArgs
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}

	targetPath := strings.TrimSpace(args.Path)
	if targetPath == "" {
		targetPath = "."
	}

	resolved, err := t.sb.ResolvePath(targetPath)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		return "", errors.New("path is not a directory")
	}

	maxDepth := args.MaxDepth
	if !args.Recursive {
		maxDepth = 1
	} else if maxDepth <= 0 {
		maxDepth = 10
	}

	var builder strings.Builder

	err = filepath.WalkDir(resolved, func(currPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if currPath == resolved {
			return nil
		}

		rel, err := filepath.Rel(resolved, currPath)
		if err != nil {
			return err
		}

		cleanRel := filepath.ToSlash(rel)
		parts := strings.Split(cleanRel, "/")

		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
		}

		if len(parts) > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			builder.WriteString(fmt.Sprintf("[DIR]  %s\n", cleanRel))
		} else {
			var size int64
			if info, err := d.Info(); err == nil {
				size = info.Size()
			}
			builder.WriteString(fmt.Sprintf("[FILE] %s (%d bytes)\n", cleanRel, size))
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(builder.String(), "\n"), nil
}
