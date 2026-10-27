//go:build windows

package dialog

import (
	"syscall"
	"unsafe"
)

// MenuItem 原生弹出菜单项
type MenuItem struct {
	ID        uint32 // 菜单命令 ID（返回给调用方）
	Text      string // 菜单文本，可用 \t 指定快捷键提示
	Separator bool   // 是否为分隔线
	Disabled  bool   // 是否禁用
	Checked   bool   // 是否显示勾选标记
}

const (
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	mfGrayed       = 0x00000001
	mfChecked      = 0x00000008
	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002
	wmNull         = 0x0000
)

type popupPoint struct {
	X, Y int32
}

var (
	createPopupMenu     = user32.NewProc("CreatePopupMenu")
	appendMenuW         = user32.NewProc("AppendMenuW")
	trackPopupMenu      = user32.NewProc("TrackPopupMenu")
	destroyMenu         = user32.NewProc("DestroyMenu")
	clientToScreen      = user32.NewProc("ClientToScreen")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	postMessageW        = user32.NewProc("PostMessageW")
)

// ShowPopupMenu 在窗口客户区坐标 (x, y) 处弹出原生上下文菜单。
// 返回值是被选中的菜单项 ID，0 表示用户取消。
// 调用方应在菜单命令返回后自行刷新界面。
func ShowPopupMenu(hwnd uintptr, x, y int32, items []MenuItem) uint32 {
	if hwnd == 0 || len(items) == 0 {
		return 0
	}
	hMenu, _, _ := createPopupMenu.Call()
	if hMenu == 0 {
		return 0
	}
	defer destroyMenu.Call(hMenu)

	for _, item := range items {
		if item.Separator {
			appendMenuW.Call(hMenu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if item.Disabled {
			flags |= mfGrayed
		}
		if item.Checked {
			flags |= mfChecked
		}
		text, err := syscall.UTF16PtrFromString(item.Text)
		if err != nil {
			continue
		}
		appendMenuW.Call(hMenu, flags, uintptr(item.ID), uintptr(unsafe.Pointer(text)))
	}

	pt := popupPoint{X: x, Y: y}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	// 菜单要求宿主窗口处于前台，否则点击外部不会关闭
	setForegroundWindow.Call(hwnd)
	ret, _, _ := trackPopupMenu.Call(
		hMenu,
		tpmReturnCmd|tpmRightButton,
		uintptr(pt.X), uintptr(pt.Y),
		0, hwnd, 0,
	)
	postMessageW.Call(hwnd, wmNull, 0, 0)
	return uint32(ret)
}
