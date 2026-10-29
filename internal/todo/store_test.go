package todo

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTodoCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "todos.json")

	store, err := NewStore(storePath, true)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Add
	item1 := store.Add("Beli susu")
	item2 := store.Add("Koding IslandWin")
	if store.IncompleteCount() != 2 {
		t.Errorf("expected 2 incomplete, got %d", store.IncompleteCount())
	}

	// Toggle
	if !store.Toggle(item1.ID) {
		t.Errorf("failed to toggle item1")
	}
	if store.IncompleteCount() != 1 {
		t.Errorf("expected 1 incomplete after toggle, got %d", store.IncompleteCount())
	}

	// Delete
	if !store.Delete(item2.ID) {
		t.Errorf("failed to delete item2")
	}
	if len(store.GetItems()) != 1 {
		t.Errorf("expected 1 item, got %d", len(store.GetItems()))
	}

	// Reload from disk to verify atomic save
	reloaded, err := NewStore(storePath, true)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	items := reloaded.GetItems()
	if len(items) != 1 || items[0].Text != "Beli susu" || !items[0].Done {
		t.Errorf("mismatch in reloaded items: %+v", items)
	}
}

func TestTodoRolloverCarryOver(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "todos.json")

	store, err := NewStore(storePath, true)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate items from yesterday
	store.Add("Tugas selesai kemarin")
	store.Add("Tugas belum selesai")
	items := store.GetItems()
	store.Toggle(items[0].ID) // item[0] is done

	// Set LastDate to yesterday
	store.LastDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	_ = store.Save()

	// Reload triggering rollover
	reloaded, err := NewStore(storePath, true)
	if err != nil {
		t.Fatal(err)
	}

	// Completed task should be archived, incomplete task carried over
	active := reloaded.GetItems()
	if len(active) != 1 {
		t.Fatalf("expected 1 carried over item, got %d", len(active))
	}
	if active[0].Text != "Tugas belum selesai" {
		t.Errorf("expected 'Tugas belum selesai', got %s", active[0].Text)
	}

	if len(reloaded.Archive) != 1 {
		t.Errorf("expected 1 archive entry, got %d", len(reloaded.Archive))
	}
}
