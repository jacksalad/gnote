package render

// Renderer 渲染器接口
type Renderer interface {
	// BeginFrame 开始新帧，返回画布
	BeginFrame() *Canvas
	// EndFrame 结束帧绘制
	EndFrame()
	// Present 将画布内容显示到窗口
	Present()
	// GetCanvas 获取当前画布
	GetCanvas() *Canvas
	// Resize 调整渲染器大小
	Resize(width, height int32)
	// Destroy 销毁渲染器
	Destroy()
}

// SoftwareRenderer 软件渲染器实现
type SoftwareRenderer struct {
	canvas      *Canvas
	width       int32
	height      int32
	inFrame     bool
	presentFunc func(buffer []uint32, width, height int32)
}

// NewSoftwareRenderer 创建新的软件渲染器
func NewSoftwareRenderer(width, height int32, presentFunc func(buffer []uint32, width, height int32)) *SoftwareRenderer {
	return &SoftwareRenderer{
		canvas:      NewCanvas(width, height),
		width:       width,
		height:      height,
		presentFunc: presentFunc,
	}
}

// BeginFrame 开始新帧
func (r *SoftwareRenderer) BeginFrame() *Canvas {
	r.inFrame = true
	return r.canvas
}

// EndFrame 结束帧
func (r *SoftwareRenderer) EndFrame() {
	r.inFrame = false
}

// Present 显示画布内容
func (r *SoftwareRenderer) Present() {
	if r.presentFunc != nil && r.canvas.IsDirty() {
		r.presentFunc(r.canvas.Buffer, r.canvas.Width, r.canvas.Height)
		r.canvas.SetDirty(false)
	}
}

// UseExternalBuffer 让画布直接渲染到外部像素内存（DIB Section），随后 Present 无需再拷贝整帧
func (r *SoftwareRenderer) UseExternalBuffer(buffer []uint32, width, height int32) bool {
	if width <= 0 || height <= 0 || len(buffer) < int(width)*int(height) {
		return false
	}
	r.canvas.AdoptBuffer(buffer, width, height)
	r.width = width
	r.height = height
	return true
}

// GetCanvas 获取画布
func (r *SoftwareRenderer) GetCanvas() *Canvas {
	return r.canvas
}

// Resize 调整大小
func (r *SoftwareRenderer) Resize(width, height int32) {
	if width == r.width && height == r.height {
		return
	}
	r.width = width
	r.height = height
	r.canvas.Resize(width, height)
}

// Destroy 销毁渲染器
func (r *SoftwareRenderer) Destroy() {
	r.canvas = nil
	r.presentFunc = nil
}

// InFrame 检查是否在帧中
func (r *SoftwareRenderer) InFrame() bool {
	return r.inFrame
}
