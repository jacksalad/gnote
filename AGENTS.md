# gnote 项目全景上下文与架构设计规范 (AGENTS.md)

> **适用环境**：Windows 11 x64 | Go 1.25+ (推荐 Go 1.27+)  
> **核心定位**：无 CGO 依赖、纯 Go 打造的高性能跨语言语法高亮 Windows 桌面记事本与代码编辑器  

---

## 1. 项目概览与开发哲学

`gnote` 是一个专为 Windows 平台定制的现代化、高性能、轻量级、零 CGO 依赖的极速代码与文本编辑器。项目融合了以下底层设计哲学与工程规范：

- **零 CGO 依赖 (Pure Go)**：完全依赖 `syscall.NewLazyDLL` 对 Windows 原生 API（`user32.dll`、`gdi32.dll`、`kernel32.dll`、`comdlg32.dll`、`shell32.dll`）进行动态绑定，无任何外部 C/C++ 编译工具链依赖，编译速度极快，产物纯粹轻盈。
- **隐藏终端启动 (Native Windows GUI Subsystem)**：生产构建默认配置 `go build -trimpath -ldflags="-H windowsgui -s -w"`，将 PE 头子系统标记为 `IMAGE_SUBSYSTEM_WINDOWS_GUI (2)`，双击可执行文件直接启动图形界面，绝不弹出黑色控制台终端窗口。
- **全内置等宽抗锯齿字体 (Zero-Dependency Font)**：通过 Go `//go:embed` 技术将等宽中英文字体（`font.ttf`，6.50MB，9431 码位，family `Gnote Mono`，由 Maple Mono CN 与 LXGW WenKai Mono 子集合并而来，中英严格 2:1）直接打包进二进制内部，配合 GDI 内存私有字体注入接口（`AddFontMemResourceEx`）与 65 级灰度抗锯齿矢量光栅化算法，确保在任何全新 Windows 系统上均呈现绝对一致的高品质清晰字体渲染；字体许可证与出处见 `cmd/gnote/font-LICENSE.txt`。
- **软件双缓冲渲染引擎**：在内存中开辟 32 位 ARGB 像素连续缓冲区（`Buffer []uint32`），上层使用纯 Go 软件光栅化渲染流水线，绘制完成后通过 Win32 DIB Section 与 `BitBlt` 快速投递到窗口设备上下文（DC），彻底消除了原生重绘闪烁。
- **极简极客原生图标注入**：采用纯 Go 光栅算法绘制 16x16 与 32x32 极简科技风格专属图标，并通过 Windows PE 资源节（`.rsrc`，`gnote_windows_amd64.syso`）静态链接进可执行文件，在 Windows 任务栏、窗口标题栏、Alt+Tab 切换器以及资源管理器中实现原生高质感视觉展现。

---

## 2. 代码组织与模块拓扑

```
gnote/
├── go.mod                    # 模块定义 (module github.com/jacksalad/gnote, go 1.25.0)
├── goui.go                   # 顶层门面导出：类型别名、颜色常量、构造函数统一入口
├── AGENTS.md                 # 智能体工程上下文与项目规范文件 (本文档，自动载入)
├── .gitattributes            # 文本统一 LF（gofmt 门禁前提），二进制资源不转换
├── build.ps1                 # 一键构建脚本 (gofmt/vet/test 门禁 + 图标资源 + 终端隐藏 + 打包)
├── gnote.exe                 # 根目录下便捷可执行程序 (带极简图标、无终端控制台)
├── bin/                      # 二进制构建输出目录
│   └── gnote.exe             # 最终编译的生产级应用程序
├── cmd/
│   └── gnote/                # gnote 应用程序核心业务层与入口
│       ├── main.go           # 启动入口 (字体注册、窗口创建、命令行文件)
│       ├── app.go            # 应用状态、菜单/状态栏布局、文件与编辑命令
│       ├── commands.go       # 命令注册表与动态 Enabled/Checked
│       ├── findbar.go        # 查找/替换条
│       ├── font.ttf          # 内嵌等宽中英文字体 Gnote Mono (6.50MB, 9431 码位, 中英 2:1)
│       ├── font-LICENSE.txt  # 内嵌字体出处、OFL-1.1 许可证与子集/合并说明
│       ├── icon.go           # 极简专属极客 Logo 矢量光栅生成器 (16x16 + 32x32) 与 ICO 打包器
│       ├── icon.ico          # 标准多尺寸 Windows ICO 图标文件
│       ├── icon_test.go      # 图标二进制生成、PE 资源节与 Windows GUI 子系统自动化测试
│       ├── gnote.manifest    # Windows 原生应用清单 (asInvoker 杜绝提权威胁、Win10/11 兼容性、DPI感知)
│       ├── gnote.rc          # Windows PE 资源描述脚本 (整合图标、Manifest 与 VERSIONINFO 元数据)
│       ├── settings.go       # Settings 模型、%AppData%/便携模式持久化与原子写
│       ├── settings_window.go# 独立模态设置窗口 (TabBar/Switch/Segmented/Slider)
│       ├── statusbar.go      # 状态栏分段与原生弹出菜单 (编码/EOL/语言/缩放)
│       ├── gnote_windows_amd64.syso # 自动合并进 .exe 的 COFF PE 资源目标文件
│       ├── *_test.go         # 命令/查找条/格式化/菜单/设置/状态栏/图标/布局稳定性测试
│       └── gnote_test.go     # 组件链路集成、布局渲染、首击响应与关闭拦截
├── graphics/                 # 图形基础设施与空间几何模型
│   ├── types.go              # Color (0xAARRGGBB), Point, Size, Rect 及几何运算
│   └── font.go               # Font 抽象与 DefaultFont 定义
├── render/                   # 纯 Go 软件光栅化渲染引擎
│   ├── canvas.go             # Canvas 像素缓冲区、剪裁栈、直线/圆角/矩形光栅化与脏标记
│   ├── font.go               # GDI 矢量字体轮廓提取 (GetGlyphOutlineW) 与抗锯齿像素混合
│   ├── image.go              # 纯 Go 像素图像抽象与解码器集成 (PNG/JPEG/GIF)
│   └── renderer.go           # SoftwareRenderer 抽象与生命周期管理
├── syntax/                   # 纯 Go 词法分词与 16 种语言语法高亮引擎
│   ├── highlighter.go        # Highlighter 接口、Span 样式片段抽象
│   ├── registry.go           # DetectHighlighter 文件名后缀嗅探与 GetLanguageName
│   ├── go.go                 # Go 语言高亮状态机
│   ├── python.go             # Python 语言高亮状态机
│   ├── c.go                  # C/C++ 语言高亮状态机
│   ├── csharp.go             # C# 语言高亮状态机
│   ├── java.go               # Java 语言高亮状态机
│   ├── rust.go               # Rust 语言高亮状态机
│   ├── r.go                  # R 语言高亮状态机
│   ├── javascript.go         # JavaScript 现代 ES 语言高亮状态机
│   ├── typescript.go         # TypeScript 语言高亮状态机
│   ├── html.go               # HTML 网页标记高亮状态机
│   ├── css.go                # CSS 样式表高亮状态机
│   ├── sql.go                # SQL 查询与数据库脚本高亮状态机
│   ├── markdown.go           # Markdown 格式高亮状态机
│   ├── json.go               # JSON 配置文件高亮状态机
│   ├── yaml.go               # YAML 映射与标量高亮状态机
│   └── toml.go               # TOML 配置文件高亮状态机
├── component/                # UI 组件库与布局系统
│   ├── component.go          # Component, Container, Clickable, CursorProvider 核心接口
│   ├── textarea.go           # TextArea 核心多行编辑器 (增量折行缓存、行虚拟化、选区、行号槽、差分 Undo/Redo)
│   ├── panel.go              # Panel 容器与 Flow/Grid/Border 经典布局引擎
│   ├── button.go             # Button 按钮 (状态响应、平滑圆角矩形)
│   ├── label.go              # Label 文本标签 (对齐、裁剪、省略号、可点击)
│   ├── menubar.go            # MenuBar 全客户区自绘菜单层 (下拉/子菜单/键盘)
│   ├── switch.go             # Switch 开关与 Segmented 分段选择器
│   ├── tabbar.go             # TabBar 纵向分组导航
│   ├── textbox.go            # TextBox 单行输入框 (Rune 寻址、光标闪烁、剪贴板支持)
│   ├── scrollview.go         # ScrollView 视口裁剪与鼠标滚轮滚动视图
│   ├── checkbox.go           # CheckBox 复选框
│   ├── radiobutton.go        # RadioButton 单选框与互斥组
│   ├── progressbar.go        # ProgressBar 进度条
│   ├── slider.go             # Slider 滑动条
│   └── imageview.go          # ImageView 图像控件
├── window/                   # Windows 操作系统级窗口封装
│   ├── win32.go              # Win32 API 动态调用、Win32 常量与结构体映射
│   ├── window.go             # Window 封装、DIBSection 建立、消息泵、动态光标、居中与关闭拦截
│   └── icon.go               # 纯 Go 32bpp 原生 HICON 图标构建、双线性插值与 ICO 解析器
├── clipboard/                # Windows 原生剪贴板系统集成 (纯 Go UTF-16 双向读写)
│   └── clipboard.go          # GetText, SetText
├── dialog/                   # Windows 原生文件与通用消息对话框
│   └── dialog.go             # OpenFileDialog, SaveFileDialog, MessageBox
├── format/                   # 零依赖文档格式化引擎 (JSON/HTML)
│   ├── format.go             # KindForPath 与统一 Format 入口
│   ├── json.go               # encoding/json token 级缩进 (键序与数字字面量不变)
│   ├── html.go               # 纯 Go HTML 分词与块级缩进 (pre/svg 原文，script/style 递归格式化)
│   ├── css.go                # 内联 CSS 规则/声明排版 (字符串与 url/data URI 原样保留)
│   └── js.go                 # 内联 JS 保守排版 (保留 ASI 换行、正则、模板字符串与注释原文)
├── theme/                    # 现代化设计令牌与主题系统
│   ├── theme.go              # 八套预设主题 (Dark/Light/Nord/Tokyo Night/One Dark/Gruvbox/Solarized Light/Rosé Pine) 与注册表
│   └── syntax.go             # 跨语言语法高亮语义令牌与自动回退补齐
├── textio/                   # 文本编码与换行保真层 (零 CGO)
│   └── textio.go             # BOM/UTF-8/UTF-16/ANSI 编码探测与编解码、EOL 分析、文件身份戳
├── event/                    # 事件类型、数据载荷与线程安全事件总线
│   └── event.go
└── docs/                     # 架构设计与各子系统深度技术文档
```

---

## 3. gnote 核心功能与技术实现

### 3.1 核心编辑器引擎 (`component/textarea.go`)
- **流式物理画笔累进排版**：采用连续物理画笔累进排版算法，彻底消除高亮加粗（Bold）或样式切换时由于字形微小膨胀造成的后续标记与字符重叠问题；
- **列对齐空格 (AlignSpace)**：内嵌 `Gnote Mono` 为等宽字体，空格/数字/字母步进一致、中文严格 2 格，1 空格 = 1 字符、`\t` 与 4 空格缩进 = 4 个英文字符；`graphics.Font.AlignSpace` 仍对编辑器字体开启，作为更换比例字体时的兜底（空格步进取数字 `0` 宽度），测量/绘制/折行/光标共用同一字形步进；
- **智能平滑自动换行 (Word Wrap)**：支持按像素级精确折行，即使包含超长连续无空格文本或宽字符（中日韩），也保证严格在视口右侧平滑换行，绝不越界穿透；
- **行级视口虚拟化 (Row Virtualization)**：仅对当前视口可见区域执行实际字形计算与光栅化混合，支持万行级别大文件丝滑滚动；
- **增量视觉行索引 (Incremental Visual Index)**：`visualLines` / `logicalToVisual` 由 `visualState{clean,partial,full}` 驱动，编辑只重算受影响逻辑行的视觉分段并原地拼接，行数变化时仅平移尾部；`logicalToVisual` 追加末位哨兵（视觉行总数），普通按键不再 O(文档行数) 重建索引；
- **增量排版缓存 (Incremental Layout)**：每个逻辑行只缓存整行像素宽度（8B/行），折行点在重建视觉行时按需计算并即时消费；等宽字体先用列数上界排除必然不折行的短行（不查字形），关闭折行的行宽在可见时按需测量；字体面与字形宽度缓存复用；
- **超长行横向虚拟化**：关闭折行时为 ≥2048 列的行构建每 256 列的前缀宽度检查点；编辑只截断到编辑列、行尾输入增量续算；绘制时跳过视口外左右片段；
- **高亮 spans 行缓存**：可见逻辑行的 `[]syntax.Span` 按行缓存（上限 512 行），编辑按失效起点删除、主题切换整体清空；重绘与光栅化不再重复分词；
- **高亮状态短路**：编辑后若行尾词法状态未变化，下游行的状态与 span 缓存保持有效，普通按键只重分词编辑行；
- **零分配绘制路径**：`Canvas.DrawRunes` 直接消费 rune 切片，256 字符批量解析字形；行号槽用 `strconv.AppendInt` + 复用缓冲；光标 X 跨帧缓存并由 `measureEpoch` 失效；离屏字形只推进笔位；
- **光标局部重绘**：窗口定时器通过 `TextArea.CaretRect` 取光标单元，`Window.renderRegion` 只光栅化并 BitBlt 该矩形；未实现该接口的控件回退整窗重绘；
- **单次分配序列化**：`GetTextWithEOL` 预计算精确 UTF-8 字节数后逐 rune 追加，6MB 文档导出由 100k 次分配降至 2 次；
- **零分配键入合并**：连续单字符输入/删除直接追加到上一条撤销记录的 `inserted/removed` 缓冲，不构造新记录、不克隆 rune；键入热路径 0 分配。
- **宽度增量维护**：单行编辑命中已测量的行时，用插入/删除 rune 的步进宽度增减 `totalWidth`，不再为刷新上界重测整行；含换行的全部替换批量重建一次行偏移表，匹配列、removed/inserted 缓冲全程复用。
- **Gap Buffer 存储**：文档保存在单个 `text []rune` 间隙缓冲中，`lineStart []int` 记录每行起始逻辑偏移；每行不再有 24B 切片头、也不再每行一次分配。`line(r)` 返回零拷贝（三索引定容）视图，间隙落在行内时先挪到最近行边界；编辑按逻辑偏移搬运间隙并写入增量，长行输入不再克隆整行（20 万字符行每次输入 3.2 MB → 156 B）。视觉索引与高亮状态按行数预分配；
- **无级平滑缩放 (Smooth Zoom)**：原生支持 `Ctrl + 鼠标滚轮` 以及 `Ctrl + +/-/0` 实时缩放，支持从 50% 到 300% 动态无级字号重排；
- **差分撤销与重做栈 (Delta Undo / Redo)**：只记录每次编辑的 `removed/inserted` 片段与光标/选区，按 32MB 上限裁剪，避免大文档撤销占用数 GB 内存；`Ctrl + Z` 撤销与 `Ctrl + Y` 重做；`ReplaceDocument` 把整篇格式化替换记录为单步差分编辑并保留光标；
- **焦点保持**：菜单打开时由 MenuLayer 独占键盘，关闭后恢复编辑器/查找框焦点；点击状态栏不会清除编辑区焦点，键盘输入与编辑器快捷键持续可用；
- **行尾保真**：文档内部统一以 `\n` 存储，`SetLineEnding` / `GetTextWithEOL` 按原文件风格（CRLF/LF/CR，混合风格取主导者）序列化，保存不再擅自改写换行符；
- **查找与替换**：`FindNext` / `FindAll` / `ReplaceCurrentSelection` / `ReplaceAll` 提供 rune 级匹配（可选大小写敏感）与自动回绕；`ReplaceAll` 以复合 `editOp` 批量提交，**整体只占一步撤销**；
- **鼠标选取**：单击定位、双击按 Unicode 单词边界选词（中文按连续文字选词）、三击选中整行；Shift+单击保留原锚点、把选区扩展到点击位置（`window` 层从鼠标消息 `wParam` 的 `MK_SHIFT`/`MK_CONTROL` 位填充修饰键）；右键不再移动光标；
- **撤销合组**：相邻 700ms 内的连续单字符输入 / 退格 / Delete 合并为一步撤销（保存点与光标移动会打断合并），复合 `editOp` 保证多行反缩进、全部替换等批量操作同样单步可撤销；
- **智能缩进**：回车自动复制当前行前导空白，前一个非空字符为 `{` 时额外缩进一级；`Shift+Tab` 对当前行或选区覆盖的所有行反缩进（单步撤销）；`Ctrl+Backspace` / `Ctrl+Delete` 删除单词，`Ctrl+←/→` 单词跳转，`Ctrl+Home/End` 文档首尾；
- **输入法 (IME)**：拦截 `WM_IME_*`，组合串在光标处内联绘制（下划线 + 背景，后续文本整体右移），`ImmSetCompositionWindow` / `ImmSetCandidateWindow` 将组合窗口与候选列表贴靠真实光标；屏蔽系统默认组合窗口避免重影；
- **高亮挂载与文件嗅探**：打开或保存文件时，根据文件后缀名自动通过 `syntax.DetectHighlighter` 嗅探最佳语言词法解析器。

### 3.2 自绘菜单栏与状态联动 (`cmd/gnote/app.go` & `component/menubar.go`)
- **顶部菜单栏**：全客户区自绘 `MenuLayer`，文件/编辑/查看/设置/帮助五个菜单，查看菜单内嵌“主题”子菜单（八套主题单选勾选），支持一级子菜单、Alt+字母、F10、方向键、Esc、点击外部关闭与窄窗口翻转；菜单状态每次打开都从命令注册表重建，撤销/重做置灰与行号/换行/高亮/主题勾选实时生效；
- **命令注册表**：`cmd/gnote/commands.go` 以 `Command{ID, Title, Accel, Run, Enabled, Checked}` 统一菜单、右键菜单与快捷键，禁止同一动作多处实现；
- **独立设置窗口**：`cmd/gnote/settings_window.go` 使用 `Owner=主窗口, Modal=true` 的独立 HWND，TabBar 分组（编辑器/外观/关于），Switch/Segmented/Slider 控件；编辑器页可选择启动页面（介绍文档 / 空白页，`Settings.StartupPage`），外观页主题选择器由 `goui.Themes()` 动态生成并即时热预览，点击“应用”立即保存且窗口保持打开（页脚显示“已保存”），取消或直接关闭窗口回到上次应用的主题；
- **底部状态栏**：分段显示文件路径（超长省略）、`行:列 · 总行数`、编码、EOL、语言与缩放；后四段点击弹原生菜单（编码/EOL/语言/缩放）；窄窗口或高 DPI 下自动收起次要段，绝不重叠；
- **右键上下文菜单**：原生 Win32 弹出菜单（`dialog.ShowPopupMenu`），提供撤销/重做、剪切/复制/粘贴、全选、查找/替换，按选区与撤销栈实时置灰；
- **查找/替换条 (Ctrl+F / Ctrl+H, F3 下一个, Esc 关闭)**：`cmd/gnote/findbar.go` 提供增量查找、大小写开关、上一个/下一个、替换当前与全部替换；查找条与编辑器互斥持有键盘焦点，未处理按键不会串入正文；
- **格式化文档 (Ctrl+Shift+F，菜单/右键共用 `cmdEditFormat`)**：仅对 `.json` / `.html` / `.htm` 启用；统一 4 空格缩进（与编辑器 Tab 一致）；JSON 使用 `encoding/json` token 级缩进，键顺序、数字字面量（大整数/高精度小数）与转义原样保留；HTML 使用纯 Go 分词器只重排块级标签缩进，`pre/textarea/svg/math` 内部逐字保留；内联 `<style>` / `<script>` 在 MIME 类型可识别时分别走 CSS/JS 格式化器（保留 `url(...)`、正则、模板字符串与 ASI 换行，解析失败或类型未知则回退原文）；整篇替换通过 `TextArea.ReplaceDocument` 提交为单步撤销，光标保持原逻辑行列；
- **编码保真读写**：打开时自动识别 BOM / UTF-8 / UTF-16(LE/BE，含无 BOM 启发式) / 系统 ANSI 代码页（中文 Windows 即 GBK），二进制文件明确拒开；保存时按原编码回写（ANSI 下不可表示字符会提示并以 `?` 代替）；
- **外部修改检测**：打开/保存成功后记录文件身份戳（存在性、大小、mtime），保存前若检测到文件被外部修改或删除，弹出确认对话框，绝不静默覆盖；
- **原子保存**：先写同目录临时文件并 `Sync` 落盘，再原子替换目标文件，避免崩溃/断电/磁盘写满导致原文件被截断；单文件打开上限 64MB，超限给出明确提示；
- **生命周期关闭拦截**：挂载 `Window.SetOnClose` 钩子，当存在未保存脏修改时，拦截红叉、Alt+F4 或关闭动作，弹出“是否将更改保存”的原生对话框（支持是/否/取消三态分支）。

### 3.3 极简风格图标与 Windows PE 资源嵌入 (`cmd/gnote/icon.go` & `icon.ico`)
- 采用去除一切杂乱杂色的极简极客设计语言：
  - **16x16 尺寸**：由现代极客深蓝圆角底板、纯白文档卡片、统一科技天蓝代码条与青色光标指示组成，小尺寸下清晰锐利；
  - **32x32 尺寸**：包含平滑抗锯齿深蓝渐变圆角底板、折角文档白卡、带排版律动感的科技天蓝代码条以及终端光标点缀；
- 通过 `windres` 预编译生成 `gnote_windows_amd64.syso`，Go 编译时自动将其静态链接进可执行文件 `.rsrc` 节区。

---

### 3.4 性能基线 (Windows 11 x64 / 1560×920 视口)

| 场景 (100k 行 / 1560×920) | v3.0 | v4.0 |
| :--- | :--- | :--- |
| 全帧渲染 | 1.39 ms / 75 allocs | 0.71 ms / 0 allocs |
| 按键 + 全帧渲染 | 1.43 ms / 82 allocs | 0.73 ms / 6 allocs |
| 回车 + 退格 + 全帧渲染 | 2.4–2.7 ms | 2.05 ms |
| 全帧渲染（含 Markdown 高亮） | 1.41 ms / 75 allocs | 0.73 ms / 0 allocs |
| 20 万字符长行稳态渲染 | 13.7 ms | 0.18 ms（v4.1 横向虚拟化） |
| 100k 行两档缩放 + 渲染 | 96 ms | 15.9 ms（v4.1 惰性排版） |
| `GetText` 6.1 MB | 12.9 ms / 2 allocs | 13.2 ms / 2 allocs |
| 装载 10 万行 | 16.6 ms / 100k allocs | 17.3 ms / **5 allocs**（无 10 万行切片头） |
| 20 万字符行首输入 | 4.9 ms / 3.2 MB | **0.09 ms / 114 B** |
| 含换行全部替换（2 万行） | 逐行 O(行数) 偏移更新 | **3.7 ms / 70 allocs / 一次 O(文档) 重建** |
| 连续单字符输入 | 每键 1–2 allocs | **0 allocs / 26 B** |
| 新建 + 装载 + 首帧 | 27.9 ms / 54.0 MB / 158 allocs | **24.5 ms / 37.4 MB / 52 allocs** |

> 剩余大项（装载/缩放/全部替换/撤销回放）与 v4.1 结构性计划见 `docs/v4-roadmap.md`。

---

## 4. 关键类型与 API 导览

| 领域 | 核心结构 / 接口 | 工厂/关键函数 | 职责说明 |
| :--- | :--- | :--- | :--- |
| **主应用** | `App`, `Command`, `Settings` | `NewApp()`, `CommandRegistry`, `LoadSettings()` | 应用状态、文件保真读写、命令分发、设置持久化与菜单/状态栏布局 |
| **主题系统** | `theme.Theme`, `theme.SyntaxPalette` | `ThemeByID()`, `Themes()`, `SetTheme()`, `SyntaxColors()` | 八套 UI + 语法双令牌主题、注册表查找、运行期热切换与高亮语义回退 |
| **菜单与设置** | `MenuBar`, `Switch`, `Segmented`, `TabBar` | `NewMenuBar()`, `NewSwitch()`, `NewSegmented()`, `NewTabBar()` | 自绘菜单、设置分组导航与开关/分段选择控件 |
| **应用图标** | `Icon`, `HICON` | `GetGnoteIcon()`, `EnsureGnoteICOFile()` | 纯 Go 光栅生成极简专属图标，支持 ICO 导出与 Windows 原生句柄绑定 |
| **文本编辑** | `TextArea` | `NewTextArea()`, `FindNext()`, `ReplaceAll()`, `ReplaceDocument()`, `ImeAnchor()` | 多行文本编辑、自动折行、行号槽、高亮器接入、查找替换、整篇替换与 IME 组合串/光标锚点 |
| **文档格式化** | `format.Kind` | `KindForPath()`, `Format()`, `FormatJSON()`, `FormatHTML()` | JSON token 级缩进（键序/数字字面量不变）与 HTML 块级排版（pre/script/style/textarea 原文保留） |
| **高亮体系** | `Highlighter`, `Span` | `DetectHighlighter()`, `GetLanguageName()` | 16 种编程语言的纯 Go 状态机词法分词与样式生成 |
| **系统窗口** | `Window`, `WindowConfig` | `NewWindow()`, `SetOnClose()`, `Activate()`, `Run()` | 零 CGO 窗口创建、DIBSection 双缓冲、owner/modal、系统键分发与退出拦截 |
| **容器与布局** | `Panel`, `BorderLayout`, `FlowLayout` | `NewSurfacePanel()`, `NewBackgroundPanel()` | 顶层及局部容器排版、反向 Z-Index 事件冒泡与视口裁剪 |
| **文本编码** | `textio.Encoding`, `textio.EOL`, `textio.Stamp` | `Decode()`, `Encode()`, `AnalyzeEOL()`, `Stat()` | BOM/UTF-8/UTF-16/ANSI 代码页双向编解码、换行风格分析与文件外部修改检测 |
| **原生能力** | `clipboard`, `dialog` | `GetText()`, `SetText()`, `OpenFileDialog()`, `SaveFileDialog()` | 零 CGO Windows 剪贴板 UTF-16 文本读写与原生文件系统对话框集成 |

---

## 5. 构建、测试与运行工程规范

### 5.1 生产编译 (推荐，完全隐藏终端控制台)
使用提供的 PowerShell 一键构建脚本或执行标准构建命令：

```powershell
# 方式 A：通过自动化脚本一键构建 (gofmt/vet/test 门禁 + PE 图标/Manifest/VersionInfo + 终端隐藏 + 产物同步)
./build.ps1

# 跳过测试门禁（仅用于本地快速迭代）
./build.ps1 -SkipTests

# 方式 B：构建并生成代码签名与绿色便携分发包 (.zip，附带一键解除锁定脚本)
./build.ps1 -Sign -Package

# 方式 C：手动执行 Go 标准构建命令
go build -trimpath -ldflags="-H windowsgui -s -w" -o bin/gnote.exe ./cmd/gnote
```

> **构建参数释义**：
> - `-H windowsgui`：声明 Windows GUI 子系统（Subsystem 2），彻底消除控制台黑框；
> - `-s`：剥离符号表；
> - `-w`：剥离 DWARF 调试信息，进一步缩减体积、加速装载；
> - `-Sign`：使用 Authenticode 为可执行程序赋予数字签名；
> - `-Package`：生成免安装绿色发布压缩包并附带 MOTW 一键解除脚本。

### 5.2 调试模式编译 (保留终端标准输出/错误回显)
```powershell
./build.ps1 -Debug
# 或直接
go build -v -o bin/gnote.exe ./cmd/gnote
```

### 5.3 重新编译 PE 图标资源 (当修改 `gnote.rc` 或 `icon.ico` 后)
```powershell
windres -i cmd/gnote/gnote.rc -O coff -o cmd/gnote/gnote_windows_amd64.syso
```

### 5.4 质量门禁与回归基线 (本地 `build.ps1`，仓库不依赖远程 CI)
- **gofmt**：`gofmt -l .` 必须为空；`.gitattributes` 已把工作树统一为 LF，避免 Windows CRLF 误报；
- **go vet**：除 `window/window.go` 中两条 Win32 消息回调 `lParam` 指针还原告警（源码已注释说明）外，不允许任何输出；
- **测试**：`go test ./...`（含高亮器 fuzz 种子语料、剪贴板真实往返、视觉行索引一致性、主题对比度）；
- **构建**：`go build -trimpath -ldflags="-H windowsgui -s -w"`；
- `./build.ps1` 默认依次执行以上四项，`-SkipTests` 可按需跳过；
- **性能基线**：`component/bench_test.go` 提供渲染/按键/回车/高亮/序列化/装载/撤销/全部替换/缩放/长行光标十组基准，全部开启 `ReportAllocs`；`go test ./component -run '^$' -bench . -benchtime 5x` 作为冒烟门禁。

### 5.5 运行全量单元测试与质量验证
```powershell
# 运行 gnote 应用程序与 PE 节区验证测试
go test -v ./cmd/gnote

# 运行整个工程（包括底层组件库、渲染器与高亮器）全量测试
go test ./...
```

### 5.6 重新生成内嵌字体 (当需要更新 `cmd/gnote/font.ttf`)
```powershell
# 1. 从当前 font.ttf 的 cmap 提取字符集（保证码位覆盖不回退）
# 2. pyftsubset 子集化 Maple Mono CN-Regular（主字形，ASCII/CJK 严格 2:1）
# 3. 对差集字符再子集化 LXGW WenKai Mono-Regular（补全 707 个全角/圈号/部首/扩展 A 字形）
# 4. fontTools.merge 合并：丢弃 vhea/vmtx/OS/2/post 等冲突表，恢复 Maple 的 OS/2，post 置 3.0
# 5. 将 family 更名为 Gnote Mono（OFL 保留字体名合规），nameID 13 写入许可声明
```
> 字体来源、版权与 OFL-1.1 全文见 `cmd/gnote/font-LICENSE.txt`；改动字体后必须重跑
> `go test ./render ./component`（覆盖空格步进、2:1 列对齐与缩进宽度断言）。
