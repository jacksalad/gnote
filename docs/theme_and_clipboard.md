# goui 主题系统与系统剪贴板集成 (Theme & Clipboard)

> 本文档详细介绍了 `goui` 现代化设计令牌与主题系统、动态热切换广播机制，以及纯 Go 实现的 Windows 原生剪贴板双向读写。

---

## 1. 现代化设计令牌体系

`goui` 内置了一套工业级的设计令牌（Design Tokens）体系，统一定义于 [`theme/theme.go`](../theme/theme.go)，跨语言语法令牌定义于 [`theme/syntax.go`](../theme/syntax.go)。所有官方组件（Button, Label, TextBox, ProgressBar, Slider 等）与 16 个高亮器均深度适配该令牌规范。

内置主题通过 `theme.All()` 注册，`theme.ByID(id)` 查找：`dark`、`light`、`nord`、`tokyo-night`、`one-dark`、`gruvbox-dark`、`solarized-light`、`rose-pine`。每套主题必须同时提供 UI 令牌与语法令牌，避免出现“界面一套配色、代码高亮另一套配色”的割裂感。

### 1.1 色彩令牌字段定义

| 令牌字段名 | 语义角色说明 | Dark 暗黑预设示例 |
| :--- | :--- | :--- |
| `Background` | 窗口底层基准背景色 | `#18181B` (Zinc-900) |
| `Surface` | 卡片容器、编辑器、对话框背景底色 | `#27272A` (Zinc-800) |
| `SurfaceVariant` | 次级面板、输入框、悬停底色 | `#3F3F46` (Zinc-700) |
| `Primary` | 核心品牌强调色（主按钮、激活项） | `#6366F1` (Indigo-500) |
| `PrimaryHover` / `PrimaryActive` | 强调色悬停 / 按下状态 | `#818CF8` / `#4F46E5` |
| `OnPrimary` | 强调色之上的文字与图标颜色 | `#FFFFFF` |
| `TextPrimary` | 主要标题、正文字体颜色 | `#F4F4F5` (Zinc-100) |
| `TextSecondary` / `TextDisabled` | 次要说明 / 禁用项文本 | `#A1A1AA` / `#71717A` |
| `Border` / `BorderFocused` | 常规边框 / 聚焦边框 | `#3F3F46` / `#6366F1` |
| `FocusRing` | 按钮等控件的键盘焦点环 | `#6366F1` |
| `Track` / `Thumb` / `ThumbHover` | 进度条、滑块、滚动条轨道与滑块 | `#2D2D32` / `#6366F1` / `#818CF8` |
| `Selection` | 文本选区底色（半透明） | `rgba(99,102,241,110)` |
| `Shadow` | 菜单与弹层阴影 | `rgba(0,0,0,90)` |
| `Success` / `Warning` / `Danger` | 状态指示色 | `#22C55E` / `#EAB308` / `#EF4444` |
| `GutterBg` / `GutterText` / `GutterBorder` | 编辑器行号槽 | `#1C1E24` / `#6E7688` / `#2D303A` |
| `ActiveLineBg` / `ActiveLineBorder` | 当前行高亮与轮廓 | `#2F313A` / `#414552` |

### 1.2 跨语言语法令牌

`syntax.SyntaxPalette` 定义 `Keyword`、`Type`、`Builtin`、`Function`、`String`、`Number`、`Comment`、`Constant`、`Operator`、`Punctuation`、`Meta`、`Boolean`、`Null`、`Key`、`Tag`、`Attribute` 等语义角色，以及 Markdown 的标题 / 代码块 / 引用 / 任务列表等专用角色。

16 个语言高亮器不再各自维护两套硬编码调色板，而是把自身语法元素映射到共享令牌：

```go
func (h *GoHighlighter) ApplyTheme(t *theme.Theme) {
    if t == nil {
        return
    }
    h.colors = goColorsFromSyntax(t.SyntaxColors())
    h.isDark = t.Dark
}
```

`Theme.SyntaxColors()` 会按语义自动回退补齐可选角色（如 `Key←Type`、`Boolean←Keyword`、`TaskChecked←Success`），新主题只需定义核心十二三个颜色即可获得完整、协调的高亮体系。

### 1.3 主题质量回归

`theme/theme_test.go` 对全部内置主题执行：
- ID / 中文标签唯一性、`ByID` 大小写与空白归一化；
- `Dark` 标志与背景亮度一致性；
- 正文、次要文本、强调色文字、行号槽的最小对比度；
- 语法核心角色非透明且与编辑器表面保持可读对比；
- 字符串 / 关键字 / 类型 / 数字 / 注释之间不得出现同色冲突。

`cmd/gnote` 另有主题 ID 归一化、菜单子项、设置窗口应用与持久化测试，语法包则覆盖全部 16 个高亮器在全部主题下的应用。

---

## 2. 主题应用与运行期动态热切换

### 2.1 获取与切换主题
框架提供了简明的全局主题控制函数：
- `goui.CurrentTheme() *Theme`：获取当前生效的主题实例。
- `goui.Themes() []*Theme`：获取全部内置主题（顺序即菜单与设置页顺序）。
- `goui.ThemeByID(id string) *Theme`：按持久化标识查找主题，未知返回 `nil`。
- `goui.ThemeDark()` / `goui.ThemeLight()`：兼容入口，等价于 `ThemeByID("dark" | "light")`。
- `goui.SetTheme(newTheme)`：**动态热切换全局主题**。

`gnote` 的菜单“查看 → 主题”与设置页“外观 → 主题”均由 `goui.Themes()` 生成。菜单选中后立即热切换并勾选；设置页中选择主题会**即时预览**到主窗口与设置窗口，点击“应用”立即写入配置文件并在页脚显示“已保存”，窗口保持打开；点击“取消”或直接关闭设置窗口会回到**上一次应用**的主题。设置文件只保存主题 ID（如 `"theme": "tokyo-night"`），新增主题无需改动持久化格式。

### 2.2 全局热切换广播原理
当你调用 [`goui.SetTheme(t)`](../goui.go) 时：
1. 更新全局主题指针；
2. 自动调用 [`window.ApplyThemeToAllWindows(t)`](../window/window.go)；
3. 遍历所有已打开的 `Window` 实例，将其背景底色更新为 `t.Background`；
4. 递归对根容器下的每一个 `Themeable` 子组件调用 `ApplyTheme(t)`；
5. 自动触发整体重绘呈现。界面无需重启或销毁重建，毫秒级实现无缝昼夜模式切换。

### 2.3 实战：一键昼夜模式切换按钮
```go
btnTheme := goui.NewPrimaryButton("当前模式: 暗黑 (点击切换)")
btnTheme.SetBounds(20, 20, 220, 36)

isDark := true
btnTheme.OnClick(func() {
    if isDark {
        goui.SetTheme(goui.ThemeLight())
        btnTheme.SetText("当前模式: 明亮 (点击切换)")
        isDark = false
    } else {
        goui.SetTheme(goui.ThemeDark())
        btnTheme.SetText("当前模式: 暗黑 (点击切换)")
        isDark = true
    }
})
```

---

## 3. Windows 原生剪贴板系统集成

`goui` 在 [`clipboard/clipboard.go`](../clipboard/clipboard.go) 中提供了完全纯 Go 实现的 Windows 剪贴板集成，无需任何三方包或外部 CGO 依赖。

### 3.1 原理实现细节
- **动态调用 Win32 API**：
  - `user32.dll`：`OpenClipboard`, `EmptyClipboard`, `SetClipboardData`, `GetClipboardData`, `CloseClipboard`
  - `kernel32.dll`：`GlobalAlloc`, `GlobalLock`, `GlobalUnlock`, `GlobalFree`
- **UTF-16 多字节宽字符转换**：直接利用 Windows `CF_UNICODETEXT` 格式，与系统剪贴板原生双向互通，不发生乱码。

### 3.2 顶层门面 API
`goui` 在顶层对外导出了极简的使用接口：

```go
// 1. 读取系统剪贴板中的纯文本
text := goui.GetClipboardText()
fmt.Println("剪贴板内容:", text)

// 2. 将文本写入系统剪贴板 (成功返回 true)
ok := goui.SetClipboardText("https://github.com/jacksalad/gnote")
if ok {
    fmt.Println("已成功复制到系统剪贴板！")
}
```

### 3.3 与输入控件 TextBox 的天然协同
该模块已被内置集成在 [`TextBox`](../component/textbox.go) 中。用户在文本框中键入：
- `Ctrl+C`：自动调用 `clipboard.SetText` 将当前文本推送到操作系统剪贴板；
- `Ctrl+V`：自动调用 `clipboard.GetText` 读取系统文本并无缝插入光标所在位置；
- `Ctrl+X`：自动执行剪切操作。
在其他自定义组件中，开发者亦可随时调用此接口实现复制分享功能。
