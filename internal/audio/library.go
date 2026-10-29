package audio

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Library manages track indexing and persistence.
type Library struct {
	mu     sync.RWMutex
	Tracks []Track `json:"tracks"`
	byPath map[string]int
}

// NewLibrary initializes an empty library.
func NewLibrary() *Library {
	return &Library{
		Tracks: make([]Track, 0),
		byPath: make(map[string]int),
	}
}

// LoadLibrary loads tracks from a JSON cache file.
func LoadLibrary(cachePath string) (*Library, error) {
	lib := NewLibrary()
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return lib, err
	}

	if err := json.Unmarshal(data, &lib.Tracks); err != nil {
		return lib, err
	}

	for i, t := range lib.Tracks {
		lib.byPath[t.Path] = i
	}
	return lib, nil
}

// Save writes the library to disk atomically.
func (l *Library) Save(cachePath string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile := cachePath + ".tmp"
	data, err := json.MarshalIndent(l.Tracks, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, cachePath)
}

// ScanFolder scans root folder recursively for supported audio files.
// Incremental: only recalculates metadata if file mod time changed.
func (l *Library) ScanFolder(root string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	newTracks := make([]Track, 0, len(l.Tracks))
	newByPath := make(map[string]int)

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !IsSupportedAudio(path) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		mtime := info.ModTime().Unix()

		// Reuse cached metadata if unmodified
		if idx, found := l.byPath[path]; found && l.Tracks[idx].ModTime == mtime {
			newTracks = append(newTracks, l.Tracks[idx])
			newByPath[path] = len(newTracks) - 1
			return nil
		}

		track := ExtractMetadata(path)
		track.ModTime = mtime
		newTracks = append(newTracks, track)
		newByPath[path] = len(newTracks) - 1
		return nil
	})

	l.Tracks = newTracks
	l.byPath = newByPath
	return len(l.Tracks)
}

// GetTrack returns a copy of track at index, or false if out of range.
func (l *Library) GetTrack(idx int) (Track, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if idx < 0 || idx >= len(l.Tracks) {
		return Track{}, false
	}
	return l.Tracks[idx], true
}

// Count returns number of tracks in library.
func (l *Library) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.Tracks)
}
