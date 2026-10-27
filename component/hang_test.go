package component

import (
	"fmt"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

func TestModernUIHierarchy(t *testing.T) {
	root := NewPanel()
	th := theme.Current()
	root.SetBackground(th.Background)
	root.SetPadding(20)

	title := NewLabel("goui 纯 Go 软件渲染 GUI 引擎")
	root.Add(title)

	leftPanel := NewPanel()
	leftPanel.SetBounds(20, 75, 420, 535)
	leftPanel.SetBackground(th.Surface)
	leftPanel.SetBorder(th.Border, 1)
	root.Add(leftPanel)

	txt := NewTextBox()
	txt.SetBounds(15, 112, 380, 34)
	leftPanel.Add(txt)

	progress := NewProgressBar()
	progress.SetBounds(15, 215, 380, 22)
	progress.SetValue(42)
	leftPanel.Add(progress)

	slider := NewSlider(0, 100, 42)
	slider.SetBounds(15, 270, 380, 24)
	leftPanel.Add(slider)

	testImg := render.NewImage(80, 80)
	iv := NewImageView(testImg)
	iv.SetBounds(15, 430, 80, 80)
	leftPanel.Add(iv)

	ivDesc := NewLabel("纯 Go 内存位图光栅化\n支持 PNG/JPEG/GIF 格式解码\n支持 Stretch/Fit/Fill 缩放模式")
	ivDesc.SetBounds(105, 435, 290, 60)
	leftPanel.Add(ivDesc)

	rightPanel := NewPanel()
	rightPanel.SetBounds(455, 75, 405, 535)
	rightPanel.SetBackground(th.Surface)
	root.Add(rightPanel)

	scrollView := NewScrollView()
	scrollView.SetBounds(15, 65, 375, 450)

	listContainer := NewPanel()
	totalHeight := int32(18 * 45)
	listContainer.SetBounds(0, 0, 360, totalHeight)

	for i := 1; i <= 18; i++ {
		btn := NewButton(fmt.Sprintf("Item #%d", i))
		btn.SetBounds(10, int32((i-1)*45+5), 340, 36)
		listContainer.Add(btn)
	}

	scrollView.SetContent(listContainer)
	scrollView.SetContentSize(360, totalHeight)
	rightPanel.Add(scrollView)

	canvas := render.NewCanvas(880, 640)
	root.SetBounds(0, 0, 880, 640)
	root.Render(canvas)

	// Simulate event
	root.OnEvent(event.NewEvent(event.EventMouseMove, event.MouseData{X: 100, Y: 100}))
}
