package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSandboxDefaultAllowed(t *testing.T) {
	tempDir := t.TempDir()
	sb := NewSandbox(tempDir, nil, []string{".ssh", ".gnupg"})

	if sb.WorkspaceRoot() != tempDir {
		t.Fatalf("expected %s, got %s", tempDir, sb.WorkspaceRoot())
	}

	allowed := sb.AllowedPaths()
	if len(allowed) != 1 || allowed[0] != tempDir {
		t.Fatalf("expected allowed paths to contain %s, got %v", tempDir, allowed)
	}
}

func TestResolvePathValid(t *testing.T) {
	tempDir := t.TempDir()
	sb := NewSandbox(tempDir, []string{"."}, []string{".ssh", ".gnupg"})

	resolved, err := sb.ResolvePath("foo/bar.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "foo", "bar.txt")
	if resolved != expected {
		t.Fatalf("expected %s, got %s", expected, resolved)
	}
}

func TestResolvePathTraversalBlocked(t *testing.T) {
	tempDir := t.TempDir()
	sb := NewSandbox(tempDir, []string{"."}, []string{".ssh", ".gnupg"})

	traversalPaths := []string{
		"../../etc/passwd",
		"../secret.txt",
		"foo/../../bar",
		filepath.Join(tempDir, "..", "outside.txt"),
	}

	for _, p := range traversalPaths {
		_, err := sb.ResolvePath(p)
		if err == nil {
			t.Fatalf("expected error for path %s, got nil", p)
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied for path %s, got %v", p, err)
		}
	}
}

func TestResolvePathDeniedPaths(t *testing.T) {
	tempDir := t.TempDir()
	deniedDir := t.TempDir()
	sb := NewSandbox(tempDir, []string{tempDir, deniedDir}, []string{".ssh", ".gnupg", deniedDir})

	deniedAttempts := []string{
		".ssh/id_rsa",
		"nested/.ssh/key",
		".gnupg/secring.gpg",
		filepath.Join(deniedDir, "secret.key"),
	}

	for _, p := range deniedAttempts {
		_, err := sb.ResolvePath(p)
		if err == nil {
			t.Fatalf("expected error for denied path %s, got nil", p)
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied for path %s, got %v", p, err)
		}
	}
}

func TestResolvePathEmpty(t *testing.T) {
	tempDir := t.TempDir()
	sb := NewSandbox(tempDir, nil, nil)

	_, err := sb.ResolvePath("   ")
	if !errors.Is(err, ErrEmptyPath) {
		t.Fatalf("expected ErrEmptyPath, got %v", err)
	}
}

func TestSymlinkEscapeBlocked(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(tempDir, "link_to_outside")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	sb := NewSandbox(tempDir, []string{"."}, []string{".ssh"})
	_, err := sb.ResolvePath("link_to_outside/secret.txt")
	if err == nil {
		t.Fatal("expected error resolving symlink pointing outside sandbox, got nil")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}
