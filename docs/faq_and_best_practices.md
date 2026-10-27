# goui 最佳实践与常见问题解答 (FAQ & Best Practices)

> 本文档汇总了在 Windows 平台上使用 `goui` 进行高并发交互、工程性能调优、无控制台分发打包及常见排错指南。

---

## 1. 线程模型与多 Goroutine 并发安全

### 1.1 Win32 UI 线程亲和性
Win32 窗口句柄（`HWND`）与系统消息队列与创建它的操作系统物理线程具有强绑定关系。为此，`goui` 在 [`win.Run()`](../window/window.go) 中默认执行了 `runtime.LockOSThread()`，防止 Go 运行时将主协程调度到其他系统线程上。

### 1.2 后台 Worker 协程更新 UI 的标准范式
**严禁**在后台未锁定的异步 Goroutine 中直接大规模调用非线程安全的底层绘图逻辑。对于耗时的后台任务（如 HTTP 请求、计算密集型运算、串口/网络监听），推荐以下两种标准范式：

#### 范式 A：通过 `win.RequestRedraw()` 触发重绘（最轻量推荐）
```go
go func() {
    for {
        time.Sleep(1 * time.Second)
        // 1. 获取最新业务数据
        val := fetchSensorData()

        // 2. 更新组件内部状态 (纯内存安全赋值)
        progressBar.SetValue(val)
        statusLabel.SetText(fmt.Sprintf("实时采集: %.1f", val))

        // 3. 通知系统窗口调度重绘
        win.RequestRedraw()
    }
}()
```

#### 范式 B：利用 Channel 进行主协程消息汇总
在应用层建立事件通知通道，结合定时器或自定义事件驱动界面刷新。

---

## 2. 性能与 GC (垃圾回收) 优化最佳实践

由于 `goui` 采用纯 CPU 软件双缓冲光栅化，避免不必要的堆内存分配对维持 60 FPS 稳定帧率至关重要：

### 2.1 避免在 `Render()` 中分配切片
自定义组件在实现 `Render(canvas *goui.Canvas)` 时，**严禁**在每帧循环中执行 `make([]...)`、大字符串拼接或内存分配。
- **推荐**：在组件构造函数中预开辟固定大小的缓冲区并循环复用（如 `TextArea` 复用 `gutterRunes` 行号缓冲；`render` 以 256 字符为批次解析字形，避免每帧 `string(runes)` 转换）。

### 2.2 善用局部裁剪栈 (Clip Stack)
在需要渲染长列表、滚动视图或嵌套卡片时，务必调用 `canvas.PushClip(bounds)` 与 `canvas.PopClip()`。视口外的图元会被底层的矩形相交检测直接剔除，避免浪费 CPU 光栅化计算。

### 2.3 字体渲染缓存
`goui` 内部对字体实例与字形灰度蒙版建立了全局带读写锁的 LRU/哈希缓存池（[`fontCache`](../render/font.go)）。无需在每次绘制时重复创建 `HFONT`，字符覆盖率会被自动缓存，第二次绘制时直接读取已有蒙版。

---

## 3. Windows 生产发布与打包指南

### 3.1 彻底去除黑框控制台窗口
在默认情况下，Go 编译的二进制文件是 `console` 控制台子系统，运行时会弹出一个黑色 CMD 窗口。生产发布时使用 `-H windowsgui` 参数将其指定为标准 GUI 子系统：

```powershell
go build -trimpath -ldflags="-H windowsgui -s -w" -o "MyApp.exe" main.go
```
- `-H windowsgui`：屏蔽黑框弹窗。
- `-s`：剥离符号表（Symbol Table）。
- `-w`：剥离 DWARF 调试信息，极大压缩最终可执行文件体积。

### 3.2 注入应用程序图标 (.ico) 与 Windows 清单 (Manifest)
要为生成的 `.exe` 文件添加专属桌面图标以及高版本 Windows 清单，推荐使用社区工具 [`goversioninfo`](https://github.com/josephspurrier/goversioninfo)：

1. 安装工具：
   ```powershell
   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
   ```
2. 在工程根目录下创建 `versioninfo.json`：
   ```json
   {
       "FixedFileInfo": {
           "FileVersion": { "Major": 1, "Minor": 0, "Patch": 0, "Build": 0 },
           "ProductVersion": { "Major": 1, "Minor": 0, "Patch": 0, "Build": 0 }
       },
       "StringFileInfo": {
           "CompanyName": "YourCompany",
           "FileDescription": "MyApp Desktop Application",
           "ProductName": "MyApp"
       },
       "IconPath": "app.ico",
       "ManifestPath": ""
   }
   ```
3. 生成资源并编译：
   ```powershell
   goversioninfo
   go build -trimpath -ldflags="-H windowsgui -s -w" -o "MyApp.exe" main.go
   ```
此时生成的 `.exe` 在 Windows 资源管理器中将直接展示你的定制矢量图标。

### 3.3 零环境依赖优势
`goui` 编译生成的二进制文件**没有任何三方 C/C++ 动态链接库依赖**，甚至不需要目标机器预装 VC++ Redistributable（MSVCR/MSVCP）。只要是 Windows 10/11 x64 系统，直接复制即可双击运行。

---

## 4. 常见问题排错 (Troubleshooting)

### Q1: 窗口内容在拉伸大小时出现留白或未撑满？
- **原因**：根组件没有使用 `win.SetContent(root)`，或者子组件完全采用了固定绝对坐标，未配置响应式布局。
- **解决方案**：
  1. 确保使用 `win.SetContent(root)`，窗口在捕获 `WM_SIZE` 时会自动将根组件边界拉伸到当前客户区物理尺寸；
  2. 推荐根容器使用 `BorderLayout`，或在需要自适应填充的容器中配置 `GridLayout`。

### Q2: 在 4K / 2K 高分屏上显示是否会模糊？
- **解答**：不会。`goui` 启动时默认开启了微软最高优先级的 `DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2` 感知上下文，并在窗口创建与跨屏拖拽时精准处理了边框与像素缩放。所有直线、矢量文字、圆角均为物理像素点对点精准光栅化，绝无二次位图放大模糊。

### Q3: 中文多字节输入时文字偶现乱码？
- **解答**：`goui` 的 [`TextBox`](../component/textbox.go) 与 [`TextArea`](../component/textarea.go) 底层全面采用 Go 原生 `[]rune` 存储；窗口层拦截 `WM_IME_STARTCOMPOSITION` / `WM_IME_COMPOSITION` / `WM_IME_ENDCOMPOSITION`，组合串在光标处以带下划线的内联形式绘制，并通过 `ImmSetCompositionWindow` / `ImmSetCandidateWindow` 把组合窗口与候选列表贴靠真实光标；上屏文本走 `GCS_RESULTSTR` 一次性提交，不会重复插入。变宽字符退格、中文光标截断与 Emoji（代理对）均已被处理。

### Q4: 如何在窗口被关闭前执行确认或清理逻辑？
- **解决方案**：通过窗口自带的事件总线订阅 [`EventClose`](../event/event.go) 事件：
  ```go
  win.EventBus().SubscribeHandler(goui.EventClose, func(e goui.Event) {
      fmt.Println("正在保存用户配置数据并释放资源...")
      // 执行清理操作
  })
  ```
