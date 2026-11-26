package component

import (
	"fmt"
	"testing"
	"time"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
)

// p2AppRoot 复刻 cmd/gnote/app.go buildLayout 的组件树与加入顺序：菜单占位、编辑器、
// 底部面板依次加入根面板，铺满客户区的 MenuBar 最后加入（反向事件遍历时最先命中）。
type p2AppRoot struct {
	root   *Panel
	ta     *TextArea
	bottom *Panel
	status *Panel
	query  *TextBox
	mb     *MenuBar
}

const (
	p2WinW     int32 = 900
	p2WinH     int32 = 640
	p2MenuH    int32 = 34
	p2StatusH  int32 = 28
	p2FindBarH int32 = 32
)

func p2BuildAppRoot() *p2AppRoot {
	a := &p2AppRoot{}
	// 底部面板：上排查找条输入框，下排状态栏。子组件先于父面板加入，
	// 父面板随后被 BorderLayout 平移时会同步平移子组件。
	a.status = NewSurfacePanel()
	a.status.SetBounds(0, p2FindBarH, p2WinW, p2StatusH)
	a.bottom = NewSurfacePanel()
	a.bottom.SetBounds(0, 0, p2WinW, p2FindBarH+p2StatusH)
	a.query = NewTextBox()
	a.query.SetBounds(10, 5, 200, 22)
	a.bottom.Add(a.query)
	a.bottom.Add(a.status)

	spacer := NewSurfacePanel()
	spacer.SetBounds(0, 0, p2WinW, p2MenuH)
	a.ta = NewTextArea()
	a.root = NewBackgroundPanel()
	a.root.SetLayout(&BorderLayout{Top: spacer, Center: a.ta, Bottom: a.bottom})
	a.root.Add(spacer)
	a.root.Add(a.ta)
	a.root.Add(a.bottom)

	// 同 App.onMenuOpenChange：菜单打开时夺走键盘焦点，关闭后还给编辑器
	a.mb = NewMenuBar()
	editorFocused := false
	a.mb.SetOnOpenChange(func(open bool) {
		if open {
			editorFocused = a.ta.IsFocused()
			RequestFocus(a.mb)
			return
		}
		if editorFocused {
			RequestFocus(a.ta)
		}
	})
	a.mb.SetMenus([]Menu{
		{Text: "文件", Items: []MenuEntry{{Text: "新建"}, {Text: "保存"}}},
		{Text: "编辑", Items: []MenuEntry{{Text: "撤销"}, {Text: "查找"}}},
	})
	a.root.Add(a.mb)
	a.root.SetBounds(0, 0, p2WinW, p2WinH)
	// MenuBar 不属于 BorderLayout 的槽位，必须手动铺满客户区
	a.mb.SetBounds(0, 0, p2WinW, p2WinH)
	return a
}

func p2Down(x, y int32) event.Event {
	return event.NewEvent(event.EventMouseDown, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft})
}

func p2Up(x, y int32) event.Event {
	return event.NewEvent(event.EventMouseUp, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft})
}

func p2Dbl(x, y int32) event.Event {
	return event.NewEvent(event.EventMouseDoubleClick, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft})
}

func p2Move(x, y int32) event.Event {
	return event.NewEvent(event.EventMouseMove, event.MouseData{X: x, Y: y})
}

type p2Selection struct {
	startRow, startCol, endRow, endCol int
}

func p2SelectionOf(ta *TextArea) p2Selection {
	sr, sc, er, ec, _ := ta.GetSelectionRange()
	return p2Selection{sr, sc, er, ec}
}

func (s p2Selection) String() string {
	return fmt.Sprintf("(%d,%d)-(%d,%d)", s.startRow, s.startCol, s.endRow, s.endCol)
}

// p2Doc 每行都有单词的多行文档：双击任意可见位置都会移动插入点并选中一个单词。
const p2Doc = "alpha beta gamma delta\n" +
	"epsilon zeta eta theta\n" +
	"iota kappa lambda mu\n" +
	"nu xi omicron pi\n" +
	"rho sigma tau upsilon\n" +
	"phi chi psi omega\n"

// p2DragFromEditor 在编辑器首行按下并向下拖一行半，返回按下点的 Y。
func p2DragFromEditor(t *testing.T, a *p2AppRoot) int32 {
	t.Helper()
	a.ta.SetText("line one\nline two\nline three\nline four\nline five\nline six\n")
	a.ta.SetFocused(true)
	pressY := a.ta.GetBounds().Y + 5
	if !a.root.OnEvent(p2Down(100, pressY)) {
		t.Fatalf("press inside the editor must reach the editor")
	}
	a.root.OnEvent(p2Move(100, pressY+3*a.ta.lineHeight/2))
	if !a.ta.HasSelection() {
		t.Fatalf("dragging with the button held must select text")
	}
	return pressY
}

// 松开鼠标后再做一次无按键移动，选区必须保持不变（选区不得继续跟随鼠标）。
func p2AssertSelectionFrozen(t *testing.T, a *p2AppRoot, before p2Selection, moveToY int32, where string) {
	t.Helper()
	a.root.OnEvent(p2Move(100, moveToY))
	after := p2SelectionOf(a.ta)
	if after != before {
		t.Fatalf("selection kept following the mouse after the button was released %s: %s -> %s", where, before, after)
	}
}

// HitTest：关闭时只占据顶部菜单栏条带，打开时占据整个客户区。
// MenuBar 的 bounds 铺满客户区，窗口的光标查找靠它穿透到下层的编辑器与输入框。
func TestP2_MenuBarHitTestOnlyCoversBarWhenClosed(t *testing.T) {
	a := p2BuildAppRoot()
	if !a.mb.HitTest(20, 0) || !a.mb.HitTest(p2WinW-1, p2MenuH-1) {
		t.Fatalf("closed menu bar must claim the top strip")
	}
	for _, pt := range [][2]int32{{20, p2MenuH}, {450, 300}, {60, p2WinH - 1}} {
		if a.mb.HitTest(pt[0], pt[1]) {
			t.Fatalf("closed menu bar must not claim (%d,%d) below the strip", pt[0], pt[1])
		}
	}
	a.mb.Open(0)
	for _, pt := range [][2]int32{{20, 0}, {450, 300}, {60, p2WinH - 1}} {
		if !a.mb.HitTest(pt[0], pt[1]) {
			t.Fatalf("open menu must claim the whole client area, rejected (%d,%d)", pt[0], pt[1])
		}
	}
	a.mb.Close()
	if a.mb.HitTest(450, 300) {
		t.Fatalf("menu bar must be transparent again after closing")
	}
}

// 回归：从编辑器拖选、在菜单栏条带上松开。关闭的 MenuBar 曾无条件吞掉条带内的 MouseUp，
// 编辑器收不到释放事件，选区在没有任何按键的情况下继续跟随鼠标。
func TestP2_MenuBarLetsDragReleaseReachEditor(t *testing.T) {
	a := p2BuildAppRoot()
	pressY := p2DragFromEditor(t, a)

	a.root.OnEvent(p2Up(450, 10))
	before := p2SelectionOf(a.ta)
	p2AssertSelectionFrozen(t, a, before, pressY+95, "over the closed menu bar strip")

	// 在菜单标题上松开同样不能截住释放事件
	a.root.OnEvent(p2Down(100, pressY+140))
	a.root.OnEvent(p2Up(20, 15))
	if a.mb.IsOpen() {
		t.Fatalf("a release over a menu title must not open the menu")
	}
	before = p2SelectionOf(a.ta)
	p2AssertSelectionFrozen(t, a, before, pressY+60, "over a menu title")
}

// 拖选过程中用键盘（Alt+F）打开菜单后松开：打开的 MenuBar 曾消费全部 MouseUp，
// 编辑器的拖选状态同样无法结束。
func TestP2_OpenMenuBarLetsDragReleaseReachEditor(t *testing.T) {
	a := p2BuildAppRoot()
	pressY := p2DragFromEditor(t, a)

	a.mb.Open(0)
	if !a.mb.IsOpen() {
		t.Fatalf("keyboard-opened menu should be open")
	}
	a.root.OnEvent(p2Up(100, pressY+20))
	a.mb.Close()
	before := p2SelectionOf(a.ta)
	p2AssertSelectionFrozen(t, a, before, pressY+95, "over the open menu")

	// 菜单打开时点击编辑器：按下关闭菜单且不落到编辑器，随后的释放不得改变选区
	a.mb.Open(0)
	a.root.OnEvent(p2Down(300, pressY+60))
	if a.mb.IsOpen() {
		t.Fatalf("clicking outside must close the menu")
	}
	before = p2SelectionOf(a.ta)
	a.root.OnEvent(p2Up(300, pressY+60))
	p2AssertSelectionFrozen(t, a, before, pressY+95, "after closing the menu by clicking the editor")
}

// 按下/释放成对的点击序列仍然可用：标题按下打开、释放不关闭；菜单项只执行一次。
func TestP2_MenuBarClickSequencesStillWork(t *testing.T) {
	clicks := map[string]int{}
	a := p2BuildAppRoot()
	a.mb.SetMenus([]Menu{
		{Text: "文件", Items: []MenuEntry{
			{Text: "新建", OnClick: func() { clicks["new"]++ }},
			{Separator: true},
			{Text: "退出", Disabled: true},
			{Text: "最近文件", Submenu: []MenuEntry{{Text: "a.txt", OnClick: func() { clicks["recent"]++ }}}},
		}},
		{Text: "编辑", Items: []MenuEntry{{Text: "撤销", OnClick: func() { clicks["undo"]++ }}}},
	})
	a.ta.SetFocused(true)

	x, y := barCenter(a.mb, 0)
	a.root.OnEvent(p2Down(x, y))
	if !a.mb.IsOpen() {
		t.Fatalf("pressing the menu title must open the menu")
	}
	a.root.OnEvent(p2Up(x, y))
	if !a.mb.IsOpen() {
		t.Fatalf("releasing over the menu title must keep the menu open")
	}

	ix, iy := menuRowCenter(a.mb, 0, 0)
	a.root.OnEvent(p2Down(ix, iy))
	if clicks["new"] != 1 || a.mb.IsOpen() {
		t.Fatalf("pressing the item must run it once and close the menu: clicks=%v open=%v", clicks, a.mb.IsOpen())
	}
	a.root.OnEvent(p2Up(ix, iy))
	if clicks["new"] != 1 {
		t.Fatalf("releasing over the item must not run it again: clicks=%v", clicks)
	}
	if !a.ta.IsFocused() {
		t.Fatalf("closing the menu must give keyboard focus back to the editor")
	}

	// 再次点击同一个标题：按下关闭
	a.root.OnEvent(p2Down(x, y))
	if !a.mb.IsOpen() {
		t.Fatalf("pressing the title again must reopen the menu")
	}
	a.root.OnEvent(p2Up(x, y))
	a.root.OnEvent(p2Down(x, y))
	if a.mb.IsOpen() {
		t.Fatalf("pressing the open title must close the menu")
	}
	a.root.OnEvent(p2Up(x, y))

	// 子菜单项
	a.root.OnEvent(p2Down(x, y))
	sx, sy := menuRowCenter(a.mb, 0, 3)
	a.root.OnEvent(p2Move(sx, sy))
	if a.mb.submenuOpen != 3 {
		t.Fatalf("hovering the submenu parent must open the submenu")
	}
	sub := a.mb.submenuRect(0, 3)
	cx, cy := sub.X+sub.Width/2, sub.Y+4+a.mb.rowHeight/2
	a.root.OnEvent(p2Down(cx, cy))
	a.root.OnEvent(p2Up(cx, cy))
	if clicks["recent"] != 1 || a.mb.IsOpen() {
		t.Fatalf("submenu item must run once and close the menu: clicks=%v open=%v", clicks, a.mb.IsOpen())
	}

	// 禁用项与分隔线：按下被消费但菜单保持打开
	a.root.OnEvent(p2Down(x, y))
	dx, dy := menuRowCenter(a.mb, 0, 2)
	if !a.root.OnEvent(p2Down(dx, dy)) || !a.mb.IsOpen() {
		t.Fatalf("pressing a disabled item must be consumed and keep the menu open")
	}
	if a.ta.IsFocused() {
		t.Fatalf("the editor must not take focus while the menu is open")
	}
	if len(clicks) != 2 || clicks["new"] != 1 || clicks["recent"] != 1 {
		t.Fatalf("unexpected click counts: %v", clicks)
	}
}

// 回归：菜单项按下即执行，双击的第二次点击以 WM_LBUTTONDBLCLK 到达，此时弹出层已经消失。
// MenuBar 不消费它，事件就穿透到下层编辑器：抢走查找框的焦点并选中一个单词。
func TestP2_MenuBarConsumesDoubleClickAfterActivatingItem(t *testing.T) {
	a := p2BuildAppRoot()
	activated := 0
	a.mb.SetMenus([]Menu{
		{Text: "编辑", Items: []MenuEntry{
			{Text: "查找…", OnClick: func() { activated++; RequestFocus(a.query) }},
			{Text: "替换…", OnClick: func() { activated++ }},
			{Text: "主题", Submenu: []MenuEntry{{Text: "深色", OnClick: func() { activated++ }}}},
		}},
	})
	a.ta.SetText(p2Doc)
	a.ta.SetFocused(true)

	x, y := barCenter(a.mb, 0)
	a.root.OnEvent(p2Down(x, y))
	a.root.OnEvent(p2Up(x, y))
	ix, iy := menuRowCenter(a.mb, 0, 0)
	// 菜单项悬在编辑器上方：弹出层收起后同一位置露出的是正文
	if !a.ta.GetBounds().Contains(graphics.Point{X: ix, Y: iy}) {
		t.Fatalf("test geometry: the menu item must sit above the editor, item at (%d,%d) editor %+v", ix, iy, a.ta.GetBounds())
	}
	if handled := a.root.OnEvent(p2Down(ix, iy)); !handled {
		t.Fatalf("pressing the item must be consumed")
	}
	a.root.OnEvent(p2Up(ix, iy))
	if activated != 1 || a.mb.IsOpen() {
		t.Fatalf("the item must run once and close the menu: activated=%d open=%v", activated, a.mb.IsOpen())
	}
	if !a.query.IsFocused() {
		t.Fatalf("opening the find bar must move keyboard focus to its query box")
	}

	if handled := a.root.OnEvent(p2Dbl(ix, iy)); !handled {
		t.Fatalf("the second click of a double-click on the menu item must be consumed by the menu bar")
	}
	if !a.query.IsFocused() || a.ta.IsFocused() {
		t.Fatalf("the double-click leaked into the editor: query focused=%v editor focused=%v", a.query.IsFocused(), a.ta.IsFocused())
	}
	if a.ta.HasSelection() {
		t.Fatalf("the double-click leaked into the editor and selected a word: %s", p2SelectionOf(a.ta))
	}
	// GetCursorPosition 为 1 起始行列：文档开头是 1:1
	if row, col := a.ta.GetCursorPosition(); row != 1 || col != 1 {
		t.Fatalf("editor caret moved to %d:%d", row, col)
	}
	if activated != 1 {
		t.Fatalf("the double-click must not run the item again: activated=%d", activated)
	}
}

// 子菜单项被鼠标激活后，紧随其后的双击同样不得穿透。
func TestP2_MenuBarConsumesDoubleClickAfterActivatingSubmenuItem(t *testing.T) {
	a := p2BuildAppRoot()
	activated := 0
	a.mb.SetMenus([]Menu{
		{Text: "查看", Items: []MenuEntry{
			{Text: "行号", OnClick: func() { activated++ }},
			{Text: "主题", Submenu: []MenuEntry{{Text: "深色", OnClick: func() { activated++ }}}},
		}},
	})
	a.ta.SetText(p2Doc)
	a.ta.SetFocused(true)

	x, y := barCenter(a.mb, 0)
	a.root.OnEvent(p2Down(x, y))
	sx, sy := menuRowCenter(a.mb, 0, 1)
	a.root.OnEvent(p2Move(sx, sy))
	sub := a.mb.submenuRect(0, 1)
	cx, cy := sub.X+sub.Width/2, sub.Y+4+a.mb.rowHeight/2
	a.root.OnEvent(p2Down(cx, cy))
	a.root.OnEvent(p2Up(cx, cy))
	if activated != 1 || a.mb.IsOpen() {
		t.Fatalf("submenu item must run once and close the menu: activated=%d open=%v", activated, a.mb.IsOpen())
	}
	if handled := a.root.OnEvent(p2Dbl(cx, cy)); !handled {
		t.Fatalf("double-click on the submenu item must be consumed")
	}
	if row, col := a.ta.GetCursorPosition(); row != 1 || col != 1 {
		t.Fatalf("the double-click leaked into the editor and moved the caret to %d:%d", row, col)
	}
	if a.ta.HasSelection() || activated != 1 {
		t.Fatalf("double-click leaked: selection=%s activated=%d", p2SelectionOf(a.ta), activated)
	}
}

// 菜单打开期间，落在弹出层（覆盖在编辑器上方）的双击必须被消费，编辑器不能选词。
func TestP2_MenuBarConsumesDoubleClickWhileOpen(t *testing.T) {
	clicks := map[string]int{}
	a := p2BuildAppRoot()
	a.mb.SetMenus([]Menu{
		{Text: "文件", Items: []MenuEntry{
			{Separator: true},
			{Text: "退出", Disabled: true, OnClick: func() { clicks["quit"]++ }},
		}},
	})
	a.ta.SetText(p2Doc)
	a.ta.SetFocused(true)

	x, y := barCenter(a.mb, 0)
	a.root.OnEvent(p2Down(x, y))
	dx, dy := menuRowCenter(a.mb, 0, 1)
	// 禁用项不会收起菜单：双击的两次点击都落在仍展开的弹出层上
	if !a.root.OnEvent(p2Down(dx, dy)) || !a.mb.IsOpen() {
		t.Fatalf("pressing a disabled item must keep the menu open")
	}
	if handled := a.root.OnEvent(p2Dbl(dx, dy)); !handled {
		t.Fatalf("double-click on an open popup must be consumed")
	}
	if a.ta.HasSelection() || a.ta.IsFocused() || !a.mb.IsOpen() {
		t.Fatalf("double-click leaked through the open menu: selection=%s editor focused=%v open=%v",
			p2SelectionOf(a.ta), a.ta.IsFocused(), a.mb.IsOpen())
	}

	// 标题与条带空白处
	bx := p2WinW - 10
	if handled := a.root.OnEvent(p2Dbl(bx, 10)); !handled {
		t.Fatalf("double-click on the open menu bar strip must be consumed")
	}
	a.mb.Close()
	if handled := a.root.OnEvent(p2Dbl(x, y)); !handled {
		t.Fatalf("double-click on a menu title must be consumed")
	}
	if handled := a.root.OnEvent(p2Dbl(bx, 10)); !handled {
		t.Fatalf("double-click on the empty menu bar strip must be consumed")
	}
	if a.mb.IsOpen() || a.ta.HasSelection() || len(clicks) != 0 {
		t.Fatalf("double-click on the bar must be swallowed without side effects: open=%v selection=%s clicks=%v",
			a.mb.IsOpen(), p2SelectionOf(a.ta), clicks)
	}
}

// 只吞掉“刚被鼠标点击过的位置”上的双击：离激活点较远的正文双击仍要正常选词。
func TestP2_MenuBarDoubleClickElsewhereStillSelectsWord(t *testing.T) {
	a := p2BuildAppRoot()
	a.mb.SetMenus([]Menu{{Text: "编辑", Items: []MenuEntry{{Text: "查找…", OnClick: func() { RequestFocus(a.query) }}}}})
	text := "alpha beta gamma delta epsilon zeta eta theta\n"
	for range 12 {
		text += text
	}
	a.ta.SetText(text)
	a.ta.SetFocused(true)

	x, y := barCenter(a.mb, 0)
	a.root.OnEvent(p2Down(x, y))
	ix, iy := menuRowCenter(a.mb, 0, 0)
	a.root.OnEvent(p2Down(ix, iy))
	a.root.OnEvent(p2Up(ix, iy))
	if !a.query.IsFocused() {
		t.Fatalf("opening the find bar must focus its query box")
	}

	// 在离激活点足够远的正文位置双击：交给编辑器选词并夺回焦点
	farX, farY := ix+300, iy+120
	if handled := a.root.OnEvent(p2Dbl(farX, farY)); !handled {
		t.Fatalf("double-click in the document must reach the editor")
	}
	if !a.ta.IsFocused() || !a.ta.HasSelection() {
		t.Fatalf("double-click far from the activated item must select a word: editor focused=%v selection=%s",
			a.ta.IsFocused(), p2SelectionOf(a.ta))
	}

	// 认领窗口有时限：同一位置上的双击在稍后（超过 doubleClickGrace）必须重新交给编辑器
	a.ta.SetSelection(0, 0, 0, 0)
	a.root.OnEvent(p2Down(x, y))
	a.root.OnEvent(p2Down(ix, iy))
	a.root.OnEvent(p2Up(ix, iy))
	a.mb.activatedAt = time.Now().Add(-2 * doubleClickGrace)
	a.root.OnEvent(p2Dbl(ix, iy))
	if !a.ta.HasSelection() {
		t.Fatalf("stale activation must not stop the editor from selecting a word: selection=%s", p2SelectionOf(a.ta))
	}
}

const (
	p2SubMenuIdx  = 5
	p2SubItemIdx  = 1
	p2SubmenuLen  = 3
	p2SubLastRowY = int32(4 + 2*30 + 15) // 子菜单末行中心相对子菜单顶边的偏移（rowHeight=30）
)

// p2SubmenuMenuBar 搭建六个菜单的菜单栏，子菜单（“主题 ▸”）放在最后一个菜单的第二项：
// 标题靠右，窗口较窄时父弹出层贴近右缘，子菜单会翻转到左侧。
func p2SubmenuMenuBar(width int32) *MenuBar {
	submenu := []MenuEntry{{Text: "深色"}, {Text: "浅色"}, {Text: "Nord"}}
	titles := []string{"文件", "编辑", "选择", "转到", "设置", "查看"}
	menus := make([]Menu, 0, len(titles))
	for _, title := range titles {
		menus = append(menus, Menu{Text: title, Items: []MenuEntry{{Text: "条目"}}})
	}
	menus[p2SubMenuIdx] = Menu{Text: titles[p2SubMenuIdx], Items: []MenuEntry{
		{Text: "行号"},
		{Text: "主题", Submenu: submenu},
	}}
	m := NewMenuBar()
	m.SetBounds(0, 0, width, 400)
	m.SetMenus(menus)
	return m
}

// 回归：子菜单与父弹出层之间留有 2px 间隙。鼠标从父菜单项逐像素移向子菜单时，
// 间隙内的点既不在弹出层也不在子菜单内，曾直接收起子菜单。
func TestP2_SubmenuStaysOpenWhileCrossingGap(t *testing.T) {
	m := p2SubmenuMenuBar(800)
	x, y := barCenter(m, p2SubMenuIdx)
	if !clickMenuBar(m, x, y) {
		t.Fatalf("clicking the menu title must open the menu")
	}
	px, py := menuRowCenter(m, p2SubMenuIdx, p2SubItemIdx)
	m.OnEvent(p2Move(px, py))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	if sub.X != parent.Right()+2 {
		t.Fatalf("test geometry: submenu must sit 2px to the right of the parent popup, parent right=%d submenu x=%d", parent.Right(), sub.X)
	}

	// 沿父菜单项中线逐像素右移：父菜单项 → 间隙 → 子菜单第一行
	for sx := px; sx <= sub.X+sub.Width/2; sx++ {
		m.OnEvent(p2Move(sx, py))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed at x=%d while moving from the parent item into the submenu (gap is [%d,%d))", sx, parent.Right(), sub.X)
		}
	}
	// 进入子菜单后悬停行照常更新
	if m.submenuFocus != 0 {
		t.Fatalf("pointer is on the first submenu row, focus=%d", m.submenuFocus)
	}
	m.OnEvent(p2Move(sub.X+sub.Width/2, sub.Y+p2SubLastRowY))
	if m.submenuOpen != p2SubItemIdx || m.submenuFocus != p2SubmenuLen-1 {
		t.Fatalf("hover inside the submenu: open=%d focus=%d", m.submenuOpen, m.submenuFocus)
	}

	// 移到父弹出层的普通行：子菜单照旧收起（原有行为）
	ox, oy := menuRowCenter(m, p2SubMenuIdx, 0)
	m.OnEvent(p2Move(ox, oy))
	if m.submenuOpen != -1 {
		t.Fatalf("hovering another parent row must still close the submenu, got %d", m.submenuOpen)
	}
}

// 父弹出层贴近右缘时子菜单翻转到左侧，间隙同样必须可以穿越。
func TestP2_SubmenuStaysOpenWhileCrossingGapFlippedLeft(t *testing.T) {
	m := p2SubmenuMenuBar(520)
	m.Open(p2SubMenuIdx)
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	if sub.Right()+2 != parent.X {
		t.Fatalf("test geometry: submenu must sit 2px to the left of the parent popup, submenu right=%d parent x=%d", sub.Right(), parent.X)
	}

	startX := parent.X + parent.Width/2
	py := parent.Y + 4 + m.rowHeight + m.rowHeight/2
	m.OnEvent(p2Move(startX, py))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}
	for sx := startX; sx >= sub.X+sub.Width/2; sx-- {
		m.OnEvent(p2Move(sx, py))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed at x=%d while moving left into the flipped submenu (gap is [%d,%d))", sx, sub.Right(), parent.X)
		}
	}
}

// 斜向移动：间隙带必须覆盖子菜单的整个纵向范围，在间隙内上下移动也不能收起子菜单。
func TestP2_SubmenuGapBandCoversSubmenuHeight(t *testing.T) {
	m := p2SubmenuMenuBar(800)
	x, y := barCenter(m, p2SubMenuIdx)
	clickMenuBar(m, x, y)
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	px, py := menuRowCenter(m, p2SubMenuIdx, p2SubItemIdx)
	m.OnEvent(p2Move(px, py))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("submenu must be open after hovering its parent item")
	}

	// 在间隙的第一列从父菜单项中线一路下到子菜单末行：只改变 Y，逐像素检查
	gapX := parent.Right()
	for gy := py; gy <= sub.Y+p2SubLastRowY; gy++ {
		m.OnEvent(p2Move(gapX, gy))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed in the gap at (%d,%d)", gapX, gy)
		}
	}
	// 横向进入子菜单末行：悬停行更新为最后一项
	m.OnEvent(p2Move(sub.X+sub.Width/2, sub.Y+p2SubLastRowY))
	if m.submenuOpen != p2SubItemIdx || m.submenuFocus != p2SubmenuLen-1 {
		t.Fatalf("hover on the last submenu row: open=%d focus=%d", m.submenuOpen, m.submenuFocus)
	}

	// 越过间隙带（子菜单底边以下）后恢复原行为：移出菜单区域收起子菜单
	m.OnEvent(p2Move(gapX, sub.Bottom()+2))
	if m.submenuOpen != -1 {
		t.Fatalf("moving below the submenu must still close it, got %d", m.submenuOpen)
	}
}

// 斜向移动：从父菜单项左侧的文字位置直接移向子菜单靠下的行，路径会先越过父菜单项的下边缘
// （弹出层内边距与层外区域，不属于任何一行），子菜单必须保持展开。
func TestP2_SubmenuStaysOpenOnDiagonalMove(t *testing.T) {
	m := p2SubmenuMenuBar(800)
	x, y := barCenter(m, p2SubMenuIdx)
	clickMenuBar(m, x, y)
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	startX, startY := parent.X+40, parent.Y+4+m.rowHeight+m.rowHeight/2
	endX, endY := sub.X+sub.Width/2, sub.Y+p2SubLastRowY
	m.OnEvent(p2Move(startX, startY))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}

	steps := int(endX - startX)
	crossedBelowPopup := false
	for i := 1; i <= steps; i++ {
		px := startX + (endX-startX)*int32(i)/int32(steps)
		py := startY + (endY-startY)*int32(i)/int32(steps)
		if py >= parent.Bottom() && px < parent.Right() {
			crossedBelowPopup = true
		}
		m.OnEvent(p2Move(px, py))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed at (%d,%d) on a diagonal move toward it (step %d of %d)", px, py, i, steps)
		}
	}
	if !crossedBelowPopup {
		t.Fatalf("test geometry: the diagonal path must pass below the parent popup")
	}
	if m.submenuFocus != p2SubmenuLen-1 {
		t.Fatalf("pointer is on the last submenu row, focus=%d", m.submenuFocus)
	}
}

// 通道只保护“朝子菜单去”的移动：从父菜单项直接向下/向左离开仍然照旧收起子菜单。
func TestP2_SubmenuStillClosesWhenLeavingAway(t *testing.T) {
	for _, tc := range []struct {
		name string
		dx   int32
	}{
		{"straight down", 0},
		{"down and slightly right", 6},
		{"down-left", -30},
	} {
		m := p2SubmenuMenuBar(800)
		x, y := barCenter(m, p2SubMenuIdx)
		clickMenuBar(m, x, y)
		parent := m.menuRect(p2SubMenuIdx)
		sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
		startX, startY := parent.X+40, parent.Y+4+m.rowHeight+m.rowHeight/2
		m.OnEvent(p2Move(startX, startY))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("%s: precondition: submenu must be open, got %d", tc.name, m.submenuOpen)
		}
		// 分步走到弹出层下方足够远处（远离子菜单底边对应的通道）
		targetY := parent.Bottom() + 40
		if targetY >= sub.Bottom() {
			t.Fatalf("%s: test geometry needs room below the popup", tc.name)
		}
		for py := startY + 1; py <= targetY; py++ {
			frac := float64(py-startY) / float64(targetY-startY)
			m.OnEvent(p2Move(startX+int32(float64(tc.dx)*frac), py))
			if m.submenuOpen < 0 {
				break
			}
		}
		if m.submenuOpen != -1 {
			t.Fatalf("%s: submenu must close when the pointer leaves the parent item away from it, open=%d", tc.name, m.submenuOpen)
		}
	}
}

// 通道顶点跟随父菜单项内的指针：先在父菜单项里横移、再转向斜向离开，同样不能收起子菜单。
func TestP2_SubmenuLaneFollowsPointerInsideParentItem(t *testing.T) {
	m := p2SubmenuMenuBar(800)
	x, y := barCenter(m, p2SubMenuIdx)
	clickMenuBar(m, x, y)
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	rowY := parent.Y + 4 + m.rowHeight + m.rowHeight/2
	m.OnEvent(p2Move(parent.X+20, rowY))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}
	turnX, turnY := parent.Right()-8, rowY
	for px := parent.X + 20; px <= turnX; px++ {
		m.OnEvent(p2Move(px, rowY))
	}
	endX, endY := sub.X+sub.Width/2, sub.Y+p2SubLastRowY
	steps := int(endX - turnX)
	for i := 1; i <= steps; i++ {
		px := turnX + (endX-turnX)*int32(i)/int32(steps)
		py := turnY + (endY-turnY)*int32(i)/int32(steps)
		m.OnEvent(p2Move(px, py))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed at (%d,%d) after turning toward it at the parent item's right end", px, py)
		}
	}
	if m.submenuFocus != p2SubmenuLen-1 {
		t.Fatalf("pointer is on the last submenu row, focus=%d", m.submenuFocus)
	}
}

// p2MidSubmenuMenuBar 把子菜单父项放在弹出层中间（下面还有普通项）。
func p2MidSubmenuMenuBar() *MenuBar {
	m := NewMenuBar()
	m.SetBounds(0, 0, 800, 400)
	m.SetMenus([]Menu{
		{Text: "文件", Items: []MenuEntry{{Text: "新建"}}},
		{Text: "查看", Items: []MenuEntry{
			{Text: "行号"},
			{Text: "主题", Submenu: []MenuEntry{{Text: "深色"}, {Text: "浅色"}, {Text: "Nord"}}},
			{Text: "放大"},
			{Text: "缩小"},
		}},
	})
	return m
}

// 父项下面还有其它行时：直接向下移到相邻行照旧收起子菜单；朝子菜单斜向移动（途经相邻行）则保持展开。
func TestP2_SubmenuLaneWithRowsBelowParentItem(t *testing.T) {
	const parentItem = 1
	m := p2MidSubmenuMenuBar()
	x, y := barCenter(m, 1)
	clickMenuBar(m, x, y)
	popup := m.menuRect(1)
	sub := m.submenuRect(1, parentItem)
	rowY := popup.Y + 4 + m.rowHeight + m.rowHeight/2
	startX := popup.X + 40
	m.OnEvent(p2Move(startX, rowY))
	if m.submenuOpen != parentItem {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}

	// 向下进入相邻的“放大”行：离开通道，子菜单收起
	closedAt := int32(-1)
	for py := rowY + 1; py <= rowY+m.rowHeight; py++ {
		m.OnEvent(p2Move(startX, py))
		if m.submenuOpen < 0 {
			closedAt = py
			break
		}
	}
	if closedAt < 0 || m.focus != parentItem+1 {
		t.Fatalf("moving straight down onto the next row must still close the submenu: open=%d focus=%d", m.submenuOpen, m.focus)
	}

	// 朝子菜单末行斜向移动：路径经过相邻行与弹出层下边缘，子菜单保持展开
	m.OnEvent(p2Move(startX, rowY))
	if m.submenuOpen != parentItem {
		t.Fatalf("re-hovering the submenu parent must reopen the submenu, got %d", m.submenuOpen)
	}
	endX, endY := sub.X+sub.Width/2, sub.Y+p2SubLastRowY
	steps := int(endX - startX)
	for i := 1; i <= steps; i++ {
		px := startX + (endX-startX)*int32(i)/int32(steps)
		py := rowY + (endY-rowY)*int32(i)/int32(steps)
		m.OnEvent(p2Move(px, py))
		if m.submenuOpen != parentItem {
			t.Fatalf("submenu closed at (%d,%d) on a diagonal move toward it past the rows below", px, py)
		}
	}
	if m.submenuFocus != p2SubmenuLen-1 {
		t.Fatalf("pointer is on the last submenu row, focus=%d", m.submenuFocus)
	}
}

// 键盘展开子菜单（没有指针位置）：以父菜单项中心为通道顶点，横移进入子菜单照常可用。
func TestP2_SubmenuLaneAfterKeyboardOpen(t *testing.T) {
	m := p2MidSubmenuMenuBar()
	m.Open(1)
	// ↓ 聚焦“主题”，Enter 展开子菜单（方向键 → 在当前实现里是切换到下一个顶级菜单）
	m.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x28}))
	m.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x0D}))
	if m.submenuOpen != 1 || m.submenuFocus != 0 {
		t.Fatalf("keyboard must open the submenu with its first item focused: open=%d focus=%d", m.submenuOpen, m.submenuFocus)
	}
	popup := m.menuRect(1)
	sub := m.submenuRect(1, 1)
	rowY := popup.Y + 4 + m.rowHeight + m.rowHeight/2
	for px := popup.X + popup.Width/2; px <= sub.X+sub.Width/2; px++ {
		m.OnEvent(p2Move(px, rowY))
		if m.submenuOpen != 1 {
			t.Fatalf("submenu closed at x=%d while moving from the parent item's centre into it", px)
		}
	}
}

// 子菜单自身的内边距（4px）也属于子菜单：指针贴着子菜单底边/顶边移动不能收起它，
// 越过底边之后才恢复“移出菜单即收起”。
func TestP2_SubmenuPaddingKeepsSubmenuOpen(t *testing.T) {
	m := p2SubmenuMenuBar(800)
	x, y := barCenter(m, p2SubMenuIdx)
	clickMenuBar(m, x, y)
	parent := m.menuRect(p2SubMenuIdx)
	sub := m.submenuRect(p2SubMenuIdx, p2SubItemIdx)
	m.OnEvent(p2Move(parent.X+40, parent.Y+4+m.rowHeight+m.rowHeight/2))
	if m.submenuOpen != p2SubItemIdx {
		t.Fatalf("hovering the submenu parent must open the submenu, got %d", m.submenuOpen)
	}
	bottomPadY := sub.Y + 4 + m.rowHeight*int32(p2SubmenuLen)
	if bottomPadY >= sub.Bottom() {
		t.Fatalf("test geometry: submenu bottom padding expected below the last row")
	}
	for _, pt := range [][2]int32{
		{sub.X + sub.Width/2, sub.Y + 2},    // 顶边内边距
		{sub.X + sub.Width/2, bottomPadY},   // 底边内边距
		{sub.X + sub.Width - 2, sub.Y + 20}, // 右侧内边距
		{sub.Right() - 1, sub.Bottom() - 1}, // 右下角
	} {
		m.OnEvent(p2Move(pt[0], pt[1]))
		if m.submenuOpen != p2SubItemIdx {
			t.Fatalf("submenu closed at (%d,%d), a padding point inside its own rect", pt[0], pt[1])
		}
	}
	m.OnEvent(p2Move(sub.X+sub.Width/2, sub.Bottom()+2))
	if m.submenuOpen != -1 {
		t.Fatalf("moving below the submenu must close it, open=%d", m.submenuOpen)
	}
}
