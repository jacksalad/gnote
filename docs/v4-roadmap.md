# gnote v4.0 路线图：可感知质量

> 基线版本：`4.0.0.0`
> 范围：正确性缺陷修复、每帧/每键固定开销、基准口径与工程门禁。
> 非目标：多文档/多标签、正则查找、LSP、插件系统（见文末 v4.1+ 待办）。

---

## 1. v4.0 已完成

### 1.1 正确性

| 项 | 修复 |
|---|---|
| 默认字体 | `graphics.SetDefaultFont` 改为原地更新，`component/goui` 的默认字体别名同时生效；内嵌 Gnote Mono 现在覆盖状态栏、设置窗口等未显式 `SetFont` 的控件 |
| 滚轮 | 1/120 刻度余数累加（触摸板小数增量不再被整数除法丢弃）；Ctrl+滚轮按增量比例缩放；修饰键取自消息 `wParam` 的 `MK_*` 位 |
| 星平面字符 | `>U+FFFF` 的 rune 回退 U+FFFD 光栅化，按原 rune 缓存，emoji/CJK 扩展 B 不再取错字形 |
| 跨线程重绘 | `RequestRedraw` 在非 UI 线程调用时投递 `WM_APP_REDRAW`，渲染只在 UI 线程执行；删除失效的 `needRedraw` |
| OS 线程绑定 | `NewWindow` 绑定、`Run` 结束时解除，消除重复 `LockOSThread` |
| 模态 owner | 改为子窗口首次 `Show` 时禁用 owner；创建失败或未显示不会让主窗口失去响应 |
| 创建期消息 | 窗口仍在 `CreateWindowEx` 返回后注册。`WM_NCCREATE` 期间渲染器尚不存在，`handleResize` 本就会直接返回，提前注册只增加重入风险；客户区尺寸与 DPI 由 `initRenderer` / `GetDpiForWindow` 重新推导 |
| 选区提取 | `extractRange` 精确预留长度，多行复制/删除不再反复扩容 |
| 撤销内存 | 按结构体、ops 后备数组与 rune 缓冲区 `cap` 记账，32MB 上限反映真实堆占用 |
| 主题可读性 | 8 套主题的注释/引用/链接/行内代码达到 WCAG AA 4.5:1，分隔线与行号达到 3:1，并加入回归测试 |

### 1.2 性能

| 改动 | 说明 |
|---|---|
| `Canvas.DrawRunes` | 直接消费 rune 切片，移除每段 `string(seg)` 转换；绘制返回值即步进宽度，省掉二次测量 |
| 批量字形解析 | 每 256 字符一次 `RLock`；离屏字形只推进笔位，不进入逐像素混合 |
| 行号槽零分配 | `strconv.AppendInt` + 复用 rune 缓冲 + `digitWidth` 算术宽度，替代 `fmt.Sprintf` + `MeasureText` |
| `GetFace` 读锁 | 字体面缓存改为 `RWMutex`，命中走读锁 |
| 矩形填充 | 不透明路径按行 memmove 倍增填充；`renderRoot` 只清客户区（DIB 预留 25% 余量，原先每帧多清 56% 像素） |
| 客户区缓存 | `WM_PAINT` / `Present` 不再每帧 `GetClientRect` |
| 光标测量缓存 | 光标 X 跨帧缓存，编辑/缩放/主题变化时按 `measureEpoch` 失效 |
| 选区测量 | 每个可见行只做一次前缀测量，再追加选区段宽度 |
| span 反查 | 像素→列反查使用单调 span 游标，O(runes × spans) → O(runes + spans) |
| span 缓存 | 上限 512 行，编辑失效只扫描视口量级条目 |

### 1.3 工程

- 基准全部 `ReportAllocs`；按键/回车基准改为成对编辑，消除 O(N²) 假象；新增 SetText / UndoRedo / ReplaceAll / Zoom / 长行光标 5 组基线。
- `.gitattributes` 统一 LF，`gofmt -l .` 在 Windows 工作树可直接作为门禁。
- `build.ps1` 默认执行 gofmt / go vet / go test（`-SkipTests` 可跳过）。
- `cmd/gnote` 两个测试补 `//go:build windows`。
- 文档移除不存在的 `examples/` 引用。

---

## 1.5 v4.1 进展（增量迭代）

| 项 | 改动 | 效果 |
|---|---|---|
| 主题注册表 | `theme.All()` 只构建一次调色板，返回切片副本 | 菜单打开/设置页重绘不再反复分配 8 套 `Theme` |
| 光标局部重绘 | `Window.renderRegion` + `TextArea.CaretRect/ToggleCaretBlink`；定时器只重绘光标单元并 BitBlt 该矩形 | 空闲聚焦时每次闪烁的光栅化面积从整窗降到 `2px × 行高` |
| 高亮状态链短路 | 编辑不改变行尾词法状态时保留 `statesValidTo` 与下游 span 缓存；状态改变才向下失效 | 普通按键只重分词编辑行，不再重扫视口剩余行 |
| `ReplaceAll` 批量应用 | 替换文本不含换行时先收集所有行改动，一次性写入并只重建一次视觉索引；匹配稀疏（区间 > 匹配数×8）退回逐行路径 | 折行文档 20k 匹配：161 → 146 ms，分配 51.9 → 35.6 MB |
| `lineLayout` 瘦身 | 折行点不再常驻，按需计算并即时消费；每行缓存 40B → 8B，`invalidateAllSegments` 从 O(rows) 降为 O(1) | 回车+退格基准分配 268 → 135 KB/op；100k 行 layout 尾部搬移 4 MB → 0.8 MB |
| 超长行横向虚拟化 | 关闭折行时按 256 列构建前缀宽度检查点；编辑只截断到编辑列，行尾输入增量续算；绘制只光栅化可见列 | 20 万字符长行稳态渲染 2.15 ms → 0.18 ms；末端输入+退格 15.4 → 4.9 ms |
| 缩放惰性排版 | 等宽字体用“列数 × 列宽”上界证明短行必然不折行，跳过逐字形测量；关闭折行的行宽改为可见时按需测量，`maxContentWidth` 保留缩放比例估算 | 100k 行两档缩放 + 渲染 96 ms → 15.9 ms；短行 `widthOK` 保持 false |
| 装载扁平化（阶段 1） | `SetText` 把整篇文本写入单个 rune backing array，行以 `cap==len` 子切片视图引用；不再 `strings.Split` + 逐行 `[]rune` | 100k 行装载分配 100k → **6 次**；装载耗时 ~15.4 ms |
| 装载期预分配 | `SetText` 按行数预分配 `visualLines`/`logicalToVisual`；有高亮器时一次性分配 `lineStates` | 新建+装载+首帧：54.0 MB / 158 allocs → **37.4 MB / 52 allocs**；27.9 → 24.5 ms |
| 存储阶段 2（gap buffer） | `text []rune` 间隙缓冲 + `lineStart []int` 行偏移，替代 `[][]rune`；编辑按偏移搬运间隙并写入增量；`line(r)` 零拷贝且三索引定容 | 10 万行装载 100k → **5 allocs**；20 万字符行首输入 4.9 ms / 3.2 MB → **1.6 ms / 156 B** |
| 宽度增量 + 批量换行替换 | 单行编辑按 rune 宽度增量维护 `totalWidth`；含换行 ReplaceAll 先原地替换、最后重建一次行偏移 | 20 万字符行首输入 1.6 ms → **0.09 ms**；2 万行换行替换 **4.4 ms**（线性） |
| ReplaceAll 缓冲复用 | 匹配列用可复用 `dst` 收集；removed/inserted 写入两条 arena，op 仅保存切片视图 | 2 万行换行替换 60,043 → **70 allocs**；4.4 → 3.7 ms |
| 键入撤销零分配 | 合并判定与缓冲追加拆开，单字符键入不再创建 rec/克隆 rune；`handleKeyPress` 用栈上单元素数组 | 连续键入 **0 allocs/op**（26 B/op 为间隙增长摊销） |

### 下一阶梯

1. **存储模型阶段 2**：`lineStart []int` 偏移 + gap buffer，去掉每行 24B 切片头并让长行编辑不再整行克隆。（大）

---

## 2. 验收数据

环境：Windows 11 x64 / 1560×920 / 100k 行 × 55 字符（Go 1.26, `-benchtime 20x`）。

| 基准 | v3.0 | v4.0 |
|---|---|---|
| 全帧渲染 | 1.39 ms / 75 allocs | 0.71 ms / **0 allocs** |
| 按键 + 渲染 | 1.43 ms / 82 allocs | 0.73 ms / 6 allocs |
| 回车 + 退格 + 渲染 | 2.4–2.7 ms | 2.05 ms |
| Markdown 全帧渲染 | 1.41 ms / 75 allocs | 0.73 ms / **0 allocs** |
| 20 万字符长行稳态渲染 | ~13.7 ms | 2.15 ms |
| `GetText` 6 MB | 12.9 ms / 2 allocs | 13.2 ms / 2 allocs（未改动） |

剩余大项（不在 v4.0 范围）：

| 基准 | 当前 | 根因 |
|---|---|---|
| `SetText` 100k 行 | 15.4 ms / **6 allocs** / 33.5 MB（v4.1） | 行头仍为 24B/行，编辑仍按行克隆 |
| 缩放（两档 + 渲染） | ~16 ms（v4.1） | 仍会重建视觉索引，但不再逐行测量短行 |
| `ReplaceAll` 20k 匹配 | ~31 ms | 每个匹配单独 splice + 索引修正 |
| Undo/Redo 400 步 | ~163 ms | 每次回放都触发视觉索引维护 |

---

## 3. v4.1 待办（结构性）

1. **回车/多行编辑的 O(rows) 尾部搬移**：`lines`、`layouts`、`visualLines`、`logicalToVisual` 四个并行数组。先批量合并多 op 编辑为一次 splice；再评估分块索引/Fenwick。
   验收：100k 行单次回车 ≤ 1.5 ms，1M 行不随文档线性恶化。
2. **高亮状态链**：新增只算状态的接口 + 每 64 行检查点，避免编辑第 0 行后滚到末尾时的整段重扫。
   验收：编辑后滚动到第 10 万行首帧 ≤ 1 个帧预算。
3. **存储模型**：`text []rune` + `lineStart []int`，消除 10 万次行分配与 4× 内存放大。
   验收：6 MB 文档装载 ≤ 5 ms、堆占用 < 1.5× UTF-8。
4. **脏矩形渲染**：光标闪烁已改为局部重绘（v4.1）；剩余 `WM_PAINT` 按 `rcPaint` 拷贝与事件批次合并。
   验收：大区域遮挡恢复只拷贝受损区域。
5. **缩放惰性排版**：只为视口及附近行重建 layout，其余标记脏。
   验收：100k 行单档缩放 ≤ 16 ms。
6. **横向虚拟化**：关闭折行时按可见列裁剪绘制，超长行不再遍历全部 rune。
7. **字形缓存 LRU**：替换 8192 条全清策略；缓存键补 DPI；评估每 DPI 私有内存 DC。

## 4. v4.2 功能待办（摘要）

最近文件菜单、拖放打开、Ctrl+G 跳转行、行编辑命令族（删除/复制/移动/合并/注释）、正则/全词/选区内查找与全部匹配高亮、`.css/.js/.go/.xml` 格式化、扩展名表驱动与文件名识别、多窗口（先于多标签）。
