package component

import (
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

func TestProgressBar(t *testing.T) {
	pb := NewProgressBar()
	pb.SetRange(0, 200)
	pb.SetValue(50)
	if pb.GetValue() != 50 {
		t.Fatalf("expected 50, got %f", pb.GetValue())
	}

	// Clamp tests
	pb.SetValue(300)
	if pb.GetValue() != 200 {
		t.Fatalf("expected clamped 200, got %f", pb.GetValue())
	}
	pb.SetValue(-50)
	if pb.GetValue() != 0 {
		t.Fatalf("expected clamped 0, got %f", pb.GetValue())
	}

	// Render test
	canvas := render.NewCanvas(200, 30)
	pb.SetBounds(10, 5, 180, 20)
	pb.SetShowText(true)
	pb.SetValue(100) // 50%
	pb.Render(canvas)
}

func TestSlider(t *testing.T) {
	slider := NewSlider(0, 100, 20)
	var changedVal float64
	slider.OnChange(func(val float64) {
		changedVal = val
	})

	slider.SetValue(45)
	if slider.GetValue() != 45 || changedVal != 45 {
		t.Fatalf("expected slider value 45, got %f (callback: %f)", slider.GetValue(), changedVal)
	}

	slider.SetBounds(0, 0, 100, 20)

	// Simulate MouseDown
	handled := slider.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X:      50,
		Y:      10,
		Button: event.MouseButtonLeft,
	}))
	if !handled {
		t.Fatal("expected mouseDown to be handled by slider")
	}

	// Render test
	canvas := render.NewCanvas(100, 30)
	slider.Render(canvas)
}

func TestImageView(t *testing.T) {
	img := render.NewImage(10, 10)
	img.SetPixel(0, 0, graphics.RGB(255, 0, 0))

	iv := NewImageView(img)
	iv.SetBounds(0, 0, 50, 50)
	iv.SetScaleMode(ScaleFit)
	iv.SetBackgroundColor(graphics.RGB(0, 0, 0))
	iv.SetCornerRadius(4)

	canvas := render.NewCanvas(60, 60)
	iv.Render(canvas)

	// Test other modes
	iv.SetScaleMode(ScaleStretch)
	iv.Render(canvas)
	iv.SetScaleMode(ScaleFill)
	iv.Render(canvas)
	iv.SetScaleMode(ScaleCenter)
	iv.Render(canvas)
}

func TestScrollView(t *testing.T) {
	sv := NewScrollView()
	sv.SetBounds(0, 0, 200, 100)

	panel := NewPanel()
	panel.SetBounds(0, 0, 200, 500)
	btn := NewButton("Item in scroll")
	btn.SetBounds(10, 20, 150, 40)
	panel.Add(btn)

	sv.SetContent(panel)
	sv.SetContentSize(200, 500)

	// Test scroll by wheel
	handled := sv.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
		X:     50,
		Y:     50,
		Delta: -120, // scroll down
	}))
	if !handled {
		t.Fatal("expected mousewheel to be handled by scrollview")
	}

	_, sy := sv.GetScrollOffset()
	if sy <= 0 {
		t.Fatalf("expected scrollOffsetY > 0 after wheel down, got %d", sy)
	}

	// Scroll bounds test
	sv.ScrollTo(0, 1000) // beyond max
	_, sy = sv.GetScrollOffset()
	if sy != 400 { // 500 - 100 = 400
		t.Fatalf("expected clamped scrollOffsetY 400, got %d", sy)
	}

	// Render test
	canvas := render.NewCanvas(200, 100)
	sv.Render(canvas)
}

func TestTheme(t *testing.T) {
	dark := theme.Dark()
	light := theme.Light()

	if dark.Name != "Dark" || light.Name != "Light" {
		t.Fatalf("invalid theme names: %s, %s", dark.Name, light.Name)
	}

	theme.SetCurrent(light)
	if theme.Current().Name != "Light" {
		t.Fatalf("expected Light theme, got %s", theme.Current().Name)
	}

	theme.SetCurrent(dark)
	if theme.Current().Name != "Dark" {
		t.Fatalf("expected Dark theme, got %s", theme.Current().Name)
	}
}
