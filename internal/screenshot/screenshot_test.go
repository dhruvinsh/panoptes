package screenshot

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePNG(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.png")
	rows := [][]byte{{255, 0, 0, 0, 255, 0}, nil} // second row missing -> black
	if err := WritePNG(p, 2, 2, rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("bounds %v", b)
	}
	r, g, _, _ := img.At(0, 0).RGBA()
	if r>>8 != 255 || g != 0 {
		t.Fatalf("pixel (0,0) = %v", img.At(0, 0))
	}
	r, g, b, a := img.At(1, 1).RGBA()
	if r != 0 || g != 0 || b != 0 || a>>8 != 255 {
		t.Fatalf("pixel (1,1) = %v", img.At(1, 1))
	}
}
