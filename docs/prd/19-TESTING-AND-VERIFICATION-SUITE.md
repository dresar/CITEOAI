# 19 - TESTING AND VERIFICATION SUITE

## 1. Testing Hierarchy
1. **Unit Tests**: Test each package (`config_test.go`, `memory_test.go`, `provider_test.go`) in isolation.
2. **Integration Tests**: Mocked HTTP servers simulating Gemini and OpenAI SSE streams.
3. **Sandboxing Tests**: Attempting path traversal (`../../etc/shadow`, `~/.ssh`) and asserting `ErrPermissionDenied`.
4. **Zero-Comment Linter**: Automated test scanning the codebase for forbidden `//` or `/*` comment tokens.

## 2. Automated Comment Verifier Script
```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	violations := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
				fmt.Printf("Comment violation at %s:%d\n", path, lineNum)
				violations++
			}
		}
		return nil
	})
	if err != nil || violations > 0 {
		os.Exit(1)
	}
}
```

## 3. Running All Tests
```bash
go vet ./...
go test -v -race ./...
```
