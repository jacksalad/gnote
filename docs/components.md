# goui UI 组件库完全指南 (Components)

> 本文档详细介绍了 `goui` 提供的全套 UI 基础控件、高级交互组件以及核心接口契约。

---

## 1. 组件核心接口契约

所有 UI 控件均遵循清晰、组合式的接口契约，定义于 [`component/component.go`](../component/component.go)：

### 1.1 [`Component`](../component/component.go) 基础接口
每个可视 UI 元素必须实现的最基础契约：
```go
type Component interface {
    Render(canvas *render.Canvas)          // 绘制自身到像素画布
    OnEvent(evt event.Event) bool          // 处理输入事件，返回 true 表示消费该事件
    SetBounds(x, y, width, height int32)  // 设置控件空间包围盒
    GetBounds() graphics.Rect             // 获取控件空间包围盒
    SetVisible(visible bool)              // 设置可见性
    IsVisible() bool                      // 查询可见性
    SetEnabled(enabled bool)              // 设置启用/禁用状态
    IsEnabled() bool                      // 查询启用/禁用状态
}
```

### 1.2 组合接口
- [`Container`](../component/component.go)：容器接口，具备 `Add`, `Remove`, `RemoveAll`, `GetChildren`, `GetChildAt`。
- [`Clickable`](../component/component.go)：可点击接口，具备 `OnClick(func())`, `SetHover`, `SetPressed`。
- [`TextComponent`](../component/component.go)：文本控件接口，具备 `SetText(string)`, `GetText() string`, `SetFont(*Font)`。
- [`Themeable`](../component/component.go)：具备 `ApplyTheme(*theme.Theme)`，当全局或容器主题切换时自动级联重绘。
- `CursorProvider`：具备 `GetCursor() int`，当鼠标移入该组件时，系统自动切换为对应的 Win32 光标（如 IDC_HAND 手型、IDC_IBEAM 工字输入标、IDC_CROSS 十字标）。

---

## 2. 按钮组件 [`Button`](../component/button.go)

`Button` 具备平滑抗锯齿圆角、自适应主题颜色以及完整的鼠标悬停（Hover）、按下（Pressed）、禁用（Disabled）状态机反馈。

### 2.1 构造与关键方法
| 构造 / 方法 | 说明 |
| :--- | :--- |
| `goui.NewButton(text string) *Button` | 创建标准次级按钮（跟随 SurfaceVariant 风格） |
| `goui.NewPrimaryButton(text string) *Button` | 创建品牌强调色按钮（跟随 Primary 风格） |
| `btn.SetPrimary(primary bool)` | 动态切换标准按钮与强调色按钮 |
| `btn.OnClick(callback func())` | 注册鼠标点击触发回调 |
| `btn.SetText(text string)` | 动态修改按钮文字内容 |
| `btn.SetCornerRadius(radius int32)` | 自定义平滑圆角半径（实际方法名为 `SetBorderRadius`，默认 6px） |
| `btn.SetEnabled(enabled bool)` | 禁用按钮（置灰并阻断点击事件） |

### 2.2 示例代码
```go
btn := goui.NewPrimaryButton("确认提交")
btn.SetBounds(20, 20, 140, 36)
btn.SetBorderRadius(8)
btn.OnClick(func() {
    fmt.Println("用户触发了提交操作！")
})
```

---

## 3. 文本标签 [`Label`](../component/label.go)

`Label` 基于 Windows GDI 矢量轮廓解析引擎，支持抗锯齿平滑渲染，提供水平与垂直排版对齐控制。

### 3.1 构造与对齐常量
- **构造函数**：
  - `goui.NewLabel(text)`：主正文标签（跟随 `TextPrimary`）
  - `goui.NewSecondaryLabel(text)`：次要说明文本（跟随 `TextSecondary`）
  - `goui.NewBrandLabel(text)`：品牌强调标题（跟随 `Primary`）
- **对齐方式**：
  - 水平对齐：`goui.AlignLeft`, `goui.AlignCenter`, `goui.AlignRight`
  - 垂直对齐：`goui.AlignTop`, `goui.AlignMiddle`, `goui.AlignBottom`
- **长文本与交互**：
  - `label.SetEllipsis(true)`：超出边界时截断并追加 `…`，渲染始终裁剪在标签矩形内；
  - `label.SetOnClick(func())`：设置点击回调并自动使用手型光标；
  - `label.SetTextColor(color)`：固定自定义颜色（不再跟随主题）。

### 3.2 示例代码
```go
title := goui.NewBrandLabel("系统性能监控中心")
title.SetFont(goui.NewFont("Microsoft YaHei", 18))
title.SetBounds(20, 15, 300, 32)
title.SetAlign(goui.AlignLeft)
title.SetVAlign(goui.AlignMiddle)
```

---

## 4. 单行输入框 [`TextBox`](../component/textbox.go)

`TextBox` 是高度完善的单行文本编辑控件，支持中文输入法内联组合与候选框定位、Unicode 变宽字符计算、光标 500ms 周期闪烁及快捷键。

### 4.1 核心特性
- **变宽字符光标精确寻址**：底层重构为 `[]rune` 切片，完美支持中文字符、Emoji 及多字节字符退格与删除。
- **系统剪贴板原生支持**：支持 `Ctrl+C`（全量复制）、`Ctrl+V`（粘贴剪贴板多字节 UTF-16 文本）、`Ctrl+X`（剪切）、`Ctrl+A`（全选定位至末尾）。
- **智能闪烁节能**：仅当输入框处于聚焦（Focused）且非只读状态时，Win32 定时器才驱动重绘，非聚焦时不消耗任何重绘开销。
- **工字光标反馈**：实现 `CursorProvider` 接口，鼠标悬停时自动切换为 `IDC_IBEAM`。

### 4.2 常用 API
| 方法 | 说明 |
| :--- | :--- |
| `goui.NewTextBox() *TextBox` | 创建文本输入框 |
| `txt.SetText(text string)` / `txt.GetText() string` | 设置或获取当前文本内容 |
| `txt.SetPlaceholder(hint string)` | 设置未输入时的占位提示符 |
| `txt.SetOnChange(func(val string))` | 文本内容变动时的实时回调 |
| `txt.SetOnSubmit(func(val string))` | 用户在输入框内按下回车键（Enter）时的提交回调 |
| `txt.SetReadOnly(ro bool)` | 设置是否为只读状态 |
| `txt.SetMaxLength(len int)` | 限制最大允许键入的 Rune 字符数（默认 512） |

### 4.3 示例代码
```go
txt := goui.NewTextBox()
txt.SetBounds(20, 60, 320, 36)
txt.SetPlaceholder("请输入用户名或邮箱地址...")
txt.SetOnChange(func(val string) {
    fmt.Println("输入变动:", val)
})
txt.SetOnSubmit(func(val string) {
    fmt.Println("用户按回车确认提交:", val)
})
```

---

## 5. 选择控件：CheckBox 与 RadioButton

### 5.1 复选框 [`CheckBox`](../component/checkbox.go)
支持点击切换选中勾选状态，并提供事件回调：
```go
cb := goui.NewCheckBox("开机自动后台静默启动")
cb.SetBounds(20, 110, 240, 24)
cb.SetChecked(true)
cb.OnChange(func(checked bool) {
    fmt.Println("配置状态更新为:", checked)
})
```

### 5.2 单选按钮 [`RadioButton`](../component/radiobutton.go) 与 单选组 [`RadioGroup`](../component/radiobutton.go)
通过 `RadioGroup` 实现多个单选框之间的自动互斥：
```go
rg := goui.NewRadioGroup()

rb1 := goui.NewRadioButton("DIB 内存双缓冲模式")
rb1.SetBounds(20, 150, 180, 24)
rb1.SetSelected(true)
rg.Add(rb1)

rb2 := goui.NewRadioButton("直接渲染极速模式")
rb2.SetBounds(210, 150, 180, 24)
rg.Add(rb2)

rb1.OnChange(func(selected bool) {
    if selected {
        fmt.Println("启用了双缓冲模式")
    }
})
```

---

## 6. 高级交互：ProgressBar 与 Slider

### 6.1 进度条 [`ProgressBar`](../component/progressbar.go)
支持平滑圆角矩形遮罩填充，可配置居中显示百分比数值文本：
```go
bar := goui.NewProgressBar()
bar.SetBounds(20, 190, 360, 22)
bar.SetRange(0, 100)
bar.SetValue(65)
bar.SetShowText(true)       // 显示 "65%" 居中文本
bar.SetCornerRadius(6)      // 设定平滑圆角
```

### 6.2 滑动条 [`Slider`](../component/slider.go)
支持鼠标悬停手型光标（`IDC_HAND`）、连续拖拽调节与点击直接寻址，支持设置调节步长（Step）：
```go
slider := goui.NewSlider(0, 100, 65)
slider.SetBounds(20, 230, 360, 24)
slider.SetStep(1.0) // 步长为 1，无级微调时设为 0
slider.OnChange(func(val float64) {
    bar.SetValue(val) // 联动更新进度条
})
```

---

## 7. 视图控件：ImageView 与 ScrollView

### 7.1 图像展示 [`ImageView`](../component/imageview.go)
支持 PNG、JPEG、GIF 纯 Go 原生解码显示，提供 4 种经典的等比例与缩放适配策略：

| 缩放模式常量 | 说明 |
| :--- | :--- |
| `goui.ScaleStretch` | 拉伸填满控件边界（可能变形） |
| `goui.ScaleFit` | **等比例完整适应**，居中展示（推荐） |
| `goui.ScaleFill` | 等比例填满容器，裁剪超出部分 |
| `goui.ScaleCenter` | 保持图像 1:1 原始物理分辨率居中显示 |

```go
iv, err := goui.NewImageViewFromFile("./assets/logo.png")
if err == nil {
    iv.SetBounds(20, 20, 200, 200)
    iv.SetScaleMode(goui.ScaleFit)
    iv.SetCornerRadius(8) // 平滑圆角边框
}
```

### 7.2 滚动视图容器 [`ScrollView`](../component/scrollview.go)
当内容尺寸超出可视视口时，`ScrollView` 提供严密的剪裁栈保护（`PushClip` / `PopClip`）、平滑鼠标滚轮滚动以及原生滚动条拖拽支持：

```go
// 1. 创建滚动视图并设定视口大小
sv := goui.NewScrollView()
sv.SetBounds(20, 20, 360, 400)

// 2. 创建内部超长内容面板
inner := goui.NewPanel()
inner.SetBounds(20, 20, 340, 1200) // 真实内容高度为 1200px

for i := 0; i < 30; i++ {
    itemBtn := goui.NewButton(fmt.Sprintf("列表项 #%02d", i+1))
    itemBtn.SetBounds(26, 26+int32(i*38), 320, 32)
    inner.Add(itemBtn)
}

// 3. 将内部组件挂载到 ScrollView 并注册内容真实宽高
sv.SetContent(inner)
sv.SetContentSize(340, 1200)
```
用户在视口范围内滑动鼠标滚轮或直接拖拽右侧滚动滑块，即可平滑滚动浏览。


## 8. 多行编辑器 [`TextArea`](../component/textarea.go)

`TextArea` 是面向大文档的多行代码/文本编辑器控件，内置行号槽、自动折行、语法高亮、增量排版、查找替换与输入法内联组合。

### 8.1 核心能力
- **增量视觉行索引**：编辑只重算受影响逻辑行的折行分段，普通按键与文档行数无关；拖拽窗口或切换折行时才整体重排。
- **查找与替换**：`FindNext` / `FindAll` / `ReplaceCurrentSelection` / `ReplaceAll`，rune 级匹配、可选大小写；`ReplaceAll` 作为单个复合编辑记录，一次撤销即可整体回退。
- **行尾保真**：内部统一以 `\n` 存储，`SetLineEnding` / `GetTextWithEOL` 按原文件风格（CRLF/LF/CR）序列化。
- **输入法**：组合串内联绘制并顺移后续文本，`ImeAnchor` 为窗口层提供组合窗口贴靠坐标。
- **撤销合组**：相邻 700ms 的连续输入/退格/Delete 合并为一步撤销，光标移动与保存点会打断合并。
- **列对齐空格**：内嵌 `Gnote Mono` 为等宽字体（空格/数字/字母步进一致、中文严格 2 格），1 个空格 = 1 个字符、`\t` 与 4 空格缩进 = 4 个英文字符宽度；`graphics.Font.AlignSpace` 作为更换比例字体时的兜底（空格步进取数字 `0` 宽度）。

### 8.2 常用 API
| 方法 | 说明 |
| :--- | :--- |
| `goui.NewTextArea() *TextArea` | 创建多行编辑器 |
| `ta.SetText(s string)` / `ta.GetText() string` | 设置/读取整篇文本 |
| `ta.SetLineEnding(eol string)` / `ta.GetTextWithEOL(eol string)` | 设置/按指定换行符导出 |
| `ta.SetHighlighter(h Highlighter)` | 挂载语法高亮器（`goui.DetectHighlighter(path)` 自动嗅探） |
| `ta.FindNext(q string, row, col int, forward, caseSensitive bool) (Match, bool)` | 查找并自动回绕 |
| `ta.ReplaceAll(q, repl string, caseSensitive bool) int` | 全部替换（单步撤销） |
| `ta.SetSelection(sr, sc, er, ec int)` / `ta.GetSelectionRange()` | 设置/读取选区 |
| `ta.Copy()` / `ta.Cut()` / `ta.Paste()` / `ta.SelectAll()` | 剪贴板与全选 |
| `ta.SetZoom(f float64)` / `ta.ZoomIn()` / `ta.ZoomOut()` | 无级缩放 |

---

## 9. 自绘菜单栏 [`MenuBar`](../component/menubar.go)

`MenuBar` 是一个**全客户区**菜单层：顶部 `BarHeight()` 像素绘制菜单栏，下拉菜单覆盖编辑器。它不能作为 34px 面板的子组件，否则会被 Panel 裁剪。

### 9.1 核心 API
| 方法 | 说明 |
| :--- | :--- |
| `goui.NewMenuBar() *MenuBar` | 创建菜单层 |
| `menu.SetBounds(0, 0, width, height)` | 必须铺满整个客户区 |
| `menu.SetModel(func() []goui.Menu)` | 设置动态模型；每次打开菜单重新求值 Enabled/Checked |
| `menu.SetMenus([]goui.Menu)` | 设置静态模型 |
| `menu.SetFont(font)` / `SetBarHeight(h)` / `SetItemPadding(p)` | 字体与顶部尺寸 |
| `menu.SetRowHeight(h)` / `SetSeparatorGap(g)` / `SetMenuMinWidth(w)` / `SetRadius(r)` | 下拉尺寸 |
| `menu.SetOnOpenChange(func(open bool))` | 打开/关闭时保存、恢复焦点 |
| `menu.Open(i)` / `Close()` / `IsOpen()` | 控制展开状态 |

`goui.Menu{Text, Items}`，`goui.MenuEntry{Text, Accel, Disabled, Checked, Separator, Submenu, OnClick}`；子菜单支持一级。

### 9.2 接入示例
```go
menu := goui.NewMenuBar()
menu.SetBounds(0, 0, 1200, 800)
menu.SetOnOpenChange(func(open bool) {
    if open {
        goui.RequestFocus(menu)
    } else {
        goui.RequestFocus(editor)
    }
})
menu.SetModel(func() []goui.Menu {
    return []goui.Menu{{
        Text: "文件",
        Items: []goui.MenuEntry{
            {Text: "新建", Accel: "Ctrl+N", OnClick: newDocument},
            {Separator: true},
            {Text: "打开…", Accel: "Ctrl+O", OnClick: openFile},
        },
    }}
})
root.Add(menu) // 最后添加：事件优先且可覆盖编辑器
```

---

## 10. 设置控件：Switch / Segmented / TabBar

| 控件 | 构造 | 关键方法 | 用途 |
| :--- | :--- | :--- | :--- |
| `Switch` | `goui.NewSwitch()` | `SetOn`, `IsOn`, `SetOnChange` | 布尔设置项 |
| `Segmented` | `goui.NewSegmented([]string{...})` | `SetSelected`, `Selected`, `OnChange` | 2–6 个互斥选项 |
| `TabBar` | `goui.NewTabBar([]string{...})` | `SetSelected`, `Selected`, `OnSelect` | 设置页左侧分组导航 |

```go
sw := goui.NewSwitch()
sw.SetBounds(500, 64, 44, 22)
sw.SetOn(true)
sw.SetOnChange(func(on bool) { fmt.Println("switch:", on) })

theme := goui.NewSegmented([]string{"深色", "浅色"})
theme.SetBounds(24, 96, 220, 30)
theme.OnChange(func(i int) { fmt.Println("theme:", i) })

tabs := goui.NewTabBar([]string{"编辑器", "外观", "关于"})
tabs.SetBounds(0, 0, 168, 444)
tabs.OnSelect(func(i int) { pages[i].SetVisible(true) })
```
