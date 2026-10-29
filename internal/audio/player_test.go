package audio

import (
	"bytes"
	"encoding/binary"
	"sync/atomic"
	"testing"
)

func TestVolumeReader(t *testing.T) {
	// Create raw 16-bit PCM samples: 1000, -1000
	samples := []int16{1000, -1000}
	buf := new(bytes.Buffer)
	for _, s := range samples {
		_ = binary.Write(buf, binary.LittleEndian, s)
	}

	var vol atomic.Value
	vol.Store(0.5) // 50% volume

	var bytesRead atomic.Int64
	vr := newVolumeReader(bytes.NewReader(buf.Bytes()), &vol, &bytesRead, nil)
	out := make([]byte, 4)
	n, err := vr.Read(out)
	if err != nil || n != 4 {
		t.Fatalf("read failed: n=%d, err=%v", n, err)
	}

	val1 := int16(binary.LittleEndian.Uint16(out[0:2]))
	val2 := int16(binary.LittleEndian.Uint16(out[2:4]))

	if val1 != 500 {
		t.Errorf("expected 500, got %d", val1)
	}
	if val2 != -500 {
		t.Errorf("expected -500, got %d", val2)
	}
}

func TestPlaybackModes(t *testing.T) {
	lib := NewLibrary()
	lib.Tracks = []Track{
		{Title: "Track 1", Path: "1.mp3"},
		{Title: "Track 2", Path: "2.mp3"},
		{Title: "Track 3", Path: "3.mp3"},
	}

	if lib.Count() != 3 {
		t.Errorf("expected 3 tracks, got %d", lib.Count())
	}

	t1, ok := lib.GetTrack(0)
	if !ok || t1.Title != "Track 1" {
		t.Errorf("failed to get track 0: %+v", t1)
	}

	_, ok = lib.GetTrack(99)
	if ok {
		t.Errorf("expected false for out of bounds track")
	}
}
