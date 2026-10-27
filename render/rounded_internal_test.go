package render

import (
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

func TestCornerCoverageCachedAndSymmetric(t *testing.T) {
	m1 := getCornerCoverage(6)
	m2 := getCornerCoverage(6)
	if m1 != m2 {
		t.Fatal("corner coverage should be cached by radius")
	}
	if got := m1.fill[5*6+5]; got != 1 {
		t.Fatalf("inner coverage = %v, want 1", got)
	}
	if m1.fill[0] >= 1 {
		t.Fatalf("outer corner should be antialiased, coverage=%v", m1.fill[0])
	}

	c := NewCanvas(24, 24)
	c.FillRect(0, 0, 24, 24, graphics.RGB(0, 0, 0))
	c.FillRoundedRect(0, 0, 24, 24, 6, graphics.RGB(255, 255, 255))
	// 四个角必须镜像对称（同一覆盖率掩码镜像使用）
	px := [4]graphics.Color{
		c.GetPixel(3, 3), c.GetPixel(20, 3), c.GetPixel(3, 20), c.GetPixel(20, 20),
	}
	for i := 1; i < len(px); i++ {
		if px[i] != px[0] {
			t.Fatalf("corner pixels not symmetric: %v", px)
		}
	}
}
