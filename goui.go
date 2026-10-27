// Package goui 提供简洁的GUI框架
package goui

import (
	"github.com/jacksalad/gnote/clipboard"
	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/dialog"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
	"github.com/jacksalad/gnote/window"
)

// 重新导出类型，方便用户使用

// Window 窗口
type Window = window.Window

// WindowConfig 窗口配置
type WindowConfig = window.WindowConfig

// NewWindow 创建新窗口
func NewWindow(config WindowConfig) (*Window, error) {
	return window.NewWindow(config)
}

// Icon 窗口图标
type Icon = window.Icon

// NewIconFromICO 从 .ico 二进制数据创建窗口图标
func NewIconFromICO(data []byte) (*Icon, error) {
	return window.NewIconFromICO(data)
}

// NewIconFromRGBA 从纯 Go ARGB 像素点阵创建双尺寸窗口图标
func NewIconFromRGBA(smallW, smallH int, smallPixels []graphics.Color, bigW, bigH int, bigPixels []graphics.Color) (*Icon, error) {
	return window.NewIconFromRGBA(smallW, smallH, smallPixels, bigW, bigH, bigPixels)
}

// NewIconFromImage 从 render.Image 图像创建窗口图标
func NewIconFromImage(img *render.Image) (*Icon, error) {
	return window.NewIconFromImage(img)
}

// GetSystemDPI 获取当前显示系统的物理像素 DPI
func GetSystemDPI() uint32 {
	return window.GetDpiForSystem()
}

// GetSystemScale 获取当前显示系统相对于标准 96 DPI 的物理缩放系数（如 1.0, 1.25, 1.5, 2.0）
func GetSystemScale() float64 {
	return float64(window.GetDpiForSystem()) / 96.0
}

// IsWindow 检查给定的 Win32 句柄是否对应一个有效的窗口
func IsWindow(hwnd uintptr) bool {
	return window.IsWindow(hwnd)
}

// IsWindowEnabled 检查窗口是否处于启用状态
func IsWindowEnabled(hwnd uintptr) bool {
	return window.IsWindowEnabled(hwnd)
}

// Event 事件
type Event = event.Event

// EventType 事件类型
type EventType = event.EventType

// 事件类型常量
const (
	EventMouseMove        = event.EventMouseMove
	EventMouseDown        = event.EventMouseDown
	EventMouseUp          = event.EventMouseUp
	EventMouseClick       = event.EventMouseClick
	EventMouseDoubleClick = event.EventMouseDoubleClick
	EventMouseWheel       = event.EventMouseWheel
	EventKeyDown          = event.EventKeyDown
	EventKeyUp            = event.EventKeyUp
	EventKeyPress         = event.EventKeyPress
	EventImeStart         = event.EventImeStart
	EventImeUpdate        = event.EventImeUpdate
	EventImeCommit        = event.EventImeCommit
	EventImeEnd           = event.EventImeEnd
	EventPaint            = event.EventPaint
	EventResize           = event.EventResize
	EventClose            = event.EventClose
	EventFocus            = event.EventFocus
	EventBlur             = event.EventBlur
	EventDpiChanged       = event.EventDpiChanged
)

// ImeData 输入法组合事件数据
type ImeData = event.ImeData

// MouseData 鼠标数据
type MouseData = event.MouseData

// MouseButton 鼠标按钮
type MouseButton = event.MouseButton

// 鼠标按钮常量
const (
	MouseButtonNone   = event.MouseButtonNone
	MouseButtonLeft   = event.MouseButtonLeft
	MouseButtonRight  = event.MouseButtonRight
	MouseButtonMiddle = event.MouseButtonMiddle
)

// KeyData 键盘数据
type KeyData = event.KeyData

// DpiData 窗口 DPI 变化数据
type DpiData = event.DpiData

// ResizeData 大小改变数据
type ResizeData = event.ResizeData

// Color 颜色
type Color = graphics.Color

// 预定义颜色
const (
	ColorTransparent = graphics.ColorTransparent
	ColorBlack       = graphics.ColorBlack
	ColorWhite       = graphics.ColorWhite
	ColorRed         = graphics.ColorRed
	ColorGreen       = graphics.ColorGreen
	ColorBlue        = graphics.ColorBlue
	ColorYellow      = graphics.ColorYellow
	ColorCyan        = graphics.ColorCyan
	ColorMagenta     = graphics.ColorMagenta
	ColorGray        = graphics.ColorGray
	ColorLightGray   = graphics.ColorLightGray
	ColorDarkGray    = graphics.ColorDarkGray
)

// RGB 创建RGB颜色
func RGB(r, g, b uint8) Color {
	return graphics.RGB(r, g, b)
}

// RGBA 创建RGBA颜色
func RGBA(r, g, b, a uint8) Color {
	return graphics.RGBA(r, g, b, a)
}

// Point 点
type Point = graphics.Point

// NewPoint 创建新点
func NewPoint(x, y int32) Point {
	return graphics.NewPoint(x, y)
}

// Size 尺寸
type Size = graphics.Size

// NewSize 创建新尺寸
func NewSize(width, height int32) Size {
	return graphics.NewSize(width, height)
}

// Rect 矩形
type Rect = graphics.Rect

// NewRect 创建新矩形
func NewRect(x, y, width, height int32) Rect {
	return graphics.NewRect(x, y, width, height)
}

// Canvas 画布
type Canvas = render.Canvas

// NewCanvas 创建指定大小的画布
func NewCanvas(width, height int32) *Canvas {
	return render.NewCanvas(width, height)
}

// Renderer 渲染器
type Renderer = render.Renderer

// ============ 组件相关导出 ============

// Component 组件接口
type Component = component.Component

// Focusable 可接收键盘焦点的组件
type Focusable = component.Focusable

// RequestFocus 将键盘焦点独占交给 c，并清除其它可聚焦组件的焦点
func RequestFocus(c Component) {
	component.RequestFocus(c)
}

// BaseComponent 基础组件实现
type BaseComponent = component.BaseComponent

// NewBaseComponent 创建基础组件
func NewBaseComponent() BaseComponent {
	return component.NewBaseComponent()
}

// Container 容器接口
type Container = component.Container

// Button 按钮组件
type Button = component.Button

// Label 标签组件
type Label = component.Label

// Panel 面板组件
type Panel = component.Panel

// Font 字体
type Font = component.Font

// RegisterMemoryFont 零 CGO 从内存二进制字节流动态注册私有字体到当前进程，并自动解析返回其 FontFamily 名称
func RegisterMemoryFont(fontData []byte) (string, error) {
	return render.RegisterMemoryFont(fontData)
}

// RegisterMemoryFontWithName 使用显式指定的 FontFamily 名称注册内存字体
func RegisterMemoryFontWithName(fontData []byte, name string) (string, error) {
	return render.RegisterMemoryFontWithName(fontData, name)
}

// SetDefaultFont 设置全局默认字体规格。
// 原地更新 graphics.DefaultFont，component/goui 暴露的默认字体别名随即可见。
func SetDefaultFont(family string, size int32) {
	graphics.SetDefaultFont(family, size)
}

// Layout 布局接口
type Layout = component.Layout

// FlowLayout 流式布局
type FlowLayout = component.FlowLayout

// GridLayout 网格布局
type GridLayout = component.GridLayout

// BorderLayout 边界布局
type BorderLayout = component.BorderLayout

// TextAlign 文本对齐
type TextAlign = component.TextAlign

// VerticalAlign 垂直对齐
type VerticalAlign = component.VerticalAlign

// 文本对齐常量
const (
	AlignLeft   = component.AlignLeft
	AlignCenter = component.AlignCenter
	AlignRight  = component.AlignRight
)

// 垂直对齐常量
const (
	AlignTop    = component.AlignTop
	AlignMiddle = component.AlignMiddle
	AlignBottom = component.AlignBottom
)

// 水平对齐常量（用于布局）
const (
	HAlignStart  = component.HAlignStart
	HAlignCenter = component.HAlignCenter
	HAlignEnd    = component.HAlignEnd
)

// Themeable 可适配主题接口
type Themeable = component.Themeable

// PanelRole 面板语义角色
type PanelRole = component.PanelRole

const (
	PanelRoleNone           = component.PanelRoleNone
	PanelRoleBackground     = component.PanelRoleBackground
	PanelRoleSurface        = component.PanelRoleSurface
	PanelRoleSurfaceVariant = component.PanelRoleSurfaceVariant
)

// NewButton 创建按钮
func NewButton(text string) *Button {
	return component.NewButton(text)
}

// NewPrimaryButton 创建主强调色按钮（自动适配主题 Primary 品牌强调色）
func NewPrimaryButton(text string) *Button {
	return component.NewPrimaryButton(text)
}

// NewLabel 创建标签
func NewLabel(text string) *Label {
	return component.NewLabel(text)
}

// NewSecondaryLabel 创建次要说明标签（自动适配主题 TextSecondary 色彩）
func NewSecondaryLabel(text string) *Label {
	return component.NewSecondaryLabel(text)
}

// NewBrandLabel 创建品牌强调色标题标签（自动适配主题 Primary 品牌色彩）
func NewBrandLabel(text string) *Label {
	return component.NewBrandLabel(text)
}

// NewPanel 创建普通面板
func NewPanel() *Panel {
	return component.NewPanel()
}

// NewBackgroundPanel 创建背景面板（自动适配主题 Background 底色）
func NewBackgroundPanel() *Panel {
	return component.NewBackgroundPanel()
}

// NewSurfacePanel 创建卡片容器面板（自动适配主题 Surface 底色与 Border 边框）
func NewSurfacePanel() *Panel {
	return component.NewSurfacePanel()
}

// NewPanelWithRole 按角色创建面板
func NewPanelWithRole(role PanelRole) *Panel {
	return component.NewPanelWithRole(role)
}

// Menu 自绘菜单
type Menu = component.Menu

// MenuEntry 自绘菜单项
type MenuEntry = component.MenuEntry

// MenuBar 全客户区自绘菜单层
type MenuBar = component.MenuBar

// NewMenuBar 创建全客户区自绘菜单层
func NewMenuBar() *MenuBar {
	return component.NewMenuBar()
}

// Switch 开关控件
type Switch = component.Switch

// NewSwitch 创建开关控件
func NewSwitch() *Switch {
	return component.NewSwitch()
}

// Segmented 横向分段选择器
type Segmented = component.Segmented

// NewSegmented 创建横向分段选择器
func NewSegmented(options []string) *Segmented {
	return component.NewSegmented(options)
}

// TabBar 纵向分组导航
type TabBar = component.TabBar

// NewTabBar 创建纵向分组导航
func NewTabBar(items []string) *TabBar {
	return component.NewTabBar(items)
}

// NewFont 创建字体
func NewFont(name string, size int32) *Font {
	return component.NewFont(name, size)
}

// NewFlowLayout 创建流式布局
func NewFlowLayout() *FlowLayout {
	return &FlowLayout{}
}

// NewGridLayout 创建网格布局
func NewGridLayout(rows, cols int) *GridLayout {
	return &GridLayout{Rows: rows, Cols: cols}
}

// DefaultFont 默认字体
var DefaultFont = component.DefaultFont

// ============ Phase3 组件导出 ============

// TextBox 文本框组件
type TextBox = component.TextBox

// CheckBox 复选框组件
type CheckBox = component.CheckBox

// RadioButton 单选按钮组件
type RadioButton = component.RadioButton

// RadioGroup 单选按钮组
type RadioGroup = component.RadioGroup

// NewTextBox 创建文本框
func NewTextBox() *TextBox {
	return component.NewTextBox()
}

// NewCheckBox 创建复选框
func NewCheckBox(text string) *CheckBox {
	return component.NewCheckBox(text)
}

// NewRadioButton 创建单选按钮
func NewRadioButton(text string) *RadioButton {
	return component.NewRadioButton(text)
}

// NewRadioGroup 创建单选按钮组
func NewRadioGroup() *RadioGroup {
	return component.NewRadioGroup()
}

// GetClipboardText 获取系统剪贴板文本
func GetClipboardText() string {
	return clipboard.GetText()
}

// SetClipboardText 向系统剪贴板写入文本
func SetClipboardText(text string) bool {
	return clipboard.SetText(text)
}

// ============ Phase4 组件与主题导出 ============

// Theme 主题定义
type Theme = theme.Theme

// ThemeDark 返回默认极客暗色主题
func ThemeDark() *Theme {
	return theme.Dark()
}

// ThemeLight 返回默认清爽亮色主题
func ThemeLight() *Theme {
	return theme.Light()
}

// Themes 返回全部内置主题，顺序与菜单/设置页一致。
func Themes() []*Theme {
	return theme.All()
}

// ThemeByID 按持久化标识查找内置主题；未知标识返回 nil。
func ThemeByID(id string) *Theme {
	return theme.ByID(id)
}

// CurrentTheme 获取当前全局生效的主题
func CurrentTheme() *Theme {
	return theme.Current()
}

// SetTheme 切换全局生效的主题，并自动通知所有已打开窗口与组件树更新样式
func SetTheme(t *Theme) {
	theme.SetCurrent(t)
	window.ApplyThemeToAllWindows(t)
}

// Image 软件像素图像
type Image = render.Image

// NewImage 创建指定宽高的新空白图像
func NewImage(width, height int32) *Image {
	return render.NewImage(width, height)
}

// LoadImageFromFile 从本地文件加载并解码图像
func LoadImageFromFile(path string) (*Image, error) {
	return render.LoadImageFromFile(path)
}

// LoadImageFromBytes 从内存字节切片解码图像
func LoadImageFromBytes(data []byte) (*Image, error) {
	return render.LoadImageFromBytes(data)
}

// ProgressBar 进度条组件
type ProgressBar = component.ProgressBar

// NewProgressBar 创建进度条
func NewProgressBar() *ProgressBar {
	return component.NewProgressBar()
}

// Slider 滑动条组件
type Slider = component.Slider

// NewSlider 创建滑动条
func NewSlider(min, max, value float64) *Slider {
	return component.NewSlider(min, max, value)
}

// ImageView 图像展示组件
type ImageView = component.ImageView

// ImageScaleMode 图像缩放模式
type ImageScaleMode = component.ImageScaleMode

const (
	ScaleStretch = component.ScaleStretch
	ScaleFit     = component.ScaleFit
	ScaleFill    = component.ScaleFill
	ScaleCenter  = component.ScaleCenter
)

// NewImageView 创建图像控件
func NewImageView(img *Image) *ImageView {
	return component.NewImageView(img)
}

// NewImageViewFromFile 从本地文件创建图像控件
func NewImageViewFromFile(path string) (*ImageView, error) {
	return component.NewImageViewFromFile(path)
}

// ScrollView 滚动视图容器组件
type ScrollView = component.ScrollView

// NewScrollView 创建滚动视图
func NewScrollView() *ScrollView {
	return component.NewScrollView()
}

// TextArea 多行文本编辑器组件
type TextArea = component.TextArea

// Match 一次查找匹配区间
type Match = component.Match

// NewTextArea 创建多行文本编辑器
func NewTextArea() *TextArea {
	return component.NewTextArea()
}

// ============ 语法高亮体系导出 ============

// Highlighter 语法高亮器接口
type Highlighter = syntax.Highlighter

// HighlightSpan 语法高亮样式片段
type HighlightSpan = syntax.Span

// Span 语法高亮样式片段别名
type Span = syntax.Span

// MarkdownHighlighter 基础 Markdown 语法高亮器
type MarkdownHighlighter = syntax.MarkdownHighlighter

// MarkdownColors Markdown 调色板令牌规范
type MarkdownColors = syntax.MarkdownColors

// JSONHighlighter JSON 语法高亮器
type JSONHighlighter = syntax.JSONHighlighter

// JSONColors JSON 调色板令牌规范
type JSONColors = syntax.JSONColors

// YAMLHighlighter YAML 语法高亮器
type YAMLHighlighter = syntax.YAMLHighlighter

// YAMLColors YAML 调色板令牌规范
type YAMLColors = syntax.YAMLColors

// TOMLHighlighter TOML 语法高亮器
type TOMLHighlighter = syntax.TOMLHighlighter

// TOMLColors TOML 调色板令牌规范
type TOMLColors = syntax.TOMLColors

// GoHighlighter Go 语言语法高亮器
type GoHighlighter = syntax.GoHighlighter

// GoColors Go 语言调色板令牌规范
type GoColors = syntax.GoColors

// PythonHighlighter Python 语法高亮器
type PythonHighlighter = syntax.PythonHighlighter

// PythonColors Python 调色板令牌规范
type PythonColors = syntax.PythonColors

// CHighlighter C/C++ 语法高亮器
type CHighlighter = syntax.CHighlighter

// CColors C/C++ 调色板令牌规范
type CColors = syntax.CColors

// JavaHighlighter Java 语法高亮器
type JavaHighlighter = syntax.JavaHighlighter

// JavaColors Java 调色板令牌规范
type JavaColors = syntax.JavaColors

// JSHighlighter JavaScript 语法高亮器
type JSHighlighter = syntax.JSHighlighter

// JSColors JavaScript 调色板令牌规范
type JSColors = syntax.JSColors

// TypeScriptHighlighter TypeScript 语法高亮器
type TypeScriptHighlighter = syntax.TypeScriptHighlighter

// TypoScriptHighlighter TypoScript 语法高亮器
type TypoScriptHighlighter = syntax.TypoScriptHighlighter

// TSColors TypeScript 调色板令牌规范
type TSColors = syntax.TSColors

// HTMLHighlighter HTML 语法高亮器
type HTMLHighlighter = syntax.HTMLHighlighter

// HTMLColors HTML 调色板令牌规范
type HTMLColors = syntax.HTMLColors

// CSSHighlighter CSS 语法高亮器
type CSSHighlighter = syntax.CSSHighlighter

// CSSColors CSS 调色板令牌规范
type CSSColors = syntax.CSSColors

// CSharpHighlighter C# 语法高亮器
type CSharpHighlighter = syntax.CSharpHighlighter

// CSharpColors C# 调色板令牌规范
type CSharpColors = syntax.CSharpColors

// SQLHighlighter SQL 语法高亮器
type SQLHighlighter = syntax.SQLHighlighter

// SQLColors SQL 调色板令牌规范
type SQLColors = syntax.SQLColors

// RustHighlighter Rust 语法高亮器
type RustHighlighter = syntax.RustHighlighter

// RustColors Rust 调色板令牌规范
type RustColors = syntax.RustColors

// RHighlighter R 语言语法高亮器
type RHighlighter = syntax.RHighlighter

// RColors R 语言调色板令牌规范
type RColors = syntax.RColors

// NewGoHighlighter 创建 Go 语言语法高亮器
func NewGoHighlighter() *GoHighlighter {
	return syntax.NewGoHighlighter()
}

// NewPythonHighlighter 创建 Python 语法高亮器
func NewPythonHighlighter() *PythonHighlighter {
	return syntax.NewPythonHighlighter()
}

// NewCHighlighter 创建 C/C++ 语法高亮器
func NewCHighlighter() *CHighlighter {
	return syntax.NewCHighlighter()
}

// NewCSharpHighlighter 创建 C# 语法高亮器
func NewCSharpHighlighter() *CSharpHighlighter {
	return syntax.NewCSharpHighlighter()
}

// NewJavaHighlighter 创建 Java 语法高亮器
func NewJavaHighlighter() *JavaHighlighter {
	return syntax.NewJavaHighlighter()
}

// NewRustHighlighter 创建 Rust 语法高亮器
func NewRustHighlighter() *RustHighlighter {
	return syntax.NewRustHighlighter()
}

// NewRHighlighter 创建 R 语言语法高亮器
func NewRHighlighter() *RHighlighter {
	return syntax.NewRHighlighter()
}

// NewJSHighlighter 创建 JavaScript 语法高亮器
func NewJSHighlighter() *JSHighlighter {
	return syntax.NewJSHighlighter()
}

// NewTypeScriptHighlighter 创建 TypeScript 语法高亮器
func NewTypeScriptHighlighter() *TypeScriptHighlighter {
	return syntax.NewTypeScriptHighlighter()
}

// NewTypoScriptHighlighter 创建 TypoScript 语法高亮器
func NewTypoScriptHighlighter() *TypoScriptHighlighter {
	return syntax.NewTypoScriptHighlighter()
}

// NewHTMLHighlighter 创建 HTML 语法高亮器
func NewHTMLHighlighter() *HTMLHighlighter {
	return syntax.NewHTMLHighlighter()
}

// NewCSSHighlighter 创建 CSS 语法高亮器
func NewCSSHighlighter() *CSSHighlighter {
	return syntax.NewCSSHighlighter()
}

// NewSQLHighlighter 创建 SQL 语法高亮器
func NewSQLHighlighter() *SQLHighlighter {
	return syntax.NewSQLHighlighter()
}

// NewMarkdownHighlighter 创建 Markdown 语法高亮器
func NewMarkdownHighlighter() *MarkdownHighlighter {
	return syntax.NewMarkdownHighlighter()
}

// NewJSONHighlighter 创建 JSON 语法高亮器
func NewJSONHighlighter() *JSONHighlighter {
	return syntax.NewJSONHighlighter()
}

// NewYAMLHighlighter 创建 YAML 语法高亮器
func NewYAMLHighlighter() *YAMLHighlighter {
	return syntax.NewYAMLHighlighter()
}

// NewTOMLHighlighter 创建 TOML 语法高亮器
func NewTOMLHighlighter() *TOMLHighlighter {
	return syntax.NewTOMLHighlighter()
}

// DetectHighlighter 根据文件名或后缀名自动匹配并创建对应语法高亮器
func DetectHighlighter(pathOrExt string) Highlighter {
	return syntax.DetectHighlighter(pathOrExt)
}

// GetLanguageName 获取高亮器实例对应的友好语言名称
func GetLanguageName(h Highlighter) string {
	return syntax.GetLanguageName(h)
}

// ============ 原生文件与消息对话框导出 ============

// OpenFileDialog 打开文件对话框
func OpenFileDialog(hwnd uintptr, title, filter, defaultExt string) (string, bool) {
	return dialog.OpenFileDialog(hwnd, title, filter, defaultExt)
}

// SaveFileDialog 另存为文件对话框
func SaveFileDialog(hwnd uintptr, title, filter, defaultExt, defaultName string) (string, bool) {
	return dialog.SaveFileDialog(hwnd, title, filter, defaultExt, defaultName)
}

// ShowMessageBox 弹出通用消息提示框
func ShowMessageBox(hwnd uintptr, title, text string, style uint32) int {
	return dialog.MessageBox(hwnd, title, text, style)
}

// ShowInfo 弹出信息框
func ShowInfo(hwnd uintptr, title, text string) {
	dialog.ShowInfo(hwnd, title, text)
}

// ShowWarning 弹出警告框
func ShowWarning(hwnd uintptr, title, text string) {
	dialog.ShowWarning(hwnd, title, text)
}

// ShowError 弹出错误框
func ShowError(hwnd uintptr, title, text string) {
	dialog.ShowError(hwnd, title, text)
}

// MenuItem 原生弹出菜单项
type MenuItem = dialog.MenuItem

// ShowPopupMenu 在窗口客户区坐标处弹出原生上下文菜单，返回选中项 ID（0 表示取消）
func ShowPopupMenu(hwnd uintptr, x, y int32, items []MenuItem) uint32 {
	return dialog.ShowPopupMenu(hwnd, x, y, items)
}

// ShowConfirmYesNoCancel 弹出“是否保存”询问对话框 (返回 IDYES, IDNO, IDCANCEL)
func ShowConfirmYesNoCancel(hwnd uintptr, title, text string) int {
	return dialog.ShowConfirmYesNoCancel(hwnd, title, text)
}

// 对话框返回常量
const (
	IDOK     = dialog.IDOK
	IDCANCEL = dialog.IDCANCEL
	IDYES    = dialog.IDYES
	IDNO     = dialog.IDNO
)
