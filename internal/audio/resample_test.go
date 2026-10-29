package audio

import (
	"bytes"
	"io"
	"testing"
)

// 22050Hz mono -> 44100Hz stereo must double the frame count (fixes "too fast" playback).
func TestNormalizeDoublesFrames(t *testing.T) {
	src := make([]byte, 22050*2) // 1s mono silence
	d := normalize(bytes.NewReader(src), 22050, 1, int64(len(src)))
	out, err := io.ReadAll(d.Stream)
	if err != nil {
		t.Fatal(err)
	}
	want := 44100 * 4
	if diff := len(out) - want; diff < -8 || diff > 8 {
		t.Fatalf("got %d bytes, want ~%d", len(out), want)
	}
}
