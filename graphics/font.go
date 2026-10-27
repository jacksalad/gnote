// Package graphics 提供基础图形与字体类型定义
package graphics

// Font 字体规格定义
type Font struct {
	Name   string
	Size   int32
	Bold   bool
	Italic bool
	// AlignSpace 让空格步进对齐为 1 个等宽字符（取数字 '0' 的步进宽度）。
	// 比例字体的原生空格通常只有 0.25em（约为普通字母的一半），会让代码缩进明显偏窄；
	// 开启后 1 个空格 = 1 个字符宽度，4 空格缩进 = 4 字符宽度。
	// 内嵌 Gnote Mono 本身即等宽（空格天然 1 格），该开关用于用户替换成比例字体时的兜底；
	// 普通 UI 文本保持字体原生的词间距。
	AlignSpace bool
}

// NewFont 创建指定名称与像素大小的字体
func NewFont(name string, size int32) *Font {
	if size <= 0 {
		size = 14
	}
	if name == "" {
		name = "Microsoft YaHei"
	}
	return &Font{
		Name: name,
		Size: size,
	}
}

// DefaultFont 默认通用字体（微软雅黑，14px）
var DefaultFont = NewFont("Microsoft YaHei", 14)

// SetDefaultFont 原地更新全局默认字体规格。
//
// component.DefaultFont / goui.DefaultFont 在包初始化时复制的是 DefaultFont 的指针，
// 因此这里必须原地修改字段而不是替换指针，否则内嵌字体只对显式 SetFont 的控件生效。
// 该函数只应在创建 UI 之前调用（main 中注册内嵌字体之后），运行期不应并发修改。
func SetDefaultFont(name string, size int32) {
	if name == "" {
		name = "Microsoft YaHei"
	}
	if size <= 0 {
		size = 14
	}
	if DefaultFont == nil {
		DefaultFont = NewFont(name, size)
		return
	}
	DefaultFont.Name = name
	DefaultFont.Size = size
}
