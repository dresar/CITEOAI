package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"citeoai/internal/sandbox"
)

func setupTestTools(t *testing.T) (*sandbox.Sandbox, *ToolRegistry, string) {
	tempDir := t.TempDir()
	sb := sandbox.NewSandbox(tempDir, []string{"."}, []string{".ssh", ".gnupg"})
	reg := NewDefaultRegistry(sb)
	return sb, reg, tempDir
}

func TestToolRegistry(t *testing.T) {
	_, reg, _ := setupTestTools(t)

	expectedTools := []string{
		"edit_file",
		"list_directory",
		"read_file",
		"run_command",
		"write_file",
	}

	defs := reg.Definitions()
	if len(defs) != len(expectedTools) {
		t.Fatalf("expected %d tool definitions, got %d", len(expectedTools), len(defs))
	}

	for i, expected := range expectedTools {
		if defs[i].Name != expected {
			t.Errorf("expected definition %d to be %s, got %s", i, expected, defs[i].Name)
		}
		if _, ok := reg.Get(expected); !ok {
			t.Errorf("tool %s not found in registry", expected)
		}
	}

	_, err := reg.Execute(context.Background(), "unknown_tool", "{}")
	if err == nil {
		t.Fatal("expected error executing unknown tool, got nil")
	}
}

func TestReadFileTool(t *testing.T) {
	_, reg, dir := setupTestTools(t)

	var sb strings.Builder
	for i := 1; i <= 20; i++ {
		sb.WriteString(fmt.Sprintf("line %d\n", i))
	}
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte(sb.String()), 0600); err != nil {
		t.Fatal(err)
	}

	argsJSON, _ := json.Marshal(map[string]any{
		"path":       "test.txt",
		"start_line": 5,
		"end_line":   8,
	})
	res, err := reg.Execute(context.Background(), "read_file", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "line 5\nline 6\nline 7\nline 8\n"
	if res != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, res)
	}

	argsJSON, _ = json.Marshal(map[string]any{
		"path": "../outside.txt",
	})
	_, err = reg.Execute(context.Background(), "read_file", string(argsJSON))
	if err == nil {
		t.Fatal("expected error for path outside sandbox, got nil")
	}
}

func TestReadFileToolLimits(t *testing.T) {
	_, reg, dir := setupTestTools(t)

	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		sb.WriteString(fmt.Sprintf("line %04d: %s\n", i, strings.Repeat("a", 100)))
	}
	filePath := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(filePath, []byte(sb.String()), 0600); err != nil {
		t.Fatal(err)
	}

	argsJSON, _ := json.Marshal(map[string]any{
		"path": "large.txt",
	})
	res, err := reg.Execute(context.Background(), "read_file", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) > 50*1024 {
		t.Fatalf("expected max 50KB, got %d bytes", len(res))
	}
}

func TestWriteFileTool(t *testing.T) {
	_, reg, dir := setupTestTools(t)

	argsJSON, _ := json.Marshal(map[string]any{
		"path":      "nested/output.txt",
		"content":   "hello citeoai",
		"overwrite": false,
	})
	res, err := reg.Execute(context.Background(), "write_file", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, "successfully") {
		t.Fatalf("unexpected result: %s", res)
	}

	data, err := os.ReadFile(filepath.Join(dir, "nested", "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello citeoai" {
		t.Fatalf("expected 'hello citeoai', got '%s'", string(data))
	}

	_, err = reg.Execute(context.Background(), "write_file", string(argsJSON))
	if err == nil {
		t.Fatal("expected error when overwrite is false and file exists")
	}

	argsJSON, _ = json.Marshal(map[string]any{
		"path":      "nested/output.txt",
		"content":   "overwritten content",
		"overwrite": true,
	})
	_, err = reg.Execute(context.Background(), "write_file", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error when overwrite is true: %v", err)
	}

	data, _ = os.ReadFile(filepath.Join(dir, "nested", "output.txt"))
	if string(data) != "overwritten content" {
		t.Fatalf("expected 'overwritten content', got '%s'", string(data))
	}
}

func TestEditFileTool(t *testing.T) {
	_, reg, dir := setupTestTools(t)

	filePath := filepath.Join(dir, "source.txt")
	initialContent := "func oldName() string {\n\treturn \"old\"\n}\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0600); err != nil {
		t.Fatal(err)
	}

	argsJSON, _ := json.Marshal(map[string]any{
		"path":        "source.txt",
		"target":      "oldName",
		"replacement": "newName",
	})
	res, err := reg.Execute(context.Background(), "edit_file", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, "successfully") {
		t.Fatalf("unexpected result: %s", res)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	expected := "func newName() string {\n\treturn \"old\"\n}\n"
	if string(data) != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, string(data))
	}

	argsJSON, _ = json.Marshal(map[string]any{
		"path":        "source.txt",
		"target":      "not_in_file",
		"replacement": "foo",
	})
	_, err = reg.Execute(context.Background(), "edit_file", string(argsJSON))
	if err == nil {
		t.Fatal("expected error when target not found, got nil")
	}
}

func TestRunCommandTool(t *testing.T) {
	_, reg, _ := setupTestTools(t)

	argsJSON, _ := json.Marshal(map[string]any{
		"command":         "echo 'citeoai command test'",
		"timeout_seconds": 5,
	})
	res, err := reg.Execute(context.Background(), "run_command", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, "citeoai command test") {
		t.Fatalf("expected output to contain 'citeoai command test', got '%s'", res)
	}
}

func TestRunCommandCircularBuffer(t *testing.T) {
	buf := newCircularLogBuffer(64 * 1024)
	largeData := []byte(strings.Repeat("a", 100*1024))
	n, err := buf.Write(largeData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(largeData) {
		t.Fatalf("expected n to be %d, got %d", len(largeData), n)
	}

	out := buf.String()
	if len(out) != 64*1024 {
		t.Fatalf("expected output length %d, got %d", 64*1024, len(out))
	}

	buf2 := newCircularLogBuffer(10)
	_, _ = buf2.Write([]byte("12345"))
	_, _ = buf2.Write([]byte("67890"))
	_, _ = buf2.Write([]byte("abcdef"))
	out2 := buf2.String()
	if len(out2) != 10 {
		t.Fatalf("expected len 10, got %d: %s", len(out2), out2)
	}
	if out2 != "7890abcdef" {
		t.Fatalf("expected '7890abcdef', got '%s'", out2)
	}
}

func TestRunCommandBackground(t *testing.T) {
	_, reg, _ := setupTestTools(t)

	argsJSON, _ := json.Marshal(map[string]any{
		"command":       "sleep 1",
		"is_background": true,
	})
	res, err := reg.Execute(context.Background(), "run_command", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, "background with PID") {
		t.Fatalf("expected background PID response, got '%s'", res)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	_, reg, _ := setupTestTools(t)

	argsJSON, _ := json.Marshal(map[string]any{
		"command":         "sleep 5",
		"timeout_seconds": 1,
	})
	_, err := reg.Execute(context.Background(), "run_command", string(argsJSON))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestListDirectoryTool(t *testing.T) {
	_, reg, dir := setupTestTools(t)

	_ = os.MkdirAll(filepath.Join(dir, "src", "sub"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "vendor", "lib"), 0755)

	_ = os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package main"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "src", "sub", "deep.go"), []byte("package sub"), 0600)
	_ = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("git config"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("js"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "vendor", "lib", "lib.go"), []byte("lib"), 0600)

	argsJSON, _ := json.Marshal(map[string]any{
		"recursive": true,
		"max_depth": 5,
	})
	res, err := reg.Execute(context.Background(), "list_directory", string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res, "root.txt") {
		t.Fatalf("expected output to contain root.txt: %s", res)
	}
	if !strings.Contains(res, "src/app.go") {
		t.Fatalf("expected output to contain src/app.go: %s", res)
	}
	if !strings.Contains(res, "src/sub/deep.go") {
		t.Fatalf("expected output to contain src/sub/deep.go: %s", res)
	}
	if strings.Contains(res, ".git") {
		t.Fatalf("expected .git to be filtered: %s", res)
	}
	if strings.Contains(res, "node_modules") {
		t.Fatalf("expected node_modules to be filtered: %s", res)
	}
	if strings.Contains(res, "vendor") {
		t.Fatalf("expected vendor to be filtered: %s", res)
	}
}
