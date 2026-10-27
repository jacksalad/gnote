# gnote v3.0 开发路线图：自绘菜单 + 独立设置窗口

> 版本基线：`3.0.0.0`
> 分支：`master`
> 提交节奏：每个里程碑一个可编译、可测试的提交；提交信息使用 `v3.N: ...`
> 发布：全部里程碑完成后打 tag `v3.0.0`

---

## 0. 目标与非目标

### 目标

1. 删除主窗口顶部 13 个平铺按钮，改为**全客户区自绘菜单栏**：文件 / 编辑 / 查看 / 设置 / 帮助。
2. 菜单状态由**命令注册表**统一提供：撤销/重做置灰、行号/换行/高亮勾选、缩放/主题勾选，不再依赖按钮文本。
3. 新增**独立 HWND 设置窗口**：`Owner = 主窗口`、`Modal = true`，关闭设置窗口不退出主进程。
4. 设置持久化到 `%AppData%\gnote\settings.json`，支持绿色便携模式覆盖。
5. 修复窗口基础能力：多窗口退出策略、`WM_SYSKEYDOWN` / `WM_SYSCHAR`、辅助窗口光标定时器。
6. 菜单/设置/状态栏按 DPI 缩放，窄窗口下所有命令可达。

### 非目标

- 多标签页、分屏、插件系统、动画。
- 原生 Win32 菜单栏；菜单栏使用自绘，保持主题与内嵌字体一致。
- 设置项热重载外部文件；v3.0 只在启动时读取、应用时写回。
- 快捷键自定义重绑定；只提供命令表与恢复默认。

---

## 1. 架构决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 菜单实现 | 自绘 `MenuLayer`，全客户区 | 暗色主题与内嵌字体一致；不受 34px 父面板裁剪 |
| 菜单层级 | 一级菜单 + 一级子菜单 | 覆盖最近文件/语言/缩放/主题，复杂度可控 |
| 菜单状态 | 打开时从命令注册表重建 | 撤销栈、选区、主题、缩放永远取当前值 |
| 设置窗口 | 独立 HWND + owner/modal | 用户明确要求；Win32 原生模态语义 |
| 设置持久化 | 标准库 `encoding/json` + 原子写 | 零依赖，沿用现有原子保存思路 |
| 多窗口生命周期 | 最后一个窗口销毁时才 `PostQuitMessage` | 关闭设置窗口不能杀死主循环 |
| 系统键 | `WM_SYSKEYDOWN` 分发 Alt/F10；保留 Alt+F4/Alt+Space | 菜单键盘可达，同时不破坏系统行为 |
| DPI | `uiMetrics{scale}` 派生所有 chrome 尺寸 | 现有编辑器已按 DPI 缩放，菜单/设置必须一致 |

### 焦点与输入约定

- 菜单打开：记录 `prevFocus`，`RequestFocus(menuLayer)`，编辑器让出键盘；关闭时恢复。
- 设置窗口：独立 HWND 自然隔离焦点；owner 被 `EnableWindow(false)`，关闭后恢复并置前。
- 全局快捷键：菜单/设置打开时不得透传到文档命令。
- 菜单下拉不放入 34px 菜单栏面板，避免 `Panel` 裁剪。

---

## 2. 里程碑与提交

### v3.0 — 版本基线与路线图

**内容**

- 新增本文档，更新 `docs/README.md` 索引。
- PE `VERSIONINFO`、Manifest、打包 zip 名统一到 `3.0.0.0`。
- 不改任何运行时代码。

**提交**：`v3.0: 确立 v3.0 基线与自绘菜单/独立设置窗口路线图`

**验收**

- `gofmt -l .` 为空。
- `go test ./...` 全绿。
- 可执行文件版本资源显示 `3.0.0.0`（构建后检查）。

---

### v3.1 — 多窗口生命周期与系统键分发

**内容**

- `window.WindowConfig` 新增 `Owner *Window`、`Modal bool`。
- 创建 owner 窗口时传入 `hWndParent`；`Modal` 时禁用 owner。
- 窗口销毁时恢复 owner 并置前；owner 先销毁时安全跳过。
- `WM_DESTROY` 只在窗口映射为空时 `PostQuitMessage(0)`。
- `WM_SYSKEYDOWN` 分发 `EventKeyDown{Alt:true}`；`WM_SYSCHAR` 吞掉系统提示音；保留 `Alt+F4` / `Alt+Space` 给 `DefWindowProc`。
- 每个窗口创建时安装光标闪烁定时器，辅助窗口（设置窗口）也能闪烁。
- 新增 `Window.Activate()`、`IsWindowEnabled()`。

**提交**：`v3.1: 多窗口生命周期、系统键分发与辅助窗口定时器`

**验收**

- 两个窗口销毁其一后不产生 `WM_QUIT`；最后一个销毁才退出。
- 模态窗口打开时 owner 处于 disabled；关闭后恢复 enabled。
- `WM_SYSKEYDOWN` 的 F10/Alt+字母进入事件总线；`WM_SYSCHAR` 返回 0。
- `Alt+F4` 仍触发 `SetOnClose`；`Alt+Space` 仍打开系统菜单。
- 现有 `window` 测试全绿。

---

### v3.2 — 应用状态与命令注册表

**内容**

- 拆分 `cmd/gnote/main.go`：
  - `app.go`：`App` 结构、文件打开/保存/新建、编码/EOL、状态刷新。
  - `commands.go`：`Command{ID,Title,Accel,Run,Enabled,Checked}` 与注册表。
  - `main.go`：字体、窗口、`App.Run()`。
- 菜单、右键菜单、快捷键统一按 command ID 分发。
- 不改变现有 UI 和快捷键行为。

**提交**：`v3.2: 抽取应用状态与命令注册表`

**验收**

- 现有 `cmd/gnote` 测试全绿。
- 撤销/重做、查找替换、文件操作、关闭拦截行为不变。
- 命令 ID 唯一；`Enabled`/`Checked` 可在无窗口环境下测试。

---

### v3.3 — 自绘菜单栏组件

**内容**

- 新增 `component/menubar.go`：
  - `Menu`、`MenuItem`、`MenuBar`/`MenuLayer`。
  - 顶部菜单栏 + 下拉层 + 一级子菜单。
  - 主题令牌染色、DPI 度量、键盘导航、点击外部关闭、子菜单翻转。
  - 打开时重建菜单模型，缓存文本宽度。
- 新增 `component/menubar_test.go`：命中、开关、禁用、勾选、子菜单、键盘、裁剪。

**提交**：`v3.3: 新增全客户区自绘菜单栏组件`

**验收**

- 单测覆盖：点击切换、hover 切换、Esc、方向键、Enter、禁用项不执行、勾选状态、子菜单越界翻转。
- 下拉可覆盖编辑器，不受父面板裁剪。
- 菜单打开时只消费自身区域；关闭时编辑器事件正常。

---

### v3.4 — 主窗口接入菜单栏

**内容**

- 删除 13 个按钮与 toolbar。
- 根布局：34px 菜单占位 + 编辑器 + 状态栏；`MenuLayer` 全客户区置于最后。
- 菜单模型由命令注册表生成：文件/编辑/查看/设置/帮助。
- 快捷键、右键菜单复用命令。
- 更新欢迎文档的快捷键说明。

**提交**：`v3.4: 主窗口接入自绘菜单栏并移除按钮工具栏`

**验收**

- 640×400 到 1560×920 所有命令可达，无裁剪。
- 撤销/重做随栈状态置灰；行号/换行/高亮/主题勾选正确。
- `Alt+F` 打开文件菜单，`F10` 打开首菜单，Esc/方向键/Enter 可用。
- 点击菜单/子菜单后编辑器焦点正确恢复。

---

### v3.5 — 设置模型与持久化

**内容**

- 新增 `cmd/gnote/settings.go`：
  - `Settings` 结构（主题、基准字号、Tab 宽度、换行、行号、高亮、最近文件、窗口尺寸）。
  - 默认值、校验/clamp、`Snapshot`/`Apply`/`Restore`。
  - `%AppData%\gnote\settings.json` 加载/原子保存；exe 同目录 `gnote.portable` 覆盖。
- 新增 `settings_test.go`：往返、损坏回退、clamp、原子写。

**提交**：`v3.5: 设置模型、持久化与原子写入`

**验收**

- 损坏 JSON 不阻塞启动，回默认值。
- 写入使用同目录临时文件 + `Sync` + 原子替换。
- 编码/EOL 等文档级状态不被设置页静默改写。

---

### v3.6 — 独立设置窗口

**内容**

- 新增 `component/tabbar.go`、`component/switch.go`、`component/segmented.go`（主题等少量互斥选项用分段选择器，替代下拉 Select）。
- 新增 `cmd/gnote/settings_window.go`：
  - `WindowConfig{Owner: main, Modal: true, Resizable: false, Center: true}`。
  - 分组：编辑器 / 外观 / 关于。
  - 应用 / 取消 / 恢复默认；取消恢复快照。
  - 使用 TabBar/Switch/Segmented/Slider，复用现有 Button/Label。
- 设置窗口关闭不会退出主进程。

**提交**：`v3.6: 独立设置窗口、模态 owner 与设置页组件`

**验收**

- 关闭设置窗口后主窗口仍运行且恢复 enabled。
- 应用后重启保留配置；取消恢复主题/字号/开关。
- 设置窗口固定 760×560，内容不溢出；主窗口在 640×400 时仍可打开设置窗口（owner 模态）。
- 150% DPI 下内容仍完整；更大 DPI 的统一缩放由 v3.7 处理。
- 设置窗口打开时主窗口快捷键不触发。

---

### v3.7 — 状态栏交互、DPI 与布局性能

**内容**

- `Label` 增加裁剪/省略号/可选点击；状态栏分段：路径、行列、编码、EOL、语言、缩放。
- 编码/EOL/语言/缩放点击弹出现有原生 `ShowPopupMenu`。
- `uiMetrics` 驱动菜单栏、状态栏、设置窗口尺寸；`EventDpiChanged` 重排。
- `Panel` 增加布局脏标记，避免 Render 每帧 `DoLayout`。

**提交**：`v3.7: 状态栏交互、DPI 缩放与布局性能`

**验收**

- 640 宽下状态栏不重叠；超长路径省略且 tooltip/完整路径可查。
- 150%/200% DPI 下菜单、状态栏、设置窗口与正文比例一致。
- 菜单 hover 全帧不重复布局测量。

---

### v3.8 — 文档、回归与发布

**内容**

- 更新 `AGENTS.md`、`docs/components.md`、`docs/layouts.md`、欢迎文档。
- 全量测试、`gofmt`、`go vet`（保留两条已知 unsafe 告警）。
- 构建 `bin/gnote.exe` 冒烟；验证版本资源 `3.0.0.0`。
- 打 tag `v3.0.0`。

**提交**：`v3.8: v3.0 文档、回归与发布准备`

**验收**

- CI 四件事全绿：gofmt、vet、test、build。
- 640×400 / 1560×920 / 150% DPI 手工冒烟通过。
- 菜单、设置、查找条、关闭拦截、编码保真无回归。

---

## 3. 文件地图

```text
component/
  menubar.go        # 新增：全客户区菜单层
  tabs.go           # 新增：设置分组导航
  switch.go         # 新增：开关
  select.go         # 新增：下拉选择
  button.go         # 焦点色改用主题令牌
  label.go          # 裁剪/省略号/可点击
  panel.go          # 布局脏标记

window/
  window.go         # 多窗口、owner/modal、系统键、定时器
  win32.go          # VK/IsWindowEnabled 等常量与包装

cmd/gnote/
  main.go           # 仅启动
  app.go            # 应用状态与文件业务
  commands.go       # 命令注册表
  menu.go           # 菜单模型
  settings.go       # 设置模型/持久化
  settings_window.go# 独立设置窗口
```

---

## 4. 测试矩阵

| 层级 | 覆盖 |
|---|---|
| `component/menubar_test.go` | 命中、hover、开关、禁用、勾选、键盘、子菜单翻转、裁剪 |
| `component/*_test.go` | 新增 TabBar/Switch/Segmented 事件与主题 |
| `window/*_test.go` | 双窗口退出、owner/modal、SYSKEYDOWN/SYSCHAR、定时器 |
| `cmd/gnote/settings_test.go` | JSON 往返、损坏回退、clamp、原子写 |
| `cmd/gnote/menu_test.go` | 命令 ID/Enabled/Checked、菜单模型生成 |
| `cmd/gnote/layout_stability_test.go` | 640 宽菜单可达、状态栏不重叠 |

手工冒烟：

- 100% / 150% / 200% DPI 切换显示器。
- 深色/浅色主题切换。
- 640×400 最小窗口。
- 10 万行文档菜单 hover 与设置窗口开关。
- 未保存文档关闭拦截与设置窗口并存。

---

## 5. 风险与回退

| 风险 | 应对 |
|---|---|
| 多窗口退出策略改坏关闭流程 | 保留单窗口路径不变；新增双窗口测试；最后窗口才退出 |
| `WM_SYSKEYDOWN` 吞掉系统键 | 仅 `Alt+F4` / `Alt+Space` 交回 `DefWindowProc`；WM_SYSCHAR 返回 0 |
| 自绘菜单键盘/焦点与编辑器冲突 | 菜单打开时独占焦点；关闭恢复 `prevFocus` |
| 设置窗口 owner 被永久禁用 | 在 `cleanup` 中恢复；owner 已销毁时跳过 |
| 配置写入失败影响启动 | 加载失败回默认；保存失败只提示，不改内存状态 |
| 菜单下拉被父面板裁剪 | 全客户区 `MenuLayer`，不放在 34px 面板内 |
| DPI 缩放遗漏 | 菜单/状态栏/设置窗口统一从 `uiScale` 派生；跨显示器 DPI 变化时重建 |
| 无障碍能力不足 | 自绘 UI 的已知债；后续评估 UIA 或原生菜单回退 |

---

## 6. 实施记录

| 里程碑 | 提交 | 结果 |
| :--- | :--- | :--- |
| v3.0 | `0331412` | 版本基线、路线图、PE/Manifest 3.0.0.0 |
| v3.1 | `83cebb1` | 多窗口退出、owner/modal、系统键、辅助窗口定时器 |
| v3.2 | `b75a9ae` | App 状态与命令注册表 |
| v3.3 | `b46be06` | 全客户区自绘 MenuBar |
| v3.4 | `1125724` | 菜单接入、删除按钮工具栏 |
| v3.5 | `ebcacf4` | Settings 模型与原子持久化 |
| v3.6 | `86d085b` | 独立设置窗口、TabBar/Switch/Segmented |
| v3.7 | `c90641d` | 状态栏分段交互、DPI 缩放、标签裁剪 |
| v3.8 | 本次发布提交 | 文档、资源重编译、全量回归、tag `v3.0.0` |

> 验证基线：`go test ./...` 全绿、`gofmt -l .` 为空、`go vet` 仅保留两条已知 `unsafe.Pointer` 告警；生产构建版本资源为 `3.0.0.0`。