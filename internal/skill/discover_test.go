package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	valid := []string{
		"go-build",
		"verify-test",
		"singleword",
		"tool-123-build",
		"a-b-c",
	}

	for _, name := range valid {
		if !ValidateSkillName(name) {
			t.Errorf("expected %s to be valid", name)
		}
	}

	invalid := []string{
		"UpperCase",
		"-leading-hyphen",
		"trailing-hyphen-",
		"double--hyphen",
		"under_score",
		"with spaces",
		"",
	}

	for _, name := range invalid {
		if ValidateSkillName(name) {
			t.Errorf("expected %s to be invalid", name)
		}
	}
}

func TestParseSkillFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "SKILL.md")

	validContent := `---
name: go-build-verification
description: Automatically verifies and compiles Go packages with race detector and vet.
triggers:
  - build
  - compile
  - test
---

# Go Build Verification Skill

## Execution Rules
Run go vet and go test before asserting task completion.
`
	if err := os.WriteFile(filePath, []byte(validContent), 0600); err != nil {
		t.Fatal(err)
	}

	sk, err := ParseSkillFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error parsing valid skill: %v", err)
	}

	if sk.Name != "go-build-verification" {
		t.Fatalf("expected name go-build-verification, got %s", sk.Name)
	}
	if len(sk.Triggers) != 3 {
		t.Fatalf("expected 3 triggers, got %d", len(sk.Triggers))
	}
	if !strings.Contains(sk.Body, "Go Build Verification Skill") {
		t.Fatalf("unexpected body: %s", sk.Body)
	}

	inlinePath := filepath.Join(tempDir, "INLINE_SKILL.md")
	inlineContent := `---
name: inline-triggers
description: Short description
triggers: [lint, style]
---
Body text
`
	if err := os.WriteFile(inlinePath, []byte(inlineContent), 0600); err != nil {
		t.Fatal(err)
	}

	inlineSk, err := ParseSkillFile(inlinePath)
	if err != nil {
		t.Fatalf("unexpected error parsing inline skill: %v", err)
	}
	if len(inlineSk.Triggers) != 2 || inlineSk.Triggers[0] != "lint" {
		t.Fatalf("unexpected triggers: %v", inlineSk.Triggers)
	}

	invalidNamePath := filepath.Join(tempDir, "INVALID_NAME.md")
	invalidNameContent := `---
name: Invalid_Name
description: Test
triggers:
  - test
---
Body
`
	_ = os.WriteFile(invalidNamePath, []byte(invalidNameContent), 0600)
	_, err = ParseSkillFile(invalidNamePath)
	if err == nil {
		t.Fatal("expected error on invalid skill name, got nil")
	}

	longDescPath := filepath.Join(tempDir, "LONG_DESC.md")
	longDescContent := "---\nname: long-desc\ndescription: " + strings.Repeat("x", 1025) + "\n---\nBody\n"
	_ = os.WriteFile(longDescPath, []byte(longDescContent), 0600)
	_, err = ParseSkillFile(longDescPath)
	if err == nil {
		t.Fatal("expected error on description > 1024 chars, got nil")
	}
}

func TestDiscoverSkills(t *testing.T) {
	tempDir := t.TempDir()
	skillDir1 := filepath.Join(tempDir, "pack1", "build-skill")
	skillDir2 := filepath.Join(tempDir, "pack2", "test-skill")
	gitDir := filepath.Join(tempDir, ".git", "ignored-skill")

	_ = os.MkdirAll(skillDir1, 0755)
	_ = os.MkdirAll(skillDir2, 0755)
	_ = os.MkdirAll(gitDir, 0755)

	content1 := "---\nname: build-skill\ndescription: Build helper\ntriggers:\n  - build\n---\nBody 1\n"
	content2 := "---\nname: test-skill\ndescription: Test helper\ntriggers:\n  - test\n---\nBody 2\n"
	contentGit := "---\nname: git-skill\ndescription: Should be ignored\n---\nBody Git\n"

	_ = os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte(content1), 0600)
	_ = os.WriteFile(filepath.Join(skillDir2, "skill.md"), []byte(content2), 0600)
	_ = os.WriteFile(filepath.Join(gitDir, "SKILL.md"), []byte(contentGit), 0600)

	skills, err := DiscoverSkills([]string{tempDir})
	if err != nil {
		t.Fatalf("unexpected error discovering skills: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	names := map[string]bool{skills[0].Name: true, skills[1].Name: true}
	if !names["build-skill"] || !names["test-skill"] {
		t.Fatalf("expected build-skill and test-skill, got %v", names)
	}
}

func TestMatchSkills(t *testing.T) {
	skills := []Skill{
		{
			Name:        "go-build",
			Description: "Compile Go packages with compiler flags",
			Triggers:    []string{"build", "compile"},
		},
		{
			Name:        "docker-deploy",
			Description: "Containerize application with Dockerfile",
			Triggers:    []string{"docker", "container"},
		},
	}

	match1 := MatchSkills(skills, "I want to build my project")
	if len(match1) != 1 || match1[0].Name != "go-build" {
		t.Fatalf("expected go-build match, got %v", match1)
	}

	match2 := MatchSkills(skills, "Deploy with docker")
	if len(match2) != 1 || match2[0].Name != "docker-deploy" {
		t.Fatalf("expected docker-deploy match, got %v", match2)
	}

	match3 := MatchSkills(skills, "go-build")
	if len(match3) != 1 || match3[0].Name != "go-build" {
		t.Fatalf("expected go-build direct match, got %v", match3)
	}

	noMatch := MatchSkills(skills, "something totally unrelated")
	if len(noMatch) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(noMatch))
	}

	emptyMatch := MatchSkills(skills, "   ")
	if len(emptyMatch) != 0 {
		t.Fatalf("expected 0 matches for empty query, got %d", len(emptyMatch))
	}
}
