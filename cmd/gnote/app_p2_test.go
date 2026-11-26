//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jacksalad/gnote/dialog"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/textio"
)

// promptRecorder 记录保存前弹出的确认框并替用户回答；测试里绝不弹出原生 MessageBox。
type promptRecorder struct {
	prompts []string // 依次记录 “标题|正文”
	answer  int      // 模拟用户点击的按钮
}

func stubSavePrompt(t *testing.T, answer int) *promptRecorder {
	t.Helper()
	rec := &promptRecorder{answer: answer}
	old := savePrompt
	savePrompt = func(_ uintptr, title, text string) int {
		rec.prompts = append(rec.prompts, title+"|"+text)
		return rec.answer
	}
	t.Cleanup(func() { savePrompt = old })
	return rec
}

// rawBytes 拼接字符串与原始字节。Go 字面量里的 "\xff" 是码位 U+00FF（UTF-8 为 C3 BF），
// 要构造带坏字节的文件必须逐字节写出；未标类型的 0xFF 传进 any 时是 int。
func rawBytes(parts ...any) []byte {
	var out []byte
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			out = append(out, v...)
		case byte:
			out = append(out, v)
		case int:
			out = append(out, byte(v))
		case []byte:
			out = append(out, v...)
		}
	}
	return out
}

func writeFileBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

var utf8WithBadByte = rawBytes("第一行 hello\n第二行 world ", 0xFF, " tail\n")

// 载入时就无法逐字节还原的文件：原样 Ctrl+S 会改写用户没有见过的字节，必须先确认；取消后磁盘原封不动。
func TestApp_SaveAsksBeforeRewritingBytesLoadedLossy(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "corrupt.txt")
	writeFileBytes(t, path, utf8WithBadByte)
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}

	rec := stubSavePrompt(t, goui.IDCANCEL)
	if app.Save() {
		t.Fatal("save that rewrites unknown bytes went through after Cancel")
	}
	if len(rec.prompts) != 1 {
		t.Fatalf("prompts = %d, want exactly one", len(rec.prompts))
	}
	if got := readFileBytes(t, path); !bytes.Equal(got, utf8WithBadByte) {
		t.Errorf("cancelled save changed the file on disk")
	}

	stubSavePrompt(t, goui.IDYES)
	if !app.Save() {
		t.Fatal("save after Yes failed")
	}
	if got := readFileBytes(t, path); !bytes.Contains(got, []byte{0xEF, 0xBF, 0xBD}) {
		t.Errorf("confirmed save should write U+FFFD in place of the bad byte, got %q", got)
	}

	// 用户确认过一次之后，文件里已经是他见过的内容，之后的保存不再询问。
	rec = stubSavePrompt(t, goui.IDCANCEL)
	if !app.Save() || len(rec.prompts) != 0 {
		t.Errorf("later saves must not ask again: ok=%v prompts=%d", len(rec.prompts) == 0, len(rec.prompts))
	}
}

// 能原样还原的文件：保存不询问，写回的字节与载入时逐字节相同。
func TestApp_SaveIsSilentWhenBytesRoundTrip(t *testing.T) {
	noBOM := textio.Encoding{Kind: textio.KindUTF16LE, NoBOM: true}
	gbk := textio.Encoding{Kind: textio.KindANSI, CodePage: 936}
	utf16LE, _, _ := textio.Encode("hi 中文\r\nsecond line", textio.Encoding{Kind: textio.KindUTF16LE})
	utf16NoBOM, _, _ := textio.Encode("plain ascii text\r\nsecond line", noBOM)
	gbkBytes, _, _ := textio.Encode("中文测试 abc\r\n第二行", gbk)

	cases := []struct {
		name string
		data []byte
		skip bool
	}{
		{"utf-8", []byte("hello 世界\r\nsecond line"), false},
		{"utf-8 bom", rawBytes([]byte{0xEF, 0xBB, 0xBF}, "hello\r\n"), false},
		{"utf-16 le", utf16LE, false},
		{"utf-16 le without bom", utf16NoBOM, false},
		{"gbk", gbkBytes, textio.GetACP() == 65001},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.skip {
				t.Skip("GBK sample does not round-trip on a UTF-8 ACP system")
			}
			app, win := newTestAppWindow(t, 900, 640)
			defer win.Destroy()

			path := filepath.Join(t.TempDir(), "sample.txt")
			writeFileBytes(t, path, c.data)
			if !app.LoadFile(path) {
				t.Fatal("LoadFile failed")
			}
			rec := stubSavePrompt(t, goui.IDCANCEL)
			if !app.Save() {
				t.Fatal("save failed")
			}
			if len(rec.prompts) != 0 {
				t.Errorf("faithful save must not prompt, got %q", rec.prompts)
			}
			if got := readFileBytes(t, path); !bytes.Equal(got, c.data) {
				t.Errorf("unedited save changed the file:\n got % X\nwant % X", got, c.data)
			}
		})
	}
}

// 载入后在状态栏切换编码：按新编码写出的字节与文件现状不同，保存前确认一次；
// 用户确认保存后文件已是新编码，之后的保存不再询问；切回去同样不必再问。
func TestApp_EncodingSwitchAsksBeforeConvertingBytes(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "plain.txt")
	original := []byte("hello 世界\n")
	writeFileBytes(t, path, original)
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}

	app.setEncoding(textio.Encoding{Kind: textio.KindUTF8BOM})
	rec := stubSavePrompt(t, goui.IDCANCEL)
	if app.Save() || len(rec.prompts) != 1 {
		t.Fatalf("converting save must ask exactly once: prompts=%d", len(rec.prompts))
	}
	if got := readFileBytes(t, path); !bytes.Equal(got, original) {
		t.Errorf("cancelled save changed the file on disk")
	}

	stubSavePrompt(t, goui.IDYES)
	if !app.Save() {
		t.Fatal("save after Yes failed")
	}
	want := rawBytes([]byte{0xEF, 0xBB, 0xBF}, original)
	if got := readFileBytes(t, path); !bytes.Equal(got, want) {
		t.Errorf("file = % X, want % X", got, want)
	}

	rec = stubSavePrompt(t, goui.IDCANCEL)
	if !app.Save() || len(rec.prompts) != 0 {
		t.Errorf("file is already saved in the chosen encoding, must not ask again: prompts=%d", len(rec.prompts))
	}

	app.setEncoding(textio.Encoding{Kind: textio.KindUTF8})
	rec = stubSavePrompt(t, goui.IDCANCEL)
	if app.Save() || len(rec.prompts) != 1 {
		t.Errorf("switching back to another byte layout must ask again: prompts=%d", len(rec.prompts))
	}
}

// 载入时有损的文档，无论之后切成什么编码，“载入时丢过字节”都是更要紧的原因。
func TestApp_LossyLoadReasonSurvivesEncodingSwitch(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "corrupt.txt")
	writeFileBytes(t, path, utf8WithBadByte)
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}
	app.setEncoding(textio.Encoding{Kind: textio.KindUTF16LE})

	rec := stubSavePrompt(t, goui.IDCANCEL)
	app.Save()
	if len(rec.prompts) != 1 {
		t.Fatalf("prompts = %d, want 1", len(rec.prompts))
	}
	if msg := rec.prompts[0]; !strings.Contains(msg, "无法按") {
		t.Errorf("prompt should explain that bytes were lost while loading: %q", msg)
	}
}

// 既载入有损、又有字符无法用目标编码表示：合成一次询问，不能弹两次。
func TestApp_SaveMergesLossyAndUnrepresentableIntoOnePrompt(t *testing.T) {
	if textio.GetACP() == 65001 {
		t.Skip("UTF-8 ACP can represent every character")
	}
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "corrupt.txt")
	writeFileBytes(t, path, utf8WithBadByte)
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}
	if !app.textArea.ReplaceDocument(app.textArea.GetText() + " \U0001F600\n") {
		t.Fatal("ReplaceDocument failed")
	}
	app.setEncoding(textio.Encoding{Kind: textio.KindANSI, CodePage: textio.GetACP()})

	rec := stubSavePrompt(t, goui.IDNO)
	if app.Save() {
		t.Fatal("save went through after No")
	}
	if len(rec.prompts) != 1 {
		t.Fatalf("prompts = %d, want a single merged prompt", len(rec.prompts))
	}
	msg := rec.prompts[0]
	if !strings.Contains(msg, "无法按") || !strings.Contains(msg, "代替保存") {
		t.Errorf("prompt must mention both reasons: %q", msg)
	}
	if got := readFileBytes(t, path); !bytes.Equal(got, utf8WithBadByte) {
		t.Errorf("declined save changed the file on disk")
	}
}

// 另存为到当前文件本身时，同样要先确认外部修改，不能绕过检测直接覆盖。
func TestApp_SaveAsOntoCurrentPathHonoursExternalChange(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "note.txt")
	writeFileBytes(t, path, []byte("loaded"))
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}
	app.textArea.ReplaceDocument("edited in gnote")
	external := []byte("edited elsewhere")
	writeFileBytes(t, path, external)

	rec := stubSavePrompt(t, goui.IDNO)
	if app.saveToPath(path) {
		t.Fatal("Save As onto the externally modified file went through after No")
	}
	if len(rec.prompts) != 1 || !strings.HasPrefix(rec.prompts[0], "文件已被外部修改|") {
		t.Fatalf("external change prompt missing: %q", rec.prompts)
	}
	if got := readFileBytes(t, path); !bytes.Equal(got, external) {
		t.Errorf("declined save overwrote the external change")
	}

	stubSavePrompt(t, goui.IDYES)
	if !app.saveToPath(path) {
		t.Fatal("save after Yes failed")
	}
	if got := readFileBytes(t, path); string(got) != "edited in gnote" {
		t.Errorf("file = %q", got)
	}
}

// Ctrl+N 之后全局高亮偏好必须保持：未命名文档只是没有高亮器，随后打开的源码照常高亮。
func TestApp_NewDocumentKeepsHighlightPreference(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	goFile := filepath.Join(t.TempDir(), "x.go")
	writeFileBytes(t, goFile, []byte("package main\r\n"))
	if !app.highlightEnabled {
		t.Fatal("precondition: highlight preference is on")
	}

	app.NewDocument()
	if app.textArea.GetHighlighter() != nil {
		t.Errorf("untitled document should have no highlighter, got %T", app.textArea.GetHighlighter())
	}
	if !app.highlightEnabled || !app.CurrentSettings().Highlight || !app.commands.Checked(cmdViewHighlight) {
		t.Errorf("NewDocument turned off the global highlight preference: enabled=%v persisted=%v menu=%v",
			app.highlightEnabled, app.CurrentSettings().Highlight, app.commands.Checked(cmdViewHighlight))
	}

	if !app.LoadFile(goFile) {
		t.Fatal("LoadFile failed")
	}
	app.UpdateHeaderAndStatus()
	if got := goui.GetLanguageName(app.textArea.GetHighlighter()); got != "Go" || app.lblLanguage.GetText() != "Go" {
		t.Errorf("source file opened after Ctrl+N is not highlighted: highlighter=%q status=%q", got, app.lblLanguage.GetText())
	}
}

// 状态栏选择 “纯文本” 只影响当前文档，不能改掉会被持久化的全局偏好。
func TestApp_PlainTextLanguageAffectsOnlyCurrentDocument(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	goFile := filepath.Join(t.TempDir(), "x.go")
	writeFileBytes(t, goFile, []byte("package main\r\n"))
	if !app.LoadFile(goFile) {
		t.Fatal("LoadFile failed")
	}

	app.setDocumentLanguage(nil)
	if app.textArea.GetHighlighter() != nil {
		t.Errorf("纯文本 should remove the highlighter")
	}
	if got := app.lblLanguage.GetText(); got != "纯文本" {
		t.Errorf("status = %q, want 纯文本", got)
	}
	if !app.highlightEnabled || !app.CurrentSettings().Highlight || !app.commands.Checked(cmdViewHighlight) {
		t.Errorf("纯文本 turned off the global highlight preference: enabled=%v persisted=%v menu=%v",
			app.highlightEnabled, app.CurrentSettings().Highlight, app.commands.Checked(cmdViewHighlight))
	}

	app.NewDocument()
	if !app.LoadFile(goFile) {
		t.Fatal("LoadFile failed")
	}
	if got := goui.GetLanguageName(app.textArea.GetHighlighter()); got != "Go" {
		t.Errorf("highlighter after reopening the source file = %q, want Go", got)
	}
}

// 全局偏好关闭时在状态栏为单个文档挑一种语言：只对这个文档生效，偏好仍是关闭。
func TestApp_LanguageChoiceDoesNotTurnOnGlobalPreference(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.highlightEnabled = false
	goFile := filepath.Join(t.TempDir(), "x.go")
	writeFileBytes(t, goFile, []byte("package main\r\n"))
	if !app.LoadFile(goFile) {
		t.Fatal("LoadFile failed")
	}
	if app.textArea.GetHighlighter() != nil {
		t.Fatalf("precondition: highlighter is off")
	}

	app.setDocumentLanguage(goui.NewGoHighlighter())
	if got := goui.GetLanguageName(app.textArea.GetHighlighter()); got != "Go" {
		t.Errorf("chosen language not applied, got %q", got)
	}
	if app.highlightEnabled || app.CurrentSettings().Highlight || app.commands.Checked(cmdViewHighlight) {
		t.Errorf("per-document language choice changed the global preference: enabled=%v persisted=%v menu=%v",
			app.highlightEnabled, app.CurrentSettings().Highlight, app.commands.Checked(cmdViewHighlight))
	}

	app.NewDocument()
	pyFile := filepath.Join(t.TempDir(), "y.py")
	writeFileBytes(t, pyFile, []byte("print(1)\n"))
	if !app.LoadFile(pyFile) {
		t.Fatal("LoadFile failed")
	}
	if app.textArea.GetHighlighter() != nil {
		t.Errorf("preference is off, the next file must stay unhighlighted")
	}
}

// “查看 → 语法高亮” 只切换全局偏好：关闭时打开的源码不高亮，重新打开后立即恢复。
func TestApp_ToggleHighlightReenablesCurrentDocument(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.highlightEnabled = false
	goFile := filepath.Join(t.TempDir(), "x.go")
	writeFileBytes(t, goFile, []byte("package main\r\n"))
	if !app.LoadFile(goFile) {
		t.Fatal("LoadFile failed")
	}
	app.ToggleHighlight()
	if !app.highlightEnabled || goui.GetLanguageName(app.textArea.GetHighlighter()) != "Go" {
		t.Errorf("turning highlight on should light up the current source file")
	}
	app.ToggleHighlight()
	if app.highlightEnabled || app.textArea.GetHighlighter() != nil {
		t.Errorf("turning highlight off should clear it again")
	}
}

// 配置文件正作为文档打开时，退出不得用内存里的旧值覆盖用户在编辑器里改过并保存的内容；
// 文档保存后会话立即跟随文件（主题、字号、缩放、换行、行号、高亮即时生效，启动页下次启动生效）。
func TestApp_EditedSettingsDocumentSurvivesExit(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")
	if err := app.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	app.OpenConfigFile()

	before := string(readFileBytes(t, app.settingsPathOverride))
	edited := strings.Replace(before, `"baseFontSize": 16`, `"baseFontSize": 20`, 1)
	edited = strings.Replace(edited, `"startupPage": "welcome"`, `"startupPage": "blank"`, 1)
	edited = strings.Replace(edited, `"theme": "dark"`, `"theme": "light"`, 1)
	edited = strings.Replace(edited, `"wordWrap": true`, `"wordWrap": false`, 1)
	if edited == before {
		t.Fatalf("could not prepare the edited settings document:\n%s", before)
	}
	app.textArea.ReplaceDocument(edited)
	if !app.Save() {
		t.Fatal("saving the settings document failed")
	}

	// 退出（handleClose）：文档已是干净状态，隐式保存不得用内存值覆盖它。
	if !app.handleClose() {
		t.Fatal("handleClose refused to close a clean document")
	}
	after := string(readFileBytes(t, app.settingsPathOverride))
	for _, want := range []string{`"baseFontSize": 20`, `"startupPage": "blank"`, `"theme": "light"`, `"wordWrap": false`} {
		if !strings.Contains(after, want) {
			t.Errorf("exit overwrote the edit %s; file now:\n%s", want, after)
		}
	}
	if got := app.textArea.GetBaseFontSize(); got != 20 {
		t.Errorf("session should follow the file: base font = %d, want 20", got)
	}
	if app.themeID != "light" {
		t.Errorf("session should follow the file: theme = %q, want light", app.themeID)
	}
	if app.textArea.IsWordWrap() {
		t.Errorf("session should follow the file: word wrap should be off")
	}
	if app.settings.StartupPage != startupPageBlank {
		t.Errorf("startup page = %q, want blank for the next start", app.settings.StartupPage)
	}
}

// 配置文档有未保存修改时再次执行 “打开配置文件”：
//   - 选 “是”：先保存文档，文件保留用户的修改；
//   - 选 “否”：丢弃未保存的修改，但不能用内存值覆盖文件里更早保存过的修改；
//   - 选 “取消”：文档与文件都保持原样。
func TestApp_OpenConfigFileWithUnsavedSettingsEdits(t *testing.T) {
	const fontSize20 = `"baseFontSize": 20`
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")
	if err := app.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	app.OpenConfigFile()
	settingsPath := app.settingsPathOverride

	stubSavePrompt(t, goui.IDYES)
	app.textArea.ReplaceDocument(strings.Replace(app.textArea.GetText(), `"baseFontSize": 16`, fontSize20, 1))
	app.OpenConfigFile()
	if got := string(readFileBytes(t, settingsPath)); !strings.Contains(got, fontSize20) {
		t.Fatalf("with Yes the unsaved edit must be saved first; file:\n%s", got)
	}

	stubSavePrompt(t, goui.IDNO)
	app.textArea.ReplaceDocument(strings.Replace(app.textArea.GetText(), fontSize20, `"baseFontSize": 22`, 1))
	app.OpenConfigFile()
	if got := string(readFileBytes(t, settingsPath)); !strings.Contains(got, fontSize20) {
		t.Errorf("with No the unsaved edit is discarded, but the saved edit (20) must survive; file:\n%s", got)
	}
	if app.textArea.IsModified() {
		t.Errorf("document should be reloaded clean from disk")
	}

	rec := stubSavePrompt(t, goui.IDCANCEL)
	app.textArea.ReplaceDocument(strings.Replace(app.textArea.GetText(), fontSize20, `"baseFontSize": 24`, 1))
	app.OpenConfigFile()
	if len(rec.prompts) == 0 {
		t.Errorf("unsaved edits must be asked about")
	}
	if got := app.textArea.GetText(); !strings.Contains(got, `"baseFontSize": 24`) || !app.textArea.IsModified() {
		t.Errorf("with Cancel the document must keep its unsaved edits")
	}
	if got := string(readFileBytes(t, settingsPath)); !strings.Contains(got, fontSize20) {
		t.Errorf("with Cancel the file must stay untouched; file:\n%s", got)
	}
}

// 设置窗口点 “应用” 时配置文件正作为文档打开：
// 文档干净则直接写盘并让编辑器重新载入；文档有未保存修改时拒绝写入并如实报错，而不是谎称 “已保存”。
func TestSettingsWindow_ApplyWhileSettingsDocumentOpen(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")
	if err := app.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	app.OpenConfigFile()

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw

	sw.fontSlider.SetValue(20)
	if err := sw.applySettings(); err != nil {
		t.Fatalf("apply with a clean settings document: %v", err)
	}
	if got := string(readFileBytes(t, app.settingsPathOverride)); !strings.Contains(got, `"baseFontSize": 20`) {
		t.Errorf("applied settings not persisted; file:\n%s", got)
	}
	if got := app.textArea.GetText(); !strings.Contains(got, `"baseFontSize": 20`) || app.textArea.IsModified() {
		t.Errorf("editor should show the file that was just written: modified=%v text:\n%s", app.textArea.IsModified(), got)
	}

	app.textArea.ReplaceDocument(strings.Replace(app.textArea.GetText(), `"baseFontSize": 20`, `"baseFontSize": 22`, 1))
	sw.fontSlider.SetValue(24)
	if err := sw.applySettings(); err == nil {
		t.Errorf("apply must refuse to write while the settings document has unsaved edits")
	}
	if got := string(readFileBytes(t, app.settingsPathOverride)); !strings.Contains(got, `"baseFontSize": 20`) {
		t.Errorf("refused apply must leave the file alone; file:\n%s", got)
	}
	if got := app.textArea.GetText(); !strings.Contains(got, `"baseFontSize": 22`) || !app.textArea.IsModified() {
		t.Errorf("refused apply must leave the unsaved document alone: modified=%v", app.textArea.IsModified())
	}
}

// Windows PowerShell 5.1 的 Set-Content -Encoding UTF8 会写 BOM；整数位写成 18.0 / 2e1 / "18" 也应读得进来。
func TestSettings_LoadToleratesBOMAndNumericFormats(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want func(Settings) Settings
	}{
		{"utf-8 bom", "\xEF\xBB\xBF" + `{"theme": "nord"}`, func(s Settings) Settings { s.Theme = "nord"; return s }},
		{"float integers", `{"baseFontSize": 18.0, "windowWidth": 1280.0, "windowHeight": 720.0}`, func(s Settings) Settings {
			s.BaseFontSize, s.WindowWidth, s.WindowHeight = 18, 1280, 720
			return s
		}},
		{"exponent", `{"baseFontSize": 2e1, "zoom": 15e-1}`, func(s Settings) Settings { s.BaseFontSize, s.Zoom = 20, 1.5; return s }},
		{"quoted numbers", `{"baseFontSize": "18", "zoom": "1.5", "windowWidth": "1024", "version": "1"}`, func(s Settings) Settings {
			s.BaseFontSize, s.Zoom, s.WindowWidth = 18, 1.5, 1024
			return s
		}},
		{"int zoom", `{"zoom": 2}`, func(s Settings) Settings { s.Zoom = 2; return s }},
		{"unknown keys", `{"theme": "light", "futureOption": {"nested": [1, 2]}, "lineNumbers": false}`, func(s Settings) Settings {
			s.Theme, s.LineNumbers = "light", false
			return s
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			writeFileBytes(t, path, []byte(c.raw))
			got, err := loadSettingsFrom(path)
			if err != nil {
				t.Fatalf("loadSettingsFrom: %v", err)
			}
			if want := c.want(DefaultSettings()); !reflect.DeepEqual(got, want) {
				t.Errorf("loaded settings mismatch:\n got=%+v\nwant=%+v", got, want)
			}
		})
	}
}

// 一个字段的类型写错只丢掉这个字段，其余照常读取；数组里读不出来的元素跳过。
func TestSettings_LoadWrongTypeOnlyDropsThatField(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want func(Settings) Settings
	}{
		{"theme number", `{"theme": 5, "baseFontSize": 20, "wordWrap": false}`, func(s Settings) Settings {
			s.BaseFontSize, s.WordWrap = 20, false
			return s
		}},
		{"bool string", `{"wordWrap": "yes", "lineNumbers": false, "highlight": false}`, func(s Settings) Settings {
			s.LineNumbers, s.Highlight = false, false
			return s
		}},
		{"font out of range", `{"baseFontSize": 1e9}`, func(s Settings) Settings { s.BaseFontSize = 32; return s }},
		{"font garbage", `{"baseFontSize": "big", "zoom": "x", "windowWidth": null}`, func(s Settings) Settings { return s }},
		{"startup page number", `{"startupPage": 1, "theme": "nord"}`, func(s Settings) Settings { s.Theme = "nord"; return s }},
		{"recent files mixed", `{"recentFiles": ["a.txt", 5, "b.txt"]}`, func(s Settings) Settings {
			s.RecentFiles = []string{"a.txt", "b.txt"}
			return s
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			writeFileBytes(t, path, []byte(c.raw))
			got, err := loadSettingsFrom(path)
			if err != nil {
				t.Fatalf("a single mistyped field must not sink the whole file: %v", err)
			}
			if want := c.want(DefaultSettings()); !reflect.DeepEqual(got, want) {
				t.Errorf("loaded settings mismatch:\n got=%+v\nwant=%+v", got, want)
			}
		})
	}
}

// 否则用户手写的配置在他发现之前就没了。
func TestSettings_BrokenFileIsNeverOverwritten(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	broken := `{"theme": "nord", "baseFontSize": 20, "wordWrap": false,}` + "\r\n"
	writeFileBytes(t, app.settingsPathOverride, []byte(broken))
	if _, err := loadSettingsFrom(app.settingsPathOverride); err == nil {
		t.Fatal("precondition: the file must not parse")
	}
	app.settingsLoadFailed = true // main 在 LoadSettings 报错时置位
	app.textArea.SetBaseFontSize(30)

	if err := app.SaveSettings(); err == nil {
		t.Errorf("SaveSettings must report that it refused to overwrite the unreadable file")
	}
	if got := readFileBytes(t, app.settingsPathOverride); string(got) != broken {
		t.Errorf("unreadable settings file was overwritten:\n%s", got)
	}

	// 用户修好文件之后，保存恢复正常。
	writeFileBytes(t, app.settingsPathOverride, []byte(`{"theme": "nord"}`))
	if err := app.SaveSettings(); err != nil {
		t.Fatalf("SaveSettings after the file was fixed: %v", err)
	}
	if got := string(readFileBytes(t, app.settingsPathOverride)); !strings.Contains(got, `"baseFontSize": 30`) {
		t.Errorf("settings not saved after the file was fixed; file:\n%s", got)
	}
}

// 窗口尺寸要有上限：手工误填的 99999 不能被当成窗口大小持久化并照搬。
func TestSettings_WindowSizeHasUpperBound(t *testing.T) {
	const maxSide = 16384
	got := (Settings{WindowWidth: 99999, WindowHeight: 99999}).Normalized()
	if got.WindowWidth > maxSide || got.WindowHeight > maxSide {
		t.Errorf("window size %dx%d exceeds the sane bound %d", got.WindowWidth, got.WindowHeight, maxSide)
	}
	if got.WindowWidth <= 0 || got.WindowHeight <= 0 {
		t.Errorf("window size must stay positive, got %dx%d", got.WindowWidth, got.WindowHeight)
	}
	normal := (Settings{WindowWidth: 2560, WindowHeight: 1440}).Normalized()
	if normal.WindowWidth != 2560 || normal.WindowHeight != 1440 {
		t.Errorf("ordinary window size must be kept, got %dx%d", normal.WindowWidth, normal.WindowHeight)
	}
}

// firstFilterPatterns 返回 fileFilter 里第一个过滤器（打开对话框默认选中的那个）的全部模式。
func firstFilterPatterns(t *testing.T) []string {
	t.Helper()
	parts := strings.Split(fileFilter, "|")
	if len(parts) < 2 {
		t.Fatalf("fileFilter has no pattern part: %q", fileFilter)
	}
	return strings.Split(parts[1], ";")
}

// filterMatches 按 Win32 通配符语义判断文件名是否匹配某个模式（只支持一个 '*'）。
func filterMatches(name, pattern string) bool {
	n, p := strings.ToLower(name), strings.ToLower(strings.TrimSpace(pattern))
	star := strings.IndexByte(p, '*')
	if star < 0 {
		return n == p
	}
	return strings.HasPrefix(n, p[:star]) && strings.HasSuffix(n, p[star+1:])
}

// 打开对话框默认只显示第一个过滤器匹配的文件：常见文本/配置/脚本文件必须直接可见，
// 不用切到 “所有文件” 才能找到。
func TestFileFilter_ShowsCommonTextFilesByDefault(t *testing.T) {
	patterns := firstFilterPatterns(t)
	for _, name := range []string{
		"a.xml", "b.ini", "c.log", "run.bat", "x.cmd", "setup.ps1", "build.sh", "data.csv",
		"app.cfg", "app.conf", ".env", "app.properties", "build.gradle", "m.lua", "i.php", "x.rb",
		"A.kt", "c.vue", "Makefile", "Dockerfile",
		"a.txt", "x.go", "y.py", "z.json", "readme.md",
	} {
		shown := false
		for _, p := range patterns {
			if filterMatches(name, p) {
				shown = true
				break
			}
		}
		if !shown {
			t.Errorf("%s is invisible in the Open dialog until the user switches to 所有文件", name)
		}
	}
	if filterMatches("program.exe", "*.txt") {
		t.Fatal("sanity: exe must not match a txt pattern")
	}
}

// 另存为：输入的文件名没有扩展名时，补上的是所选过滤器的扩展名，而不是一律 .txt。
func TestSaveDialog_ExtensionFollowsSelectedFilter(t *testing.T) {
	const filter = "文本 (*.txt;*.md)|*.txt;*.md|Go 源码文件 (*.go)|*.go|所有文件 (*.*)|*.*|无扩展名文件 (Makefile)|Makefile"
	cases := []struct {
		path  string
		index int
		want  string
	}{
		{`C:\work\main`, 2, `C:\work\main.go`},
		{`C:\work\notes`, 1, `C:\work\notes.txt`},
		{`C:\work\main.go`, 2, `C:\work\main.go`}, // 用户自己带了扩展名：不改动
		{`C:\work\README`, 3, `C:\work\README`},   // *.* 没有可补的扩展名
		{`C:\work\Makefile`, 4, `C:\work\Makefile`},
		{`C:\work\main`, 0, `C:\work\main`}, // 没有选中任何过滤器
		{`C:\work\main`, 9, `C:\work\main`}, // 越界
	}
	for _, c := range cases {
		if got := dialog.WithFilterExtension(c.path, filter, c.index); got != c.want {
			t.Errorf("WithFilterExtension(%q, %d) = %q, want %q", c.path, c.index, got, c.want)
		}
	}
}

// 复制 / 剪切写不进剪贴板（被其他程序占用）时在状态栏给出提示；
// 没有选区是正常的空操作，写入成功也不该打扰用户。
func TestApp_ClipboardFailureShowsStatusHint(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	app.textArea.SetText("hello world")
	app.UpdateHeaderAndStatus()
	normal := app.lblFileStatus.GetText()

	calls := 0
	fail := func() bool {
		calls++
		return false
	}
	app.transferSelection(fail)
	if calls != 1 {
		t.Fatalf("copy must still be attempted without a selection, calls=%d", calls)
	}
	if got := app.lblFileStatus.GetText(); got != normal {
		t.Errorf("no selection is a legitimate no-op, status = %q", got)
	}

	app.textArea.SelectAll()
	app.transferSelection(func() bool { return true })
	if got := app.lblFileStatus.GetText(); got != normal {
		t.Errorf("a successful copy must not show a hint, status = %q", got)
	}

	app.transferSelection(fail)
	if got := app.lblFileStatus.GetText(); got != clipboardBusyHint {
		t.Fatalf("status = %q, want %q", got, clipboardBusyHint)
	}
	// 提示不随下一次状态栏刷新消失，到时间才恢复原来的文字
	app.UpdateHeaderAndStatus()
	if got := app.lblFileStatus.GetText(); got != clipboardBusyHint {
		t.Errorf("hint lost on the next status refresh, status = %q", got)
	}
	app.clearStatusHint(clipboardBusyHint)
	if got := app.lblFileStatus.GetText(); got != normal {
		t.Errorf("status after the hint expired = %q, want %q", got, normal)
	}
}

// saveWarning 汇总保存前的确认需求：三种原因各自成立时都要问，合并成一条说明，都不成立时不问。
func TestSaveWarning_MergesReasonsIntoOnePrompt(t *testing.T) {
	utf8 := textio.Encoding{Kind: textio.KindUTF8}
	cases := []struct {
		name             string
		replaced         bool
		alter            saveAlterReason
		need             bool
		wantSubstrings   []string
		notWantSubstring string
	}{
		{"nothing to ask", false, alterNone, false, nil, ""},
		{"unrepresentable only", true, alterNone, true, []string{"代替保存", "是否仍要保存"}, "无法按"},
		{"lossy load", false, alterDecode, true, []string{"无法按", "UTF-8"}, "切换为"},
		{"encoding switched", false, alterConvert, true, []string{"切换为", "UTF-8 BOM"}, "无法按"},
		{"all reasons at once", true, alterConvert, true, []string{"切换为", "代替保存"}, "无法按"},
	}
	for _, c := range cases {
		msg, need := saveWarning("UTF-8 BOM", c.replaced, c.alter, utf8)
		if need != c.need {
			t.Errorf("%s: need = %v, want %v", c.name, need, c.need)
		}
		for _, want := range c.wantSubstrings {
			if !strings.Contains(msg, want) {
				t.Errorf("%s: prompt %q lacks %q", c.name, msg, want)
			}
		}
		if c.notWantSubstring != "" && strings.Contains(msg, c.notWantSubstring) {
			t.Errorf("%s: prompt %q must not mention %q", c.name, msg, c.notWantSubstring)
		}
		if got := strings.Count(msg, "是否仍要保存"); need && got != 1 {
			t.Errorf("%s: prompt asks %d times, want exactly one question", c.name, got)
		}
	}
}
