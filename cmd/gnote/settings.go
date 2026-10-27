//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	goui "github.com/jacksalad/gnote"
)

const settingsVersion = 1

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

// LoadSettings 从默认路径加载设置；文件不存在或损坏时回退默认值。
func LoadSettings() Settings {
	s, err := loadSettingsFrom(settingsPath())
	if err != nil {
		return DefaultSettings()
	}
	return s
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
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings(), err
	}
	return s.Normalized(), nil
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

// SaveSettings 将当前偏好写入默认配置路径。
func (a *App) SaveSettings() error {
	s := a.CurrentSettings()
	a.settings = s
	return saveSettingsTo(a.settingsFilePath(), s)
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

	a.highlightEnabled = s.Highlight
	if a.highlightEnabled {
		if a.currentHighlighter == nil {
			a.currentHighlighter = goui.DetectHighlighter(a.currentFileName)
			if a.currentHighlighter == nil {
				a.currentHighlighter = goui.NewMarkdownHighlighter()
			}
		}
		a.textArea.SetHighlighter(a.currentHighlighter)
	} else {
		a.textArea.SetHighlighter(nil)
	}

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
