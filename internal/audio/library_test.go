package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMetadataFallback(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Radiohead - Creep.mp3")
	if err := os.WriteFile(filePath, []byte("fake mp3 content"), 0644); err != nil {
		t.Fatal(err)
	}

	track := ExtractMetadata(filePath)
	if track.Artist != "Radiohead" {
		t.Errorf("expected Radiohead, got %s", track.Artist)
	}
	if track.Title != "Creep" {
		t.Errorf("expected Creep, got %s", track.Title)
	}
}

func TestLibraryScanAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "Song One.mp3")
	f2 := filepath.Join(tmpDir, "Song Two.wav")
	f3 := filepath.Join(tmpDir, "Readme.txt") // non-audio
	_ = os.WriteFile(f1, []byte("audio1"), 0644)
	_ = os.WriteFile(f2, []byte("audio2"), 0644)
	_ = os.WriteFile(f3, []byte("ignore"), 0644)

	lib := NewLibrary()
	count := lib.ScanFolder(tmpDir)
	if count != 2 {
		t.Fatalf("expected 2 audio files, got %d", count)
	}

	cachePath := filepath.Join(tmpDir, "library.json")
	if err := lib.Save(cachePath); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadLibrary(cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Count() != 2 {
		t.Errorf("expected 2 tracks in loaded library, got %d", loaded.Count())
	}
}
