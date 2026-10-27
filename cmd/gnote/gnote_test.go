//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/textio"
)

func TestGnote_ComponentIntegration(t *testing.T) {
	// 测试 TextArea 在 gnote 中的典型集成链路
	ta := goui.NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText("Hello gnote!\r\nSecond Line\r\nThird Line")

	if ta.GetLineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", ta.GetLineCount())
	}

	// 验证光标起始位置 (1, 1)
	r, c := ta.GetCursorPosition()
	if r != 1 || c != 1 {
		t.Fatalf("expected cursor at (1, 1), got (%d, %d)", r, c)
	}

	// 验证行号开关切换
	if !ta.IsShowLineNumbers() {
		t.Fatalf("expected line numbers visible by default")
	}
	ta.SetShowLineNumbers(false)
	if ta.IsShowLineNumbers() {
		t.Fatalf("expected line numbers hidden")
	}

	// 验证撤销栈状态
	ta.SetText("Line 1")
	if ta.IsModified() {
		t.Fatalf("expected modified to be reset on SetText")
	}
}

func TestGnote_FullLayoutAndRender(t *testing.T) {
	// 完整测试主界面组件树组装与光栅化渲染
	toolbar := goui.NewSurfacePanel()
	toolbar.SetBounds(0, 0, 960, 36)
	btn := goui.NewButton("测试按钮")
	btn.SetBounds(0, 0, 80, 26)
	toolbar.Add(btn)

	textArea := goui.NewTextArea()
	textArea.SetText("测试行号与正文内容\r\n第二行内容")

	statusBar := goui.NewSurfacePanel()
	statusBar.SetBounds(0, 0, 960, 28)
	lbl := goui.NewLabel("就绪")
	statusBar.Add(lbl)

	root := goui.NewBackgroundPanel()
	root.SetBounds(0, 0, 960, 640)
	root.SetLayout(&goui.BorderLayout{
		Top:    toolbar,
		Center: textArea,
		Bottom: statusBar,
		Gap:    0,
	})
	root.Add(toolbar)
	root.Add(textArea)
	root.Add(statusBar)

	// 执行布局计算
	root.DoLayout()

	// 验证各区域尺寸是否正确分发
	tbBounds := toolbar.GetBounds()
	if tbBounds.Height != 36 || tbBounds.Width != 960 {
		t.Fatalf("unexpected toolbar bounds: %+v", tbBounds)
	}

	sbBounds := statusBar.GetBounds()
	if sbBounds.Height != 28 || sbBounds.Width != 960 {
		t.Fatalf("unexpected statusBar bounds: %+v", sbBounds)
	}

	taBounds := textArea.GetBounds()
	expectedTaH := int32(640 - 36 - 28)
	if taBounds.Height != expectedTaH || taBounds.Width != 960 {
		t.Fatalf("unexpected textArea bounds: %+v, expected height %d", taBounds, expectedTaH)
	}

	// 执行完整画布渲染，确保无越界崩溃并正常产生有效像素
	canvas := render.NewCanvas(960, 640)
	canvas.Clear(goui.CurrentTheme().Background)
	root.Render(canvas)

	// 验证画布并非全黑/全底色（必须包含按钮、文字、行号等不同像素值）
	hasDifferentPixels := false
	bgPixel := canvas.Buffer[0]
	for _, px := range canvas.Buffer {
		if px != bgPixel {
			hasDifferentPixels = true
			break
		}
	}

	if !hasDifferentPixels {
		t.Fatalf("canvas should have non-background pixels after rendering components")
	}
}

func TestGnote_FirstClickResponsiveness(t *testing.T) {
	// 验证当文本编辑区聚焦时，点击工具栏按钮能够第 1 次点击立即响应，不被失焦事件截断
	toolbar := goui.NewSurfacePanel()
	toolbar.SetBounds(0, 0, 960, 36)
	clickedCount := 0
	btn := goui.NewButton("新建")
	btn.SetBounds(10, 5, 80, 26)
	btn.OnClick(func() {
		clickedCount++
	})
	toolbar.Add(btn)

	textArea := goui.NewTextArea()
	textArea.SetBounds(0, 36, 960, 576)
	// 默认让文本框处于聚焦输入状态
	textArea.SetFocused(true)

	statusBar := goui.NewSurfacePanel()
	statusBar.SetBounds(0, 612, 960, 28)

	root := goui.NewBackgroundPanel()
	root.SetBounds(0, 0, 960, 640)
	root.Add(toolbar)
	root.Add(textArea)
	root.Add(statusBar)

	// 模拟用户鼠标点击按钮 (X=20, Y=10)
	downEvt := event.NewEvent(event.EventMouseDown, event.MouseData{
		X:      20,
		Y:      10,
		Button: event.MouseButtonLeft,
	})
	consumed := root.OnEvent(downEvt)
	if !consumed {
		t.Fatalf("expected EventMouseDown to be consumed by button")
	}
	if !btn.IsPressed() {
		t.Fatalf("expected button to be pressed on first click down")
	}
	if !textArea.IsFocused() {
		t.Fatalf("editor must keep focus after clicking the toolbar, otherwise typing and Ctrl+Z stop working")
	}

	// 模拟鼠标抬起
	upEvt := event.NewEvent(event.EventMouseUp, event.MouseData{
		X:      20,
		Y:      10,
		Button: event.MouseButtonLeft,
	})
	root.OnEvent(upEvt)

	// 验证：第 1 次点击必须立即触发 onClick
	if clickedCount != 1 {
		t.Fatalf("expected onClick to be fired on 1st click, but got count = %d", clickedCount)
	}

	// 关键稳定性回归：点击工具栏后键盘输入必须仍然生效（不能出现"键盘失灵"）
	if !textArea.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'K'})) {
		t.Fatalf("expected the editor to keep consuming key presses after a toolbar click")
	}
	if got := textArea.GetText(); got != "K" {
		t.Fatalf("expected typed character to be inserted after toolbar click, got %q", got)
	}
}

func TestGnote_CloseConfirmModifiedLogic(t *testing.T) {
	ta := goui.NewTextArea()
	ta.SetText("原始文本内容")

	// 初始状态下未被修改
	if ta.IsModified() {
		t.Fatalf("newly loaded text should NOT be modified")
	}

	// 模拟未修改时关闭：直接允许关闭
	mockCheckSave := func(isMod bool, userChoice int) bool {
		if !isMod {
			return true // 未修改，直接放行关闭
		}
		switch userChoice {
		case goui.IDYES:
			return true // 模拟保存成功后关闭
		case goui.IDNO:
			return true // 不保存直接关闭
		case goui.IDCANCEL:
			return false // 取消关闭，留在界面
		}
		return false
	}

	if !mockCheckSave(ta.IsModified(), goui.IDCANCEL) {
		t.Fatalf("expected clean document to close without prompt/interruption")
	}

	// 模拟用户键入新字符引发修改
	ta.SetFocused(true)
	charEvt := event.NewEvent(event.EventKeyPress, event.KeyData{
		Char: 'A',
	})
	ta.OnEvent(charEvt)

	if !ta.IsModified() {
		t.Fatalf("expected textArea to be modified after user typed")
	}

	// 此时尝试关闭：
	// 1. 若用户选择取消 (IDCANCEL)，必须阻止关闭
	if mockCheckSave(ta.IsModified(), goui.IDCANCEL) {
		t.Fatalf("expected cancel to prevent window close")
	}

	// 2. 若用户选择否 (IDNO)，允许关闭
	if !mockCheckSave(ta.IsModified(), goui.IDNO) {
		t.Fatalf("expected IDNO to allow window close")
	}

	// 3. 若用户选择是 (IDYES)，保存后允许关闭
	if !mockCheckSave(ta.IsModified(), goui.IDYES) {
		t.Fatalf("expected IDYES to allow window close upon save")
	}
}

func TestParseTargetFileFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "无参数",
			args:     []string{},
			expected: "",
		},
		{
			name:     "单个文件路径",
			args:     []string{`C:\notes\test.txt`},
			expected: `C:\notes\test.txt`,
		},
		{
			name:     "带双引号的路径",
			args:     []string{`"D:\My Documents\notes.md"`},
			expected: `D:\My Documents\notes.md`,
		},
		{
			name:     "带空格与前后空格的路径",
			args:     []string{`   "E:\code\main.go"   `},
			expected: `E:\code\main.go`,
		},
		{
			name:     "跳过前置命令行参数",
			args:     []string{"-v", "--debug", "/p", `C:\test\sample.py`},
			expected: `C:\test\sample.py`,
		},
		{
			name:     "全为空白或参数",
			args:     []string{"-v", "   ", "--all"},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTargetFileFromArgs(tc.args)
			if got != tc.expected {
				t.Fatalf("parseTargetFileFromArgs(%v) = %q, expected %q", tc.args, got, tc.expected)
			}
		})
	}
}

func TestFileLoadingAndStateUpdate(t *testing.T) {
	// 创建临时测试文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_sample.go")
	content := []byte("// package sample\r\npackage main\r\n\r\nfunc main() {}\r\n")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// 模拟加载文件
	ta := goui.NewTextArea()
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	text, _, err := textio.Decode(data)
	if err != nil {
		t.Fatalf("failed to decode test file: %v", err)
	}
	ta.SetText(text)
	ta.SetModified(false)

	// 验证文本内容与修改状态
	if ta.GetText() != string(content) {
		t.Fatalf("text mismatch: got %q, expected %q", ta.GetText(), string(content))
	}
	if ta.IsModified() {
		t.Fatalf("newly loaded text must not be marked modified")
	}

	// 验证语法高亮匹配
	h := goui.DetectHighlighter(testFile)
	if h == nil {
		t.Fatalf("expected Go highlighter to be detected for %s", testFile)
	}
	lang := goui.GetLanguageName(h)
	if lang != "Go" {
		t.Fatalf("expected language Go, got %s", lang)
	}
}

func TestWriteFileAtomic_ReplacesWithoutLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	newContent := []byte("new content\r\nsecond line")
	if err := writeFileAtomic(path, newContent); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(newContent) {
		t.Fatalf("content mismatch: got %q want %q", got, newContent)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "note.txt" {
		t.Fatalf("expected only the target file to remain, got %v", entries)
	}
}
