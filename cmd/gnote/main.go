//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goui "github.com/jacksalad/gnote"
)

//go:embed font.ttf
var embeddedFontData []byte

func main() {
	// 0. 优先加载并注册内嵌的等宽中英文字体 (6.5MB, 9431 码位, 中英严格 2:1, 零外部依赖)
	appFontFamily := "Microsoft YaHei"
	if len(embeddedFontData) > 0 {
		fam, err := goui.RegisterMemoryFont(embeddedFontData)
		if err == nil && fam != "" {
			appFontFamily = fam
			goui.SetDefaultFont(appFontFamily, 16)
		}
	}

	// 1. 读取用户偏好并先应用主题，保证窗口创建时就是正确的背景色
	settings := LoadSettings()
	if th := goui.ThemeByID(settings.Theme); th != nil {
		goui.SetTheme(th)
	} else {
		goui.SetTheme(goui.ThemeDark())
	}

	// 2. 创建主窗口 (恢复上次窗口尺寸，屏幕居中显示，挂载专属现代极客 Logo)
	gnoteIcon := GetGnoteIcon()
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:     "未命名 - gnote",
		Width:     settings.WindowWidth,
		Height:    settings.WindowHeight,
		Center:    true,
		Icon:      gnoteIcon,
		Resizable: true,
		Visible:   true,
	})
	if err != nil {
		fmt.Printf("创建窗口失败: %v\n", err)
		return
	}

	// 3. 构建应用状态、界面与命令，进入主消息循环
	app := NewApp(win, appFontFamily, settings)
	app.Build()
	app.Start(parseTargetFileFromArgs(os.Args[1:]), welcomeDoc)
}

// welcomeDoc 首次启动展示的欢迎文档与快捷键指引。
const welcomeDoc = "# 欢迎使用 gnote！\r\n\r\n" +
	"> 纯 Go 零 CGO 软件光栅化双缓冲引擎，具备毫秒级冷启动与原生级流畅度。\r\n\r\n" +
	"---\r\n\r\n" +
	"## 核心技术特性\r\n" +
	"- **零 CGO 依赖**：纯 Windows API 动态绑定，轻量级极速编译\r\n" +
	"- **物理像素抗锯齿**：GDI 矢量轮廓 1:1 线性灰度光栅化，高分屏清晰锐利\r\n" +
	"- **智能自动折行**：超长单行文本自动平滑换行显示，严谨对齐且不超出窗口右侧\r\n" +
	"- **内置多语言高亮**：纯 Go 词法分词状态机，原生支持 Go、Python、C/C++、C#、Java、Rust、R、JavaScript、TypeScript、HTML、CSS、SQL、Markdown、JSON、YAML、TOML 语法高亮\r\n" +
	"- **无级平滑缩放**：支持 `Ctrl + 滚轮` 与 `Ctrl + +/-/0` 实时缩放\r\n" +
	"- **行级视口虚拟化**：万行超大文档丝滑滚动\r\n" +
	"- **JSON / HTML 格式化**：`Ctrl+Shift+F` 缩进排版，保持 JSON 键序与数字字面量，HTML 内联 CSS/JS 同步格式化，保留预格式化内容与模板字符串\r\n" +
	"- **Windows 原生剪贴板**：完整支持撤销 (Ctrl+Z) 与重做 (Ctrl+Y)\r\n\r\n" +
	"## 多语言支持速览\r\n" +
	"- `.go`               : Go 关键字、内置类型、函数声明/调用、多行块注释、反引号原生文本\r\n" +
	"- `.py` / `.pyw`      : Python 关键字、内置函数/类型、三引号文档注释、前缀字符串、装饰器\r\n" +
	"- `.c` / `.cpp` / `.h`: C/C++ 预处理指令、类/命名空间、STL 类型、函数调用、多行块注释、指针操作符\r\n" +
	"- `.cs` / `.csx`      : C# 核心关键字、LINQ 查询、Task 异步、逐字跨行字符串、预处理器、属性标记\r\n" +
	"- `.java`             : Java 关键字、核心类库、注解、Javadoc/多行注释、多行文本块、Lambda\r\n" +
	"- `.rs`               : Rust 所有权生命周期、宏调用(println!/vec!)、属性注解、原生字符串、类型后缀\r\n" +
	"- `.r` / `.R`         : R 语言特殊赋值(<-)、管道符(%>%/|>)、二元操作符(%in%)、点号函数、数据分析函数\r\n" +
	"- `.js` / `.jsx`      : JavaScript 现代语法、异步函数、模板字符串、箭头函数、内置对象\r\n" +
	"- `.ts` / `.tsx`      : TypeScript / TypoScript 接口、类型别名、泛型、装饰器、类型断言\r\n" +
	"- `.html` / `.htm`    : HTML 标签配对、属性键值、<!DOCTYPE>、注释与实体字符\r\n" +
	"- `.css`              : CSS 选择器、属性与单位、@media 规则块、十六进制色值与 !important\r\n" +
	"- `.sql`              : SQL 标准查询、DDL/DML、内置聚合函数、数据类型、单双引号与注释\r\n" +
	"- `.md` / `.markdown` : Markdown 标题、代码块、列表、粗体、链接\r\n" +
	"- `.json`             : JSON 键名、字符串、数值、布尔值、转义字符、结构括号\r\n" +
	"- `.yaml` / `.yml`    : YAML 映射键、列表项、锚点、别名、多行块标量、注释\r\n" +
	"- `.toml`             : TOML 表头 [table]、键名、时间戳、多行字符串、数字\r\n\r\n" +
	"## 快捷键速查清单\r\n" +
	"- [x] `Ctrl + N`       : 新建文档\r\n" +
	"- [x] `Ctrl + O`       : 打开文件\r\n" +
	"- [x] `Ctrl + S`       : 保存文档\r\n" +
	"- [x] `Ctrl + Shift+S` : 另存为文档\r\n" +
	"- [x] `Ctrl + F / H`   : 查找 / 替换（含大小写开关与全部替换）\r\n" +
	"- [x] `Ctrl + Shift+F` : 格式化 JSON / HTML 文档\r\n" +
	"- [x] `Ctrl + A`       : 全选内容\r\n" +
	"- [x] `Ctrl + Z / Y`   : 撤销 / 重做（连续输入自动合组）\r\n" +
	"- [x] `双击 / 三击`    : 选中单词 / 选中整行\r\n" +
	"- [x] `右键`           : 撤销、剪切、复制、粘贴、全选、查找替换、格式化\r\n" +
	"- [x] `Ctrl + 滚轮`     : 实时放大 / 缩小字体\r\n" +
	"- [x] `Ctrl + +/-/0`   : 放大 / 缩小 / 恢复 100%\r\n" +
	"- [x] `Alt + F / F10` : 打开顶部菜单\r\n" +
	"- [x] `Alt + F4`       : 退出\r\n" +
	"- [x] `Ctrl + ,`       : 打开设置窗口\r\n" +
	"- [x] `状态栏右侧`     : 点击切换编码 / EOL / 语言 / 缩放\r\n\r\n" +
	"开源项目仓库: [gnote on GitHub](https://github.com/jacksalad/gnote)\r\n"

// parseTargetFileFromArgs 从命令行参数中提取待打开的目标文件路径（用于支持 Windows“打开方式”、快捷方式传参以及命令行调用）
func parseTargetFileFromArgs(args []string) string {
	for _, arg := range args {
		trimmed := strings.Trim(strings.TrimSpace(arg), "\"")
		if trimmed != "" && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "/") {
			return trimmed
		}
	}
	return ""
}

// maxOpenFileSize 单文件打开上限：超过该尺寸的文本会显著拖慢 UI 线程并占用大量内存
const maxOpenFileSize = 64 << 20

// writeFileAtomic 先写同目录临时文件并落盘，再原子替换目标文件。
// 避免写入中途崩溃/断电/磁盘写满导致原文件被截断。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gnote-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	_ = os.Chmod(tmpName, mode)

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
