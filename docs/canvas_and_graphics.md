# goui 自定义绘图与图像光栅化 (Canvas & Graphics)

> 本文档深入介绍 `goui` 的空间几何数据模型、内存软件光栅化画布 [`Canvas`](../render/canvas.go)、纯 Go 像素位图 [`Image`](../render/image.go) 以及如何编写高性能自定义渲染组件。

---

## 1. 空间几何与色彩系统

定义于 [`graphics/types.go`](../graphics/types.go)：

### 1.1 色彩模型 [`Color`](../graphics/types.go)
`Color` 基于 32 位无符号整数表示，内存布局与 Windows DIB 32 位表面保持完全一致（`0xAARRGGBB`）：

```go
// 构造完全不透明的 RGB 颜色 (A 默认为 255)
c1 := goui.RGB(99, 102, 241)

// 构造支持半透明度的 RGBA 颜色
c2 := goui.RGBA(255, 0, 0, 128) // 50% 半透明红

// 获取各色彩通道 (0~255)
alpha := c2.A()
red   := c2.R()
green := c2.G()
blue  := c2.B()
```

### 1.2 空间几何运算
- [`Point`](../graphics/types.go)：二维点 `NewPoint(x, y int32)`
- [`Size`](../graphics/types.go)：二维尺寸 `NewSize(width, height int32)`
- [`Rect`](../graphics/types.go)：包围盒矩形 `NewRect(x, y, width, height int32)`
  - `rect.Contains(point)`：碰撞检测点是否在矩形内。
  - `rect.Intersect(other)`：计算两个矩形的相交区域（用于视口裁剪交集推导）。

---

## 2. 软件光栅化画布：Canvas

[`Canvas`](../render/canvas.go) 在内存中维护了一块平铺的连续像素切片 `Buffer []uint32`。所有的绘图指令全部由纯 Go 数学算法实现，无任何硬件加速与 CGO 桥接开销。

### 2.1 几何图元绘制 API

| 方法分类 | 方法签名 | 底层算法与特性 |
| :--- | :--- | :--- |
| **矩形填充** | `canvas.FillRect(x, y, w, h int32, color Color)` | 连续内存扫描线极速填充，受 Clip 裁剪 |
| **矩形边框** | `canvas.DrawRect(x, y, w, h int32, color Color)` | 边界防护四边线 |
| **平滑圆角矩形** | `canvas.FillRoundedRect(x, y, w, h, radius int32, color Color)` | **解析几何亚像素距离场平滑抗锯齿** |
| **圆角边框** | `canvas.DrawRoundedRect(x, y, w, h, radius int32, color Color)` | 亚像素弧线平滑抗锯齿 |
| **圆形填充** | `canvas.FillCircle(cx, cy, radius int32, color Color)` | 8x8 子采样覆盖率抗锯齿，全覆盖区段走扫描线极速填充 |
| **圆形线框** | `canvas.DrawCircle(cx, cy, radius int32, color Color)` | 1px 圆环覆盖率抗锯齿，与填充共用按半径缓存的掩码 |
| **任意斜线** | `canvas.DrawLine(x1, y1, x2, y2 int32, color Color)` | 整数 Bresenham 算法，支持越界保护与裁剪 |
| **水平/垂直线** | `canvas.DrawHorizontalLine(...)` / `DrawVerticalLine(...)` | 连续内存极速寻址版 |

### 2.2 亚像素平滑圆角算法
在绘制圆角矩形时，`goui` 避免了粗糙的锯齿边缘。四个角落应用了欧几里得解析几何距离场公式：
$$dist = \sqrt{\Delta x^2 + \Delta y^2}$$
当像素中心处于 $r - 0.5 < dist < r + 0.5$ 的亚像素过渡带时，根据边界覆盖率（Coverage）动态混合 Alpha，实现媲美 Direct2D / Skia 的细腻平滑圆角效果。

### 2.3 亚像素平滑圆形
`FillCircle` / `DrawCircle` 不使用中点圆算法或整数扫描线（这两者都会在斜向产生单像素锯齿颗粒），而是对直径 $2r+1$ 的包围盒做 8x8 子采样求精确面积覆盖率：实心圆统计子采样点落入 $dist \le r$ 的比例，1px 描边统计落入 $|dist - r| \le 0.5$ 的比例。

掩码按半径缓存（`getCircleCoverage`，与圆角掩码同样限制容量，半径 > 128 时只构建不缓存），实心填充时覆盖率恒为 1 的连续区段直接走 `FillRect` 扫描线快速填充，仅边界像素做 alpha 混合。因此 `NewSwitch` 旋钮、`Slider` 滑块、`RadioButton` 圆环等小半径图形在 100% DPI 下同样是连续光滑的圆弧，不再出现阶梯与颗粒。

### 2.4 定点整数 Alpha 像素混合 [`BlendPixel`](../render/canvas.go)
在合成半透明像素时，摒弃浮点除法运算，采用高性能定点整数混合：
```go
outColor = (srcColor * alpha + dstColor * (255 - alpha) + 127) / 255
```
既消除了色阶截断失真，又具备极致的执行性能。

---

## 3. 视口裁剪栈 (Clip Stack)

为了确保组件树嵌套（如 `ScrollView`、面板卡片、圆角遮罩）不会发生像素越界与层级穿透，`Canvas` 内置了严密的视口裁剪栈：

- `canvas.PushClip(rect graphics.Rect)`：压入新的裁剪矩形，新视口自动取与父级当前视口的**相交区域（Intersect）**。
- `canvas.PopClip()`：弹栈并恢复上一级视口。
- `canvas.Clip()`：查询当前生效的视口物理边界。

所有绘图 API（线条、矩形、文本、图像）底层均严格服从当前裁剪栈的边界限制。

---

## 4. 矢量文字排版与量算

`Canvas` 提供了与当前裁剪栈完全融合的中英文矢量文本排版方法：

```go
font := goui.NewFont("Microsoft YaHei", 14)

// 1. 精确测量文本所占用的像素宽高
text := "goui 软件光栅化渲染引擎"
width, height := canvas.MeasureText(text, font)

// 2. 绘制文本 (支持 \n 自动折行，服从当前 Clip 视口)
canvas.DrawText(text, 20, 20, font, goui.RGB(255, 255, 255))
```

**空格列对齐**：比例字体的原生空格通常只有 0.25em（约半个字母宽），代码缩进会偏窄；等宽字体天然满足“空格 = 1 个字符”。若使用比例字体又需要列对齐，可设置 `font.AlignSpace = true` 把空格步进固定为数字 `0` 的宽度（1 个字符），`\t` 与 4 空格缩进即等于 4 个英文字符，且测量、绘制、折行与光标定位一致生效：

```go
codeFont := goui.NewFont("Gnote Mono", 16) // 内嵌等宽字体，中英 2:1
codeFont.AlignSpace = true                 // 兜底：即使换成比例字体也按 1 字符推进空格
```

---

## 5. 纯 Go 像素图像：Image

[`render.Image`](../render/image.go) 是纯 Go 实现的 32 位 ARGB 像素位图结构。

### 5.1 加载与转换
```go
// 1. 从本地文件加载并解码 (自动识别 PNG / JPEG / GIF)
img1, err := goui.LoadImageFromFile("./logo.png")

// 2. 从内存字节切片解码 (如嵌入在 go:embed 中的静态资源)
img2, err := goui.LoadImageFromBytes(rawPngBytes)

// 3. 创建空白画布并在上面动态逐像素绘制
img3 := goui.NewImage(100, 100)
img3.SetPixel(50, 50, goui.RGB(255, 0, 0))

// 4. 将 Go 标准库 image.Image 转换为 *render.Image
stdImg, _, _ := image.Decode(reader)
gouiImg := render.FromGoImage(stdImg)
```

### 5.2 在 Canvas 上绘制图像
```go
// 1:1 原始物理分辨率绘制
canvas.DrawImage(img, dstX, dstY)

// 缩放绘制到目标矩形区域 (支持 Alpha 半透明融合与 Clip 裁剪)
targetRect := goui.NewRect(20, 20, 150, 150)
canvas.DrawImageScaled(img, targetRect)
```

---

## 6. 实战：开发高性能自定义组件

通过继承 [`BaseComponent`](../component/component.go)，你可以极速编写自定义控件。

以下范例演示了一个**动态波形/仪表盘监视器**自定义组件：

```go
type WaveformMonitor struct {
    goui.BaseComponent
    history   []float64
    maxPoints int
}

func NewWaveformMonitor() *WaveformMonitor {
    return &WaveformMonitor{
        BaseComponent: goui.NewBaseComponent(),
        history:       make([]float64, 0, 100),
        maxPoints:     100,
    }
}

// 接收新的数据采样点
func (w *WaveformMonitor) Push(val float64) {
    if len(w.history) >= w.maxPoints {
        w.history = w.history[1:]
    }
    w.history = append(w.history, val)
}

// 1. 实现 CursorProvider 接口：鼠标悬停在波形图上呈现十字准星
func (w *WaveformMonitor) GetCursor() int {
    return 32515 // Win32 IDC_CROSS
}

// 2. 实现 OnEvent：处理鼠标点击交互
func (w *WaveformMonitor) OnEvent(evt goui.Event) bool {
    if evt.Type == goui.EventMouseDown {
        if mouse, ok := evt.Data.(goui.MouseData); ok {
            if w.GetBounds().Contains(goui.NewPoint(mouse.X, mouse.Y)) {
                fmt.Println("用户在波形监视器上进行了标记采样！")
                return true // 消费事件，阻断向下冒泡
            }
        }
    }
    return false
}

// 3. 实现 Render：执行软件光栅化渲染
func (w *WaveformMonitor) Render(canvas *goui.Canvas) {
    if !w.IsVisible() {
        return
    }
    b := w.GetBounds()

    // 压入自身裁剪视口，防止线条越界
    canvas.PushClip(b)
    defer canvas.PopClip()

    // 绘制深灰背景与边框
    canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, 8, goui.RGB(20, 20, 24))
    canvas.DrawRoundedRect(b.X, b.Y, b.Width, b.Height, 8, goui.RGB(60, 60, 70))

    // 绘制网格基准线
    midY := b.Y + b.Height/2
    canvas.DrawHorizontalLine(b.X+4, b.X+b.Width-5, midY, goui.RGBA(255, 255, 255, 40))

    // 绘制波形数据折线
    if len(w.history) > 1 {
        stepX := float64(b.Width-16) / float64(w.maxPoints-1)
        for i := 0; i < len(w.history)-1; i++ {
            x1 := b.X + 8 + int32(float64(i)*stepX)
            y1 := midY - int32(w.history[i]*float64(b.Height/2-10))
            x2 := b.X + 8 + int32(float64(i+1)*stepX)
            y2 := midY - int32(w.history[i+1]*float64(b.Height/2-10))

            canvas.DrawLine(x1, y1, x2, y2, goui.RGB(34, 197, 94)) // 翠绿折线
        }
    }
}
```
通过上述标准模式，任何基于内存像素光栅化的复杂专业图表、手绘画板或 CAD 绘图板均可在 `goui` 中轻快稳定地构建。
