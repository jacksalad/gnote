# goui 使用文档与开发指南

> **核心定位**：无 CGO 依赖、纯 Go 实现的 Windows 高性能软件双缓冲渲染 GUI 开发框架  
> **运行环境**：Windows 10 / 11 x64 | Go 1.25+ (推荐 Go 1.27+)  
> **工程主页与顶层门面**：[`goui.go`](../goui.go)

---

## 1. 项目概览

[`goui`](../) 是一个专为 Windows 平台打造的高性能、轻量级、零 CGO 依赖的现代化 Go 语言 GUI 开发库。

不同于传统基于 WebView/Chromium 的重型框架（如 Electron、Wails），也不同于需要繁琐 MinGW/GCC 编译工具链的 CGO 绑定库，`goui` 追求极限的编译速度与极小的分发体积：
- **纯 Go 驱动 (Zero CGO)**：底层全面采用 `syscall.NewLazyDLL` 动态绑定 Windows 原生 Win32 与 GDI 系统库，Windows 原生编译仅需数秒，无需额外安装任何 C/C++ 编译器。
- **软件双缓冲光栅化引擎**：在纯内存中开辟 32 位 ARGB 像素线性缓冲区，通过纯 Go 算法实现亚像素级平滑抗锯齿绘图（直线、圆、圆角矩形、Alpha 像素混合），渲染完成后通过 Win32 DIB Section 与 `BitBlt` 投递屏幕，拦截 `WM_ERASEBKGND`，彻底杜绝闪烁与撕裂。
- **Windows GDI 矢量字体解析**：采用 `GetGlyphOutlineW` 获取矢量字形灰度蒙版，按线性灰度覆盖率映射混合，避免深色主题下描边过度提亮发虚；字形完全服从渲染引擎的局部裁剪栈与半透明混合规则，中英文显示平滑细腻。
- **响应式与组件化架构**：提供完整的现代 UI 组件库（Button, Label, TextBox, CheckBox, RadioButton, ProgressBar, Slider, ImageView, ScrollView, MenuBar, Switch, Segmented, TabBar）与自动布局引擎（FlowLayout, GridLayout, BorderLayout），支持组件嵌套、事件冒泡与反向 Z-Order 命中测试。
- **设计令牌与主题系统**：内置 Dark / Light / Nord / Tokyo Night / One Dark / Gruvbox / Solarized Light / Rosé Pine 八套完整预设，UI 令牌与跨语言语法令牌同源，支持运行期动态热切换。

---

## 2. 文档索引导航

为了便于系统学习与日常 API 查阅，本使用文档划分为以下专业模块：

| 文档章节 | 核心内容提要 |
| :--- | :--- |
| **[1. 快速上手 (Quickstart)](./quickstart.md)** | 环境要求、模块引入、编写首个窗口程序、去除控制台黑框编译、范例库运行 |
| **[2. 架构设计与渲染原理 (Architecture)](./architecture.md)** | Win32 零 CGO 机制、DIB Section 双缓冲、软件光栅化与抗锯齿、事件总线、DPI 感知 |
| **[3. UI 组件库完全指南 (Components)](./components.md)** | 核心接口契约、基础与输入控件 (Button, Label, TextBox, CheckBox, RadioButton)、高级交互 (ProgressBar, Slider)、视图 (ImageView, ScrollView)、菜单与设置控件 (MenuBar, Switch, Segmented, TabBar) |
| **[4. 布局系统与容器排版 (Layouts)](./layouts.md)** | Panel 容器角色与边框、FlowLayout 流式布局、GridLayout 网格切分、BorderLayout 边界布局、响应式自适应 |
| **[5. 自定义绘图与图像光栅化 (Canvas & Graphics)](./canvas_and_graphics.md)** | Canvas 绘图 API (线条/矩形/圆角/圆)、Alpha 混合、视口裁剪栈、矢量字体测量与排版、纯 Go Image 解码与绘制、自定义组件开发 |
| **[6. 主题系统与剪贴板集成 (Theme & Clipboard)](./theme_and_clipboard.md)** | 八套 UI + 语法双令牌主题、统一高亮语义角色、动态主题广播热切换、纯 Go 原生 Win32 剪贴板 UTF-16 读写 |
| **[7. 最佳实践与常见问题 (FAQ & Best Practices)](./faq_and_best_practices.md)** | 多 Goroutine 并发更新 UI 范式、GC 与内存连续性优化、Windows 资源打包 (图标/Manifest)、排错自查清单 |
| **[8. v3.0 开发路线图 (v3 Roadmap)](./v3-roadmap.md)** | 自绘菜单栏、独立设置窗口、多窗口生命周期、系统键与 DPI 的里程碑与提交计划 |

---

## 3. 极速尝鲜：5 分钟 Hello World

以下是一个完整的最简应用程序，展示了窗口创建、面板卡片嵌套、矢量标题以及具备状态反馈的强调色交互按钮：

```go
//go:build windows

package main

import (
	"fmt"
	goui "github.com/jacksalad/gnote"
)

func main() {
	// 1. 初始化系统窗口
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:     "goui Hello World",
		Width:     420,
		Height:    260,
		Resizable: false,
		Visible:   true,
	})
	if err != nil {
		panic(err)
	}

	// 2. 创建背景面板容器 (自动适配主题背景色)
	root := goui.NewBackgroundPanel()
	root.SetPadding(20)

	// 3. 添加品牌标题
	title := goui.NewBrandLabel("你好，goui！")
	title.SetFont(goui.NewFont("Microsoft YaHei", 18))
	title.SetBounds(20, 20, 380, 32)
	root.Add(title)

	// 4. 添加次要说明文本
	desc := goui.NewSecondaryLabel("无 CGO 依赖 · 纯 Go 软件双缓冲光栅化渲染")
	desc.SetBounds(20, 58, 380, 20)
	root.Add(desc)

	// 5. 添加交互按钮
	clickCount := 0
	btn := goui.NewPrimaryButton("点击交互")
	btn.SetBounds(20, 100, 160, 36)
	btn.OnClick(func() {
		clickCount++
		btn.SetText(fmt.Sprintf("已点击 %d 次", clickCount))
	})
	root.Add(btn)

	// 6. 挂载到窗口并进入 Win32 消息循环
	win.SetContent(root)
	win.Run()
}
```

### 编译与运行

在 Windows PowerShell 终端中执行：

```powershell
# 1. 调试运行 (带有控制台日志回显)
go run ./cmd/gnote

# 2. 生产构建 (自动隐藏黑色控制台窗口，并执行 gofmt/vet/test 门禁)
./build.ps1
```

---

## 4. 参考应用 (cmd/gnote)

仓库不单独维护示例目录；`cmd/gnote` 本身就是覆盖全部子系统的参考应用：

- `cmd/gnote/main.go`：内嵌字体注册、窗口创建与启动流程。
- `cmd/gnote/app.go`：命令注册表、菜单/状态栏布局、文件打开/保存与编码保真。
- `component/textarea.go`：虚拟化多行编辑器（折行、选区、IME、差分撤销）。
- `window/window.go`：零 CGO 窗口封装、DIB 双缓冲与消息泵。
- `render/font.go`：GDI 字形轮廓提取、批量字形解析与软件抗锯齿混合。
- `format/`：JSON / HTML（含内联 CSS/JS）格式化。

运行与构建方式同上一节；打包发布使用 `./build.ps1 -Package`。
