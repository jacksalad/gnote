package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// Menu 顶部菜单栏中的一个菜单。
type Menu struct {
	Text  string
	Items []MenuEntry
}

// MenuEntry 菜单项。Separator 为 true 时其余字段忽略。
// Submenu 目前支持一级子菜单；子菜单项不再递归。
type MenuEntry struct {
	Text      string
	Accel     string
	Disabled  bool
	Checked   bool
	Separator bool
	Submenu   []MenuEntry
	OnClick   func()
}

// MenuBar 全客户区自绘菜单层。
//
// 它不是 34px 高的普通面板：下拉菜单需要覆盖编辑器，因此 MenuBar 的 bounds
// 必须设置为整个客户区，顶部 barHeight 像素绘制菜单栏，其余区域透明。
// 关闭时只消费菜单栏区域的鼠标事件；打开时消费全部鼠标与键盘事件。
type MenuBar struct {
	BaseComponent

	font         *Font
	barHeight    int32
	itemPadding  int32
	rowHeight    int32
	separatorGap int32
	menuMinWidth int32
	radius       int32

	build func() []Menu
	menus []Menu

	open         int
	hover        int
	focus        int
	submenuOpen  int
	submenuFocus int
	focused      bool

	onOpenChange func(open bool)

	barWidths  []int32
	menuWidths []int32

	// 主题色
	barBg       graphics.Color
	popupBg     graphics.Color
	itemHoverBg graphics.Color
	border      graphics.Color
	textColor   graphics.Color
	mutedColor  graphics.Color
	disabledCol graphics.Color
	accentColor graphics.Color
	shadowColor graphics.Color
}

// NewMenuBar 创建全客户区菜单层。
func NewMenuBar() *MenuBar {
	m := &MenuBar{
		BaseComponent: NewBaseComponent(),
		font:          DefaultFont,
		barHeight:     34,
		itemPadding:   12,
		rowHeight:     30,
		separatorGap:  9,
		menuMinWidth:  180,
		radius:        8,
		open:          -1,
		hover:         -1,
		focus:         -1,
		submenuOpen:   -1,
		submenuFocus:  -1,
	}
	m.ApplyTheme(theme.Current())
	return m
}

// SetFont 设置菜单字体。
func (m *MenuBar) SetFont(font *Font) {
	if font == nil {
		return
	}
	m.font = font
	m.rebuildWidths()
}

// SetBarHeight 设置顶部菜单栏高度。
func (m *MenuBar) SetBarHeight(height int32) {
	if height > 0 {
		m.barHeight = height
	}
}

// BarHeight 返回顶部菜单栏高度，供宿主预留内容区域。
func (m *MenuBar) BarHeight() int32 {
	return m.barHeight
}

// SetRowHeight 设置下拉菜单行高。
func (m *MenuBar) SetRowHeight(height int32) {
	if height > 0 {
		m.rowHeight = height
	}
}

// SetSeparatorGap 设置分隔线高度。
func (m *MenuBar) SetSeparatorGap(height int32) {
	if height > 0 {
		m.separatorGap = height
	}
}

// SetMenuMinWidth 设置下拉菜单最小宽度。
func (m *MenuBar) SetMenuMinWidth(width int32) {
	if width > 0 {
		m.menuMinWidth = width
		m.rebuildWidths()
	}
}

// SetRadius 设置下拉菜单圆角。
func (m *MenuBar) SetRadius(radius int32) {
	if radius > 0 {
		m.radius = radius
	}
}

// SetItemPadding 设置菜单标题左右内边距。
func (m *MenuBar) SetItemPadding(padding int32) {
	if padding > 0 {
		m.itemPadding = padding
		m.rebuildWidths()
	}
}

// SetModel 设置动态菜单模型。每次打开菜单都会重新调用 build，
// 以便 Enabled/Checked 取到最新的撤销栈、选区、主题等状态。
func (m *MenuBar) SetModel(build func() []Menu) {
	m.build = build
	m.rebuild()
}

// SetMenus 设置静态菜单模型。
func (m *MenuBar) SetMenus(menus []Menu) {
	m.ensureDefaults()
	m.build = nil
	m.menus = menus
	m.rebuildWidths()
}

// SetOnOpenChange 设置打开/关闭回调。宿主用它保存并恢复编辑器焦点。
func (m *MenuBar) SetOnOpenChange(fn func(open bool)) {
	m.onOpenChange = fn
}

// IsOpen 返回是否有菜单处于展开状态。
func (m *MenuBar) IsOpen() bool {
	return m.open >= 0
}

// Open 打开指定菜单，并重新构建动态模型。
func (m *MenuBar) Open(index int) {
	if index < 0 || index >= len(m.menus) {
		return
	}
	wasOpen := m.open >= 0
	m.rebuild()
	if index >= len(m.menus) {
		return
	}
	m.openMenu(index)
	if !wasOpen && m.onOpenChange != nil {
		m.onOpenChange(true)
	}
}

// Close 关闭菜单与子菜单。
func (m *MenuBar) Close() {
	if m.open < 0 && m.submenuOpen < 0 {
		return
	}
	m.open = -1
	m.hover = -1
	m.focus = -1
	m.submenuOpen = -1
	m.submenuFocus = -1
	if m.onOpenChange != nil {
		m.onOpenChange(false)
	}
}

// SetFocused 实现 Focusable。
func (m *MenuBar) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused 实现 Focusable。
func (m *MenuBar) IsFocused() bool {
	return m.focused
}

// ApplyTheme 应用主题色。
func (m *MenuBar) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	m.barBg = t.Surface
	m.popupBg = t.Surface
	m.itemHoverBg = t.SurfaceVariant
	m.border = t.Border
	m.textColor = t.TextPrimary
	m.mutedColor = t.TextSecondary
	m.disabledCol = t.TextDisabled
	m.accentColor = t.Primary
	m.shadowColor = t.Shadow
}

// Render 绘制顶部菜单栏、下拉菜单与子菜单。
func (m *MenuBar) Render(canvas *render.Canvas) {
	if !m.visible {
		return
	}
	m.ensureDefaults()
	if m.build != nil && len(m.menus) == 0 {
		m.rebuild()
	}
	m.drawBar(canvas)
	if m.open < 0 || m.open >= len(m.menus) {
		return
	}
	m.drawPopup(canvas, m.menus[m.open].Items, m.menuRect(m.open), m.focus)
	if m.submenuOpen >= 0 && m.submenuOpen < len(m.menus[m.open].Items) {
		item := m.menus[m.open].Items[m.submenuOpen]
		if len(item.Submenu) > 0 {
			m.drawPopup(canvas, item.Submenu, m.submenuRect(m.open, m.submenuOpen), m.submenuFocus)
		}
	}
}

// OnEvent 处理菜单栏/下拉/子菜单的鼠标与键盘事件。
func (m *MenuBar) OnEvent(evt event.Event) bool {
	if !m.visible || !m.enabled {
		return false
	}
	m.ensureDefaults()

	switch evt.Type {
	case event.EventMouseMove:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		return m.handleMouseMove(data)

	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		// 菜单打开时任何按钮都先收起菜单，避免右键菜单与自绘菜单同时存在。
		if m.open >= 0 && data.Button != event.MouseButtonLeft {
			m.Close()
			return true
		}
		if data.Button != event.MouseButtonLeft {
			return false
		}
		return m.handleMouseDown(data)

	case event.EventMouseUp:
		if m.open >= 0 {
			return true
		}
		data, ok := evt.Data.(event.MouseData)
		return ok && m.pointInBar(data.Y)

	case event.EventKeyDown:
		key, ok := evt.Data.(event.KeyData)
		if !ok || m.open < 0 {
			return false
		}
		return m.handleKeyDown(key)

	case event.EventKeyPress:
		// 菜单打开时吞掉字符输入，避免键入编辑器正文。
		return m.open >= 0

	case event.EventBlur:
		wasOpen := m.open >= 0
		m.Close()
		m.hover = -1
		return wasOpen
	}

	return false
}

func (m *MenuBar) pointInBar(y int32) bool {
	return y >= m.bounds.Y && y < m.bounds.Y+m.barHeight
}

func (m *MenuBar) handleMouseMove(data event.MouseData) bool {
	if m.open < 0 {
		idx := m.barItemAt(data.X, data.Y)
		if idx != m.hover {
			m.hover = idx
			return true
		}
		return false
	}

	// 打开状态下在菜单标题间切换
	if idx := m.barItemAt(data.X, data.Y); idx >= 0 {
		if idx != m.open {
			m.openMenu(idx)
			return true
		}
		if m.submenuOpen >= 0 {
			m.submenuOpen = -1
			m.submenuFocus = -1
			return true
		}
		return false
	}

	// 子菜单 hover
	if m.submenuOpen >= 0 {
		if j := m.submenuRowAt(data.X, data.Y); j >= 0 {
			if j != m.submenuFocus {
				m.submenuFocus = j
				return true
			}
			return false
		}
	}

	// 主菜单 hover
	if j := m.menuRowAt(data.X, data.Y); j >= 0 {
		items := m.menus[m.open].Items
		changed := j != m.focus
		m.focus = j
		if len(items[j].Submenu) > 0 {
			if m.submenuOpen != j {
				m.submenuOpen = j
				m.submenuFocus = firstSelectable(items[j].Submenu)
				changed = true
			}
		} else if m.submenuOpen >= 0 {
			m.submenuOpen = -1
			m.submenuFocus = -1
			changed = true
		}
		return changed
	}

	// 移出菜单但仍在窗口内：收起子菜单，保持主菜单展开
	if m.submenuOpen >= 0 {
		m.submenuOpen = -1
		m.submenuFocus = -1
		return true
	}
	return false
}

func (m *MenuBar) handleMouseDown(data event.MouseData) bool {
	if m.open < 0 {
		if idx := m.barItemAt(data.X, data.Y); idx >= 0 {
			m.Open(idx)
			return true
		}
		return m.pointInBar(data.Y)
	}

	// 点击菜单标题：同一个关闭，不同则切换
	if idx := m.barItemAt(data.X, data.Y); idx >= 0 {
		if idx == m.open {
			m.Close()
		} else {
			m.openMenu(idx)
		}
		return true
	}

	// 点击子菜单
	if m.submenuOpen >= 0 {
		if j := m.submenuRowAt(data.X, data.Y); j >= 0 {
			items := m.menus[m.open].Items[m.submenuOpen].Submenu
			item := items[j]
			if !item.Disabled {
				m.activate(item)
			}
			return true
		}
	}

	// 点击主菜单
	if j := m.menuRowAt(data.X, data.Y); j >= 0 {
		items := m.menus[m.open].Items
		item := items[j]
		if item.Disabled {
			return true
		}
		if len(item.Submenu) > 0 {
			m.focus = j
			m.submenuOpen = j
			m.submenuFocus = firstSelectable(item.Submenu)
			return true
		}
		m.activate(item)
		return true
	}

	// 点击菜单外：关闭并消费，避免同时把编辑器光标定位到点击处
	m.Close()
	return true
}

func (m *MenuBar) handleKeyDown(key event.KeyData) bool {
	const (
		vkEscape = 0x1B
		vkHome   = 0x24
		vkLeft   = 0x25
		vkUp     = 0x26
		vkRight  = 0x27
		vkDown   = 0x28
		vkEnd    = 0x23
		vkEnter  = 0x0D
		vkSpace  = 0x20
	)

	switch key.KeyCode {
	case vkEscape:
		if m.submenuOpen >= 0 {
			m.submenuOpen = -1
			m.submenuFocus = -1
		} else {
			m.Close()
		}
		return true

	case vkLeft:
		m.switchMenu(-1)
		return true
	case vkRight:
		m.switchMenu(1)
		return true

	case vkUp:
		if m.submenuOpen >= 0 {
			m.moveSubmenuFocus(-1)
		} else {
			m.moveFocus(-1)
		}
		return true
	case vkDown:
		if m.submenuOpen >= 0 {
			m.moveSubmenuFocus(1)
		} else {
			m.moveFocus(1)
		}
		return true
	case vkHome:
		if m.submenuOpen >= 0 {
			m.submenuFocus = firstSelectable(m.currentSubmenu())
		} else {
			m.focus = firstSelectable(m.currentItems())
		}
		return true
	case vkEnd:
		if m.submenuOpen >= 0 {
			m.submenuFocus = lastSelectable(m.currentSubmenu())
		} else {
			m.focus = lastSelectable(m.currentItems())
		}
		return true
	case vkEnter, vkSpace:
		m.activateFocused()
		return true
	}

	// 菜单打开时消费所有按键，避免方向键/字符同时进入编辑器。
	return true
}

func (m *MenuBar) currentItems() []MenuEntry {
	if m.open < 0 || m.open >= len(m.menus) {
		return nil
	}
	return m.menus[m.open].Items
}

func (m *MenuBar) currentSubmenu() []MenuEntry {
	items := m.currentItems()
	if m.submenuOpen < 0 || m.submenuOpen >= len(items) {
		return nil
	}
	return items[m.submenuOpen].Submenu
}

func (m *MenuBar) activateFocused() {
	if m.submenuOpen >= 0 {
		items := m.currentSubmenu()
		if m.submenuFocus < 0 || m.submenuFocus >= len(items) {
			return
		}
		if !items[m.submenuFocus].Disabled {
			m.activate(items[m.submenuFocus])
		}
		return
	}
	items := m.currentItems()
	if m.focus < 0 || m.focus >= len(items) {
		return
	}
	item := items[m.focus]
	if item.Disabled {
		return
	}
	if len(item.Submenu) > 0 {
		m.submenuOpen = m.focus
		m.submenuFocus = firstSelectable(item.Submenu)
		return
	}
	m.activate(item)
}

func (m *MenuBar) activate(item MenuEntry) {
	onClick := item.OnClick
	m.Close()
	if onClick != nil {
		onClick()
	}
}

func (m *MenuBar) switchMenu(delta int) {
	if len(m.menus) == 0 {
		return
	}
	idx := (m.open + delta) % len(m.menus)
	if idx < 0 {
		idx += len(m.menus)
	}
	m.openMenu(idx)
}

func (m *MenuBar) openMenu(index int) {
	if index < 0 || index >= len(m.menus) {
		return
	}
	m.open = index
	m.hover = index
	m.focus = firstSelectable(m.menus[index].Items)
	m.submenuOpen = -1
	m.submenuFocus = -1
}

func (m *MenuBar) moveFocus(delta int) {
	items := m.currentItems()
	if len(items) == 0 {
		return
	}
	if m.focus < 0 {
		m.focus = 0
	}
	for range len(items) {
		m.focus = (m.focus + delta + len(items)) % len(items)
		if !items[m.focus].Separator {
			return
		}
	}
}

func (m *MenuBar) moveSubmenuFocus(delta int) {
	items := m.currentSubmenu()
	if len(items) == 0 {
		return
	}
	if m.submenuFocus < 0 {
		m.submenuFocus = 0
	}
	for range len(items) {
		m.submenuFocus = (m.submenuFocus + delta + len(items)) % len(items)
		if !items[m.submenuFocus].Separator {
			return
		}
	}
}

func firstSelectable(items []MenuEntry) int {
	for i, item := range items {
		if !item.Separator {
			return i
		}
	}
	return -1
}

func lastSelectable(items []MenuEntry) int {
	for i := len(items) - 1; i >= 0; i-- {
		if !items[i].Separator {
			return i
		}
	}
	return -1
}

func (m *MenuBar) ensureDefaults() {
	if m.font == nil {
		m.font = DefaultFont
	}
	if m.barHeight <= 0 {
		m.barHeight = 34
	}
	if m.itemPadding <= 0 {
		m.itemPadding = 12
	}
	if m.rowHeight <= 0 {
		m.rowHeight = 30
	}
	if m.separatorGap <= 0 {
		m.separatorGap = 9
	}
	if m.menuMinWidth <= 0 {
		m.menuMinWidth = 180
	}
	if m.radius <= 0 {
		m.radius = 8
	}
}

func (m *MenuBar) rebuild() {
	m.ensureDefaults()
	if m.build != nil {
		m.menus = m.build()
	}
	m.rebuildWidths()
}

func (m *MenuBar) rebuildWidths() {
	fm := render.GetFontManager()
	m.barWidths = make([]int32, len(m.menus))
	m.menuWidths = make([]int32, len(m.menus))
	for i, menu := range m.menus {
		w, _ := fm.MeasureText(m.font, menu.Text)
		m.barWidths[i] = w + m.itemPadding*2
		if m.barWidths[i] < 44 {
			m.barWidths[i] = 44
		}
		maxW := int32(0)
		for _, item := range menu.Items {
			if item.Separator {
				continue
			}
			tw, _ := fm.MeasureText(m.font, item.Text)
			w := tw + 56
			if item.Accel != "" {
				aw, _ := fm.MeasureText(m.font, item.Accel)
				w += aw + 24
			}
			if len(item.Submenu) > 0 {
				w += 18
			}
			if w > maxW {
				maxW = w
			}
		}
		if maxW < m.menuMinWidth {
			maxW = m.menuMinWidth
		}
		m.menuWidths[i] = maxW
	}
}

func (m *MenuBar) barItemRect(index int) graphics.Rect {
	x := m.bounds.X + 4
	for _, width := range m.barWidths[:min(index, len(m.barWidths))] {
		x += width
	}
	width := int32(0)
	if index >= 0 && index < len(m.barWidths) {
		width = m.barWidths[index]
	}
	return graphics.NewRect(x, m.bounds.Y, width, m.barHeight)
}

func (m *MenuBar) barItemAt(x, y int32) int {
	if y < m.bounds.Y || y >= m.bounds.Y+m.barHeight {
		return -1
	}
	px := m.bounds.X + 4
	for i, width := range m.barWidths {
		if x >= px && x < px+width {
			return i
		}
		px += width
	}
	return -1
}

func (m *MenuBar) entriesHeight(items []MenuEntry) int32 {
	h := int32(8)
	for _, item := range items {
		if item.Separator {
			h += m.separatorGap
		} else {
			h += m.rowHeight
		}
	}
	return h
}

func (m *MenuBar) menuRect(index int) graphics.Rect {
	x := m.barItemRect(index).X
	w := m.menuWidths[index]
	h := m.entriesHeight(m.menus[index].Items)
	if x+w > m.bounds.X+m.bounds.Width-2 {
		x = m.bounds.X + m.bounds.Width - w - 2
	}
	if x < m.bounds.X+2 {
		x = m.bounds.X + 2
	}
	y := m.bounds.Y + m.barHeight + 2
	if y+h > m.bounds.Y+m.bounds.Height-2 {
		y = m.bounds.Y + m.bounds.Height - h - 2
	}
	if y < m.bounds.Y+m.barHeight+2 {
		y = m.bounds.Y + m.barHeight + 2
	}
	return graphics.NewRect(x, y, w, h)
}

func (m *MenuBar) entryRectAt(items []MenuEntry, popup graphics.Rect, x, y int32) int {
	if !containsRect(popup, x, y) {
		return -1
	}
	rowY := popup.Y + 4
	for i, item := range items {
		h := m.rowHeight
		if item.Separator {
			h = m.separatorGap
		}
		if y >= rowY && y < rowY+h {
			return i
		}
		rowY += h
	}
	return -1
}

func (m *MenuBar) menuRowAt(x, y int32) int {
	if m.open < 0 {
		return -1
	}
	return m.entryRectAt(m.menus[m.open].Items, m.menuRect(m.open), x, y)
}

func (m *MenuBar) submenuRect(menuIndex, itemIndex int) graphics.Rect {
	parent := m.menuRect(menuIndex)
	item := m.menus[menuIndex].Items[itemIndex]
	w := m.measureSubmenuWidth(item.Submenu)
	h := m.entriesHeight(item.Submenu)

	rowY := parent.Y + 4
	for i := 0; i < itemIndex; i++ {
		if m.menus[menuIndex].Items[i].Separator {
			rowY += m.separatorGap
		} else {
			rowY += m.rowHeight
		}
	}

	x := parent.X + parent.Width + 2
	if x+w > m.bounds.X+m.bounds.Width-2 {
		x = parent.X - w - 2
	}
	if x < m.bounds.X+2 {
		x = m.bounds.X + 2
	}
	y := rowY
	if y+h > m.bounds.Y+m.bounds.Height-2 {
		y = m.bounds.Y + m.bounds.Height - h - 2
	}
	if y < m.bounds.Y+m.barHeight+2 {
		y = m.bounds.Y + m.barHeight + 2
	}
	return graphics.NewRect(x, y, w, h)
}

func (m *MenuBar) submenuRowAt(x, y int32) int {
	if m.open < 0 || m.submenuOpen < 0 {
		return -1
	}
	items := m.menus[m.open].Items
	if m.submenuOpen >= len(items) {
		return -1
	}
	return m.entryRectAt(items[m.submenuOpen].Submenu, m.submenuRect(m.open, m.submenuOpen), x, y)
}

func (m *MenuBar) measureSubmenuWidth(items []MenuEntry) int32 {
	fm := render.GetFontManager()
	maxW := int32(160)
	for _, item := range items {
		if item.Separator {
			continue
		}
		tw, _ := fm.MeasureText(m.font, item.Text)
		w := tw + 56
		if item.Accel != "" {
			aw, _ := fm.MeasureText(m.font, item.Accel)
			w += aw + 24
		}
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (m *MenuBar) drawBar(canvas *render.Canvas) {
	bar := graphics.NewRect(m.bounds.X, m.bounds.Y, m.bounds.Width, m.barHeight)
	canvas.FillRect(bar.X, bar.Y, bar.Width, bar.Height, m.barBg)
	canvas.DrawHorizontalLine(bar.X, bar.X+bar.Width-1, bar.Y+bar.Height-1, m.border)

	for i, menu := range m.menus {
		r := m.barItemRect(i)
		active := i == m.open
		hovered := i == m.hover
		if active || hovered {
			canvas.FillRoundedRect(r.X+2, r.Y+4, r.Width-4, r.Height-8, 5, m.itemHoverBg)
		}
		if active {
			canvas.FillRect(r.X+8, r.Y+r.Height-2, r.Width-16, 2, m.accentColor)
		}
		tw, textH := canvas.MeasureText(menu.Text, m.font)
		x := r.X + (r.Width-tw)/2
		y := r.Y + (r.Height-textH)/2
		canvas.DrawText(menu.Text, x, y, m.font, m.textColor)
	}
}

func (m *MenuBar) drawPopup(canvas *render.Canvas, items []MenuEntry, rect graphics.Rect, focused int) {
	// 阴影 + 背景 + 边框
	canvas.FillRoundedRect(rect.X+3, rect.Y+3, rect.Width, rect.Height, m.radius, m.shadowColor)
	canvas.FillRoundedRect(rect.X, rect.Y, rect.Width, rect.Height, m.radius, m.popupBg)
	canvas.DrawRoundedRect(rect.X, rect.Y, rect.Width, rect.Height, m.radius, m.border)

	fm := render.GetFontManager()
	rowY := rect.Y + 4
	for i, item := range items {
		h := m.rowHeight
		if item.Separator {
			h = m.separatorGap
		}
		row := graphics.NewRect(rect.X+4, rowY, rect.Width-8, h)
		if item.Separator {
			lineY := row.Y + row.Height/2
			canvas.DrawHorizontalLine(row.X+8, row.Right()-8, lineY, m.border)
			rowY += h
			continue
		}

		if i == focused && !item.Disabled {
			canvas.FillRoundedRect(row.X, row.Y+1, row.Width, row.Height-2, 4, m.itemHoverBg)
		}

		color := m.textColor
		if item.Disabled {
			color = m.disabledCol
		}

		// 勾选标记
		if item.Checked {
			cx := row.X + 12
			cy := row.Y + row.Height/2
			checkColor := m.accentColor
			if item.Disabled {
				checkColor = m.disabledCol
			}
			canvas.DrawLine(cx-4, cy, cx-1, cy+3, checkColor)
			canvas.DrawLine(cx-1, cy+3, cx+5, cy-4, checkColor)
		}

		// 主文本
		textX := row.X + 24
		_, textH := fm.MeasureText(m.font, item.Text)
		textY := row.Y + (row.Height-textH)/2
		canvas.DrawText(item.Text, textX, textY, m.font, color)

		// 快捷键
		if item.Accel != "" {
			aw, _ := fm.MeasureText(m.font, item.Accel)
			ax := row.Right() - 10 - aw
			canvas.DrawText(item.Accel, ax, textY, m.font, m.mutedColor)
		}

		// 子菜单箭头
		if len(item.Submenu) > 0 {
			ax := row.Right() - 14
			ay := row.Y + row.Height/2
			canvas.DrawLine(ax-4, ay-4, ax, ay, m.mutedColor)
			canvas.DrawLine(ax, ay, ax-4, ay+4, m.mutedColor)
		}

		rowY += h
	}
}

func containsRect(rect graphics.Rect, x, y int32) bool {
	return x >= rect.X && x < rect.X+rect.Width && y >= rect.Y && y < rect.Y+rect.Height
}
