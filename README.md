# gnote

纯 Go、零 CGO 的 Windows 原生记事本与代码编辑器。软件光栅化双缓冲渲染、内嵌等宽中英字体、16 种语言语法高亮，单文件可执行程序，无任何运行时依赖。

- 仓库：<https://github.com/jacksalad/gnote>
- 平台：Windows 10 / 11 x64
- 工具链：Go 1.25+（推荐 1.27+）

## 特性

- **零 CGO**：`syscall.NewLazyDLL` 动态绑定 Win32 / GDI，无需 MinGW 或 C/C++ 工具链。
- **软件双缓冲渲染**：32 位 ARGB 内存缓冲 + DIB Section + `BitBlt`，拦截 `WM_ERASEBKGND`，无闪烁。
- **内嵌字体**：`go:embed` 打包 `Gnote Mono`（Maple Mono CN + LXGW WenKai Mono 子集合并），中英严格 2:1，全新系统上渲染一致。
- **高性能编辑**：gap buffer、行级视口虚拟化、增量折行索引、高亮 span 行缓存、差分撤销，10 万行文档秒开。
- **16 种语法高亮**：Go / Python / C/C++ / C# / Java / Rust / R / JavaScript / TypeScript / HTML / CSS / SQL / Markdown / JSON / YAML / TOML。
- **编辑能力**：查找替换（大小写开关、全部替换单步撤销）、多级撤销重做、智能缩进、单词跳转、平滑缩放、IME 组合串内联。
- **编码保真**：BOM / UTF-8 / UTF-16 LE/BE / ANSI 自动识别，原编码回写，CRLF/LF/CR 保留。
- **文档格式化**：`Ctrl+Shift+F` 格式化 JSON / HTML（内联 CSS、JS 同步处理，键序与数字字面量不变）。
- **原生集成**：自绘菜单栏、状态栏分段菜单、右键菜单、独立设置窗口、系统剪贴板、命令行/资源管理器打开文件、未保存关闭拦截。
- **启动页面可选**：设置 → 编辑器 中选择启动时显示介绍文档或空白页。
- **原子保存**：临时文件 + `Sync` + 原子替换，配合外部修改检测，避免覆盖丢失。
- **八套主题**：Dark / Light / Nord / Tokyo Night / One Dark / Gruvbox / Solarized Light / Rosé Pine。

## 快速开始

```powershell
# 调试运行（保留控制台日志）
go run ./cmd/gnote

# 生产构建（gofmt / vet / test 门禁 + 隐藏控制台 + 图标与版本资源）
./build.ps1

# 跳过测试门禁
./build.ps1 -SkipTests

# 生成便携发布包
./build.ps1 -Package
```

产物：`bin/gnote.exe`（同时复制一份到仓库根目录）。

命令行直接打开文件：

```powershell
go run ./cmd/gnote .\docs\README.md
.\bin\gnote.exe .\cmd\gnote\main.go
```

## 作为 GUI 库使用

仓库根包 `goui` 同时是完整的 Windows GUI 框架：

```go
import goui "github.com/jacksalad/gnote"

func main() {
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:   "Hello gnote",
		Width:   420,
		Height:  260,
		Center:  true,
		Visible: true,
	})
	if err != nil {
		panic(err)
	}

	root := goui.NewBackgroundPanel()
	root.SetPadding(20)

	title := goui.NewBrandLabel("你好，goui！")
	title.SetFont(goui.NewFont("Microsoft YaHei", 18))
	title.SetBounds(20, 20, 380, 32)
	root.Add(title)

	desc := goui.NewSecondaryLabel("纯 Go · 零 CGO · 软件光栅化渲染")
	desc.SetBounds(20, 58, 380, 20)
	root.Add(desc)

	win.SetContent(root)
	win.Run()
}
```

组件、布局、主题与渲染 API 见 [`docs/`](./docs/README.md)。

## 项目结构

| 目录 | 职责 |
| :--- | :--- |
| `cmd/gnote/` | 应用入口、命令注册表、菜单栏、查找条、设置窗口、状态栏、图标资源 |
| `component/` | 编辑器与 UI 控件（TextArea、Button、MenuBar、TabBar…） |
| `render/` `graphics/` | 软件光栅化、GDI 字形轮廓、几何模型 |
| `window/` `dialog/` `clipboard/` | Win32 窗口、原生对话框、剪贴板 |
| `syntax/` `format/` `textio/` | 16 种语言高亮、JSON/HTML 格式化、编码与换行 |
| `theme/` `event/` | 主题令牌、事件总线 |
| `docs/` | 架构与各子系统文档 |

## 构建要求

- Windows 10/11 x64，Go 1.25+
- 可选：`windres`（MinGW binutils），仅当修改 `cmd/gnote/gnote.rc` 或图标后重新生成 `gnote_windows_amd64.syso` 时需要：

```powershell
windres -i cmd/gnote/gnote.rc -O coff -o cmd/gnote/gnote_windows_amd64.syso
```

仓库已提交预编译的 `.syso`，普通构建无需 `windres`。

## 测试

```powershell
gofmt -l .
go vet ./...
go test ./...
go test ./component -run '^$' -bench . -benchtime 5x
```

`go vet` 在 `window/window.go` 会报告两条已知的 Win32 回调 `lParam` 指针还原告警（源码已注释说明），`build.ps1` 门禁仅放行这两条。

## 许可证

代码以 [MIT License](./LICENSE) 发布。

`cmd/gnote/font.ttf` 为 Maple Mono CN 与 LXGW WenKai Mono 的 OFL-1.1 子集衍生物，版权与许可全文见 [`cmd/gnote/font-LICENSE.txt`](./cmd/gnote/font-LICENSE.txt)。