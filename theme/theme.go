// Package theme 提供 goui 统一的主题样式与设计令牌系统
package theme

import (
	"strings"
	"sync"

	"github.com/jacksalad/gnote/graphics"
)

// Theme 定义整套 UI 控件与语法高亮所需的色彩令牌规范。
//
// 一套主题必须同时给出界面令牌与语法令牌，保证编辑器、菜单、
// 设置窗口与代码高亮在视觉上属于同一套配色，而不是“UI 一套、代码一套”。
type Theme struct {
	ID    string // 持久化标识 (dark, light, nord ...)
	Name  string // 英文名称 (Dark, Light, Nord ...)
	Label string // 中文显示名，用于菜单与设置页
	Dark  bool   // 是否为暗色主题

	Background     graphics.Color // 窗口最底层背景色
	Surface        graphics.Color // 面板、卡片、容器背景色
	SurfaceVariant graphics.Color // 次级容器、悬停底色、输入框背景
	Primary        graphics.Color // 主题核心强调色（主按钮、进度条、激活状态）
	PrimaryHover   graphics.Color // 强调色悬停
	PrimaryActive  graphics.Color // 强调色按下
	OnPrimary      graphics.Color // 强调色之上的文字/图标颜色
	TextPrimary    graphics.Color // 主要正文文本颜色
	TextSecondary  graphics.Color // 次要说明文本颜色
	TextDisabled   graphics.Color // 禁用状态文本颜色
	Border         graphics.Color // 常规边框线条颜色
	BorderFocused  graphics.Color // 聚焦或高亮边框线条颜色
	FocusRing      graphics.Color // 按钮等控件的键盘焦点环颜色
	Track          graphics.Color // 进度条、滑块、滚动条等轨道底色
	Thumb          graphics.Color // 滑块、滚动条等活动滑块颜色
	ThumbHover     graphics.Color // 滑块悬停颜色
	Selection      graphics.Color // 文本选区底色（可带 alpha）
	Shadow         graphics.Color // 菜单、弹层阴影颜色（可带 alpha）
	Success        graphics.Color // 成功状态颜色
	Warning        graphics.Color // 警告状态颜色
	Danger         graphics.Color // 危险/错误状态颜色

	// 编辑器专属令牌，避免 TextArea 内部再按明暗分支硬编码颜色。
	GutterBg         graphics.Color // 行号槽底色
	GutterText       graphics.Color // 行号文字
	GutterBorder     graphics.Color // 行号槽右边界线
	ActiveLineBg     graphics.Color // 当前行高亮底色
	ActiveLineBorder graphics.Color // 当前行上下边界线

	Syntax SyntaxPalette // 跨语言语法高亮语义令牌
}

var (
	currentMu sync.RWMutex
	current   = Dark()
)

// Current 获取当前全局生效的主题
func Current() *Theme {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current
}

// SetCurrent 切换当前全局主题
func SetCurrent(t *Theme) {
	if t == nil {
		return
	}
	currentMu.Lock()
	current = t
	currentMu.Unlock()
}

var (
	builtinOnce   sync.Once
	builtinThemes []*Theme
)

// All 返回内置主题列表。顺序即菜单与设置页的展示顺序。
// 调色板只在首次调用时构建；返回切片副本，调用方可以安全排序/截取。
func All() []*Theme {
	builtinOnce.Do(func() {
		builtinThemes = []*Theme{
			Dark(), Light(), Nord(), TokyoNight(), OneDark(),
			GruvboxDark(), SolarizedLight(), RosePine(),
		}
	})
	out := make([]*Theme, len(builtinThemes))
	copy(out, builtinThemes)
	return out
}

// ByID 按持久化标识查找主题；未知标识返回 nil。
func ByID(id string) *Theme {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, t := range All() {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Dark 返回现代化极客暗色调主题 (Zinc / Indigo 调色板)
func Dark() *Theme {
	return &Theme{
		ID:             "dark",
		Name:           "Dark",
		Label:          "深色",
		Dark:           true,
		Background:     graphics.RGB(24, 24, 27),    // #18181B Zinc-900
		Surface:        graphics.RGB(39, 39, 42),    // #27272A Zinc-800
		SurfaceVariant: graphics.RGB(63, 63, 70),    // #3F3F46 Zinc-700
		Primary:        graphics.RGB(99, 102, 241),  // #6366F1 Indigo-500
		PrimaryHover:   graphics.RGB(129, 140, 248), // #818CF8 Indigo-400
		PrimaryActive:  graphics.RGB(79, 70, 229),   // #4F46E5 Indigo-600
		OnPrimary:      graphics.ColorWhite,
		TextPrimary:    graphics.RGB(244, 244, 245), // #F4F4F5 Zinc-100
		TextSecondary:  graphics.RGB(161, 161, 170), // #A1A1AA Zinc-400
		TextDisabled:   graphics.RGB(113, 113, 122), // #71717A Zinc-500
		Border:         graphics.RGB(63, 63, 70),    // #3F3F46 Zinc-700
		BorderFocused:  graphics.RGB(99, 102, 241),  // #6366F1
		FocusRing:      graphics.RGB(99, 102, 241),  // #6366F1
		Track:          graphics.RGB(45, 45, 50),    // 深灰轨道
		Thumb:          graphics.RGB(99, 102, 241),  // #6366F1
		ThumbHover:     graphics.RGB(129, 140, 248), // #818CF8
		Selection:      graphics.RGBA(99, 102, 241, 110),
		Shadow:         graphics.RGBA(0, 0, 0, 90),
		Success:        graphics.RGB(34, 197, 94), // #22C55E
		Warning:        graphics.RGB(234, 179, 8), // #EAB308
		Danger:         graphics.RGB(239, 68, 68), // #EF4444

		GutterBg:         graphics.RGB(28, 30, 36),
		GutterText:       graphics.RGB(110, 118, 136),
		GutterBorder:     graphics.RGB(45, 48, 58),
		ActiveLineBg:     graphics.RGB(47, 49, 58),
		ActiveLineBorder: graphics.RGB(65, 69, 82),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(192, 132, 252), // Purple 400
			Type:           graphics.RGB(56, 189, 248),  // Sky 400
			Builtin:        graphics.RGB(45, 212, 191),  // Teal 400
			Function:       graphics.RGB(253, 224, 71),  // Yellow 300
			Key:            graphics.RGB(56, 189, 248),  // Sky 400
			String:         graphics.RGB(167, 243, 208), // Emerald 200
			StringStrong:   graphics.RGB(52, 211, 153),  // Emerald 400
			Character:      graphics.RGB(251, 146, 60),  // Orange 400
			Number:         graphics.RGB(251, 146, 60),  // Orange 400
			NumberSoft:     graphics.RGB(251, 191, 36),  // Amber 400
			Constant:       graphics.RGB(248, 113, 113), // Red 400
			Operator:       graphics.RGB(226, 232, 240), // Slate 200
			Punctuation:    graphics.RGB(148, 163, 184), // Slate 400
			PunctuationAlt: graphics.RGB(148, 163, 184), // Slate 400
			Bracket:        graphics.RGB(203, 213, 225), // Slate 300
			Comment:        graphics.RGB(141, 141, 148), // Zinc 400（4.5:1 on surface）
			CommentSoft:    graphics.RGB(130, 143, 162), // Slate 400（4.5:1 on surface）
			Attribute:      graphics.RGB(192, 132, 252), // Purple 400
			Boolean:        graphics.RGB(167, 139, 250), // Violet 400
			Null:           graphics.RGB(244, 114, 182), // Pink 400
			Meta:           graphics.RGB(244, 114, 182), // Pink 400
			Escape:         graphics.RGB(249, 115, 22),  // Orange 500
			Entity:         graphics.RGB(251, 146, 60),  // Orange 400
			Anchor:         graphics.RGB(251, 146, 60),  // Orange 400
			Table:          graphics.RGB(217, 70, 239),  // Fuchsia 500
			DateTime:       graphics.RGB(34, 211, 238),  // Cyan 400
			Directive:      graphics.RGB(129, 140, 248), // Indigo 400
			Lifetime:       graphics.RGB(252, 211, 77),  // Amber 300

			Heading1:      graphics.RGB(56, 189, 248),  // Sky 400
			Heading2:      graphics.RGB(52, 211, 153),  // Emerald 400
			Heading3:      graphics.RGB(251, 191, 36),  // Amber 400
			Heading4:      graphics.RGB(129, 140, 248), // Indigo 400
			HeadingMarker: graphics.RGB(99, 102, 241),  // Indigo 500

			CodeBlockText:    graphics.RGB(167, 243, 208), // Emerald 200
			CodeBlockBg:      graphics.RGBA(30, 41, 59, 100),
			CodeBlockMarker:  graphics.RGB(100, 116, 139), // Slate 500
			CodeLang:         graphics.RGB(249, 115, 22),  // Orange 500
			InlineCodeText:   graphics.RGB(252, 211, 77),  // Amber 300
			InlineCodeBg:     graphics.RGBA(63, 63, 70, 120),
			InlineCodeMarker: graphics.RGB(113, 113, 122), // Zinc 500
			QuoteText:        graphics.RGB(148, 163, 184), // Slate 400
			QuoteMarker:      graphics.RGB(96, 165, 250),  // Blue 400
			ListMarker:       graphics.RGB(251, 146, 60),  // Orange 400
			TaskChecked:      graphics.RGB(74, 222, 128),  // Green 400
			TaskUnchecked:    graphics.RGB(253, 224, 71),  // Yellow 300
			BoldText:         graphics.RGB(254, 240, 138), // Yellow 200
			BoldMarker:       graphics.RGB(202, 138, 4),   // Yellow 600
			ItalicText:       graphics.RGB(244, 114, 182), // Pink 400
			StrikeText:       graphics.RGB(130, 143, 162), // Slate 400（4.5:1）
			LinkText:         graphics.RGB(96, 165, 250),  // Blue 400
			LinkUrl:          graphics.RGB(130, 143, 162), // Slate 400（4.5:1）
			ImageMarker:      graphics.RGB(251, 113, 133), // Rose 400
			Hr:               graphics.RGB(101, 113, 130), // Slate 500（3:1 on surface）
		},
	}
}

// Light 返回现代化亮色调主题 (Slate / Indigo 调色板)
func Light() *Theme {
	return &Theme{
		ID:             "light",
		Name:           "Light",
		Label:          "浅色",
		Dark:           false,
		Background:     graphics.RGB(248, 250, 252), // #F8FAFC Slate-50
		Surface:        graphics.RGB(255, 255, 255), // #FFFFFF White
		SurfaceVariant: graphics.RGB(241, 245, 249), // #F1F5F9 Slate-100
		Primary:        graphics.RGB(79, 70, 229),   // #4F46E5 Indigo-600
		PrimaryHover:   graphics.RGB(99, 102, 241),  // #6366F1 Indigo-500
		PrimaryActive:  graphics.RGB(67, 56, 202),   // #4338CA Indigo-700
		OnPrimary:      graphics.ColorWhite,
		TextPrimary:    graphics.RGB(15, 23, 42),    // #0F172A Slate-900
		TextSecondary:  graphics.RGB(100, 116, 139), // #64748B Slate-500
		TextDisabled:   graphics.RGB(148, 163, 184), // #94A3B8 Slate-400
		Border:         graphics.RGB(226, 232, 240), // #E2E8F0 Slate-200
		BorderFocused:  graphics.RGB(79, 70, 229),   // #4F46E5
		FocusRing:      graphics.RGB(79, 70, 229),   // #4F46E5
		Track:          graphics.RGB(226, 232, 240), // #E2E8F0
		Thumb:          graphics.RGB(79, 70, 229),   // #4F46E5
		ThumbHover:     graphics.RGB(99, 102, 241),  // #6366F1
		Selection:      graphics.RGBA(79, 70, 229, 110),
		Shadow:         graphics.RGBA(15, 23, 42, 40),
		Success:        graphics.RGB(22, 163, 74), // #16A34A
		Warning:        graphics.RGB(202, 138, 4), // #CA8A04
		Danger:         graphics.RGB(220, 38, 38), // #DC2626

		GutterBg:         graphics.RGB(243, 244, 246),
		GutterText:       graphics.RGB(124, 135, 152),
		GutterBorder:     graphics.RGB(229, 231, 235),
		ActiveLineBg:     graphics.RGB(241, 245, 249),
		ActiveLineBorder: graphics.RGB(226, 232, 240),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(126, 34, 206),  // Purple 700
			Type:           graphics.RGB(2, 132, 199),   // Sky 600
			Builtin:        graphics.RGB(15, 118, 110),  // Teal 700
			Function:       graphics.RGB(180, 83, 9),    // Amber 700
			Key:            graphics.RGB(3, 105, 161),   // Sky 700
			String:         graphics.RGB(21, 128, 61),   // Green 700
			StringStrong:   graphics.RGB(4, 120, 87),    // Emerald 700
			Character:      graphics.RGB(194, 65, 12),   // Orange 700
			Number:         graphics.RGB(194, 65, 12),   // Orange 700
			NumberSoft:     graphics.RGB(180, 83, 9),    // Amber 700
			Constant:       graphics.RGB(190, 18, 60),   // Rose 700
			Operator:       graphics.RGB(51, 65, 85),    // Slate 700
			Punctuation:    graphics.RGB(100, 116, 139), // Slate 500
			PunctuationAlt: graphics.RGB(71, 85, 105),   // Slate 600
			Bracket:        graphics.RGB(51, 65, 85),    // Slate 700
			Comment:        graphics.RGB(100, 116, 139), // Slate 500
			CommentSoft:    graphics.RGB(108, 119, 134), // Slate 500（4.5:1 on white）
			Attribute:      graphics.RGB(126, 34, 206),  // Purple 700
			Boolean:        graphics.RGB(109, 40, 217),  // Violet 700
			Null:           graphics.RGB(190, 24, 93),   // Pink 700
			Meta:           graphics.RGB(190, 24, 93),   // Pink 700
			Escape:         graphics.RGB(194, 65, 12),   // Orange 700
			Entity:         graphics.RGB(194, 65, 12),   // Orange 700
			Anchor:         graphics.RGB(194, 65, 12),   // Orange 700
			Table:          graphics.RGB(162, 28, 175),  // Fuchsia 700
			DateTime:       graphics.RGB(14, 116, 144),  // Cyan 700
			Directive:      graphics.RGB(79, 70, 229),   // Indigo 600
			Lifetime:       graphics.RGB(180, 83, 9),    // Amber 700

			Heading1:      graphics.RGB(3, 105, 161), // Sky 700
			Heading2:      graphics.RGB(4, 120, 87),  // Emerald 700
			Heading3:      graphics.RGB(180, 83, 9),  // Amber 700
			Heading4:      graphics.RGB(67, 56, 202), // Indigo 700
			HeadingMarker: graphics.RGB(79, 70, 229), // Indigo 600

			CodeBlockText:    graphics.RGB(6, 95, 70), // Emerald 800
			CodeBlockBg:      graphics.RGBA(226, 232, 240, 120),
			CodeBlockMarker:  graphics.RGB(100, 116, 139), // Slate 500
			CodeLang:         graphics.RGB(194, 65, 12),   // Orange 700
			InlineCodeText:   graphics.RGB(180, 83, 9),    // Amber 700
			InlineCodeBg:     graphics.RGBA(228, 228, 231, 140),
			InlineCodeMarker: graphics.RGB(113, 113, 122),
			QuoteText:        graphics.RGB(71, 85, 105), // Slate 600
			QuoteMarker:      graphics.RGB(37, 99, 235), // Blue 600
			ListMarker:       graphics.RGB(234, 88, 12), // Orange 600
			TaskChecked:      graphics.RGB(22, 163, 74), // Green 600
			TaskUnchecked:    graphics.RGB(217, 119, 6), // Amber 600
			BoldText:         graphics.RGB(120, 53, 15), // Amber 900
			BoldMarker:       graphics.RGB(180, 83, 9),
			ItalicText:       graphics.RGB(190, 24, 93),   // Pink 700
			StrikeText:       graphics.RGB(108, 119, 134), // Slate 500（4.5:1）
			LinkText:         graphics.RGB(29, 78, 216),   // Blue 700
			LinkUrl:          graphics.RGB(100, 116, 139),
			ImageMarker:      graphics.RGB(225, 29, 72),   // Rose 600
			Hr:               graphics.RGB(136, 150, 169), // 3:1 on white
		},
	}
}

// Nord 返回 Nord 极地蓝灰主题。
func Nord() *Theme {
	return &Theme{
		ID:             "nord",
		Name:           "Nord",
		Label:          "北欧",
		Dark:           true,
		Background:     graphics.RGB(46, 52, 64),    // #2E3440 Polar Night 0
		Surface:        graphics.RGB(59, 66, 82),    // #3B4252 Polar Night 1
		SurfaceVariant: graphics.RGB(67, 76, 94),    // #434C5E Polar Night 2
		Primary:        graphics.RGB(136, 192, 208), // #88C0D0 Frost 2
		PrimaryHover:   graphics.RGB(143, 188, 187), // #8FBCBB Frost 1
		PrimaryActive:  graphics.RGB(94, 129, 172),  // #5E81AC Frost 4
		OnPrimary:      graphics.RGB(46, 52, 64),    // #2E3440
		TextPrimary:    graphics.RGB(236, 239, 244), // #ECEFF4 Snow Storm 2
		TextSecondary:  graphics.RGB(216, 222, 233), // #D8DEE9 Snow Storm 1
		TextDisabled:   graphics.RGB(123, 136, 161),
		Border:         graphics.RGB(76, 86, 106), // #4C566A Polar Night 3
		BorderFocused:  graphics.RGB(136, 192, 208),
		FocusRing:      graphics.RGB(136, 192, 208),
		Track:          graphics.RGB(59, 66, 82),
		Thumb:          graphics.RGB(136, 192, 208),
		ThumbHover:     graphics.RGB(143, 188, 187),
		Selection:      graphics.RGBA(94, 129, 172, 120),
		Shadow:         graphics.RGBA(20, 24, 31, 110),
		Success:        graphics.RGB(163, 190, 140), // #A3BE8C Aurora Green
		Warning:        graphics.RGB(235, 203, 139), // #EBCB8B Aurora Yellow
		Danger:         graphics.RGB(191, 97, 106),  // #BF616A Aurora Red

		GutterBg:         graphics.RGB(43, 48, 59),
		GutterText:       graphics.RGB(123, 136, 161), // #7B88A1
		GutterBorder:     graphics.RGB(59, 66, 82),
		ActiveLineBg:     graphics.RGB(53, 60, 74),
		ActiveLineBorder: graphics.RGB(67, 76, 94),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(129, 161, 193), // #81A1C1 Frost 3
			Type:           graphics.RGB(143, 188, 187), // #8FBCBB Frost 1
			Builtin:        graphics.RGB(136, 192, 208), // #88C0D0 Frost 2
			Function:       graphics.RGB(136, 192, 208), // #88C0D0
			Key:            graphics.RGB(143, 188, 187), // #8FBCBB
			String:         graphics.RGB(163, 190, 140), // #A3BE8C
			StringStrong:   graphics.RGB(163, 190, 140),
			Number:         graphics.RGB(180, 142, 173), // #B48EAD Aurora Purple
			NumberSoft:     graphics.RGB(235, 203, 139), // #EBCB8B
			Constant:       graphics.RGB(208, 135, 112), // #D08770 Aurora Orange
			Operator:       graphics.RGB(216, 222, 233), // #D8DEE9
			Punctuation:    graphics.RGB(216, 222, 233), // #D8DEE9
			PunctuationAlt: graphics.RGB(129, 161, 193), // #81A1C1
			Bracket:        graphics.RGB(129, 161, 193),
			Comment:        graphics.RGB(165, 174, 191), // #A5AEBF（4.5:1 on surface）
			CommentSoft:    graphics.RGB(164, 174, 193), // #A4AEC1（4.5:1 on surface）
			Attribute:      graphics.RGB(180, 142, 173), // #B48EAD
			Boolean:        graphics.RGB(129, 161, 193),
			Null:           graphics.RGB(208, 135, 112),
			Meta:           graphics.RGB(180, 142, 173),
			Escape:         graphics.RGB(208, 135, 112),
			Entity:         graphics.RGB(208, 135, 112),
			Anchor:         graphics.RGB(208, 135, 112),
			Table:          graphics.RGB(180, 142, 173),
			DateTime:       graphics.RGB(136, 192, 208),
			Directive:      graphics.RGB(94, 129, 172),
			Lifetime:       graphics.RGB(235, 203, 139),

			Heading1:      graphics.RGB(136, 192, 208),
			Heading2:      graphics.RGB(163, 190, 140),
			Heading3:      graphics.RGB(235, 203, 139),
			Heading4:      graphics.RGB(180, 142, 173),
			HeadingMarker: graphics.RGB(94, 129, 172),

			CodeLang:      graphics.RGB(208, 135, 112),
			QuoteMarker:   graphics.RGB(94, 129, 172),
			TaskChecked:   graphics.RGB(163, 190, 140),
			TaskUnchecked: graphics.RGB(235, 203, 139),
			BoldText:      graphics.RGB(236, 239, 244),
			BoldMarker:    graphics.RGB(235, 203, 139),
			ItalicText:    graphics.RGB(180, 142, 173),
			ImageMarker:   graphics.RGB(191, 97, 106),
		},
	}
}

// TokyoNight 返回 Tokyo Night 霓虹夜色主题。
func TokyoNight() *Theme {
	return &Theme{
		ID:             "tokyo-night",
		Name:           "Tokyo Night",
		Label:          "东京夜",
		Dark:           true,
		Background:     graphics.RGB(26, 27, 38),    // #1A1B26
		Surface:        graphics.RGB(31, 35, 53),    // #1F2335
		SurfaceVariant: graphics.RGB(41, 46, 66),    // #292E42
		Primary:        graphics.RGB(122, 162, 247), // #7AA2F7
		PrimaryHover:   graphics.RGB(141, 176, 255),
		PrimaryActive:  graphics.RGB(61, 89, 161), // #3D59A1
		OnPrimary:      graphics.RGB(26, 27, 38),
		TextPrimary:    graphics.RGB(192, 202, 245), // #C0CAF5
		TextSecondary:  graphics.RGB(169, 177, 214), // #A9B1D6
		TextDisabled:   graphics.RGB(86, 95, 137),   // #565F89
		Border:         graphics.RGB(47, 53, 73),
		BorderFocused:  graphics.RGB(122, 162, 247),
		FocusRing:      graphics.RGB(122, 162, 247),
		Track:          graphics.RGB(41, 46, 66),
		Thumb:          graphics.RGB(122, 162, 247),
		ThumbHover:     graphics.RGB(141, 176, 255),
		Selection:      graphics.RGBA(122, 162, 247, 110),
		Shadow:         graphics.RGBA(10, 11, 18, 120),
		Success:        graphics.RGB(158, 206, 106), // #9ECE6A
		Warning:        graphics.RGB(224, 175, 104), // #E0AF68
		Danger:         graphics.RGB(247, 118, 142), // #F7768E

		GutterBg:         graphics.RGB(22, 22, 30),
		GutterText:       graphics.RGB(107, 115, 150),
		GutterBorder:     graphics.RGB(31, 35, 53),
		ActiveLineBg:     graphics.RGB(30, 32, 46),
		ActiveLineBorder: graphics.RGB(41, 46, 66),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(187, 154, 247), // #BB9AF7
			Type:           graphics.RGB(42, 195, 222),  // #2AC3DE
			Builtin:        graphics.RGB(125, 207, 255), // #7DCFFF
			Function:       graphics.RGB(122, 162, 247), // #7AA2F7
			Key:            graphics.RGB(125, 207, 255), // #7DCFFF
			String:         graphics.RGB(158, 206, 106), // #9ECE6A
			StringStrong:   graphics.RGB(158, 206, 106),
			Number:         graphics.RGB(255, 158, 100), // #FF9E64
			NumberSoft:     graphics.RGB(224, 175, 104), // #E0AF68
			Constant:       graphics.RGB(255, 158, 100),
			Operator:       graphics.RGB(137, 221, 255), // #89DDFF
			Punctuation:    graphics.RGB(154, 165, 206), // #9AA5CE
			PunctuationAlt: graphics.RGB(122, 136, 184),
			Bracket:        graphics.RGB(137, 221, 255),
			Comment:        graphics.RGB(130, 137, 166), // #8289A6（4.5:1 on surface）
			CommentSoft:    graphics.RGB(123, 137, 184), // #7B89B8（4.5:1 on surface）
			Attribute:      graphics.RGB(187, 154, 247),
			Boolean:        graphics.RGB(255, 158, 100),
			Null:           graphics.RGB(247, 118, 142),
			Meta:           graphics.RGB(247, 118, 142), // #F7768E
			Escape:         graphics.RGB(255, 158, 100),
			Entity:         graphics.RGB(255, 158, 100),
			Anchor:         graphics.RGB(255, 158, 100),
			Table:          graphics.RGB(187, 154, 247),
			DateTime:       graphics.RGB(125, 207, 255),
			Directive:      graphics.RGB(122, 162, 247),
			Lifetime:       graphics.RGB(224, 175, 104),

			Heading1:      graphics.RGB(122, 162, 247),
			Heading2:      graphics.RGB(158, 206, 106),
			Heading3:      graphics.RGB(224, 175, 104),
			Heading4:      graphics.RGB(187, 154, 247),
			HeadingMarker: graphics.RGB(125, 207, 255),

			CodeLang:      graphics.RGB(255, 158, 100),
			QuoteMarker:   graphics.RGB(125, 207, 255),
			TaskChecked:   graphics.RGB(158, 206, 106),
			TaskUnchecked: graphics.RGB(224, 175, 104),
			BoldText:      graphics.RGB(192, 202, 245),
			BoldMarker:    graphics.RGB(224, 175, 104),
			ItalicText:    graphics.RGB(187, 154, 247),
			ImageMarker:   graphics.RGB(247, 118, 142),
		},
	}
}

// OneDark 返回 Atom One Dark 主题。
func OneDark() *Theme {
	return &Theme{
		ID:             "one-dark",
		Name:           "One Dark",
		Label:          "原子",
		Dark:           true,
		Background:     graphics.RGB(40, 44, 52),   // #282C34
		Surface:        graphics.RGB(44, 49, 58),   // #2C313A
		SurfaceVariant: graphics.RGB(58, 63, 75),   // #3A3F4B
		Primary:        graphics.RGB(97, 175, 239), // #61AFEF
		PrimaryHover:   graphics.RGB(123, 189, 246),
		PrimaryActive:  graphics.RGB(74, 143, 208),
		OnPrimary:      graphics.RGB(40, 44, 52),
		TextPrimary:    graphics.RGB(171, 178, 191), // #ABB2BF
		TextSecondary:  graphics.RGB(130, 137, 151), // #828997
		TextDisabled:   graphics.RGB(92, 99, 112),   // #5C6370
		Border:         graphics.RGB(62, 68, 81),    // #3E4451
		BorderFocused:  graphics.RGB(97, 175, 239),
		FocusRing:      graphics.RGB(97, 175, 239),
		Track:          graphics.RGB(58, 63, 75),
		Thumb:          graphics.RGB(97, 175, 239),
		ThumbHover:     graphics.RGB(123, 189, 246),
		Selection:      graphics.RGBA(97, 175, 239, 105),
		Shadow:         graphics.RGBA(18, 20, 24, 120),
		Success:        graphics.RGB(152, 195, 121), // #98C379
		Warning:        graphics.RGB(229, 192, 123), // #E5C07B
		Danger:         graphics.RGB(224, 108, 117), // #E06C75

		GutterBg:         graphics.RGB(35, 39, 46),
		GutterText:       graphics.RGB(118, 126, 138),
		GutterBorder:     graphics.RGB(44, 49, 58),
		ActiveLineBg:     graphics.RGB(47, 52, 61),
		ActiveLineBorder: graphics.RGB(62, 68, 81),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(198, 120, 221), // #C678DD
			Type:           graphics.RGB(229, 192, 123), // #E5C07B
			Builtin:        graphics.RGB(86, 182, 194),  // #56B6C2
			Function:       graphics.RGB(97, 175, 239),  // #61AFEF
			Key:            graphics.RGB(229, 192, 123),
			String:         graphics.RGB(152, 195, 121), // #98C379
			StringStrong:   graphics.RGB(152, 195, 121),
			Number:         graphics.RGB(209, 154, 102), // #D19A66
			NumberSoft:     graphics.RGB(229, 192, 123),
			Constant:       graphics.RGB(209, 154, 102),
			Operator:       graphics.RGB(86, 182, 194),
			Punctuation:    graphics.RGB(171, 178, 191),
			PunctuationAlt: graphics.RGB(130, 137, 151),
			Bracket:        graphics.RGB(171, 178, 191),
			Comment:        graphics.RGB(146, 152, 162), // #9298A2（4.5:1 on surface）
			CommentSoft:    graphics.RGB(146, 152, 164), // #9298A4（4.5:1 on surface）
			Attribute:      graphics.RGB(224, 108, 117), // #E06C75
			Boolean:        graphics.RGB(209, 154, 102),
			Null:           graphics.RGB(224, 108, 117),
			Meta:           graphics.RGB(224, 108, 117),
			Escape:         graphics.RGB(86, 182, 194),
			Entity:         graphics.RGB(209, 154, 102),
			Anchor:         graphics.RGB(209, 154, 102),
			Table:          graphics.RGB(224, 108, 117),
			DateTime:       graphics.RGB(86, 182, 194),
			Directive:      graphics.RGB(97, 175, 239),
			Lifetime:       graphics.RGB(209, 154, 102),

			Heading1:      graphics.RGB(97, 175, 239),
			Heading2:      graphics.RGB(152, 195, 121),
			Heading3:      graphics.RGB(229, 192, 123),
			Heading4:      graphics.RGB(198, 120, 221),
			HeadingMarker: graphics.RGB(86, 182, 194),

			CodeLang:      graphics.RGB(209, 154, 102),
			QuoteMarker:   graphics.RGB(86, 182, 194),
			TaskChecked:   graphics.RGB(152, 195, 121),
			TaskUnchecked: graphics.RGB(229, 192, 123),
			BoldText:      graphics.RGB(229, 192, 123),
			BoldMarker:    graphics.RGB(209, 154, 102),
			ItalicText:    graphics.RGB(198, 120, 221),
			ImageMarker:   graphics.RGB(224, 108, 117),
		},
	}
}

// GruvboxDark 返回 Gruvbox 暖色复古暗色主题。
func GruvboxDark() *Theme {
	return &Theme{
		ID:             "gruvbox-dark",
		Name:           "Gruvbox",
		Label:          "格鲁布",
		Dark:           true,
		Background:     graphics.RGB(40, 40, 40),   // #282828
		Surface:        graphics.RGB(50, 48, 47),   // #32302F
		SurfaceVariant: graphics.RGB(60, 56, 54),   // #3C3836
		Primary:        graphics.RGB(250, 189, 47), // #FABD2F
		PrimaryHover:   graphics.RGB(250, 204, 107),
		PrimaryActive:  graphics.RGB(215, 153, 33), // #D79921
		OnPrimary:      graphics.RGB(40, 40, 40),
		TextPrimary:    graphics.RGB(235, 219, 178), // #EBDBB2
		TextSecondary:  graphics.RGB(168, 153, 132), // #A89984
		TextDisabled:   graphics.RGB(102, 92, 84),   // #665C54
		Border:         graphics.RGB(80, 73, 69),    // #504945
		BorderFocused:  graphics.RGB(250, 189, 47),
		FocusRing:      graphics.RGB(250, 189, 47),
		Track:          graphics.RGB(60, 56, 54),
		Thumb:          graphics.RGB(250, 189, 47),
		ThumbHover:     graphics.RGB(250, 204, 107),
		Selection:      graphics.RGBA(215, 153, 33, 105),
		Shadow:         graphics.RGBA(20, 20, 20, 120),
		Success:        graphics.RGB(184, 187, 38), // #B8BB26
		Warning:        graphics.RGB(250, 189, 47), // #FABD2F
		Danger:         graphics.RGB(251, 73, 52),  // #FB4934

		GutterBg:         graphics.RGB(29, 32, 33), // #1D2021
		GutterText:       graphics.RGB(124, 111, 100),
		GutterBorder:     graphics.RGB(50, 48, 47),
		ActiveLineBg:     graphics.RGB(58, 55, 53),
		ActiveLineBorder: graphics.RGB(80, 73, 69),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(251, 73, 52),   // #FB4934
			Type:           graphics.RGB(250, 189, 47),  // #FABD2F
			Builtin:        graphics.RGB(131, 165, 152), // #83A598
			Function:       graphics.RGB(184, 187, 38),  // #B8BB26
			Key:            graphics.RGB(250, 189, 47),
			String:         graphics.RGB(142, 192, 124), // #8EC07C
			StringStrong:   graphics.RGB(142, 192, 124),
			Number:         graphics.RGB(211, 134, 155), // #D3869B
			NumberSoft:     graphics.RGB(250, 189, 47),
			Constant:       graphics.RGB(254, 128, 25), // #FE8019
			Operator:       graphics.RGB(235, 219, 178),
			Punctuation:    graphics.RGB(168, 153, 132),
			PunctuationAlt: graphics.RGB(124, 111, 100),
			Bracket:        graphics.RGB(168, 153, 132),
			Comment:        graphics.RGB(162, 150, 137), // #A29689（4.5:1 on surface）
			CommentSoft:    graphics.RGB(159, 150, 142), // #9F968E（4.5:1 on surface）
			Attribute:      graphics.RGB(250, 189, 47),
			Boolean:        graphics.RGB(211, 134, 155),
			Null:           graphics.RGB(251, 73, 52),
			Meta:           graphics.RGB(211, 134, 155),
			Escape:         graphics.RGB(254, 128, 25),
			Entity:         graphics.RGB(254, 128, 25),
			Anchor:         graphics.RGB(254, 128, 25),
			Table:          graphics.RGB(211, 134, 155),
			DateTime:       graphics.RGB(131, 165, 152),
			Directive:      graphics.RGB(131, 165, 152),
			Lifetime:       graphics.RGB(250, 189, 47),

			Heading1:      graphics.RGB(131, 165, 152),
			Heading2:      graphics.RGB(184, 187, 38),
			Heading3:      graphics.RGB(250, 189, 47),
			Heading4:      graphics.RGB(211, 134, 155),
			HeadingMarker: graphics.RGB(254, 128, 25),

			CodeLang:      graphics.RGB(254, 128, 25),
			QuoteMarker:   graphics.RGB(131, 165, 152),
			TaskChecked:   graphics.RGB(184, 187, 38),
			TaskUnchecked: graphics.RGB(250, 189, 47),
			BoldText:      graphics.RGB(250, 189, 47),
			BoldMarker:    graphics.RGB(254, 128, 25),
			ItalicText:    graphics.RGB(211, 134, 155),
			ImageMarker:   graphics.RGB(251, 73, 52),
		},
	}
}

// SolarizedLight 返回 Solarized 精确对比亮色主题。
func SolarizedLight() *Theme {
	return &Theme{
		ID:             "solarized-light",
		Name:           "Solarized Light",
		Label:          "日光",
		Dark:           false,
		Background:     graphics.RGB(253, 246, 227), // #FDF6E3 base3
		Surface:        graphics.RGB(255, 251, 240),
		SurfaceVariant: graphics.RGB(238, 232, 213), // #EEE8D5 base2
		Primary:        graphics.RGB(33, 121, 184),  // #2179B8 blue (对比度适配)
		PrimaryHover:   graphics.RGB(63, 169, 221),
		PrimaryActive:  graphics.RGB(27, 106, 165),
		OnPrimary:      graphics.ColorWhite,
		TextPrimary:    graphics.RGB(7, 54, 66),     // #073642 base02
		TextSecondary:  graphics.RGB(101, 123, 131), // #657B83 base00
		TextDisabled:   graphics.RGB(147, 161, 161), // #93A1A1 base1
		Border:         graphics.RGB(228, 220, 200),
		BorderFocused:  graphics.RGB(38, 139, 210),
		FocusRing:      graphics.RGB(38, 139, 210),
		Track:          graphics.RGB(238, 232, 213),
		Thumb:          graphics.RGB(38, 139, 210),
		ThumbHover:     graphics.RGB(63, 169, 221),
		Selection:      graphics.RGBA(38, 139, 210, 80),
		Shadow:         graphics.RGBA(7, 54, 66, 45),
		Success:        graphics.RGB(133, 153, 0), // #859900 green
		Warning:        graphics.RGB(181, 137, 0), // #B58900 yellow
		Danger:         graphics.RGB(220, 50, 47), // #DC322F red

		GutterBg:         graphics.RGB(245, 239, 220),
		GutterText:       graphics.RGB(125, 141, 141), // 3:1 on gutter
		GutterBorder:     graphics.RGB(238, 232, 213),
		ActiveLineBg:     graphics.RGB(243, 237, 217),
		ActiveLineBorder: graphics.RGB(228, 220, 200),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(133, 153, 0),  // #859900
			Type:           graphics.RGB(181, 137, 0),  // #B58900
			Builtin:        graphics.RGB(42, 161, 152), // #2AA198
			Function:       graphics.RGB(38, 139, 210), // #268BD2
			Key:            graphics.RGB(38, 139, 210),
			String:         graphics.RGB(42, 161, 152), // #2AA198
			StringStrong:   graphics.RGB(42, 161, 152),
			Number:         graphics.RGB(211, 54, 130), // #D33682
			NumberSoft:     graphics.RGB(181, 137, 0),
			Constant:       graphics.RGB(203, 75, 22), // #CB4B16
			Operator:       graphics.RGB(101, 123, 131),
			Punctuation:    graphics.RGB(131, 148, 150), // #839496 base0
			PunctuationAlt: graphics.RGB(147, 161, 161),
			Bracket:        graphics.RGB(101, 123, 131),
			Comment:        graphics.RGB(105, 118, 120), // #697678（4.5:1 on surface）
			CommentSoft:    graphics.RGB(105, 118, 120), // #697678（4.5:1 on surface）
			Attribute:      graphics.RGB(181, 137, 0),
			Boolean:        graphics.RGB(211, 54, 130),
			Null:           graphics.RGB(203, 75, 22),
			Meta:           graphics.RGB(211, 54, 130),
			Escape:         graphics.RGB(203, 75, 22),
			Entity:         graphics.RGB(203, 75, 22),
			Anchor:         graphics.RGB(203, 75, 22),
			Table:          graphics.RGB(211, 54, 130),
			DateTime:       graphics.RGB(42, 161, 152),
			Directive:      graphics.RGB(38, 139, 210),
			Lifetime:       graphics.RGB(181, 137, 0),

			Heading1:      graphics.RGB(38, 139, 210),
			Heading2:      graphics.RGB(42, 161, 152),
			Heading3:      graphics.RGB(181, 137, 0),
			Heading4:      graphics.RGB(108, 113, 196), // #6C71C4 violet
			HeadingMarker: graphics.RGB(38, 139, 210),

			InlineCodeText: graphics.RGB(146, 110, 0),   // #926E00（4.5:1 on surface）
			QuoteText:      graphics.RGB(105, 118, 120), // 引用正文与注释同亮度
			CodeLang:       graphics.RGB(203, 75, 22),
			QuoteMarker:    graphics.RGB(38, 139, 210),
			TaskChecked:    graphics.RGB(133, 153, 0),
			TaskUnchecked:  graphics.RGB(181, 137, 0),
			BoldText:       graphics.RGB(203, 75, 22),
			BoldMarker:     graphics.RGB(181, 137, 0),
			ItalicText:     graphics.RGB(211, 54, 130),
			ImageMarker:    graphics.RGB(220, 50, 47),
		},
	}
}

// RosePine 返回 Rosé Pine 柔和夜色主题。
func RosePine() *Theme {
	return &Theme{
		ID:             "rose-pine",
		Name:           "Rosé Pine",
		Label:          "玫瑰",
		Dark:           true,
		Background:     graphics.RGB(25, 23, 36),    // #191724 base
		Surface:        graphics.RGB(31, 29, 46),    // #1F1D2E surface
		SurfaceVariant: graphics.RGB(38, 35, 58),    // #26233A overlay
		Primary:        graphics.RGB(196, 167, 231), // #C4A7E7 iris
		PrimaryHover:   graphics.RGB(216, 185, 240),
		PrimaryActive:  graphics.RGB(167, 139, 203),
		OnPrimary:      graphics.RGB(25, 23, 36),
		TextPrimary:    graphics.RGB(224, 222, 244), // #E0DEF4 text
		TextSecondary:  graphics.RGB(144, 140, 170), // #908CAA subtle
		TextDisabled:   graphics.RGB(110, 106, 134), // #6E6A86 muted
		Border:         graphics.RGB(64, 61, 82),    // #403D52 highlight med
		BorderFocused:  graphics.RGB(196, 167, 231),
		FocusRing:      graphics.RGB(196, 167, 231),
		Track:          graphics.RGB(38, 35, 58),
		Thumb:          graphics.RGB(196, 167, 231),
		ThumbHover:     graphics.RGB(216, 185, 240),
		Selection:      graphics.RGBA(196, 167, 231, 100),
		Shadow:         graphics.RGBA(12, 10, 18, 120),
		Success:        graphics.RGB(156, 207, 216), // #9CCFD8 foam
		Warning:        graphics.RGB(246, 193, 119), // #F6C177 gold
		Danger:         graphics.RGB(235, 111, 146), // #EB6F92 love

		GutterBg:         graphics.RGB(22, 20, 31),
		GutterText:       graphics.RGB(126, 122, 150),
		GutterBorder:     graphics.RGB(31, 29, 46),
		ActiveLineBg:     graphics.RGB(35, 33, 53),
		ActiveLineBorder: graphics.RGB(64, 61, 82),

		Syntax: SyntaxPalette{
			Keyword:        graphics.RGB(196, 167, 231), // #C4A7E7 iris
			Type:           graphics.RGB(156, 207, 216), // #9CCFD8 foam
			Builtin:        graphics.RGB(235, 188, 186), // #EBBCBA rose
			Function:       graphics.RGB(235, 188, 186),
			Key:            graphics.RGB(156, 207, 216),
			String:         graphics.RGB(246, 193, 119), // #F6C177 gold
			StringStrong:   graphics.RGB(246, 193, 119),
			Number:         graphics.RGB(235, 111, 146), // #EB6F92 love
			NumberSoft:     graphics.RGB(246, 193, 119),
			Constant:       graphics.RGB(235, 111, 146),
			Operator:       graphics.RGB(144, 140, 170), // #908CAA
			Punctuation:    graphics.RGB(144, 140, 170),
			PunctuationAlt: graphics.RGB(110, 106, 134),
			Bracket:        graphics.RGB(156, 207, 216),
			Comment:        graphics.RGB(134, 131, 157), // #86839D（4.5:1 on surface）
			CommentSoft:    graphics.RGB(144, 140, 170),
			Attribute:      graphics.RGB(196, 167, 231),
			Boolean:        graphics.RGB(235, 111, 146),
			Null:           graphics.RGB(235, 111, 146),
			Meta:           graphics.RGB(235, 111, 146),
			Escape:         graphics.RGB(246, 193, 119),
			Entity:         graphics.RGB(246, 193, 119),
			Anchor:         graphics.RGB(246, 193, 119),
			Table:          graphics.RGB(196, 167, 231),
			DateTime:       graphics.RGB(156, 207, 216),
			Directive:      graphics.RGB(49, 116, 143), // #31748F pine
			Lifetime:       graphics.RGB(246, 193, 119),

			Heading1:      graphics.RGB(156, 207, 216),
			Heading2:      graphics.RGB(246, 193, 119),
			Heading3:      graphics.RGB(235, 188, 186),
			Heading4:      graphics.RGB(196, 167, 231),
			HeadingMarker: graphics.RGB(196, 167, 231),

			CodeLang:      graphics.RGB(246, 193, 119),
			QuoteMarker:   graphics.RGB(156, 207, 216),
			TaskChecked:   graphics.RGB(156, 207, 216),
			TaskUnchecked: graphics.RGB(246, 193, 119),
			BoldText:      graphics.RGB(246, 193, 119),
			BoldMarker:    graphics.RGB(235, 188, 186),
			ItalicText:    graphics.RGB(235, 111, 146),
			ImageMarker:   graphics.RGB(235, 111, 146),
		},
	}
}
