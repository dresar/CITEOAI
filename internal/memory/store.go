package memory

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"citeoai/internal/config"
)

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrEmptyContent = errors.New("content cannot be empty")
)

type MemoryNote struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Category        string    `json:"category"`
	Tags            []string  `json:"tags"`
	Content         string    `json:"content"`
	SourceSessionID string    `json:"source_session_id"`
	Tombstoned      bool      `json:"tombstoned,omitempty"`
}

type MemoryStore struct {
	mu       sync.RWMutex
	filePath string
	notes    map[string]*MemoryNote
	order    []string
}

func Open(path string) (*MemoryStore, error) {
	resolvedPath := strings.TrimSpace(path)
	if resolvedPath == "" {
		home := config.UserHome()
		if home == "" {
			resolvedPath = filepath.Join(".", "notes.jsonl")
		} else {
			resolvedPath = filepath.Join(home, "memory", "notes.jsonl")
		}
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	store := &MemoryStore{
		filePath: resolvedPath,
		notes:    make(map[string]*MemoryNote),
		order:    make([]string, 0),
	}

	if err := store.loadExisting(); err != nil {
		return nil, err
	}

	return store, nil
}

func (m *MemoryStore) FilePath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filePath
}

func (m *MemoryStore) loadExisting() error {
	file, err := os.Open(m.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var note MemoryNote
		if err := json.Unmarshal([]byte(line), &note); err != nil {
			continue
		}

		if _, exists := m.notes[note.ID]; !exists {
			m.order = append(m.order, note.ID)
		}
		noteCopy := note
		m.notes[note.ID] = &noteCopy
	}

	return scanner.Err()
}

func generateNoteID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mem_%x_%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

func (m *MemoryStore) AddNote(category string, tags []string, content, sessionID string) (*MemoryNote, error) {
	cleanContent := strings.TrimSpace(content)
	if cleanContent == "" {
		return nil, ErrEmptyContent
	}

	now := time.Now().UTC()
	var cleanTags []string
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}

	note := &MemoryNote{
		ID:              generateNoteID(),
		CreatedAt:       now,
		UpdatedAt:       now,
		Category:        strings.TrimSpace(category),
		Tags:            cleanTags,
		Content:         cleanContent,
		SourceSessionID: strings.TrimSpace(sessionID),
		Tombstoned:      false,
	}

	data, err := json.Marshal(note)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	m.mu.Lock()
	defer m.mu.Unlock()

	f, err := os.OpenFile(m.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return nil, err
	}

	m.notes[note.ID] = note
	m.order = append(m.order, note.ID)

	noteCopy := *note
	return &noteCopy, nil
}

func (m *MemoryStore) Forget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	note, exists := m.notes[id]
	if !exists || note.Tombstoned {
		return ErrNoteNotFound
	}

	note.Tombstoned = true
	note.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(m.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}

func (m *MemoryStore) List() []MemoryNote {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []MemoryNote
	for _, id := range m.order {
		if note, ok := m.notes[id]; ok && !note.Tombstoned {
			results = append(results, *note)
		}
	}
	return results
}

func (m *MemoryStore) Compact() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := filepath.Dir(m.filePath)
	tmpFile, err := os.CreateTemp(dir, ".notes_compact_*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			_ = os.Remove(tmpName)
		}
	}()

	var newOrder []string
	newNotes := make(map[string]*MemoryNote)

	for _, id := range m.order {
		note, ok := m.notes[id]
		if ok && !note.Tombstoned {
			data, err := json.Marshal(note)
			if err != nil {
				_ = tmpFile.Close()
				return err
			}
			data = append(data, '\n')
			if _, err := tmpFile.Write(data); err != nil {
				_ = tmpFile.Close()
				return err
			}
			newOrder = append(newOrder, id)
			newNotes[id] = note
		}
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, m.filePath); err != nil {
		return err
	}
	cleanedUp = true

	m.notes = newNotes
	m.order = newOrder
	return nil
}

func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

type scoredNote struct {
	note  MemoryNote
	score float64
}

func (m *MemoryStore) Search(query string, limit int) []MemoryNote {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []MemoryNote{}
	}

	queryTokens := tokenize(cleanQuery)
	if len(queryTokens) == 0 {
		return []MemoryNote{}
	}

	m.mu.RLock()
	var activeNotes []*MemoryNote
	for _, id := range m.order {
		if note, ok := m.notes[id]; ok && !note.Tombstoned {
			activeNotes = append(activeNotes, note)
		}
	}
	m.mu.RUnlock()

	totalDocs := len(activeNotes)
	if totalDocs == 0 {
		return []MemoryNote{}
	}

	docTokens := make([][]string, totalDocs)
	docLengths := make([]int, totalDocs)
	totalLength := 0

	for i, note := range activeNotes {
		fullText := note.Content + " " + note.Category + " " + strings.Join(note.Tags, " ")
		tokens := tokenize(fullText)
		docTokens[i] = tokens
		docLengths[i] = len(tokens)
		totalLength += len(tokens)
	}

	avgDocLength := float64(totalLength) / float64(totalDocs)
	if avgDocLength == 0 {
		avgDocLength = 1.0
	}

	docFreq := make(map[string]int)
	for _, qTerm := range queryTokens {
		count := 0
		for _, tokens := range docTokens {
			for _, t := range tokens {
				if t == qTerm {
					count++
					break
				}
			}
		}
		docFreq[qTerm] = count
	}

	const k1 = 1.2
	const b = 0.75

	var scored []scoredNote

	for i, note := range activeNotes {
		termCounts := make(map[string]int)
		for _, t := range docTokens[i] {
			termCounts[t]++
		}

		score := 0.0
		matched := false

		for _, qTerm := range queryTokens {
			df := docFreq[qTerm]
			if df == 0 {
				continue
			}

			tf := float64(termCounts[qTerm])
			if tf == 0 {
				continue
			}
			matched = true

			idf := math.Log(((float64(totalDocs) - float64(df) + 0.5) / (float64(df) + 0.5)) + 1.0)
			numerator := tf * (k1 + 1.0)
			denominator := tf + k1*(1.0-b+b*(float64(docLengths[i])/avgDocLength))
			score += idf * (numerator / denominator)

			for _, tag := range note.Tags {
				if strings.EqualFold(tag, qTerm) {
					score += 1.5
				}
			}

			if strings.EqualFold(note.Category, qTerm) {
				score += 1.0
			}
		}

		if matched && score > 0 {
			ageHours := time.Since(note.UpdatedAt).Hours()
			recencyBoost := 1.0 / (1.0 + (ageHours / 720.0))
			finalScore := score * (1.0 + 0.1*recencyBoost)

			scored = append(scored, scoredNote{
				note:  *note,
				score: finalScore,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	resultCount := len(scored)
	if limit > 0 && resultCount > limit {
		resultCount = limit
	}

	results := make([]MemoryNote, resultCount)
	for i := 0; i < resultCount; i++ {
		results[i] = scored[i].note
	}

	return results
}
