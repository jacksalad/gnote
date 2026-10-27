//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/format"
	"github.com/jacksalad/gnote/textio"
)

// fileFilter 文件对话框的统一过滤器，打开/保存共用。
const fileFilter = "支持的文档 (*.txt;*.md;*.json;*.yaml;*.yml;*.toml;*.go;*.py;*.c;*.cpp;*.h;*.hpp;*.cs;*.java;*.rs;*.r;*.js;*.jsx;*.ts;*.tsx;*.html;*.htm;*.css;*.sql)|*.txt;*.md;*.json;*.yaml;*.yml;*.toml;*.go;*.py;*.c;*.cpp;*.h;*.hpp;*.cs;*.java;*.rs;*.r;*.js;*.jsx;*.ts;*.tsx;*.html;*.htm;*.css;*.sql|" +
	"Go 源码文件 (*.go)|*.go|" +
	"Python 脚本文件 (*.py;*.pyw)|*.py;*.pyw|" +
	"C/C++ 源码文件 (*.c;*.cpp;*.cc;*.cxx;*.h;*.hpp)|*.c;*.cpp;*.cc;*.cxx;*.h;*.hpp|" +
	"C# 源码文件 (*.cs;*.csx)|*.cs;*.csx|" +
	"Java 源码文件 (*.java)|*.java|" +
	"Rust 源码文件 (*.rs)|*.rs|" +
	"R 统计分析脚本 (*.r;*.R;*.rhistory;*.rprofile)|*.r;*.R;*.rhistory;*.rprofile|" +
	"JavaScript 脚本 (*.js;*.jsx;*.mjs;*.cjs)|*.js;*.jsx;*.mjs;*.cjs|" +
	"TypeScript 源码 (*.ts;*.tsx;*.mts;*.cts;*.typoscript)|*.ts;*.tsx;*.mts;*.cts;*.typoscript|" +
	"HTML 网页文档 (*.html;*.htm)|*.html;*.htm|" +
	"CSS 样式表文件 (*.css)|*.css|" +
	"SQL 查询脚本 (*.sql;*.dsql;*.cql)|*.sql;*.dsql;*.cql|" +
	"Markdown 文档 (*.md;*.markdown)|*.md;*.markdown|" +
	"JSON 配置文件 (*.json)|*.json|" +
	"YAML 配置文件 (*.yaml;*.yml)|*.yaml;*.yml|" +
	"TOML 配置文件 (*.toml)|*.toml|" +
	"文本文档 (*.txt)|*.txt|" +
	"所有文件 (*.*)|*.*"

// App 持有主窗口的全部应用状态与 UI 引用。
// 命令注册表只负责“动作 + 启用/勾选状态”，文件绑定、编码/EOL、高亮等
// 文档状态集中在这里，避免继续散落在 main() 的闭包中。
type App struct {
	win        *goui.Window
	fontFamily string

	textArea *goui.TextArea
	findBar  *findBar

	// 文档状态
	currentFilePath    string
	currentFileName    string
	currentEncoding    textio.Encoding
	currentEOL         textio.EOL
	currentStamp       textio.Stamp
	currentHighlighter goui.Highlighter
	highlightEnabled   bool
	themeID            string

	// UI 引用
	menuBar       *goui.MenuBar
	menuSpacer    *goui.Panel
	statusBar     *goui.Panel
	bottomPanel   *goui.Panel
	root          *goui.Panel
	lblFileStatus *goui.Label
	lblPosStatus  *goui.Label
	lblEncoding   *goui.Label
	lblEOL        *goui.Label
	lblLanguage   *goui.Label
	lblZoom       *goui.Label

	commands        *commandRegistry
	focusBeforeMenu goui.Component
	updateLayout    func()
	settingsWin     *settingsWindow

	settings         Settings
	autoZoom         bool
	applyingSettings bool
	uiScale          float64
	// settingsPathOverride 仅用于测试，生产环境为空。
	settingsPathOverride string
}

// NewApp 创建应用状态与核心编辑器，但尚不构建窗口内容。
func NewApp(win *goui.Window, fontFamily string, settings Settings) *App {
	settings = settings.Normalized()
	th := goui.ThemeByID(settings.Theme)
	if th == nil {
		th = goui.ThemeDark()
	}
	goui.SetTheme(th)

	a := &App{
		win:                win,
		fontFamily:         fontFamily,
		currentFileName:    "未命名.md",
		currentEncoding:    textio.DefaultEncoding(),
		currentEOL:         textio.DefaultEOL(),
		currentHighlighter: goui.NewMarkdownHighlighter(),
		highlightEnabled:   settings.Highlight,
		themeID:            th.ID,
		settings:           settings,
		autoZoom:           settings.Zoom <= 0,
	}
	a.uiScale = win.GetScale()
	if a.uiScale < 1 {
		a.uiScale = 1
	}

	a.textArea = goui.NewTextArea()
	a.textArea.SetFontFamily(fontFamily)
	a.textArea.SetBaseFontSize(settings.BaseFontSize)
	a.textArea.SetWordWrap(settings.WordWrap)
	a.textArea.SetShowLineNumbers(settings.LineNumbers)
	if settings.Highlight {
		a.textArea.SetHighlighter(a.currentHighlighter)
	}

	if settings.Zoom > 0 {
		a.textArea.SetZoom(settings.Zoom)
	} else if scale := win.GetScale(); scale > 1.05 {
		// 跟随系统 DPI：仅在高分屏自动放大，100% 屏幕保持 1.0
		a.textArea.SetZoom(scale)
	}
	return a
}

// Build 构建工具栏、状态栏、查找条、根容器、命令与事件绑定。
func (a *App) Build() {
	a.buildCommands()
	a.buildMenuBar()
	a.buildStatusBar()
	a.buildFindBar()
	a.buildLayout()
	a.bindTextEvents()
	a.bindWindowEvents()
}

// Start 载入启动文档、挂载根组件、注册关闭拦截并进入消息循环。
func (a *App) Start(initialTarget, welcomeDoc string) {
	if initialTarget != "" {
		a.LoadFile(initialTarget)
	} else {
		a.showStartupDocument(welcomeDoc)
	}

	// 默认让文本编辑区获取焦点
	a.textArea.SetFocused(true)

	a.win.SetContent(a.root)
	a.updateLayout()
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()

	// 注册窗口直接关闭拦截钩子（未保存修改提示确认），关闭成功后保存偏好。
	a.win.SetOnClose(a.handleClose)

	a.win.Run()
}

// showStartupDocument 按设置打开介绍文档或空白页。
func (a *App) showStartupDocument(welcomeDoc string) {
	if a.settings.StartupPage == startupPageBlank {
		a.showBlankDoc()
		return
	}
	a.showWelcomeDoc(welcomeDoc)
}

func (a *App) showWelcomeDoc(welcomeDoc string) {
	a.currentEncoding = textio.DefaultEncoding()
	a.currentEOL = textio.DefaultEOL()
	a.currentStamp = textio.Stamp{}
	a.textArea.SetLineEnding(a.currentEOL.Sequence())
	a.textArea.SetText(welcomeDoc)
	a.textArea.SetModified(false)
	a.ApplyHighlighterForFile("welcome.md")
}

// showBlankDoc 重置为未命名空白文档，不写入任何提示文本。
func (a *App) showBlankDoc() {
	a.currentEncoding = textio.DefaultEncoding()
	a.currentEOL = textio.DefaultEOL()
	a.currentStamp = textio.Stamp{}
	a.currentFilePath = ""
	a.textArea.SetLineEnding(a.currentEOL.Sequence())
	a.textArea.SetText("")
	a.textArea.SetModified(false)
	a.ApplyHighlighterForFile(a.currentFileName)
}

func (a *App) clientWidth() int32 {
	w, _ := a.win.GetSize()
	if w <= 0 {
		w = 1560
	}
	return w
}

func (a *App) clientHeight() int32 {
	_, h := a.win.GetSize()
	if h <= 0 {
		h = 920
	}
	return h
}

// scale 把 96 DPI 下的逻辑尺寸换算为当前窗口的物理像素。
func (a *App) scale(v int32) int32 {
	if a.uiScale <= 1 {
		return v
	}
	return int32(float64(v)*a.uiScale + 0.5)
}

// applyDPIScale 在窗口跨显示器 DPI 变化时重建菜单/状态栏尺寸。
func (a *App) applyDPIScale(dpi uint32) {
	if dpi == 0 || a.menuBar == nil {
		return
	}
	scale := float64(dpi) / 96
	if scale < 1 {
		scale = 1
	}
	diff := scale - a.uiScale
	if diff > -0.01 && diff < 0.01 {
		return
	}
	a.uiScale = scale

	a.menuBar.SetFont(goui.NewFont(a.fontFamily, a.scale(15)))
	a.menuBar.SetBarHeight(a.scale(34))
	a.menuBar.SetItemPadding(a.scale(12))
	a.menuBar.SetRowHeight(a.scale(30))
	a.menuBar.SetSeparatorGap(a.scale(9))
	a.menuBar.SetMenuMinWidth(a.scale(180))
	a.menuBar.SetRadius(a.scale(8))

	statusFont := goui.NewFont(a.fontFamily, a.scale(13))
	for _, label := range []*goui.Label{
		a.lblFileStatus, a.lblPosStatus, a.lblEncoding,
		a.lblEOL, a.lblLanguage, a.lblZoom,
	} {
		if label != nil {
			label.SetFont(statusFont)
		}
	}

	a.updateLayout()
	a.win.RequestRedraw()
}

func (a *App) statusHeight() int32 {
	return a.scale(28)
}

func (a *App) buildCommands() {
	a.commands = newCommandRegistry()

	add := func(c *Command) { a.commands.Register(c) }

	add(&Command{ID: cmdFileNew, Title: "新建", Accel: "Ctrl+N", Run: a.NewDocument})
	add(&Command{ID: cmdFileOpen, Title: "打开…", Accel: "Ctrl+O", Run: a.Open})
	add(&Command{ID: cmdFileSave, Title: "保存", Accel: "Ctrl+S", Run: func() { a.Save() }})
	add(&Command{ID: cmdFileSaveAs, Title: "另存为…", Accel: "Ctrl+Shift+S", Run: func() { a.SaveAs() }})
	add(&Command{ID: cmdFileQuit, Title: "退出", Accel: "Alt+F4", Run: a.win.Close})

	add(&Command{ID: cmdEditUndo, Title: "撤销", Accel: "Ctrl+Z", Run: func() { a.textArea.Undo() },
		Enabled: a.textArea.CanUndo})
	add(&Command{ID: cmdEditRedo, Title: "重做", Accel: "Ctrl+Y", Run: func() { a.textArea.Redo() },
		Enabled: a.textArea.CanRedo})
	add(&Command{ID: cmdEditCut, Title: "剪切", Accel: "Ctrl+X", Run: func() { a.textArea.Cut() },
		Enabled: func() bool { return a.textArea.HasSelection() && !a.textArea.IsReadOnly() }})
	add(&Command{ID: cmdEditCopy, Title: "复制", Accel: "Ctrl+C", Run: func() { a.textArea.Copy() },
		Enabled: a.textArea.HasSelection})
	add(&Command{ID: cmdEditPaste, Title: "粘贴", Accel: "Ctrl+V", Run: func() { a.textArea.Paste() },
		Enabled: func() bool { return !a.textArea.IsReadOnly() }})
	add(&Command{ID: cmdEditSelectAll, Title: "全选", Accel: "Ctrl+A", Run: a.textArea.SelectAll})
	add(&Command{ID: cmdEditFind, Title: "查找…", Accel: "Ctrl+F",
		Run: func() { a.findBar.show(false, a.clientWidth()) }})
	add(&Command{ID: cmdEditReplace, Title: "替换…", Accel: "Ctrl+H",
		Run: func() { a.findBar.show(true, a.clientWidth()) }})
	add(&Command{ID: cmdEditFormat, Title: "格式化文档", Accel: "Ctrl+Shift+F",
		Run: a.FormatDocument, Enabled: func() bool { return a.formatTarget() != format.KindNone }})

	add(&Command{ID: cmdViewLineNumbers, Title: "行号", Accel: "",
		Run: a.ToggleLineNumbers, Checked: a.textArea.IsShowLineNumbers})
	add(&Command{ID: cmdViewWordWrap, Title: "自动换行", Accel: "",
		Run: a.ToggleWordWrap, Checked: a.textArea.IsWordWrap})
	add(&Command{ID: cmdViewHighlight, Title: "语法高亮", Accel: "",
		Run: a.ToggleHighlight, Checked: func() bool { return a.highlightEnabled }})
	add(&Command{ID: cmdViewZoomIn, Title: "放大", Accel: "Ctrl++", Run: a.ZoomIn})
	add(&Command{ID: cmdViewZoomOut, Title: "缩小", Accel: "Ctrl+-", Run: a.ZoomOut})
	add(&Command{ID: cmdViewZoomReset, Title: "重置缩放", Accel: "Ctrl+0", Run: a.ResetZoom,
		Checked: func() bool { return a.textArea.GetZoomPercent() == 100 }})
	for _, item := range goui.Themes() {
		th := item
		add(&Command{
			ID:      themeCommandID(th.ID),
			Title:   th.Label,
			Run:     func() { a.SetThemeID(th.ID) },
			Checked: func() bool { return a.themeID == th.ID },
		})
	}

	add(&Command{ID: cmdHelpShortcuts, Title: "快捷键一览", Accel: "", Run: a.showShortcuts})
	add(&Command{ID: cmdHelpAbout, Title: "关于 gnote", Accel: "", Run: a.showAbout})

	add(&Command{ID: cmdAppSettings, Title: "首选项…", Accel: "Ctrl+,", Run: a.OpenSettings})
	add(&Command{ID: cmdAppOpenConfig, Title: "打开配置文件", Accel: "", Run: a.OpenConfigFile})
}

func (a *App) showShortcuts() {
	goui.ShowInfo(a.win.Hwnd(), "快捷键", shortcutsText)
}

func (a *App) showAbout() {
	goui.ShowInfo(a.win.Hwnd(), "关于 gnote", aboutText)
}

const shortcutsText = "Ctrl+N/O/S/Shift+S 新建 / 打开 / 保存 / 另存为\n" +
	"Ctrl+Shift+F          格式化 JSON / HTML\n" +
	"Ctrl+F / Ctrl+H       查找 / 替换（F3 下一个）\n" +
	"Ctrl+Z / Ctrl+Y       撤销 / 重做\n" +
	"Ctrl+A / X / C / V    全选 / 剪切 / 复制 / 粘贴\n" +
	"Ctrl+滚轮 / Ctrl+±/0  缩放 / 重置\n" +
	"Alt+F / F10           打开菜单\n" +
	"Alt+F4                退出"

// appVersion 开源正式版本号；历史上的 v3/v4 标记仅为开发期迭代号。
const appVersion = "1.0.0"

const aboutText = "gnote " + appVersion + "\n\n纯 Go、零 CGO 的 Windows 原生记事本与代码编辑器。\n" +
	"软件光栅化渲染 · 内嵌字体 · 16 种语法高亮\n\n" +
	"https://github.com/jacksalad/gnote"

func (a *App) buildMenuBar() {
	a.menuBar = goui.NewMenuBar()
	a.menuBar.SetFont(goui.NewFont(a.fontFamily, a.scale(15)))
	a.menuBar.SetBarHeight(a.scale(34))
	a.menuBar.SetItemPadding(a.scale(12))
	a.menuBar.SetRowHeight(a.scale(30))
	a.menuBar.SetSeparatorGap(a.scale(9))
	a.menuBar.SetMenuMinWidth(a.scale(180))
	a.menuBar.SetRadius(a.scale(8))
	a.menuBar.SetModel(a.buildMenuModel)
	a.menuBar.SetOnOpenChange(a.onMenuOpenChange)
}

func (a *App) buildMenuModel() []goui.Menu {
	entry := func(id commandID) goui.MenuEntry {
		c := a.commands.Get(id)
		if c == nil {
			return goui.MenuEntry{Text: string(id), Disabled: true}
		}
		return goui.MenuEntry{
			Text:     c.Title,
			Accel:    c.Accel,
			Disabled: !c.isEnabled(),
			Checked:  c.isChecked(),
			OnClick:  func() { a.commands.Run(id) },
		}
	}
	themeEntries := make([]goui.MenuEntry, 0, len(goui.Themes()))
	for _, th := range goui.Themes() {
		themeEntries = append(themeEntries, entry(themeCommandID(th.ID)))
	}
	sep := goui.MenuEntry{Separator: true}
	return []goui.Menu{
		{Text: "文件", Items: []goui.MenuEntry{
			entry(cmdFileNew), entry(cmdFileOpen), sep,
			entry(cmdFileSave), entry(cmdFileSaveAs), sep,
			entry(cmdFileQuit),
		}},
		{Text: "编辑", Items: []goui.MenuEntry{
			entry(cmdEditUndo), entry(cmdEditRedo), sep,
			entry(cmdEditCut), entry(cmdEditCopy), entry(cmdEditPaste), sep,
			entry(cmdEditSelectAll), sep,
			entry(cmdEditFind), entry(cmdEditReplace), sep,
			entry(cmdEditFormat),
		}},
		{Text: "查看", Items: []goui.MenuEntry{
			entry(cmdViewLineNumbers), entry(cmdViewWordWrap), entry(cmdViewHighlight), sep,
			entry(cmdViewZoomIn), entry(cmdViewZoomOut), entry(cmdViewZoomReset), sep,
			{Text: "主题", Submenu: themeEntries},
		}},
		{Text: "设置", Items: []goui.MenuEntry{
			entry(cmdAppSettings), entry(cmdAppOpenConfig),
		}},
		{Text: "帮助", Items: []goui.MenuEntry{
			entry(cmdHelpShortcuts), entry(cmdHelpAbout),
		}},
	}
}

func (a *App) onMenuOpenChange(open bool) {
	if open {
		a.focusBeforeMenu = a.currentFocus()
		goui.RequestFocus(a.menuBar)
		return
	}
	target := a.focusBeforeMenu
	a.focusBeforeMenu = nil
	if target == nil {
		target = a.textArea
	}
	goui.RequestFocus(target)
	a.win.SetLastFocused(target)
}

func (a *App) currentFocus() goui.Component {
	if a.findBar != nil && a.findBar.visible {
		if a.findBar.query.IsFocused() {
			return a.findBar.query
		}
		if a.findBar.replaceBox.IsFocused() {
			return a.findBar.replaceBox
		}
	}
	return a.textArea
}

func (a *App) buildStatusBar() {
	statusFont := goui.NewFont(a.fontFamily, a.scale(13))
	a.statusBar = goui.NewSurfacePanel()
	a.statusBar.SetBounds(0, 0, 1560, a.statusHeight())

	a.lblFileStatus = goui.NewLabel("新文档 (未保存)")
	a.lblFileStatus.SetFont(statusFont)
	a.lblFileStatus.SetEllipsis(true)

	a.lblPosStatus = goui.NewSecondaryLabel("1:1 · 1 行")
	a.lblPosStatus.SetFont(statusFont)
	a.lblPosStatus.SetAlign(goui.AlignRight)

	a.lblEncoding = goui.NewSecondaryLabel(a.currentEncoding.Name())
	a.lblEncoding.SetFont(statusFont)
	a.lblEncoding.SetAlign(goui.AlignRight)
	a.lblEncoding.SetOnClick(a.showEncodingMenu)

	a.lblEOL = goui.NewSecondaryLabel(a.currentEOL.Label())
	a.lblEOL.SetFont(statusFont)
	a.lblEOL.SetAlign(goui.AlignRight)
	a.lblEOL.SetOnClick(a.showEOLMenu)

	a.lblLanguage = goui.NewSecondaryLabel("Markdown")
	a.lblLanguage.SetFont(statusFont)
	a.lblLanguage.SetAlign(goui.AlignRight)
	a.lblLanguage.SetOnClick(a.showLanguageMenu)

	a.lblZoom = goui.NewSecondaryLabel(fmt.Sprintf("%d%%", a.textArea.GetZoomPercent()))
	a.lblZoom.SetFont(statusFont)
	a.lblZoom.SetAlign(goui.AlignRight)
	a.lblZoom.SetOnClick(a.showZoomMenu)

	a.statusBar.Add(a.lblFileStatus)
	a.statusBar.Add(a.lblPosStatus)
	a.statusBar.Add(a.lblEncoding)
	a.statusBar.Add(a.lblEOL)
	a.statusBar.Add(a.lblLanguage)
	a.statusBar.Add(a.lblZoom)

	a.updateStatusLayout(1560)
}

// updateStatusLayout 从右向左排布状态段；左侧文件路径自动省略。
func (a *App) updateStatusLayout(width int32) {
	if a.statusBar == nil || a.lblFileStatus == nil {
		return
	}
	if width <= 0 {
		width = 1560
	}
	margin := a.scale(14)
	gap := a.scale(8)
	labelH := a.scale(20)
	baseY := a.statusBar.GetBounds().Y + a.scale(4)
	rightEdge := a.statusBar.GetBounds().X + width - margin

	type segment struct {
		label *goui.Label
		width int32
	}
	all := []segment{
		{a.lblZoom, a.scale(50)},
		{a.lblLanguage, a.scale(80)},
		{a.lblEOL, a.scale(56)},
		{a.lblEncoding, a.scale(82)},
		{a.lblPosStatus, a.scale(100)},
	}
	for _, s := range all {
		s.label.SetVisible(true)
	}
	place := func(segs []segment) int32 {
		right := rightEdge
		for _, s := range segs {
			s.label.SetBounds(right-s.width, baseY, s.width, labelH)
			right -= s.width + gap
		}
		return right
	}

	right := place(all)
	leftW := right - margin
	minLeft := a.scale(80)

	// 极窄窗口或高 DPI：先收起次要段，保证文件路径与光标位置不重叠。
	if leftW < minLeft {
		a.lblLanguage.SetVisible(false)
		a.lblEOL.SetVisible(false)
		right = place([]segment{
			{a.lblZoom, a.scale(50)},
			{a.lblEncoding, a.scale(82)},
			{a.lblPosStatus, a.scale(100)},
		})
		leftW = right - margin
	}
	if leftW < minLeft {
		a.lblEncoding.SetVisible(false)
		right = place([]segment{
			{a.lblZoom, a.scale(50)},
			{a.lblPosStatus, a.scale(100)},
		})
		leftW = right - margin
	}
	if leftW < minLeft {
		leftW = minLeft
	}

	a.lblFileStatus.SetBounds(a.statusBar.GetBounds().X+margin, baseY, leftW, labelH)
}

func (a *App) buildFindBar() {
	a.findBar = newFindBar(a.textArea, a.win, func() {
		if a.updateLayout != nil {
			a.updateLayout()
		}
	})
}

func (a *App) buildLayout() {
	a.bottomPanel = goui.NewSurfacePanel()
	a.bottomPanel.SetBounds(0, 0, 1560, a.statusHeight())
	a.statusBar.SetBounds(0, 0, 1560, a.statusHeight())
	a.bottomPanel.Add(a.statusBar)
	a.bottomPanel.Add(a.findBar.panel)

	// 顶部菜单占位：真正绘制与事件由全客户区 MenuBar 负责。
	a.menuSpacer = goui.NewSurfacePanel()
	a.menuSpacer.SetBounds(0, 0, 1560, a.menuBar.BarHeight())

	a.root = goui.NewBackgroundPanel()
	a.root.SetLayout(&goui.BorderLayout{
		Top:    a.menuSpacer,
		Center: a.textArea,
		Bottom: a.bottomPanel,
		Gap:    0,
	})
	a.root.Add(a.menuSpacer)
	a.root.Add(a.textArea)
	a.root.Add(a.bottomPanel)
	// 最后添加：反向事件遍历时菜单优先，且下拉可覆盖编辑器。
	a.root.Add(a.menuBar)

	a.updateLayout = func() {
		width := a.root.GetBounds().Width
		height := a.root.GetBounds().Height
		if width <= 0 {
			width = a.clientWidth()
		}
		if height <= 0 {
			height = a.clientHeight()
		}
		a.menuSpacer.SetBounds(0, 0, width, a.menuBar.BarHeight())
		if a.findBar.visible {
			a.bottomPanel.SetBounds(0, 0, width, a.statusHeight()+findBarHeight)
			a.findBar.panel.SetBounds(0, 0, width, findBarHeight)
			a.statusBar.SetBounds(0, findBarHeight, width, a.statusHeight())
			a.findBar.layout(width)
		} else {
			a.bottomPanel.SetBounds(0, 0, width, a.statusHeight())
			a.statusBar.SetBounds(0, 0, width, a.statusHeight())
		}
		a.root.DoLayout()
		a.updateStatusLayout(width)
		// MenuBar 不属于 BorderLayout 的槽位，必须手动铺满客户区。
		a.menuBar.SetBounds(0, 0, width, height)
	}
	a.updateLayout()
}

func (a *App) bindTextEvents() {
	a.textArea.SetOnChange(func() {
		a.UpdateHeaderAndStatus()
	})
	a.textArea.SetOnCursorChange(func(row, col int) {
		a.UpdateHeaderAndStatus()
	})
	a.textArea.SetOnZoomChange(func(percent int) {
		if !a.applyingSettings {
			a.autoZoom = false
		}
		a.UpdateHeaderAndStatus()
	})
}

func (a *App) bindWindowEvents() {
	a.win.EventBus().SubscribeHandler(goui.EventKeyDown, func(evt goui.Event) {
		key, ok := evt.Data.(goui.KeyData)
		if !ok {
			return
		}
		// Alt+字母 / F10 打开菜单；菜单打开时由 MenuBar 处理方向键与 Esc。
		if key.Alt {
			switch key.KeyCode {
			case 'F', 'f':
				a.menuBar.Open(0)
			case 'E', 'e':
				a.menuBar.Open(1)
			case 'V', 'v':
				a.menuBar.Open(2)
			case 'S', 's':
				a.menuBar.Open(3)
			case 'H', 'h':
				a.menuBar.Open(4)
			}
			return
		}
		if key.KeyCode == 0x79 { // F10
			a.menuBar.Open(0)
			return
		}
		if a.menuBar.IsOpen() {
			// 菜单打开时拦截文档级快捷键，键盘导航交给 MenuBar.OnEvent。
			return
		}
		if key.KeyCode == 0x1B { // Esc 关闭查找条
			if a.findBar.visible {
				a.findBar.hide()
			}
			return
		}
		if key.KeyCode == 0x72 { // F3 查找下一个
			if a.findBar.visible {
				a.findBar.find(true)
			}
			return
		}
		if !key.Ctrl {
			return
		}
		switch key.KeyCode {
		case 'N', 'n':
			a.commands.Run(cmdFileNew)
		case 'O', 'o':
			a.commands.Run(cmdFileOpen)
		case 'S', 's':
			if key.Shift {
				a.commands.Run(cmdFileSaveAs)
			} else {
				a.commands.Run(cmdFileSave)
			}
		case 'F', 'f':
			if key.Shift {
				a.commands.Run(cmdEditFormat)
			} else {
				a.commands.Run(cmdEditFind)
			}
		case 'H', 'h':
			a.commands.Run(cmdEditReplace)
		case 0xBC: // Ctrl+,
			a.commands.Run(cmdAppSettings)
		}
	})

	// 查找条与编辑器共享键盘焦点会串键：点击查找条时编辑器让出焦点，点击编辑器时输入框让出焦点
	a.win.EventBus().SubscribeHandler(goui.EventMouseDown, func(evt goui.Event) {
		if a.menuBar.IsOpen() {
			return
		}
		if !a.findBar.visible {
			return
		}
		data, ok := evt.Data.(goui.MouseData)
		if !ok {
			return
		}
		if a.findBar.contains(data.X, data.Y) {
			a.textArea.SetFocused(false)
		} else {
			a.findBar.blurInputs()
			a.textArea.SetFocused(true)
		}
	})

	// 右键菜单：全部条目由命令注册表生成，启用状态实时求值
	a.win.EventBus().SubscribeHandler(goui.EventMouseUp, func(evt goui.Event) {
		data, ok := evt.Data.(goui.MouseData)
		if !ok || data.Button != goui.MouseButtonRight {
			return
		}
		if a.findBar.contains(data.X, data.Y) {
			return
		}
		b := a.textArea.GetBounds()
		if data.X < b.X || data.X >= b.X+b.Width || data.Y < b.Y || data.Y >= b.Y+b.Height {
			return
		}

		type ctxEntry struct {
			id   commandID
			text string
			sep  bool
		}
		entries := []ctxEntry{
			{id: cmdEditUndo, text: "撤销\tCtrl+Z"},
			{id: cmdEditRedo, text: "重做\tCtrl+Y"},
			{sep: true},
			{id: cmdEditCut, text: "剪切\tCtrl+X"},
			{id: cmdEditCopy, text: "复制\tCtrl+C"},
			{id: cmdEditPaste, text: "粘贴\tCtrl+V"},
			{sep: true},
			{id: cmdEditSelectAll, text: "全选\tCtrl+A"},
			{sep: true},
			{id: cmdEditFind, text: "查找…\tCtrl+F"},
			{id: cmdEditReplace, text: "替换…\tCtrl+H"},
			{sep: true},
			{id: cmdEditFormat, text: "格式化文档\tCtrl+Shift+F"},
		}

		items := make([]goui.MenuItem, 0, len(entries))
		menuIDs := make(map[uint32]commandID, len(entries))
		for i, entry := range entries {
			if entry.sep {
				items = append(items, goui.MenuItem{Separator: true})
				continue
			}
			menuID := uint32(i + 1)
			menuIDs[menuID] = entry.id
			items = append(items, goui.MenuItem{
				ID:       menuID,
				Text:     entry.text,
				Disabled: !a.commands.Enabled(entry.id),
			})
		}

		if chosen := goui.ShowPopupMenu(a.win.Hwnd(), data.X, data.Y, items); chosen != 0 {
			if id, ok := menuIDs[chosen]; ok {
				a.commands.Run(id)
			}
		}
		a.win.RequestRedraw()
	})

	a.win.EventBus().SubscribeHandler(goui.EventResize, func(goui.Event) {
		a.updateLayout()
	})

	a.win.EventBus().SubscribeHandler(goui.EventBlur, func(goui.Event) {
		if a.menuBar.IsOpen() {
			a.menuBar.Close()
		}
	})

	// 多显示器 DPI 切换：按新旧 DPI 比例调整编辑区缩放，保持文字物理尺寸一致。
	a.win.EventBus().SubscribeHandler(goui.EventDpiChanged, func(evt goui.Event) {
		data, ok := evt.Data.(goui.DpiData)
		if !ok || data.OldDPI == 0 || data.NewDPI == 0 || data.OldDPI == data.NewDPI {
			return
		}
		a.applyDPIScale(data.NewDPI)
		if a.autoZoom {
			a.applyingSettings = true
			a.textArea.SetZoom(float64(data.NewDPI) / 96)
			a.applyingSettings = false
		}
	})
}

// UpdateHeaderAndStatus 刷新标题栏、状态栏与缩放百分比。
func (a *App) UpdateHeaderAndStatus() {
	modMarker := ""
	if a.textArea.IsModified() {
		modMarker = "* "
	}

	displayTitle := fmt.Sprintf("%s%s - gnote", modMarker, a.currentFileName)
	if a.win.GetTitle() != displayTitle {
		a.win.SetTitle(displayTitle)
	}

	if a.currentFilePath == "" {
		if a.textArea.IsModified() {
			a.lblFileStatus.SetText("新文档 * (未保存)")
		} else {
			a.lblFileStatus.SetText("新文档")
		}
	} else {
		if a.textArea.IsModified() {
			a.lblFileStatus.SetText(fmt.Sprintf("%s *", a.currentFilePath))
		} else {
			a.lblFileStatus.SetText(a.currentFilePath)
		}
	}

	docType := "纯文本"
	if a.highlightEnabled && a.textArea.GetHighlighter() != nil {
		docType = goui.GetLanguageName(a.textArea.GetHighlighter())
	}

	r, c := a.textArea.GetCursorPosition()
	a.lblPosStatus.SetText(fmt.Sprintf("%d:%d · %d 行", r, c, a.textArea.GetLineCount()))
	a.lblEncoding.SetText(a.currentEncoding.Name())
	a.lblEOL.SetText(a.currentEOL.Label())
	a.lblLanguage.SetText(docType)
	a.lblZoom.SetText(fmt.Sprintf("%d%%", a.textArea.GetZoomPercent()))
}

// ApplyHighlighterForFile 根据文件后缀嗅探高亮器；高亮关闭时只记录，不挂载。
func (a *App) ApplyHighlighterForFile(path string) {
	a.currentHighlighter = goui.DetectHighlighter(path)
	if a.highlightEnabled {
		a.textArea.SetHighlighter(a.currentHighlighter)
	}
}

// formatTarget 返回当前文档可用的格式化类型。
// 仅对已绑定文件名的 JSON/HTML 文档启用，避免误判未命名文档中的代码片段。
func (a *App) formatTarget() format.Kind {
	if a.textArea == nil || a.textArea.IsReadOnly() || a.currentFilePath == "" {
		return format.KindNone
	}
	return format.KindForPath(a.currentFilePath)
}

// FormatDocument 按文件类型格式化当前文档。
// 整篇替换作为一步撤销提交，光标保持原逻辑位置（越界时收敛）。
func (a *App) FormatDocument() {
	kind := a.formatTarget()
	if kind == format.KindNone {
		goui.ShowInfo(a.win.Hwnd(), "格式化", "当前文档不是 JSON 或 HTML 文件，无需格式化。")
		return
	}

	src := a.textArea.GetTextWithEOL("\n")
	formatted, err := format.Format(kind, src)
	if err != nil {
		goui.ShowError(a.win.Hwnd(), "格式化失败", err.Error())
		return
	}
	if formatted == src {
		return
	}
	if !a.textArea.ReplaceDocument(formatted) {
		return
	}

	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}

func (a *App) ToggleLineNumbers() {
	a.textArea.SetShowLineNumbers(!a.textArea.IsShowLineNumbers())
	a.win.RequestRedraw()
}

func (a *App) ToggleWordWrap() {
	a.textArea.SetWordWrap(!a.textArea.IsWordWrap())
	a.win.RequestRedraw()
}

func (a *App) ToggleHighlight() {
	a.highlightEnabled = !a.highlightEnabled
	if a.highlightEnabled {
		if a.currentHighlighter == nil {
			// 纯文本（.txt/无后缀）没有语言高亮器，保持纯文本即可，
			// 不能回退到 Markdown 而把普通文本误标成 Markdown。
			a.currentHighlighter = goui.DetectHighlighter(a.currentFileName)
		}
		a.textArea.SetHighlighter(a.currentHighlighter)
	} else {
		a.textArea.SetHighlighter(nil)
	}
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}

// SetThemeID 切换当前主题并按持久化标识记录，供菜单、设置页与退出保存共用。
func (a *App) SetThemeID(id string) {
	th := goui.ThemeByID(id)
	if th == nil {
		return
	}
	goui.SetTheme(th)
	a.themeID = th.ID
	a.settings.Theme = th.ID
	if a.win != nil {
		a.win.RequestRedraw()
	}
}

func (a *App) ZoomIn() {
	a.autoZoom = false
	a.textArea.ZoomIn()
	a.win.RequestRedraw()
}

func (a *App) ZoomOut() {
	a.autoZoom = false
	a.textArea.ZoomOut()
	a.win.RequestRedraw()
}

func (a *App) ResetZoom() {
	a.autoZoom = false
	a.textArea.ResetZoom()
	a.win.RequestRedraw()
}

// handleClose 关闭前检查未保存修改，成功后持久化窗口尺寸与偏好。
func (a *App) handleClose() bool {
	if !a.CheckSaveIfModified() {
		return false
	}
	_ = a.SaveSettings()
	return true
}

// WriteDocumentTo 按当前编码与换行风格写入指定路径。
// 写盘失败时返回 false，文件绑定与身份戳由调用方在成功后再提交。
func (a *App) WriteDocumentTo(path string) bool {
	text := a.textArea.GetTextWithEOL(a.currentEOL.Sequence())
	data, replaced, err := textio.Encode(text, a.currentEncoding)
	if err != nil {
		goui.ShowError(a.win.Hwnd(), "保存错误",
			fmt.Sprintf("无法用 %s 编码保存该文档:\n%v", a.currentEncoding.Name(), err))
		return false
	}
	if replaced {
		msg := fmt.Sprintf("文档中包含无法用 %s 编码表示的字符（如 emoji 或生僻字），\n这些字符将以 \"?\" 代替保存。是否继续？", a.currentEncoding.Name())
		if goui.ShowConfirmYesNoCancel(a.win.Hwnd(), "编码警告", msg) != goui.IDYES {
			return false
		}
	}
	if err := writeFileAtomic(path, data); err != nil {
		goui.ShowError(a.win.Hwnd(), "保存错误", fmt.Sprintf("无法保存文件:\n%v", err))
		return false
	}
	return true
}

// Save 执行保存；未命名文件会先触发另存为。
func (a *App) Save() bool {
	target := a.currentFilePath
	if target == "" {
		path, ok := goui.SaveFileDialog(a.win.Hwnd(), "保存文档", fileFilter, ".txt", a.currentFileName)
		if !ok || path == "" {
			return false
		}
		target = path
	} else if !a.currentStamp.Same(target) {
		msg := fmt.Sprintf("文件 \"%s\" 已在 gnote 之外被修改或删除。\n继续保存将覆盖外部更改，是否继续？", a.currentFileName)
		if goui.ShowConfirmYesNoCancel(a.win.Hwnd(), "文件已被外部修改", msg) != goui.IDYES {
			return false
		}
	}

	if !a.WriteDocumentTo(target) {
		return false
	}
	// 写盘成功后才提交文件绑定：编码确认取消、写盘失败时，
	// 文档仍保留原来的路径/标题，不会在下次 Ctrl+S 时静默写到新路径。
	if target != a.currentFilePath {
		a.currentFilePath = target
		a.currentFileName = filepath.Base(target)
		a.ApplyHighlighterForFile(target)
	}
	a.currentStamp = textio.Stat(target)
	a.textArea.SetModified(false)
	a.UpdateHeaderAndStatus()
	return true
}

// SaveAs 执行另存为。
func (a *App) SaveAs() bool {
	path, ok := goui.SaveFileDialog(a.win.Hwnd(), "另存为", fileFilter, ".txt", a.currentFileName)
	if !ok || path == "" {
		return false
	}

	if !a.WriteDocumentTo(path) {
		return false
	}
	a.currentFilePath = path
	a.currentFileName = filepath.Base(path)
	a.ApplyHighlighterForFile(path)
	a.currentStamp = textio.Stat(path)
	a.textArea.SetModified(false)
	a.UpdateHeaderAndStatus()
	return true
}

// CheckSaveIfModified 检查未保存修改并提示保存。
// 返回 true 表示允许继续后续动作，false 表示取消动作。
func (a *App) CheckSaveIfModified() bool {
	if !a.textArea.IsModified() {
		return true
	}

	docName := a.currentFileName
	if docName == "" {
		docName = "未命名"
	}
	prompt := fmt.Sprintf("你想将更改保存到 \"%s\" 吗？", docName)
	ret := goui.ShowConfirmYesNoCancel(a.win.Hwnd(), "gnote", prompt)
	switch ret {
	case goui.IDYES:
		return a.Save()
	case goui.IDNO:
		return true
	case goui.IDCANCEL:
		return false
	default:
		return false
	}
}

// LoadFile 打开指定路径的文件；路径不存在时作为新文件路径绑定。
func (a *App) LoadFile(targetPath string) bool {
	targetPath = strings.Trim(strings.TrimSpace(targetPath), "\"")
	if targetPath == "" {
		return false
	}
	if absPath, err := filepath.Abs(targetPath); err == nil {
		targetPath = absPath
	}

	fi, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件尚不存在，作为新文件路径打开
			a.currentFilePath = targetPath
			a.currentFileName = filepath.Base(targetPath)
			a.currentEncoding = textio.DefaultEncoding()
			a.currentEOL = textio.DefaultEOL()
			a.currentStamp = textio.Stamp{}
			a.textArea.SetText("")
			a.textArea.SetLineEnding(a.currentEOL.Sequence())
			a.textArea.SetModified(false)
			a.ApplyHighlighterForFile(a.currentFilePath)
			a.textArea.SetFocused(true)
			a.UpdateHeaderAndStatus()
			return true
		}
		goui.ShowError(a.win.Hwnd(), "打开错误", fmt.Sprintf("无法访问文件:\n%v", err))
		return false
	}

	if fi.IsDir() {
		goui.ShowError(a.win.Hwnd(), "打开错误", fmt.Sprintf("指定的路径是一个文件夹，无法作为文本文件打开:\n%s", targetPath))
		return false
	}

	// 超大文件会长时间占用 UI 线程并产生巨量内存占用，先拒绝并给出明确提示
	if fi.Size() > maxOpenFileSize {
		goui.ShowError(a.win.Hwnd(), "文件过大",
			fmt.Sprintf("该文件大小为 %.1f MB，超过 %.0f MB 的安全打开上限。\n为避免界面卡顿或内存耗尽，已取消打开。",
				float64(fi.Size())/(1<<20), float64(maxOpenFileSize)/(1<<20)))
		return false
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		goui.ShowError(a.win.Hwnd(), "打开错误", fmt.Sprintf("读取文件失败:\n%v", err))
		return false
	}

	text, enc, err := textio.Decode(data)
	if err != nil {
		if errors.Is(err, textio.ErrBinary) {
			goui.ShowError(a.win.Hwnd(), "打开错误",
				fmt.Sprintf("该文件看起来是二进制文件，无法作为文本打开:\n%s", targetPath))
		} else {
			goui.ShowError(a.win.Hwnd(), "打开错误",
				fmt.Sprintf("无法识别该文件的文本编码:\n%v", err))
		}
		return false
	}

	a.currentEOL = textio.AnalyzeEOL(text)
	a.currentEncoding = enc
	a.textArea.SetText(text)
	a.textArea.SetLineEnding(a.currentEOL.Sequence())
	a.currentFilePath = targetPath
	a.currentFileName = filepath.Base(targetPath)
	a.currentStamp = textio.Stat(targetPath)
	a.textArea.SetModified(false)

	a.ApplyHighlighterForFile(a.currentFilePath)
	a.textArea.SetFocused(true)
	a.UpdateHeaderAndStatus()
	return true
}

// Open 执行打开文件对话框。
func (a *App) Open() {
	if !a.CheckSaveIfModified() {
		return
	}

	path, ok := goui.OpenFileDialog(a.win.Hwnd(), "打开文本文件", fileFilter, ".txt")
	if !ok || path == "" {
		return
	}
	a.LoadFile(path)
}

// NewDocument 执行新建文档。
func (a *App) NewDocument() {
	if !a.CheckSaveIfModified() {
		return
	}

	a.textArea.SetText("")
	a.currentFilePath = ""
	a.currentFileName = "未命名.txt"
	a.currentEncoding = textio.DefaultEncoding()
	a.currentEOL = textio.DefaultEOL()
	a.currentStamp = textio.Stamp{}
	a.textArea.SetLineEnding(a.currentEOL.Sequence())

	// 新文档没有后缀可嗅探：同步关闭高亮开关，避免菜单勾选与实际状态不一致
	a.highlightEnabled = false
	a.currentHighlighter = nil
	a.textArea.SetHighlighter(nil)
	a.textArea.SetModified(false)
	a.textArea.SetFocused(true)
	a.UpdateHeaderAndStatus()
}
