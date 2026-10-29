package render

import (
	"os"
	"testing"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

func TestLoadSegoeUI(t *testing.T) {
	fontData, err := os.ReadFile("C:\\Windows\\Fonts\\segoeui.ttf")
	if err != nil {
		t.Skip("segoeui.ttf not found:", err)
	}

	ft, err := truetype.Parse(fontData)
	if err != nil {
		t.Fatalf("Failed to parse segoeui.ttf: %v", err)
	}

	face := truetype.NewFace(ft, &truetype.Options{
		Size:    12,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if face == nil {
		t.Fatal("Failed to create font face")
	}
}
