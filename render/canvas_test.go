package render_test

import (
	"testing"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

func TestCanvasClipping(t *testing.T) {
	c := render.NewCanvas(100, 100)
	c.Clear(graphics.ColorWhite)

	// Test default clip
	if c.Clip().Width != 100 || c.Clip().Height != 100 {
		t.Fatalf("unexpected default clip: %+v", c.Clip())
	}

	// Push clip
	c.PushClip(graphics.NewRect(10, 10, 20, 20))
	if c.Clip().X != 10 || c.Clip().Y != 10 || c.Clip().Width != 20 || c.Clip().Height != 20 {
		t.Fatalf("unexpected pushed clip: %+v", c.Clip())
	}

	// Fill rect covering 0..100, should only affect 10..30
	c.FillRect(0, 0, 100, 100, graphics.ColorRed)

	// Pixel at (5, 5) should still be White
	if c.GetPixel(5, 5) != graphics.ColorWhite {
		t.Fatalf("pixel outside clip was modified: %x", c.GetPixel(5, 5))
	}

	// Pixel at (15, 15) should be Red
	if c.GetPixel(15, 15) != graphics.ColorRed {
		t.Fatalf("pixel inside clip was not modified: %x", c.GetPixel(15, 15))
	}

	// Pop clip
	c.PopClip()
	if c.Clip().Width != 100 || c.Clip().Height != 100 {
		t.Fatalf("unexpected clip after pop: %+v", c.Clip())
	}
}

func TestCanvasRoundedRect(t *testing.T) {
	c := render.NewCanvas(50, 50)
	c.Clear(graphics.ColorTransparent)

	// Fill rounded rect
	c.FillRoundedRect(5, 5, 40, 40, 10, graphics.ColorBlue)

	// The corner point (5, 5) should remain untouched (Transparent) because it is outside the arc
	if c.GetPixel(5, 5) != graphics.ColorTransparent {
		t.Fatalf("corner (5, 5) should be transparent, got: %x", c.GetPixel(5, 5))
	}

	// Center point (25, 25) should be Blue
	if c.GetPixel(25, 25) != graphics.ColorBlue {
		t.Fatalf("center (25, 25) should be blue, got: %x", c.GetPixel(25, 25))
	}
}

func TestCanvasBresenhamLine(t *testing.T) {
	c := render.NewCanvas(20, 20)
	c.Clear(graphics.ColorBlack)

	c.DrawLine(0, 0, 19, 19, graphics.ColorGreen)
	if c.GetPixel(0, 0) != graphics.ColorGreen {
		t.Fatalf("start pixel should be green")
	}
	if c.GetPixel(19, 19) != graphics.ColorGreen {
		t.Fatalf("end pixel should be green")
	}
}

func TestCanvasBoundarySafety(t *testing.T) {
	c := render.NewCanvas(50, 50)
	c.Clear(graphics.ColorWhite)

	// All out-of-bound operations MUST not panic
	c.SetPixel(-10, -10, graphics.ColorBlack)
	c.SetPixel(100, 100, graphics.ColorBlack)
	c.GetPixel(-5, 20)
	c.GetPixel(60, 20)

	c.DrawHorizontalLine(-20, 100, -5, graphics.ColorRed)
	c.DrawHorizontalLine(-20, 100, 10, graphics.ColorRed)
	c.DrawHorizontalLine(-20, 100, 60, graphics.ColorRed)

	c.DrawVerticalLine(-5, -20, 100, graphics.ColorBlue)
	c.DrawVerticalLine(20, -20, 100, graphics.ColorBlue)
	c.DrawVerticalLine(60, -20, 100, graphics.ColorBlue)

	c.FillRect(-10, -10, 100, 100, graphics.ColorGreen)
	c.DrawLine(-50, -50, 150, 150, graphics.ColorBlack)
	c.BlendPixel(-5, -5, graphics.RGBA(255, 0, 0, 128))
	c.BlendPixel(25, 25, graphics.RGBA(255, 0, 0, 128))
	c.BlendPixel(70, 70, graphics.RGBA(255, 0, 0, 128))
}

func TestCanvasCircleAntiAliasing(t *testing.T) {
	c := render.NewCanvas(40, 40)
	c.Clear(graphics.ColorTransparent)

	const cx, cy, radius = int32(20), int32(20), int32(9)
	c.FillCircle(cx, cy, radius, graphics.ColorBlue)

	if got := c.GetPixel(cx, cy); got != graphics.ColorBlue {
		t.Fatalf("center pixel should be fully covered: %x", got)
	}

	// 圆弧边界像素必须是半透明混合结果，既不是纯色也不是完全透明
	edge := c.GetPixel(cx+radius, cy)
	if edge == graphics.ColorBlue || edge == graphics.ColorTransparent {
		t.Fatalf("edge pixel should be partially covered, got: %x", edge)
	}

	// 半径之外 1 像素开始完全不受影响
	if got := c.GetPixel(cx+radius+1, cy); got != graphics.ColorTransparent {
		t.Fatalf("pixel outside circle should stay transparent: %x", got)
	}

	// 覆盖率掩码左右对称
	if a, b := c.GetPixel(cx+5, cy+7), c.GetPixel(cx-5, cy+7); a != b {
		t.Fatalf("circle must be horizontally symmetric: %x != %x", a, b)
	}

	// 1px 描边：中心线着色，内部保持透明
	c2 := render.NewCanvas(40, 40)
	c2.Clear(graphics.ColorTransparent)
	c2.DrawCircle(cx, cy, radius, graphics.ColorRed)
	if got := c2.GetPixel(cx+radius, cy); got == graphics.ColorTransparent {
		t.Fatalf("ring centerline pixel should be painted")
	}
	if got := c2.GetPixel(cx, cy); got != graphics.ColorTransparent {
		t.Fatalf("circle interior should stay untouched, got: %x", got)
	}
	if got := c2.GetPixel(cx+radius+1, cy); got != graphics.ColorTransparent {
		t.Fatalf("pixel beyond ring should stay transparent: %x", got)
	}
}
