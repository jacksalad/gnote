package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

func TestImageOperations(t *testing.T) {
	img := NewImage(10, 10)
	if img.Width != 10 || img.Height != 10 {
		t.Fatalf("expected 10x10, got %dx%d", img.Width, img.Height)
	}

	red := graphics.RGBA(255, 0, 0, 255)
	img.SetPixel(2, 2, red)
	if img.GetPixel(2, 2) != red {
		t.Fatalf("expected pixel red, got %v", img.GetPixel(2, 2))
	}

	// Test bounds and at
	c := img.At(2, 2)
	nrgba, ok := c.(color.NRGBA)
	if !ok || nrgba.R != 255 || nrgba.A != 255 {
		t.Fatalf("expected NRGBA red, got %v", c)
	}

	// Test SubImage
	sub := img.SubImage(graphics.NewRect(1, 1, 3, 3))
	if sub.Width != 3 || sub.Height != 3 {
		t.Fatalf("expected 3x3 subimage, got %dx%d", sub.Width, sub.Height)
	}
	if sub.GetPixel(1, 1) != red {
		t.Fatalf("expected subimage (1,1) to be red, got %v", sub.GetPixel(1, 1))
	}
}

func TestFromGoImageAndDrawImage(t *testing.T) {
	// Create stdlib RGBA image
	rgba := image.NewRGBA(image.Rect(0, 0, 4, 4))
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			rgba.Set(x, y, blue)
		}
	}

	img := FromGoImage(rgba)
	if img.Width != 4 || img.Height != 4 {
		t.Fatalf("expected 4x4, got %dx%d", img.Width, img.Height)
	}

	canvas := NewCanvas(20, 20)
	canvas.Clear(graphics.RGB(0, 0, 0))

	// Draw 1:1 at (5, 5)
	canvas.DrawImage(img, 5, 5)
	if canvas.Buffer[5*20+5] != uint32(graphics.RGB(0, 0, 255)) {
		t.Fatalf("expected drawn pixel blue, got 0x%08X", canvas.Buffer[5*20+5])
	}

	// Draw Scaled to (10, 10, 8, 8)
	canvas.DrawImageScaled(img, graphics.NewRect(10, 10, 8, 8))
	if canvas.Buffer[12*20+12] != uint32(graphics.RGB(0, 0, 255)) {
		t.Fatalf("expected scaled pixel blue, got 0x%08X", canvas.Buffer[12*20+12])
	}
}
