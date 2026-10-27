//go:build windows

package main

import (
	"fmt"

	goui "github.com/jacksalad/gnote"
)

// settingsWindow 独立 HWND 设置窗口。Owner 为主窗口，Modal 创建时禁用主窗口，
// 关闭后由 Window.cleanup 自动恢复 owner。
type settingsWindow struct {
	app      *App
	win      *goui.Window
	root     *goui.Panel
	tabs     *goui.TabBar
	pageHost *goui.Panel
	pages    []*goui.Panel
	snapshot Settings
	scale    float64

	wrap       *goui.Switch
	lineNums   *goui.Switch
	highlight  *goui.Switch
	startup    *goui.Segmented
	autoZoom   *goui.Switch
	fontSlider *goui.Slider
	zoomSlider *goui.Slider
	fontValue  *goui.Label
	zoomValue  *goui.Label
	theme      *goui.Segmented
	status     *goui.Label
}

const (
	settingsWindowWidth  = int32(760)
	settingsWindowHeight = int32(560)
	settingsTabWidth     = int32(168)
	settingsHeaderHeight = int32(56)
	settingsFooterHeight = int32(60)
)

func scaleDim(scale float64, v int32) int32 {
	if scale <= 1 {
		return v
	}
	return int32(float64(v)*scale + 0.5)
}

func (sw *settingsWindow) px(v int32) int32 {
	return scaleDim(sw.scale, v)
}

func (sw *settingsWindow) windowWidth() int32 {
	return sw.px(settingsWindowWidth)
}

func (sw *settingsWindow) windowHeight() int32 {
	return sw.px(settingsWindowHeight)
}

// newSettingsWindow 构建设置窗口（保持隐藏）。调用方记录 App.settingsWin 后调用 show()。
func newSettingsWindow(app *App) (*settingsWindow, error) {
	scale := app.uiScale
	if scale < 1 {
		scale = 1
	}
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:     "设置 - gnote",
		Width:     scaleDim(scale, settingsWindowWidth),
		Height:    scaleDim(scale, settingsWindowHeight),
		Center:    true,
		Icon:      GetGnoteIcon(),
		Resizable: false,
		Visible:   false,
		Owner:     app.win,
		Modal:     true,
	})
	if err != nil {
		return nil, err
	}

	sw := &settingsWindow{
		app:      app,
		win:      win,
		snapshot: app.CurrentSettings(),
		scale:    scale,
	}
	sw.build()

	win.SetOnClose(func() bool {
		// 关闭窗口时回到最后一次“应用”的配置；已应用时该操作等价于无变化。
		sw.app.ApplySettings(sw.snapshot)
		if app.settingsWin == sw {
			app.settingsWin = nil
		}
		return true
	})
	win.EventBus().SubscribeHandler(goui.EventKeyDown, func(evt goui.Event) {
		if key, ok := evt.Data.(goui.KeyData); ok && key.KeyCode == 0x1B {
			sw.cancel()
		}
	})
	win.EventBus().SubscribeHandler(goui.EventResize, func(goui.Event) {
		sw.layoutPages()
	})

	win.SetContent(sw.root)
	sw.layoutPages()
	return sw, nil
}

func (sw *settingsWindow) show() {
	sw.win.Show()
	sw.win.Activate()
}

func (sw *settingsWindow) build() {
	titleFont := goui.NewFont(sw.app.fontFamily, sw.px(18))
	labelFont := goui.NewFont(sw.app.fontFamily, sw.px(15))
	smallFont := goui.NewFont(sw.app.fontFamily, sw.px(13))

	// Header
	header := goui.NewSurfacePanel()
	header.SetBounds(0, 0, sw.windowWidth(), sw.px(settingsHeaderHeight))
	title := goui.NewBrandLabel("设置")
	title.SetFont(titleFont)
	title.SetBounds(sw.px(24), sw.px(14), sw.px(300), sw.px(28))
	header.Add(title)
	subtitle := goui.NewSecondaryLabel("gnote 偏好")
	subtitle.SetFont(smallFont)
	subtitle.SetBounds(sw.windowWidth()-sw.px(160), sw.px(20), sw.px(136), sw.px(20))
	subtitle.SetAlign(goui.AlignRight)
	header.Add(subtitle)

	// Body: 左侧页签 + 右侧页面
	sw.tabs = goui.NewTabBar([]string{"编辑器", "外观", "关于"})
	sw.tabs.SetFont(labelFont)
	sw.tabs.SetBounds(0, 0, sw.px(settingsTabWidth),
		sw.windowHeight()-sw.px(settingsHeaderHeight)-sw.px(settingsFooterHeight))

	sw.pageHost = goui.NewBackgroundPanel()
	body := goui.NewPanel()
	body.SetLayout(&goui.BorderLayout{Left: sw.tabs, Center: sw.pageHost})
	body.Add(sw.tabs)
	body.Add(sw.pageHost)

	editorPage := sw.buildEditorPage(titleFont, labelFont, smallFont)
	appearancePage := sw.buildAppearancePage(titleFont, labelFont, smallFont)
	aboutPage := sw.buildAboutPage(titleFont, labelFont, smallFont)
	sw.pages = []*goui.Panel{editorPage, appearancePage, aboutPage}
	for _, page := range sw.pages {
		page.SetBounds(0, 0, 1, 1)
		page.SetVisible(false)
		sw.pageHost.Add(page)
	}
	sw.pages[0].SetVisible(true)
	sw.tabs.OnSelect(func(index int) {
		for i, page := range sw.pages {
			page.SetVisible(i == index)
		}
	})

	// Footer
	footer := goui.NewSurfacePanel()
	footer.SetBounds(0, 0, sw.windowWidth(), sw.px(settingsFooterHeight))

	reset := goui.NewButton("恢复默认")
	reset.SetFont(labelFont)
	reset.SetBorderRadius(sw.px(6))
	reset.SetBounds(sw.px(24), sw.px(16), sw.px(104), sw.px(28))
	reset.OnClick(sw.reset)

	cancel := goui.NewButton("取消")
	cancel.SetFont(labelFont)
	cancel.SetBorderRadius(sw.px(6))
	cancel.SetBounds(sw.windowWidth()-sw.px(228), sw.px(16), sw.px(88), sw.px(28))
	cancel.OnClick(sw.cancel)

	apply := goui.NewPrimaryButton("应用")
	apply.SetFont(labelFont)
	apply.SetBorderRadius(sw.px(6))
	apply.SetBounds(sw.windowWidth()-sw.px(124), sw.px(16), sw.px(100), sw.px(28))
	apply.OnClick(sw.apply)

	sw.status = goui.NewSecondaryLabel("")
	sw.status.SetFont(smallFont)
	sw.status.SetBounds(sw.px(148), sw.px(19), sw.px(240), sw.px(24))
	footer.Add(sw.status)

	footer.Add(reset)
	footer.Add(cancel)
	footer.Add(apply)

	sw.root = goui.NewBackgroundPanel()
	sw.root.SetLayout(&goui.BorderLayout{
		Top:    header,
		Center: body,
		Bottom: footer,
		Gap:    0,
	})
	sw.root.Add(header)
	sw.root.Add(body)
	sw.root.Add(footer)

	sw.setControls(sw.snapshot)
}

func (sw *settingsWindow) buildEditorPage(titleFont, labelFont, smallFont *goui.Font) *goui.Panel {
	page := goui.NewPanel()
	heading := goui.NewLabel("编辑器")
	heading.SetFont(titleFont)
	heading.SetBounds(sw.px(24), sw.px(16), sw.px(360), sw.px(26))
	page.Add(heading)

	sw.wrap = sw.addSwitchRow(page, labelFont, "自动换行", sw.px(64), sw.snapshot.WordWrap)
	sw.lineNums = sw.addSwitchRow(page, labelFont, "显示行号", sw.px(106), sw.snapshot.LineNumbers)
	sw.highlight = sw.addSwitchRow(page, labelFont, "语法高亮", sw.px(148), sw.snapshot.Highlight)

	startupLabel := goui.NewLabel("启动页面")
	startupLabel.SetFont(labelFont)
	startupLabel.SetBounds(sw.px(24), sw.px(196), sw.px(200), sw.px(24))
	page.Add(startupLabel)

	sw.startup = goui.NewSegmented([]string{"介绍文档", "空白页"})
	sw.startup.SetFont(labelFont)
	sw.startup.SetBounds(sw.px(24), sw.px(228), sw.px(280), sw.px(30))
	sw.startup.OnChange(func(int) { sw.clearStatus() })
	page.Add(sw.startup)

	fontLabel := goui.NewLabel("基准字号")
	fontLabel.SetFont(labelFont)
	fontLabel.SetBounds(sw.px(24), sw.px(278), sw.px(200), sw.px(24))
	page.Add(fontLabel)

	sw.fontSlider = goui.NewSlider(12, 32, float64(sw.snapshot.BaseFontSize))
	sw.fontSlider.SetStep(1)
	sw.fontSlider.SetThumbRadius(sw.px(8))
	sw.fontSlider.SetTrackHeight(sw.px(6))
	sw.fontSlider.SetBounds(sw.px(24), sw.px(312), sw.px(400), sw.px(24))
	sw.fontSlider.OnChange(func(v float64) {
		sw.clearStatus()
		sw.fontValue.SetText(fmt.Sprintf("%d px", int(v+0.5)))
	})
	page.Add(sw.fontSlider)

	sw.fontValue = goui.NewSecondaryLabel(fmt.Sprintf("%d px", sw.snapshot.BaseFontSize))
	sw.fontValue.SetFont(smallFont)
	sw.fontValue.SetAlign(goui.AlignRight)
	sw.fontValue.SetBounds(sw.px(440), sw.px(312), sw.px(100), sw.px(24))
	page.Add(sw.fontValue)

	note := goui.NewSecondaryLabel("启动页面下次启动生效；其余选项立即生效。")
	note.SetFont(smallFont)
	note.SetBounds(sw.px(24), sw.px(366), sw.px(460), sw.px(22))
	page.Add(note)
	return page
}

func (sw *settingsWindow) buildAppearancePage(titleFont, labelFont, smallFont *goui.Font) *goui.Panel {
	page := goui.NewPanel()
	heading := goui.NewLabel("外观")
	heading.SetFont(titleFont)
	heading.SetBounds(sw.px(24), sw.px(16), sw.px(360), sw.px(26))
	page.Add(heading)

	themeLabel := goui.NewLabel("主题")
	themeLabel.SetFont(labelFont)
	themeLabel.SetBounds(sw.px(24), sw.px(64), sw.px(200), sw.px(24))
	page.Add(themeLabel)

	sw.theme = goui.NewSegmented(themeLabels())
	sw.theme.SetFont(labelFont)
	sw.theme.SetBounds(sw.px(24), sw.px(96), sw.px(520), sw.px(30))
	sw.theme.OnChange(sw.previewTheme)
	page.Add(sw.theme)

	themeNote := goui.NewSecondaryLabel("选择即预览；“应用”保存且不关窗，取消或关闭则还原上次保存的主题。")
	themeNote.SetFont(smallFont)
	themeNote.SetBounds(sw.px(24), sw.px(136), sw.px(540), sw.px(20))
	page.Add(themeNote)

	sw.autoZoom = sw.addSwitchRow(page, labelFont, "跟随系统 DPI 缩放", sw.px(174), sw.snapshot.Zoom <= 0)
	sw.autoZoom.SetOnChange(func(on bool) {
		sw.clearStatus()
		sw.zoomSlider.SetEnabled(!on)
		if on {
			sw.zoomSlider.SetValue(float64(sw.app.textArea.GetZoomPercent()))
		}
		sw.updateZoomLabel()
	})

	zoomLabel := goui.NewLabel("固定缩放")
	zoomLabel.SetFont(labelFont)
	zoomLabel.SetBounds(sw.px(24), sw.px(222), sw.px(200), sw.px(24))
	page.Add(zoomLabel)

	zoom := float64(sw.app.textArea.GetZoomPercent())
	if sw.snapshot.Zoom > 0 {
		zoom = sw.snapshot.Zoom * 100
	}
	sw.zoomSlider = goui.NewSlider(50, 300, zoom)
	sw.zoomSlider.SetStep(10)
	sw.zoomSlider.SetThumbRadius(sw.px(8))
	sw.zoomSlider.SetTrackHeight(sw.px(6))
	sw.zoomSlider.SetBounds(sw.px(24), sw.px(256), sw.px(400), sw.px(24))
	sw.zoomSlider.SetEnabled(sw.snapshot.Zoom > 0)
	sw.zoomSlider.OnChange(func(v float64) {
		sw.clearStatus()
		sw.updateZoomLabel()
	})
	page.Add(sw.zoomSlider)

	sw.zoomValue = goui.NewSecondaryLabel("")
	sw.zoomValue.SetFont(smallFont)
	sw.zoomValue.SetAlign(goui.AlignRight)
	sw.zoomValue.SetBounds(sw.px(440), sw.px(256), sw.px(100), sw.px(24))
	page.Add(sw.zoomValue)
	sw.updateZoomLabel()

	note := goui.NewSecondaryLabel("跟随系统时按显示器 DPI 自动缩放；关闭后可手动设置 50%～300%。")
	note.SetFont(smallFont)
	note.SetBounds(sw.px(24), sw.px(304), sw.px(520), sw.px(22))
	page.Add(note)
	return page
}

func (sw *settingsWindow) buildAboutPage(titleFont, labelFont, smallFont *goui.Font) *goui.Panel {
	page := goui.NewPanel()
	heading := goui.NewLabel("关于")
	heading.SetFont(titleFont)
	heading.SetBounds(sw.px(24), sw.px(16), sw.px(360), sw.px(26))
	page.Add(heading)

	lines := []struct {
		text  string
		font  *goui.Font
		y     int32
		color bool
	}{
		{"gnote " + appVersion, labelFont, sw.px(72), false},
		{"纯 Go · 零 CGO · 软件光栅化渲染", smallFont, sw.px(108), true},
		{"16 种语言语法高亮 · 内嵌中英文字体", smallFont, sw.px(136), true},
		{"https://github.com/jacksalad/gnote", smallFont, sw.px(176), true},
	}
	for _, line := range lines {
		var label *goui.Label
		if line.color {
			label = goui.NewSecondaryLabel(line.text)
		} else {
			label = goui.NewLabel(line.text)
		}
		label.SetFont(line.font)
		label.SetBounds(sw.px(24), line.y, sw.px(520), sw.px(24))
		page.Add(label)
	}
	return page
}

func (sw *settingsWindow) addSwitchRow(page *goui.Panel, font *goui.Font, text string, y int32, on bool) *goui.Switch {
	label := goui.NewLabel(text)
	label.SetFont(font)
	label.SetBounds(sw.px(24), y, sw.px(420), sw.px(24))
	page.Add(label)

	s := goui.NewSwitch()
	s.SetBounds(sw.px(500), y+sw.px(1), sw.px(44), sw.px(22))
	s.SetOn(on)
	s.SetOnChange(func(bool) { sw.clearStatus() })
	page.Add(s)
	return s
}

func themeLabels() []string {
	themes := goui.Themes()
	labels := make([]string, len(themes))
	for i, th := range themes {
		labels[i] = th.Label
	}
	return labels
}

func (sw *settingsWindow) updateZoomLabel() {
	if sw.zoomValue == nil || sw.zoomSlider == nil {
		return
	}
	value := int(sw.zoomSlider.GetValue() + 0.5)
	if sw.autoZoom != nil && sw.autoZoom.IsOn() {
		sw.zoomValue.SetText(fmt.Sprintf("%d%% (自动)", value))
	} else {
		sw.zoomValue.SetText(fmt.Sprintf("%d%%", value))
	}
}

func (sw *settingsWindow) setControls(s Settings) {
	s = s.Normalized()
	if sw.wrap != nil {
		sw.wrap.SetOn(s.WordWrap)
		sw.lineNums.SetOn(s.LineNumbers)
		sw.highlight.SetOn(s.Highlight)
	}
	if sw.fontSlider != nil {
		sw.fontSlider.SetValue(float64(s.BaseFontSize))
		sw.fontValue.SetText(fmt.Sprintf("%d px", s.BaseFontSize))
	}
	if sw.startup != nil {
		if s.StartupPage == startupPageBlank {
			sw.startup.SetSelected(1)
		} else {
			sw.startup.SetSelected(0)
		}
	}
	if sw.theme != nil {
		for i, th := range goui.Themes() {
			if th.ID == s.Theme {
				sw.theme.SetSelected(i)
				break
			}
		}
	}
	if sw.autoZoom != nil {
		auto := s.Zoom <= 0
		sw.autoZoom.SetOn(auto)
		if auto {
			sw.zoomSlider.SetValue(float64(sw.app.textArea.GetZoomPercent()))
		} else {
			sw.zoomSlider.SetValue(s.Zoom * 100)
		}
		sw.zoomSlider.SetEnabled(!auto)
		sw.updateZoomLabel()
	}
}

func (sw *settingsWindow) reset() {
	s := DefaultSettings()
	sw.setControls(s)
	for i, th := range goui.Themes() {
		if th.ID == s.Theme {
			sw.previewTheme(i)
			break
		}
	}
}

// previewTheme 即时切换全局主题，但不持久化；取消或关闭窗口时回滚到 snapshot。
func (sw *settingsWindow) previewTheme(index int) {
	if index < 0 {
		return
	}
	themes := goui.Themes()
	if index >= len(themes) {
		return
	}
	sw.clearStatus()
	sw.app.SetThemeID(themes[index].ID)
}

// clearStatus 清除“已保存”提示；任何控件变化都意味着提示已过期。
func (sw *settingsWindow) clearStatus() {
	if sw.status != nil {
		sw.status.SetText("")
	}
}

func (sw *settingsWindow) apply() {
	s := sw.app.CurrentSettings()
	if themes := goui.Themes(); sw.theme.Selected() >= 0 && sw.theme.Selected() < len(themes) {
		s.Theme = themes[sw.theme.Selected()].ID
	}
	s.BaseFontSize = int32(sw.fontSlider.GetValue() + 0.5)
	s.WordWrap = sw.wrap.IsOn()
	s.LineNumbers = sw.lineNums.IsOn()
	s.Highlight = sw.highlight.IsOn()
	if sw.startup.Selected() == 1 {
		s.StartupPage = startupPageBlank
	} else {
		s.StartupPage = startupPageWelcome
	}
	if sw.autoZoom.IsOn() {
		s.Zoom = 0
	} else {
		s.Zoom = sw.zoomSlider.GetValue() / 100
	}

	sw.app.ApplySettings(s)
	if err := sw.app.SaveSettings(); err != nil {
		goui.ShowError(sw.win.Hwnd(), "设置保存失败", err.Error())
		return
	}
	sw.snapshot = s
	if sw.status != nil {
		sw.status.SetText("已保存")
	}
}

func (sw *settingsWindow) cancel() {
	sw.win.Close()
}

func (sw *settingsWindow) layoutPages() {
	if sw.pageHost == nil || len(sw.pages) == 0 {
		return
	}
	b := sw.pageHost.GetBounds()
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	for _, page := range sw.pages {
		if page.GetBounds() != b {
			page.SetBounds(b.X, b.Y, b.Width, b.Height)
		}
	}
}

// OpenSettings 打开独立设置窗口；已打开时仅置前。
func (a *App) OpenSettings() {
	if a.settingsWin != nil {
		if goui.IsWindow(a.settingsWin.win.Hwnd()) {
			a.settingsWin.win.Activate()
			return
		}
		a.settingsWin = nil
	}

	sw, err := newSettingsWindow(a)
	if err != nil {
		goui.ShowError(a.win.Hwnd(), "设置", fmt.Sprintf("无法打开设置窗口:\n%v", err))
		return
	}
	a.settingsWin = sw
	sw.show()
}

// OpenConfigFile 先保存当前设置，再把 settings.json 作为普通文档打开。
func (a *App) OpenConfigFile() {
	if !a.CheckSaveIfModified() {
		return
	}
	if err := a.SaveSettings(); err != nil {
		goui.ShowError(a.win.Hwnd(), "设置", fmt.Sprintf("无法保存设置文件:\n%v", err))
		return
	}
	a.LoadFile(a.settingsFilePath())
}
