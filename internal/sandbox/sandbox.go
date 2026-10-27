package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrAccessDenied = errors.New("access denied")
	ErrEmptyPath    = errors.New("empty path")
	ErrPathNotFound = errors.New("path not found")
)

type Sandbox struct {
	workspaceRoot string
	allowedPaths  []string
	deniedPaths   []string
}

func NewSandbox(workspaceRoot string, allowedPaths, deniedPaths []string) *Sandbox {
	cleanRoot := filepath.Clean(workspaceRoot)
	if absRoot, err := filepath.Abs(cleanRoot); err == nil {
		cleanRoot = absRoot
	}

	var cleanAllowed []string
	if len(allowedPaths) == 0 {
		cleanAllowed = append(cleanAllowed, cleanRoot)
	} else {
		for _, p := range allowedPaths {
			trimmed := strings.TrimSpace(p)
			if trimmed == "" {
				continue
			}
			if !filepath.IsAbs(trimmed) {
				trimmed = filepath.Join(cleanRoot, trimmed)
			}
			cleanAllowed = append(cleanAllowed, filepath.Clean(trimmed))
		}
	}

	var cleanDenied []string
	homeDir, _ := os.UserHomeDir()
	for _, p := range deniedPaths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "~/") && homeDir != "" {
			trimmed = filepath.Join(homeDir, trimmed[2:])
		} else if trimmed == "~" && homeDir != "" {
			trimmed = homeDir
		}
		cleanDenied = append(cleanDenied, filepath.Clean(trimmed))
	}

	return &Sandbox{
		workspaceRoot: cleanRoot,
		allowedPaths:  cleanAllowed,
		deniedPaths:   cleanDenied,
	}
}

func (s *Sandbox) WorkspaceRoot() string {
	return s.workspaceRoot
}

func (s *Sandbox) AllowedPaths() []string {
	result := make([]string, len(s.allowedPaths))
	copy(result, s.allowedPaths)
	return result
}

func (s *Sandbox) DeniedPaths() []string {
	result := make([]string, len(s.deniedPaths))
	copy(result, s.deniedPaths)
	return result
}

func (s *Sandbox) ResolvePath(inputPath string) (string, error) {
	trimmed := strings.TrimSpace(inputPath)
	if trimmed == "" {
		return "", ErrEmptyPath
	}

	target := filepath.Clean(trimmed)
	if !filepath.IsAbs(target) {
		target = filepath.Join(s.workspaceRoot, target)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	absTarget = filepath.Clean(absTarget)

	if err := s.ValidatePath(absTarget); err != nil {
		return "", err
	}

	return absTarget, nil
}

func (s *Sandbox) ValidatePath(inputPath string) error {
	trimmed := strings.TrimSpace(inputPath)
	if trimmed == "" {
		return ErrEmptyPath
	}

	target := filepath.Clean(trimmed)
	if !filepath.IsAbs(target) {
		target = filepath.Join(s.workspaceRoot, target)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	absTarget = filepath.Clean(absTarget)

	if s.isPathDenied(absTarget) {
		return ErrAccessDenied
	}

	if !s.isPathAllowed(absTarget) {
		return ErrAccessDenied
	}

	if err := s.validateSymlinks(absTarget); err != nil {
		return err
	}

	return nil
}

func (s *Sandbox) isPathDenied(target string) bool {
	segments := strings.Split(target, string(filepath.Separator))
	for _, denied := range s.deniedPaths {
		if !filepath.IsAbs(denied) {
			for _, seg := range segments {
				if seg == denied {
					return true
				}
			}
		}

		deniedAbs := denied
		if !filepath.IsAbs(deniedAbs) {
			deniedAbs = filepath.Join(s.workspaceRoot, denied)
		}
		deniedAbs = filepath.Clean(deniedAbs)

		rel, err := filepath.Rel(deniedAbs, target)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (s *Sandbox) isPathAllowed(target string) bool {
	for _, allowed := range s.allowedPaths {
		rel, err := filepath.Rel(allowed, target)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (s *Sandbox) validateSymlinks(target string) error {
	evalTarget, err := filepath.EvalSymlinks(target)
	if err == nil {
		evalTarget = filepath.Clean(evalTarget)
		if s.isPathDenied(evalTarget) || !s.isPathAllowed(evalTarget) {
			return ErrAccessDenied
		}
		return nil
	}

	curr := filepath.Dir(target)
	for {
		evalParent, err := filepath.EvalSymlinks(curr)
		if err == nil {
			evalParent = filepath.Clean(evalParent)
			if s.isPathDenied(evalParent) || !s.isPathAllowed(evalParent) {
				return ErrAccessDenied
			}
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return nil
}
