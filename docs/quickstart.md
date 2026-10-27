# goui 快速入门指南 (Quickstart)

> 本文档将指引你从零搭建工程、理解核心配置，并在几分钟内构建出第一个现代化的 Windows 桌面应用程序。

---

## 1. 环境准备

### 1.1 系统与工具链要求
- **操作系统**：Windows 10 / Windows 11 x64（Win32 API 原生适配）
- **Go 编译器**：Go 1.25+（推荐使用 Go 1.27+）
- **C/C++ 工具链**：**完全不需要**（零 CGO 依赖，无需安装 MinGW、TDM-GCC 或 MSVC）

### 1.2 确认 Go 环境
打开 Windows PowerShell 7 (`pwsh`) 验证版本：

```powershell
go version
# 输出示例: go version go1.27.0 windows/amd64
```

---

## 2. 引入与依赖配置

在你的工程目录下初始化 Go Module 并引入 `goui`：

```powershell
mkdir my-goui-app
cd my-goui-app
go mod init my-goui-app
```

在代码中统一通过门面包导入：
```go
import goui "github.com/jacksalad/gnote"
```

或在 `go.mod` 中用 `replace` 指向本地克隆的仓库：
```go
// go.mod
module my-goui-app

go 1.25.0

require github.com/jacksalad/gnote v0.0.0
replace github.com/jacksalad/gnote => ../gnote // 本地开发路径
```

---

## 3. 编写首个应用程序

新建 `main.go` 并写入以下内容：

```go
//go:build windows

package main

import (
	"fmt"

	goui "github.com/jacksalad/gnote"
)

func main() {
	// 1. 定义窗口配置
	cfg := goui.WindowConfig{
		Title:     "goui 快速入门应用",
		Width:     480,
		Height:    320,
		X:         150,  // 初始屏幕 X 坐标 (可选，为 0 时居中或由系统决定)
		Y:         150,  // 初始屏幕 Y 坐标 (可选)
		Resizable: true, // 允许用户拉伸调整窗口大小
		Visible:   true, // 创建后立即呈现
	}

	// 2. 创建系统原生窗口
	win, err := goui.NewWindow(cfg)
	if err != nil {
		fmt.Printf("创建窗口失败: %v\n", err)
		return
	}

	// 3. 构建现代组件树
	// 使用 NewBackgroundPanel 作为全窗口根容器，自动铺满底色
	root := goui.NewBackgroundPanel()
	root.SetPadding(24)

	// 标题
	title := goui.NewBrandLabel("欢迎使用 goui GUI 引擎")
	title.SetFont(goui.NewFont("Microsoft YaHei", 18))
	title.SetBounds(24, 24, 400, 32)
	root.Add(title)

	// 次要文本
	desc := goui.NewSecondaryLabel("基于 Win32 DIB Section 内存双缓冲光栅化，零 CGO，轻快稳定。")
	desc.SetBounds(24, 60, 420, 20)
	root.Add(desc)

	// 计数卡片容器
	card := goui.NewSurfacePanel()
	card.SetBounds(24, 95, 430, 160)
	card.SetPadding(16)
	root.Add(card)

	// 状态反馈 Label
	infoLabel := goui.NewLabel("当前状态: 等待操作")
	infoLabel.SetBounds(40, 115, 380, 24)
	card.Add(infoLabel)

	// 交互主按钮 (NewPrimaryButton)
	count := 0
	btnIncrement := goui.NewPrimaryButton("递增计数")
	btnIncrement.SetBounds(40, 150, 120, 36)
	btnIncrement.OnClick(func() {
		count++
		infoLabel.SetText(fmt.Sprintf("当前计数: %d 次", count))
	})
	card.Add(btnIncrement)

	// 重置按钮 (NewButton)
	btnReset := goui.NewButton("重置")
	btnReset.SetBounds(170, 150, 80, 36)
	btnReset.OnClick(func() {
		count = 0
		infoLabel.SetText("当前状态: 已重置为 0")
	})
	card.Add(btnReset)

	// 4. 将根组件挂载到窗口
	// SetContent 会自动将根组件铺满客户区，并在窗口 Resize 时自动触发布局与呈现
	win.SetContent(root)

	// 5. 启动 Win32 消息驱动循环（阻塞至窗口关闭）
	win.Run()
}
```

---

## 4. 关键概念解析

### 4.1 窗口配置 [`WindowConfig`](../window/window.go)
- `Title`：窗口标题栏文本，支持动态通过 [`win.SetTitle(title)`](../window/window.go) 修改。
- `Width` / `Height`：设计时窗口客户区（Client Area）像素尺寸。`goui` 内部调用 Win32 `AdjustWindowRectExForDpi` 自动加上标题栏和边框厚度，确保你的绘制区域与设定完全吻合。
- `Resizable`：若设为 `false`，窗口边框固定，并自动移除最大化按钮与调整光标。
- `Visible`：创建完成后是否立即显示。

### 4.2 容器与自闭环渲染
- 调用 [`win.SetContent(root)`](../window/window.go) 将面板注册为窗口根节点。
- `goui` 采用**自闭环事件驱动重绘**：当按钮被点击、鼠标悬停状态变更或调用控件的 `SetText()` 时，组件事件处理器会返回 `true`，窗口内部调度层立即自动触发重绘，无需开发者手动调用 `Invalidate` 或 `Repaint`。

### 4.3 消息循环 [`win.Run()`](../window/window.go)
- 内部绑定操作系统线程（`runtime.LockOSThread()`），确保 Win32 消息泵与窗口句柄强亲和。
- 采用非阻塞与休眠等待机制，当无系统消息时自动挂起降低 CPU 占用至接近 0%。

---

## 5. 编译与分发

### 5.1 调试运行
在开发阶段，直接使用 `go run`。此时会保留一个控制台终端窗口，方便通过 `fmt.Println` 打印日志：

```powershell
go run main.go
```

### 5.2 生产发布（去除控制台黑框）
在最终打包桌面分发程序时，通过 `-ldflags="-H windowsgui"` 参数将子系统指定为 Windows GUI，即可彻底去除黑色控制台弹窗，并通过 `-s -w` 去除符号表与调试信息压缩体积：

```powershell
go build -trimpath -ldflags="-H windowsgui -s -w" -o "MyApp.exe" main.go
```

编译产物为一个独立的单二进制 `.exe` 文件，体积通常仅数兆字节，即拷即用。

---

## 6. 运行参考应用

`cmd/gnote` 是使用全部子系统的参考应用：

```powershell
# 1. 调试运行（保留控制台输出）
go run ./cmd/gnote

# 2. 一键构建（gofmt / vet / test 门禁 + 隐藏终端 + PE 资源）
./build.ps1

# 3. 构建绿色发布包
./build.ps1 -Package
```
