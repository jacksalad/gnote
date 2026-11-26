//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	goui "github.com/jacksalad/gnote"
)

const settingsVersion = 1

// maxWindowSide 持久化窗口尺寸的上限：超过任何现实显示器的数字只可能是手误。
// 窗口层创建时还会再按工作区裁剪，这里只保证不会把离谱的值原样存下来。
const maxWindowSide = 16384

// 启动页面选项：显示软件介绍文档，或直接打开空白页。
const (
	startupPageWelcome = "welcome"
	startupPageBlank   = "blank"
)

// Settings 持久化的用户偏好。文档级状态（编码、EOL、当前路径）不在此列，
// 避免设置页静默改写当前文件。
type Settings struct {
	Version      int      `json:"version"`
	Theme        string   `json:"theme"` // 主题标识: dark | light | nord | tokyo-night | ...
	BaseFontSize int32    `json:"baseFontSize"`
	Zoom         float64  `json:"zoom"` // 0 = 跟随系统 DPI；>0 = 固定缩放
	WordWrap     bool     `json:"wordWrap"`
	LineNumbers  bool     `json:"lineNumbers"`
	Highlight    bool     `json:"highlight"`
	StartupPage  string   `json:"startupPage"` // welcome = 介绍文档；blank = 空白页
	RecentFiles  []string `json:"recentFiles,omitempty"`
	WindowWidth  int32    `json:"windowWidth"`
	WindowHeight int32    `json:"windowHeight"`
}

// DefaultSettings 返回首次启动的默认设置。
func DefaultSettings() Settings {
	return Settings{
		Version:      settingsVersion,
		Theme:        "dark",
		BaseFontSize: 16,
		Zoom:         0,
		WordWrap:     true,
		LineNumbers:  true,
		Highlight:    true,
		StartupPage:  startupPageWelcome,
		WindowWidth:  1560,
		WindowHeight: 920,
	}
}

// Normalized 返回经过范围校验与去重的设置副本。
func (s Settings) Normalized() Settings {
	if s.Version <= 0 {
		s.Version = settingsVersion
	}
	s.Theme = strings.ToLower(strings.TrimSpace(s.Theme))
	if th := goui.ThemeByID(s.Theme); th != nil {
		s.Theme = th.ID
	} else {
		s.Theme = "dark"
	}

	if s.BaseFontSize == 0 {
		s.BaseFontSize = 16
	}
	if s.BaseFontSize < 12 {
		s.BaseFontSize = 12
	}
	if s.BaseFontSize > 32 {
		s.BaseFontSize = 32
	}

	if s.Zoom < 0 {
		s.Zoom = 0
	}
	if s.Zoom > 0 && s.Zoom < 0.5 {
		s.Zoom = 0.5
	}
	if s.Zoom > 3.0 {
		s.Zoom = 3.0
	}

	// 旧版设置没有该字段：空值一律视为默认的介绍文档。
	s.StartupPage = strings.ToLower(strings.TrimSpace(s.StartupPage))
	if s.StartupPage != startupPageBlank {
		s.StartupPage = startupPageWelcome
	}

	if s.WindowWidth == 0 {
		s.WindowWidth = 1560
	}
	if s.WindowWidth < 640 {
		s.WindowWidth = 640
	}
	if s.WindowHeight == 0 {
		s.WindowHeight = 920
	}
	if s.WindowHeight < 400 {
		s.WindowHeight = 400
	}
	if s.WindowWidth > maxWindowSide {
		s.WindowWidth = maxWindowSide
	}
	if s.WindowHeight > maxWindowSide {
		s.WindowHeight = maxWindowSide
	}

	seen := make(map[string]struct{}, len(s.RecentFiles))
	recent := s.RecentFiles[:0]
	for _, path := range s.RecentFiles {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		recent = append(recent, path)
		if len(recent) == 10 {
			break
		}
	}
	s.RecentFiles = recent
	return s
}

// settingsPath 返回配置文件路径：
//   - exe 同目录存在 gnote.portable 时使用 exe/settings.json（绿色便携模式）
//   - 否则使用 %AppData%\gnote\settings.json
func settingsPath() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "gnote.portable")); err == nil {
			return filepath.Join(dir, "settings.json")
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "gnote", "settings.json")
}

// LoadSettings 从默认路径加载设置；文件不存在时用默认值。
// 文件存在却读不出来（JSON 语法错误、被占用、无权限）时返回默认值和错误：
// 调用方要记住这个错误（App.settingsLoadFailed），退出时不能拿默认值覆盖用户的配置文件。
func LoadSettings() (Settings, error) {
	return loadSettingsFrom(settingsPath())
}

// loadSettingsFrom 从显式路径加载。缺失文件返回默认值且不报错，便于首次启动。
func loadSettingsFrom(path string) (Settings, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return DefaultSettings(), err
	}
	return parseSettings(data)
}

// parseSettings 宽松地解析配置文件：
//   - 去掉 UTF-8 BOM（Windows PowerShell 5.1 的 Set-Content -Encoding UTF8 会写入）；
//   - 逐字段读取，某个字段类型不对只让它自己回退默认值，不牵连其它字段；
//   - 数字接受 18、18.0、2e1、"18" 这些写法；不认识的键忽略。
//
// 只有 JSON 语法本身有错（多一个逗号、被截断）才整体报错。
func parseSettings(data []byte) (Settings, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return DefaultSettings(), err
	}
	s := DefaultSettings()
	settingString(raw, "theme", &s.Theme)
	settingString(raw, "startupPage", &s.StartupPage)
	if v, ok := settingInt(raw, "version"); ok {
		s.Version = int(v)
	}
	if v, ok := settingInt(raw, "baseFontSize"); ok {
		s.BaseFontSize = v
	}
	if v, ok := settingInt(raw, "windowWidth"); ok {
		s.WindowWidth = v
	}
	if v, ok := settingInt(raw, "windowHeight"); ok {
		s.WindowHeight = v
	}
	settingFloat(raw, "zoom", &s.Zoom)
	settingBool(raw, "wordWrap", &s.WordWrap)
	settingBool(raw, "lineNumbers", &s.LineNumbers)
	settingBool(raw, "highlight", &s.Highlight)
	if recent, ok := settingStrings(raw, "recentFiles"); ok {
		s.RecentFiles = recent
	}
	return s.Normalized(), nil
}

// settingNumber 取出数字字段：JSON 数字（整数、小数、指数）或带引号的数字都接受。
func settingNumber(raw map[string]json.RawMessage, key string) (float64, bool) {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return 0, false
	}
	var n float64
	if err := json.Unmarshal(v, &n); err == nil {
		return n, true
	}
	var quoted string
	if json.Unmarshal(v, &quoted) != nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(quoted), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// settingInt 取出整数字段；放不进 int32 的值（1e30、-1e30）当作没写，保留默认值。
func settingInt(raw map[string]json.RawMessage, key string) (int32, bool) {
	n, ok := settingNumber(raw, key)
	if !ok || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, false
	}
	return int32(math.Round(n)), true
}

func settingFloat(raw map[string]json.RawMessage, key string, dst *float64) {
	if n, ok := settingNumber(raw, key); ok {
		*dst = n
	}
}

func settingBool(raw map[string]json.RawMessage, key string, dst *bool) {
	v, ok := raw[key]
	if !ok {
		return
	}
	if err := json.Unmarshal(v, dst); err == nil {
		return
	}
	var quoted string
	if json.Unmarshal(v, &quoted) == nil {
		switch strings.ToLower(strings.TrimSpace(quoted)) {
		case "true":
			*dst = true
		case "false":
			*dst = false
		}
	}
}

func settingString(raw map[string]json.RawMessage, key string, dst *string) {
	v, ok := raw[key]
	if ok {
		_ = json.Unmarshal(v, dst)
	}
}

// settingStrings 取出字符串数组；读不出或者元素不是字符串的，一律跳过。
func settingStrings(raw map[string]json.RawMessage, key string) ([]string, bool) {
	v, ok := raw[key]
	if !ok {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(v, &items); err != nil {
		return nil, false
	}
	var out []string
	for _, item := range items {
		var s string
		if json.Unmarshal(item, &s) == nil {
			out = append(out, s)
		}
	}
	return out, true
}

// saveSettingsTo 原子写入显式路径，便于测试与便携模式复用。
func saveSettingsTo(path string, s Settings) error {
	s = s.Normalized()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// CurrentSettings 汇总 App 当前生效的偏好，用于设置窗口或退出时保存。
func (a *App) CurrentSettings() Settings {
	s := a.settings
	s.Version = settingsVersion
	s.Theme = a.themeID
	s.BaseFontSize = a.textArea.GetBaseFontSize()
	s.WordWrap = a.textArea.IsWordWrap()
	s.LineNumbers = a.textArea.IsShowLineNumbers()
	s.Highlight = a.highlightEnabled
	s.Zoom = 0
	if !a.autoZoom {
		s.Zoom = a.textArea.GetZoom()
	}
	if a.win != nil {
		s.WindowWidth, s.WindowHeight = a.win.GetSize()
	}
	return s.Normalized()
}

// errSettingsDocDirty 配置文件正在编辑器里修改且尚未保存，不能由内存回写覆盖它。
var errSettingsDocDirty = errors.New("配置文件正在编辑器中修改且尚未保存，无法写入设置。\n请先保存或放弃对该文档的修改。")

// SaveSettings 将当前偏好写入默认配置路径，供用户明确要求保存时调用（设置窗口的 “应用”）。
//   - 配置文件损坏（读不出来）：不覆盖它，如实返回错误；
//   - 配置文件正作为文档打开：编辑器里的内容才是事实来源。文档有未保存修改时拒绝写入；
//     文档干净时写盘并重新载入文档，保证编辑器显示的就是文件现状。
func (a *App) SaveSettings() error {
	if err := a.checkSettingsReadable(); err != nil {
		return err
	}
	s := a.CurrentSettings()
	if a.isSettingsDocument() {
		if a.textArea.IsModified() {
			return errSettingsDocDirty
		}
		if err := saveSettingsTo(a.settingsFilePath(), s); err != nil {
			return err
		}
		a.settings = s.Normalized()
		a.LoadFile(a.settingsFilePath())
		return nil
	}
	a.settings = s
	return saveSettingsTo(a.settingsFilePath(), s)
}

// saveSettingsImplicit 退出或 “打开配置文件” 之前的隐式保存。
// 配置文件正作为文档打开时什么都不写：内存里的值可能比文件旧，回写会冲掉用户在编辑器里改好并保存的内容。
func (a *App) saveSettingsImplicit() error {
	if a.isSettingsDocument() {
		return nil
	}
	return a.SaveSettings()
}

// checkSettingsReadable 上次启动没读出配置文件时，先确认用户是否已经把它修好；没修好就不覆盖。
func (a *App) checkSettingsReadable() error {
	if !a.settingsLoadFailed {
		return nil
	}
	if _, err := loadSettingsFrom(a.settingsFilePath()); err != nil {
		return fmt.Errorf("配置文件不是有效的 JSON，为避免覆盖已保留原文件。\n请通过 “设置 → 打开配置文件” 修复后再保存。")
	}
	a.settingsLoadFailed = false
	return nil
}

// isSettingsDocument 当前文档是否就是配置文件本身。
func (a *App) isSettingsDocument() bool {
	return a.currentFilePath != "" && sameFilePath(a.currentFilePath, a.settingsFilePath())
}

// reloadSettingsFromDisk 配置文件刚被编辑器里的文档保存：让会话跟随文件内容。
// 主题、基准字号、缩放、自动换行、行号与高亮偏好立即生效；窗口尺寸不改动当前窗口，
// 启动页在下次启动生效——这两项只记录在内存里，留待之后保存。
func (a *App) reloadSettingsFromDisk() {
	s, err := loadSettingsFrom(a.settingsFilePath())
	if err != nil {
		// 文档被改坏了（例如少了一个括号）：保持现有设置，也不要在退出时覆盖它。
		a.settingsLoadFailed = true
		return
	}
	a.settingsLoadFailed = false
	a.ApplySettings(s)
}

func (a *App) settingsFilePath() string {
	if a.settingsPathOverride != "" {
		return a.settingsPathOverride
	}
	return settingsPath()
}

// ApplySettings 应用一份设置并刷新界面。文档内容、编码与 EOL 不受影响。
func (a *App) ApplySettings(s Settings) {
	s = s.Normalized()
	a.settings = s

	a.SetThemeID(s.Theme)

	a.textArea.SetBaseFontSize(s.BaseFontSize)
	a.textArea.SetWordWrap(s.WordWrap)
	a.textArea.SetShowLineNumbers(s.LineNumbers)

	// 只切换全局偏好；当前文档没有语言（纯文本、无后缀）时不会回退成 Markdown。
	a.highlightEnabled = s.Highlight
	a.applyDocumentHighlight()

	a.applyingSettings = true
	if s.Zoom <= 0 {
		a.autoZoom = true
		scale := 1.0
		if a.win != nil {
			scale = a.win.GetScale()
		}
		if scale > 1.05 {
			a.textArea.SetZoom(scale)
		} else {
			a.textArea.SetZoom(1)
		}
	} else {
		a.autoZoom = false
		a.textArea.SetZoom(s.Zoom)
	}
	a.applyingSettings = false

	if a.win != nil {
		a.UpdateHeaderAndStatus()
		a.win.RequestRedraw()
	}
}
