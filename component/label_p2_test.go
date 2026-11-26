package component

import "testing"

// p2StatusSegment 在底部状态栏右侧放一个可点击的状态分段（同 buildStatusBar 的编码/EOL 段）。
func p2StatusSegment(a *p2AppRoot, text string, onClick func()) *Label {
	seg := NewSecondaryLabel(text)
	seg.SetAlign(AlignRight)
	seg.SetOnClick(onClick)
	sb := a.status.GetBounds()
	seg.SetBounds(sb.X+sb.Width-14-82, sb.Y+4, 82, 20)
	a.status.Add(seg)
	return seg
}

// 回归：可点击标签在 MouseDown 时就触发回调，却无条件吞掉 MouseUp。
// 从编辑器拖选后 在状态栏分段上松开，编辑器收不到释放事件，选区在没有任何按键的情况下继续跟随鼠标。
func TestP2_ClickableLabelLetsDragReleaseReachEditor(t *testing.T) {
	a := p2BuildAppRoot()
	clicks := 0
	seg := p2StatusSegment(a, "UTF-8", func() { clicks++ })
	pressY := p2DragFromEditor(t, a)

	b := seg.GetBounds()
	sx, sy := b.X+b.Width/2, b.Y+b.Height/2
	a.root.OnEvent(p2Up(sx, sy))
	if clicks != 0 {
		t.Fatalf("releasing a drag over the status segment must not click it, clicks=%d", clicks)
	}
	before := p2SelectionOf(a.ta)
	p2AssertSelectionFrozen(t, a, before, pressY+95, "over a clickable status label")

	// 松开之后在标签上移动、再离开：选区同样不得变化
	a.root.OnEvent(p2Move(sx-20, sy+3))
	a.root.OnEvent(p2Move(300, pressY+40))
	if after := p2SelectionOf(a.ta); after != before {
		t.Fatalf("selection changed after the release: %s -> %s", before, after)
	}
}

// 对照：在不可点击的普通标签上松开，释放事件本来就能到达编辑器。
func TestP2_PlainLabelLetsDragReleaseReachEditor(t *testing.T) {
	a := p2BuildAppRoot()
	name := NewLabel("新文档 (未保存)")
	sb := a.status.GetBounds()
	name.SetBounds(sb.X+14, sb.Y+4, 300, 20)
	a.status.Add(name)
	pressY := p2DragFromEditor(t, a)

	b := name.GetBounds()
	a.root.OnEvent(p2Up(b.X+100, b.Y+b.Height/2))
	before := p2SelectionOf(a.ta)
	p2AssertSelectionFrozen(t, a, before, pressY+95, "over a plain label")
}

// 正常点击照常生效：按下触发一次回调；按下后在标签外松开也不会再触发。
func TestP2_ClickableLabelStillClicks(t *testing.T) {
	a := p2BuildAppRoot()
	clicks := 0
	seg := p2StatusSegment(a, "UTF-8", func() { clicks++ })
	a.ta.SetFocused(true)

	b := seg.GetBounds()
	sx, sy := b.X+b.Width/2, b.Y+b.Height/2
	if handled := a.root.OnEvent(p2Down(sx, sy)); !handled {
		t.Fatalf("pressing a clickable label must be consumed")
	}
	if clicks != 1 {
		t.Fatalf("pressing the label must run its callback once, clicks=%d", clicks)
	}
	a.root.OnEvent(p2Up(sx, sy))
	if clicks != 1 {
		t.Fatalf("releasing over the label must not run the callback again, clicks=%d", clicks)
	}

	// 按下后拖到标签外松开
	a.root.OnEvent(p2Down(sx, sy))
	a.root.OnEvent(p2Move(sx-40, sy))
	a.root.OnEvent(p2Up(sx-40, sy))
	if clicks != 2 {
		t.Fatalf("a press outside-after press inside must not add a click, clicks=%d", clicks)
	}

	// 不可点击的标签不消费按下
	plain := NewLabel("1:1 · 1 行")
	plain.SetBounds(b.X-120, b.Y, 100, 20)
	a.status.Add(plain)
	if handled := a.root.OnEvent(p2Down(plain.GetBounds().X+10, sy)); handled {
		t.Fatalf("plain label must not consume the press")
	}
	if clicks != 2 {
		t.Fatalf("clicks=%d", clicks)
	}
}
