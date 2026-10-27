package skill

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"citeoai/internal/config"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"`
	Body        string   `json:"body"`
	Path        string   `json:"path"`
}

func ValidateSkillName(name string) bool {
	return skillNamePattern.MatchString(name)
}

func ParseSkillFile(filePath string) (*Skill, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	content := string(data)
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, errors.New("missing opening frontmatter delimiter")
	}

	closingIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closingIndex = i
			break
		}
	}

	if closingIndex == -1 {
		return nil, errors.New("missing closing frontmatter delimiter")
	}

	frontmatterLines := lines[1:closingIndex]
	bodyLines := lines[closingIndex+1:]
	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))

	var name, description string
	var triggers []string
	inTriggers := false

	for _, rawLine := range frontmatterLines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "name:") {
			inTriggers = false
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			name = strings.Trim(val, "\"'")
		} else if strings.HasPrefix(trimmed, "description:") {
			inTriggers = false
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			description = strings.Trim(val, "\"'")
		} else if strings.HasPrefix(trimmed, "triggers:") {
			inTriggers = true
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "triggers:"))
			if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				inner := strings.Trim(val, "[]")
				parts := strings.Split(inner, ",")
				for _, p := range parts {
					clean := strings.Trim(strings.TrimSpace(p), "\"'")
					if clean != "" {
						triggers = append(triggers, clean)
					}
				}
				inTriggers = false
			}
		} else if inTriggers && strings.HasPrefix(trimmed, "-") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			val = strings.Trim(val, "\"'")
			if val != "" {
				triggers = append(triggers, val)
			}
		} else {
			inTriggers = false
		}
	}

	if !ValidateSkillName(name) {
		return nil, fmt.Errorf("invalid skill name: %s", name)
	}

	if len(description) > 1024 {
		return nil, errors.New("skill description exceeds 1024 characters")
	}

	return &Skill{
		Name:        name,
		Description: description,
		Triggers:    triggers,
		Body:        body,
		Path:        filePath,
	}, nil
}

func DefaultDiscoveryRoots() []string {
	var roots []string
	roots = append(roots, filepath.Join(".", ".citeoai", "skills"))
	roots = append(roots, filepath.Join(".", "skills"))

	home := config.UserHome()
	if home != "" {
		roots = append(roots, filepath.Join(home, "skills"))
	}

	userHome, err := os.UserHomeDir()
	if err == nil && userHome != "" {
		roots = append(roots, filepath.Join(userHome, ".claude", "skills"))
	}

	return roots
}

func DiscoverSkills(roots []string) ([]Skill, error) {
	searchRoots := roots
	if len(searchRoots) == 0 {
		searchRoots = DefaultDiscoveryRoots()
	}

	seenNames := make(map[string]bool)
	var discovered []Skill

	userHome, _ := os.UserHomeDir()

	for _, root := range searchRoots {
		expandedRoot := strings.TrimSpace(root)
		if expandedRoot == "" {
			continue
		}

		if strings.HasPrefix(expandedRoot, "~/") && userHome != "" {
			expandedRoot = filepath.Join(userHome, expandedRoot[2:])
		} else if expandedRoot == "~" && userHome != "" {
			expandedRoot = userHome
		}

		stat, err := os.Stat(expandedRoot)
		if err != nil || !stat.IsDir() {
			continue
		}

		_ = filepath.WalkDir(expandedRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}

			if strings.EqualFold(d.Name(), "SKILL.md") {
				skill, err := ParseSkillFile(path)
				if err == nil && skill != nil {
					if !seenNames[skill.Name] {
						seenNames[skill.Name] = true
						discovered = append(discovered, *skill)
					}
				}
			}

			return nil
		})
	}

	return discovered, nil
}

func tokenizeWords(text string) []string {
	var words []string
	var current strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func MatchSkills(skills []Skill, query string) []Skill {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []Skill{}
	}

	queryLower := strings.ToLower(cleanQuery)
	queryTokens := tokenizeWords(cleanQuery)
	tokenSet := make(map[string]bool)
	for _, t := range queryTokens {
		tokenSet[t] = true
	}

	var matched []Skill

	for _, s := range skills {
		skillNameLower := strings.ToLower(s.Name)
		if tokenSet[skillNameLower] || strings.Contains(queryLower, skillNameLower) {
			matched = append(matched, s)
			continue
		}

		triggerMatched := false
		for _, trigger := range s.Triggers {
			trigLower := strings.ToLower(strings.TrimSpace(trigger))
			if trigLower == "" {
				continue
			}
			if tokenSet[trigLower] || strings.Contains(queryLower, trigLower) {
				triggerMatched = true
				break
			}
		}

		if triggerMatched {
			matched = append(matched, s)
			continue
		}

		descTokens := tokenizeWords(s.Description)
		matchingTokens := 0
		for _, dt := range descTokens {
			if tokenSet[dt] {
				matchingTokens++
			}
		}

		if len(queryTokens) > 0 && matchingTokens >= 2 {
			matched = append(matched, s)
		}
	}

	return matched
}
