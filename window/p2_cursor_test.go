//go:build windows

package window

import (
	"testing"

	"github.com/jacksalad/gnote/component"
)

const (
	p2ClientW = 900
	p2ClientH = 640
	p2BarH    = 34
	p2BottomH = 30
)

// p2AppLikeRoot 按 cmd/gnote/app.go buildLayout 的顺序搭建组件树：根背景面板依次加入
// 菜单占位、编辑器、底部面板（内含输入框），铺满客户区的 MenuBar 最后加入。
func p2AppLikeRoot() (root *component.Panel, ta *component.TextArea, query *component.TextBox, mb *component.MenuBar) {
	root = component.NewBackgroundPanel()
	ta = component.NewTextArea()
	spacer := component.NewSurfacePanel()
	spacer.SetBounds(0, 0, p2ClientW, p2BarH)
	bottom := component.NewSurfacePanel()
	bottom.SetBounds(0, 0, p2ClientW, p2BottomH)
	// 子组件先于父面板加入：父面板随后被 BorderLayout 平移时会同步平移子组件
	query = component.NewTextBox()
	query.SetBounds(10, 4, 200, 22)
	bottom.Add(query)
	root.SetLayout(&component.BorderLayout{Top: spacer, Center: ta, Bottom: bottom})
	root.Add(spacer)
	root.Add(ta)
	root.Add(bottom)
	mb = component.NewMenuBar()
	mb.SetMenus([]component.Menu{
		{Text: "文件", Items: []component.MenuEntry{{Text: "新建"}}},
		{Text: "编辑", Items: []component.MenuEntry{{Text: "查找"}, {Text: "替换"}}},
	})
	root.Add(mb)
	root.SetBounds(0, 0, p2ClientW, p2ClientH)
	// MenuBar 不属于 BorderLayout 槽位，宿主手动把它铺满客户区
	mb.SetBounds(0, 0, p2ClientW, p2ClientH)
	return
}

func cursorIdOf(c component.Component) int {
	if aware, ok := c.(interface{ GetCursor() int }); ok {
		return aware.GetCursor()
	}
	return 0
}

// 菜单关闭时 MenuBar 只占顶部菜单栏条带：编辑器与输入框上的光标查询必须穿透到下层组件。
func TestP2_CursorLookupPassesThroughClosedMenuBar(t *testing.T) {
	root, _, _, mb := p2AppLikeRoot()
	if mb.IsOpen() {
		t.Fatalf("menu must start closed")
	}

	cases := []struct {
		name string
		x, y int32
		want int
	}{
		{"editor body", 450, 300, IDC_IBEAM},
		{"editor line-number gutter", 8, 300, IDC_IBEAM},
		{"editor last line", 450, p2ClientH - p2BottomH - 5, IDC_IBEAM},
		{"text box in bottom panel", 60, p2ClientH - p2BottomH + 15, IDC_IBEAM},
	}
	for _, tc := range cases {
		c := findCursorComponent(root, tc.x, tc.y)
		if c == mb {
			t.Fatalf("%s (%d,%d): lookup resolved to the MenuBar, which has no cursor", tc.name, tc.x, tc.y)
		}
		if got := cursorIdOf(c); got != tc.want {
			t.Fatalf("%s (%d,%d): component %T cursor = %d, want %d", tc.name, tc.x, tc.y, c, got, tc.want)
		}
	}

	// 菜单栏条带（标题与空白处）仍由 MenuBar 占据：箭头光标
	for _, pt := range [][2]int32{{20, 15}, {p2ClientW - 10, p2BarH - 1}} {
		if c := findCursorComponent(root, pt[0], pt[1]); c != mb {
			t.Fatalf("bar strip (%d,%d): lookup resolved to %T, want the MenuBar", pt[0], pt[1], c)
		}
	}
	if c := findCursorComponent(root, p2ClientW-10, p2BarH); c == mb {
		t.Fatalf("point just below the bar strip must not resolve to the MenuBar, got %T", c)
	}
}

// 菜单打开后弹出层覆盖编辑器：整片客户区的光标查询都应落在 MenuBar（箭头），不能露出下层的工字光标。
func TestP2_CursorLookupOpenMenuBarCoversEditor(t *testing.T) {
	root, _, _, mb := p2AppLikeRoot()
	mb.Open(1)
	for _, pt := range [][2]int32{{450, 300}, {20, 15}, {60, p2ClientH - p2BottomH + 15}} {
		if c := findCursorComponent(root, pt[0], pt[1]); c != mb {
			t.Fatalf("menu open at (%d,%d): lookup resolved to %T (cursor %d), want the MenuBar",
				pt[0], pt[1], c, cursorIdOf(c))
		}
	}
	mb.Close()
	if c := findCursorComponent(root, 450, 300); cursorIdOf(c) != IDC_IBEAM {
		t.Fatalf("after closing the menu the editor cursor must be back, got %T cursor %d", c, cursorIdOf(c))
	}
}

// 端到端：隐藏窗口上的光标句柄随编辑器/菜单栏切换。
func TestP2_WindowCursorFollowsEditorUnderMenuBar(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "p2-cursor", Width: p2ClientW, Height: p2ClientH, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	root, _, _, mb := p2AppLikeRoot()
	w.SetContent(root)

	ibeam := LoadCursor(0, IDC_IBEAM)
	arrow := LoadCursor(0, IDC_ARROW)
	if ibeam == 0 || arrow == 0 {
		t.Fatalf("failed to load system cursors")
	}

	w.updateCursorAt(450, 300)
	if w.currentCursor != ibeam {
		t.Fatalf("cursor over the editor = %#x, want I-beam %#x", w.currentCursor, ibeam)
	}
	w.updateCursorAt(60, p2ClientH-p2BottomH+15)
	if w.currentCursor != ibeam {
		t.Fatalf("cursor over the text box = %#x, want I-beam %#x", w.currentCursor, ibeam)
	}
	w.updateCursorAt(400, 10)
	if w.currentCursor != arrow {
		t.Fatalf("cursor over the menu bar strip = %#x, want arrow %#x", w.currentCursor, arrow)
	}
	w.updateCursorAt(450, 300)
	if w.currentCursor != ibeam {
		t.Fatalf("cursor back over the editor = %#x, want I-beam", w.currentCursor)
	}
	mb.Open(1)
	w.updateCursorAt(450, 300)
	if w.currentCursor != arrow {
		t.Fatalf("cursor over the open popup = %#x, want arrow", w.currentCursor)
	}
}

func p2LParam(x, y int32) uintptr {
	return uintptr(uint32(uint16(x)) | uint32(uint16(y))<<16)
}

func p2Selection(ta *component.TextArea) [4]int {
	sr, sc, er, ec, _ := ta.GetSelectionRange()
	return [4]int{sr, sc, er, ec}
}

func p2PointerWindow(t *testing.T, title string) *Window {
	t.Helper()
	w, err := NewWindow(WindowConfig{Title: title, Width: p2ClientW, Height: p2ClientH, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	t.Cleanup(func() {
		if IsWindow(w.Hwnd()) {
			w.Destroy()
		}
	})
	return w
}

// 端到端（窗口消息 → 事件 → 组件树）：在编辑器按下拖选、移到菜单栏条带上松开，
// 之后的无按键移动不得继续改变选区。
func TestP2_WindowDragReleaseOverMenuBarEndsSelection(t *testing.T) {
	w := p2PointerWindow(t, "p2-drag-release")
	root, ta, _, _ := p2AppLikeRoot()
	w.SetContent(root)
	ta.SetText("line one\nline two\nline three\nline four\nline five\nline six\n")

	pressY := ta.GetBounds().Y + 5
	w.handleMessage(w.hwnd, WM_LBUTTONDOWN, MK_LBUTTON, p2LParam(100, pressY))
	w.handleMessage(w.hwnd, WM_MOUSEMOVE, MK_LBUTTON, p2LParam(100, pressY+45))
	if !ta.HasSelection() {
		t.Fatalf("dragging with the button held must select text")
	}
	before := p2Selection(ta)

	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, p2LParam(450, 10))
	w.handleMessage(w.hwnd, WM_MOUSEMOVE, 0, p2LParam(100, pressY+95))
	if after := p2Selection(ta); after != before {
		t.Fatalf("selection kept following the mouse after the release over the menu bar: %v -> %v", before, after)
	}
}

// 端到端：双击菜单项的第二下是 WM_LBUTTONDBLCLK，此时弹出层已消失；
// 它不得穿透到下层编辑器抢走查找框的焦点。
func TestP2_WindowDoubleClickOnMenuItemDoesNotLeak(t *testing.T) {
	w := p2PointerWindow(t, "p2-dblclick")
	root, ta, query, mb := p2AppLikeRoot()
	mb.SetMenus([]component.Menu{
		{Text: "编辑", Items: []component.MenuEntry{
			{Text: "查找…", OnClick: func() { component.RequestFocus(query) }},
		}},
	})
	w.SetContent(root)
	ta.SetText("alpha beta gamma delta\n")
	ta.SetFocused(true)

	// 默认几何：弹出层左上角在 (4, 36)，首行中心 y=55，宽度不小于 180
	itemX, itemY := int32(94), int32(55)
	w.handleMessage(w.hwnd, WM_LBUTTONDOWN, MK_LBUTTON, p2LParam(30, 15))
	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, p2LParam(30, 15))
	if !mb.IsOpen() {
		t.Fatalf("clicking the menu title must open the menu")
	}
	w.handleMessage(w.hwnd, WM_LBUTTONDOWN, MK_LBUTTON, p2LParam(itemX, itemY))
	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, p2LParam(itemX, itemY))
	if mb.IsOpen() || !query.IsFocused() {
		t.Fatalf("activating the item must close the menu and focus the query box: open=%v queryFocused=%v", mb.IsOpen(), query.IsFocused())
	}

	w.handleMessage(w.hwnd, WM_LBUTTONDBLCLK, MK_LBUTTON, p2LParam(itemX, itemY))
	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, p2LParam(itemX, itemY))
	if !query.IsFocused() || ta.IsFocused() {
		t.Fatalf("the double-click leaked into the editor: queryFocused=%v editorFocused=%v", query.IsFocused(), ta.IsFocused())
	}
	if ta.HasSelection() {
		t.Fatalf("the double-click leaked into the editor and selected a word: %v", p2Selection(ta))
	}
}
