package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// TabBar 纵向分组导航，用于设置窗口左侧的“编辑器 / 外观 / 关于”等页签。
type TabBar struct {
	BaseComponent

	items    []string
	selected int
	font     *Font
	rowH     int32
	onSelect func(int)

	activeBg graphics.Color
	activeFg graphics.Color
	text     graphics.Color
	accent   graphics.Color
}

// NewTabBar 创建纵向页签栏。
func NewTabBar(items []string) *TabBar {
	t := &TabBar{
		BaseComponent: NewBaseComponent(),
		items:         items,
		font:          DefaultFont,
		rowH:          42,
		selected:      0,
	}
	t.ApplyTheme(theme.Current())
	return t
}

// SetFont 设置字体。
func (t *TabBar) SetFont(font *Font) {
	if font != nil {
		t.font = font
	}
}

// SetSelected 设置选中项，不触发回调。
func (t *TabBar) SetSelected(index int) {
	if index >= 0 && index < len(t.items) {
		t.selected = index
	}
}

// Selected 返回当前选中索引。
func (t *TabBar) Selected() int {
	return t.selected
}

// OnSelect 设置页签切换回调。
func (t *TabBar) OnSelect(fn func(int)) {
	t.onSelect = fn
}

// ApplyTheme 应用主题色。
func (t *TabBar) ApplyTheme(th *theme.Theme) {
	if th == nil {
		return
	}
	t.activeBg = th.SurfaceVariant
	t.activeFg = th.TextPrimary
	t.text = th.TextSecondary
	t.accent = th.Primary
}

// Render 绘制纵向页签。
func (t *TabBar) Render(canvas *render.Canvas) {
	if !t.visible || len(t.items) == 0 {
		return
	}
	for i, label := range t.items {
		row := graphics.NewRect(t.bounds.X, t.bounds.Y+int32(i)*t.rowH, t.bounds.Width, t.rowH)
		if row.Y+row.Height > t.bounds.Bottom() {
			break
		}
		color := t.text
		if i == t.selected {
			canvas.FillRoundedRect(row.X+8, row.Y+4, row.Width-16, row.Height-8, 6, t.activeBg)
			canvas.FillRect(row.X+2, row.Y+10, 3, row.Height-20, t.accent)
			color = t.activeFg
		}
		_, th := canvas.MeasureText(label, t.font)
		canvas.DrawText(label, row.X+22, row.Y+(row.Height-th)/2, t.font, color)
	}
}

// GetCursor 返回手型光标。
func (t *TabBar) GetCursor() int {
	return 32649
}

// OnEvent 处理页签点击。
func (t *TabBar) OnEvent(evt event.Event) bool {
	if !t.visible || !t.enabled || len(t.items) == 0 {
		return false
	}
	data, ok := evt.Data.(event.MouseData)
	if !ok || data.Button != event.MouseButtonLeft || evt.Type != event.EventMouseDown {
		return false
	}
	if !IsInBounds(data.X, data.Y, t.bounds) {
		return false
	}
	index := int((data.Y - t.bounds.Y) / t.rowH)
	if index < 0 {
		index = 0
	}
	if index >= len(t.items) {
		index = len(t.items) - 1
	}
	if index != t.selected {
		t.selected = index
		if t.onSelect != nil {
			t.onSelect(index)
		}
	}
	return true
}
