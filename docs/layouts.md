# goui 布局系统与容器排版 (Layouts)

> 本文档深入介绍 `goui` 的容器模型 [`Panel`](../component/panel.go) 以及三大自动布局引擎：[`FlowLayout`](../component/panel.go)、[`GridLayout`](../component/panel.go) 和 [`BorderLayout`](../component/panel.go)。

---

## 1. 核心容器：Panel

`Panel` 是 `goui` 中构建组件树与界面的核心基石，支持任意层级的树状嵌套、内边距（Padding）、边框（Border）、背景色以及主题角色染色。

### 1.1 面板语义角色 [`PanelRole`](../component/panel.go)
通过指定语义角色，面板在全局主题切换（如 Dark/Light 切换）时会自动应用规范的设计令牌，无需逐一手写改色：

| 角色常量 | 构造工厂 | 适用场景与默认样式 |
| :--- | :--- | :--- |
| `goui.PanelRoleBackground` | `goui.NewBackgroundPanel()` | 应用程序根容器；跟随 `t.Background` 底色 |
| `goui.PanelRoleSurface` | `goui.NewSurfacePanel()` | 卡片、对话框主体；跟随 `t.Surface`，带 1px `t.Border` |
| `goui.PanelRoleSurfaceVariant` | `goui.NewPanelWithRole(...)` | 次级操作卡片、状态预览块；跟随 `t.SurfaceVariant` |
| `goui.PanelRoleNone` | `goui.NewPanel()` | 透明或完全自定义背景色，不被主题全局策略覆盖 |

### 1.2 常用属性与方法
```go
panel := goui.NewSurfacePanel()
panel.SetBounds(20, 20, 400, 300) // 设置绝对物理包围盒
panel.SetPadding(16)              // 设置内部组件安全内边距 (Padding)
panel.SetBorder(th.Border, 1)     // 设置边框颜色与宽度
panel.SetBackground(th.Surface)   // 手动设置背景颜色
```

---

## 2. 布局接口与级联计算

在 `goui` 中，布局器需实现 [`Layout`](../component/panel.go) 接口：

```go
type Layout interface {
    Calculate(container graphics.Rect, children []Component)
}
```

当调用 `panel.SetBounds(...)` 或向面板 `Add(child)` 时，容器会自动调用 [`panel.DoLayout()`](../component/panel.go)。如果子元素中包含嵌套子面板，布局计算会**递归向下级联传递**，确保整棵树的位置与尺寸同步刷新。

---

## 3. 流式布局：FlowLayout

[`FlowLayout`](../component/panel.go) 类似 Web 中的 Flex-wrap 流式排版。子组件按添加顺序从左到右排列，超出容器宽度时自动换行，并支持行内对齐。

### 3.1 配置参数
- `HGap` / `VGap`：水平与垂直间距（像素）。若为 0，则退化使用全局 `Gap`（默认 4px）。
- `Wrap`：是否允许自动换行（`bool`）。
- `HAlign`：行内水平对齐方式：
  - `goui.HAlignStart`：左对齐（默认）
  - `goui.HAlignCenter`：居中对齐
  - `goui.HAlignEnd`：右对齐

### 3.2 示例：标签栏自动换行排版
```go
tagContainer := goui.NewPanelWithRole(goui.PanelRoleSurfaceVariant)
tagContainer.SetBounds(20, 20, 360, 180)
tagContainer.SetPadding(12)

layout := goui.NewFlowLayout()
layout.HGap = 8
layout.VGap = 8
layout.Wrap = true
layout.HAlign = goui.HAlignStart
tagContainer.SetLayout(layout)

tags := []string{"Go 1.25+", "零 CGO", "Win32 API", "DIB 双缓冲", "矢量字体", "抗锯齿", "流式排版"}
for _, tag := range tags {
    btn := goui.NewButton(tag)
    btn.SetBounds(0, 0, 90, 32) // 提供设计时基础宽高
    tagContainer.Add(btn)
}
```

---

## 4. 网格布局：GridLayout

[`GridLayout`](../component/panel.go) 将容器空间划分为均匀的行与列网格。所有子组件被自动拉伸适配到对应单元格的物理尺寸中。

### 4.1 配置参数
- `Rows` / `Cols`：设定的行数与列数。
  - 若 `Rows` 设为 0，布局引擎会根据子组件总数自动推导所需行数。
- `HGap` / `VGap`：网格单元格之间的水平与垂直间隙。

### 4.2 示例：计算器键盘网格 (5行4列)
```go
grid := goui.NewPanel()
grid.SetBounds(20, 100, 320, 400)

gridLayout := goui.NewGridLayout(5, 4)
gridLayout.HGap = 8
gridLayout.VGap = 8
grid.SetLayout(gridLayout)

keys := []string{
    "C", "⌫", "%", "÷",
    "7", "8", "9", "×",
    "4", "5", "6", "-",
    "1", "2", "3", "+",
    "±", "0", ".", "=",
}

for _, key := range keys {
    btn := goui.NewButton(key)
    if key == "=" {
        btn.SetPrimary(true) // 强调主按键
    }
    grid.Add(btn)
}
```

---

## 5. 边界布局：BorderLayout

[`BorderLayout`](../component/panel.go) 采用经典的“东西南北中”五区域空间切分模型，非常适合构建桌面软件的标准主界面（导航栏、侧边栏、状态栏与工作区）：

- `Top` / `Bottom`：顶部与底部区域，保持组件原有设计高度，水平方向自动铺满宽度。
- `Left` / `Right`：左侧与右侧区域，保持组件原有设计宽度，垂直方向填满剩余高度。
- `Center`：核心中心区域，**自动吸收并填满剩余的所有空间**。
- `Gap`：各区域之间的间距。

### 5.1 示例：桌面应用典型主框架
```go
root := goui.NewBackgroundPanel()
root.SetBounds(0, 0, 800, 600)

// 1. 顶部标题工具栏
header := goui.NewSurfacePanel()
header.SetBounds(0, 0, 800, 48)

// 2. 左侧快捷导航树
sidebar := goui.NewPanelWithRole(goui.PanelRoleSurfaceVariant)
sidebar.SetBounds(0, 0, 180, 500)

// 3. 底部状态栏
footer := goui.NewSurfacePanel()
footer.SetBounds(0, 0, 800, 28)

// 4. 中央主工作区
workspace := goui.NewSurfacePanel()

// 5. 应用边界布局
root.SetLayout(&goui.BorderLayout{
    Top:    header,
    Left:   sidebar,
    Bottom: footer,
    Center: workspace,
    Gap:    6,
})

root.Add(header)
root.Add(sidebar)
root.Add(footer)
root.Add(workspace)
```

---

## 6. 响应式拉伸与自适应最佳实践

当窗口尺寸发生改变（用户拖拽窗口边框）时：
1. `Window` 拦截 Win32 `WM_SIZE` 消息，根据新宽高自动重新调整根组件大小：`root.SetBounds(0, 0, newWidth, newHeight)`。
2. 根组件若配置了 `BorderLayout`、`GridLayout` 等布局引擎，会自动重新计算并把子容器撑开。
3. 推荐将窗口顶层根节点配置为 `BorderLayout`，各模块内部卡片配置为 `FlowLayout` 或 `GridLayout`，从而实现专业桌面软件级别的自适应响应式界面。
