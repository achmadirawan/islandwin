package audio

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Track represents a single audio track in the library.
type Track struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMs int64  `json:"duration_ms"`
	ModTime    int64  `json:"mod_time"`
}

// ExtractMetadata attempts to parse ID3v1/ID3v2 tags or falls back to filename.
func ExtractMetadata(path string) Track {
	fi, err := os.Stat(path)
	var mtime int64
	if err == nil {
		mtime = fi.ModTime().Unix()
	}

	ext := strings.ToLower(filepath.Ext(path))
	name := strings.TrimSuffix(filepath.Base(path), ext)

	// Fallback defaults
	artist := "Unknown Artist"
	title := name

	// Try splitting "Artist - Title" format commonly used in audio files
	if parts := strings.SplitN(name, " - ", 2); len(parts) == 2 {
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])
	}

	track := Track{
		Path:    path,
		Title:   title,
		Artist:  artist,
		Album:   "",
		ModTime: mtime,
	}

	// Try reading ID3v1 (last 128 bytes of MP3 file)
	if ext == ".mp3" {
		if id3 := readID3v1(path); id3 != nil {
			if id3.Title != "" {
				track.Title = id3.Title
			}
			if id3.Artist != "" {
				track.Artist = id3.Artist
			}
			if id3.Album != "" {
				track.Album = id3.Album
			}
		}
	}

	return track
}

type id3v1Tag struct {
	Title  string
	Artist string
	Album  string
}

func readID3v1(path string) *id3v1Tag {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.Size() < 128 {
		return nil
	}

	buf := make([]byte, 128)
	if _, err := f.ReadAt(buf, fi.Size()-128); err != nil {
		return nil
	}

	if string(buf[0:3]) != "TAG" {
		return nil
	}

	clean := func(b []byte) string {
		n := bytes.IndexByte(b, 0)
		if n != -1 {
			b = b[:n]
		}
		return strings.TrimSpace(string(b))
	}

	return &id3v1Tag{
		Title:  clean(buf[3:33]),
		Artist: clean(buf[33:63]),
		Album:  clean(buf[63:93]),
	}
}

// SupportedExtensions lists playable audio file extensions.
var SupportedExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".ogg":  true,
	".flac": true,
}

// IsSupportedAudio returns true if the file path has a supported audio extension.
func IsSupportedAudio(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return SupportedExtensions[ext]
}

// Discard reads until EOF to safely close or skip unneeded data without allocations.
func Discard(r io.Reader) error {
	buf := make([]byte, 8192)
	for {
		_, err := r.Read(buf)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
