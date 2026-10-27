package tests

import (
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root not found")
		}
		dir = parent
	}
}

func TestZeroCommentsInGoCodebase(t *testing.T) {
	root := findRepoRoot(t)

	var goFiles []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(d.Name(), ".go") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk repository: %v", err)
	}

	if len(goFiles) < 10 {
		t.Fatalf("too few go files discovered: %d", len(goFiles))
	}

	var commentViolations []string

	for _, filePath := range goFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", filePath, err)
		}

		fset := token.NewFileSet()
		f := fset.AddFile(filePath, fset.Base(), len(content))

		var s scanner.Scanner
		s.Init(f, content, nil, scanner.ScanComments)

		for {
			pos, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			if tok == token.COMMENT {
				relPath, _ := filepath.Rel(root, filePath)
				commentViolations = append(commentViolations, relPath+":"+fset.Position(pos).String()+": "+lit)
			}
		}
	}

	if len(commentViolations) > 0 {
		t.Errorf("NOKOMEN violation! Found %d forbidden comments in Go code:\n%s",
			len(commentViolations),
			strings.Join(commentViolations, "\n"))
	}
}
