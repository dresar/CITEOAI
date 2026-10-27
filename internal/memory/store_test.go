package memory

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestMemoryStoreOpen(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "notes.jsonl")

	store, err := Open(storePath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	note, err := store.AddNote("test", []string{"go"}, "sample note content", "sess_1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	if note.ID == "" {
		t.Fatal("expected non-empty note ID")
	}

	reopened, err := Open(storePath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}

	notes := reopened.List()
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].ID != note.ID {
		t.Fatalf("expected note ID %s, got %s", note.ID, notes[0].ID)
	}
}

func TestMemoryStoreAddNote(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "notes.jsonl")
	store, _ := Open(storePath)

	note, err := store.AddNote("preference", []string{"nokomen", "rules"}, "zero comments allowed", "sess_main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if note.Category != "preference" {
		t.Fatalf("expected category preference, got %s", note.Category)
	}
	if len(note.Tags) != 2 || note.Tags[0] != "nokomen" {
		t.Fatalf("unexpected tags: %v", note.Tags)
	}
	if note.Content != "zero comments allowed" {
		t.Fatalf("unexpected content: %s", note.Content)
	}
	if note.SourceSessionID != "sess_main" {
		t.Fatalf("unexpected session ID: %s", note.SourceSessionID)
	}
	if note.Tombstoned {
		t.Fatal("new note should not be tombstoned")
	}

	_, err = store.AddNote("preference", nil, "   ", "sess_main")
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("expected ErrEmptyContent, got %v", err)
	}
}

func TestMemoryStoreForget(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "notes.jsonl")
	store, _ := Open(storePath)

	note1, _ := store.AddNote("task", []string{"todo"}, "finish task one", "s1")
	note2, _ := store.AddNote("task", []string{"todo"}, "finish task two", "s1")

	if err := store.Forget(note1.ID); err != nil {
		t.Fatalf("failed to forget note: %v", err)
	}

	err := store.Forget(note1.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound on already forgotten note, got %v", err)
	}

	err = store.Forget("non_existent_id")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound on non-existent note, got %v", err)
	}

	active := store.List()
	if len(active) != 1 {
		t.Fatalf("expected 1 active note, got %d", len(active))
	}
	if active[0].ID != note2.ID {
		t.Fatalf("expected remaining note to be %s, got %s", note2.ID, active[0].ID)
	}

	reopened, _ := Open(storePath)
	reopenedActive := reopened.List()
	if len(reopenedActive) != 1 || reopenedActive[0].ID != note2.ID {
		t.Fatalf("reopened store should not list forgotten note")
	}
}

func TestMemoryStoreCompact(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "notes.jsonl")
	store, _ := Open(storePath)

	note1, _ := store.AddNote("cat1", nil, "content one", "s1")
	note2, _ := store.AddNote("cat2", nil, "content two", "s1")
	_ = store.Forget(note1.ID)

	if err := store.Compact(); err != nil {
		t.Fatalf("failed to compact store: %v", err)
	}

	active := store.List()
	if len(active) != 1 || active[0].ID != note2.ID {
		t.Fatalf("expected 1 note after compact, got %d", len(active))
	}

	reopened, _ := Open(storePath)
	reopenedActive := reopened.List()
	if len(reopenedActive) != 1 || reopenedActive[0].ID != note2.ID {
		t.Fatalf("expected 1 note in reopened compacted store, got %d", len(reopenedActive))
	}
}

func TestMemoryStoreSearch(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "notes.jsonl")
	store, _ := Open(storePath)

	note1, _ := store.AddNote("preference", []string{"go", "nokomen"}, "Zero comments in Go source files is mandatory.", "sess_1")
	_, _ = store.AddNote("project", []string{"ui", "css"}, "Terminal UI cards use 4px border radius with light theme.", "sess_1")
	note3, _ := store.AddNote("preference", []string{"memory", "limits"}, "Target resident set size below 100MB RAM.", "sess_1")

	results := store.Search("zero comments go", 10)
	if len(results) == 0 {
		t.Fatal("expected search results, got 0")
	}
	if results[0].ID != note1.ID {
		t.Fatalf("expected top result to be %s, got %s", note1.ID, results[0].ID)
	}

	resultsMem := store.Search("memory 100MB", 1)
	if len(resultsMem) != 1 {
		t.Fatalf("expected 1 result with limit, got %d", len(resultsMem))
	}
	if resultsMem[0].ID != note3.ID {
		t.Fatalf("expected %s, got %s", note3.ID, resultsMem[0].ID)
	}

	emptyRes := store.Search("nonexistentwordxyz", 10)
	if len(emptyRes) != 0 {
		t.Fatalf("expected 0 results, got %d", len(emptyRes))
	}

	blankRes := store.Search("   ", 10)
	if len(blankRes) != 0 {
		t.Fatalf("expected 0 results for blank query, got %d", len(blankRes))
	}

	_ = store.Forget(note1.ID)
	afterForget := store.Search("zero comments go", 10)
	for _, n := range afterForget {
		if n.ID == note1.ID {
			t.Fatal("tombstoned note should not appear in search results")
		}
	}
}
