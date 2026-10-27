//go:build windows

package main

import (
	"fmt"
	"time"

	goui "github.com/jacksalad/gnote"
)

const findBarHeight = int32(40)

// findBar 编辑器底部查找/替换条。Ctrl+F 打开并聚焦查找框，Ctrl+H 打开并聚焦替换框。
// 查找支持大小写切换、上一个/下一个（自动回绕）、替换当前匹配与全部替换（单步撤销）。
type findBar struct {
	area  *goui.TextArea
	win   *goui.Window
	panel *goui.Panel

	query      *goui.TextBox
	replaceBox *goui.TextBox
	btnCase    *goui.Button
	btnPrev    *goui.Button
	btnNext    *goui.Button
	btnReplace *goui.Button
	btnAll     *goui.Button
	btnClose   *goui.Button
	status     *goui.Label

	visible       bool
	caseSensitive bool
	hasMatch      bool
	current       goui.Match

	openRow, openCol int
	searchGen        int // 输入防抖代次：只有最后一次调度会真正执行
	onLayoutChange   func()
}

func newFindBar(area *goui.TextArea, win *goui.Window, onLayoutChange func()) *findBar {
	f := &findBar{
		area:           area,
		win:            win,
		onLayoutChange: onLayoutChange,
		openRow:        0,
		openCol:        0,
	}

	f.panel = goui.NewSurfacePanel()
	f.panel.SetBounds(0, 0, 1560, findBarHeight)

	barFont := goui.NewFont(area.GetFontFamily(), 14)

	f.query = goui.NewTextBox()
	f.query.SetPlaceholder("查找内容")
	f.query.SetFont(barFont)
	f.query.SetMaxLength(512)
	f.query.SetOnChange(func(string) { f.scheduleSearch() })
	f.query.SetOnSubmit(func(string) { f.find(true) })

	f.replaceBox = goui.NewTextBox()
	f.replaceBox.SetPlaceholder("替换为")
	f.replaceBox.SetFont(barFont)
	f.replaceBox.SetMaxLength(512)
	f.replaceBox.SetOnSubmit(func(string) { f.replaceOne() })

	f.btnCase = goui.NewButton("Aa")
	f.btnCase.SetFont(barFont)
	f.btnCase.OnClick(func() {
		f.caseSensitive = !f.caseSensitive
		if f.caseSensitive {
			f.btnCase.SetText("[Aa]")
		} else {
			f.btnCase.SetText("Aa")
		}
		f.searchFromOrigin()
		f.focusQuery()
	})

	f.btnPrev = goui.NewButton("上一个")
	f.btnPrev.SetFont(barFont)
	f.btnPrev.OnClick(func() {
		f.find(false)
		f.focusQuery()
	})

	f.btnNext = goui.NewButton("下一个")
	f.btnNext.SetFont(barFont)
	f.btnNext.OnClick(func() {
		f.find(true)
		f.focusQuery()
	})

	f.btnReplace = goui.NewButton("替换")
	f.btnReplace.SetFont(barFont)
	f.btnReplace.OnClick(func() {
		f.replaceOne()
		f.focusReplace()
	})

	f.btnAll = goui.NewButton("全部替换")
	f.btnAll.SetFont(barFont)
	f.btnAll.OnClick(func() {
		f.replaceAll()
		f.focusReplace()
	})

	f.btnClose = goui.NewButton("X")
	f.btnClose.SetFont(barFont)
	f.btnClose.OnClick(func() { f.hide() })

	f.status = goui.NewLabel("")
	f.status.SetFont(barFont)
	f.status.SetAlign(goui.AlignRight)

	for _, c := range []goui.Component{
		f.query, f.btnCase, f.btnPrev, f.btnNext,
		f.replaceBox, f.btnReplace, f.btnAll, f.status, f.btnClose,
	} {
		f.panel.Add(c)
	}

	f.layout(1560)
	f.panel.SetVisible(false)
	return f
}

// redraw 请求重绘（测试环境下允许无窗口）
func (f *findBar) redraw() {
	if f.win != nil {
		f.win.RequestRedraw()
	}
}

// layout 根据可用宽度重排控件（查找/替换框固定宽度，状态文本自适应剩余空间）
func (f *findBar) layout(width int32) {
	const pad, gap, ctrlH, top = int32(10), int32(6), int32(26), int32(7)
	const caseW, prevW, nextW, replaceBtnW, allW, closeW = int32(38), int32(64), int32(64), int32(60), int32(84), int32(30)

	// 固定控件与间距占用；剩余宽度先分给查找/替换框，最后留给状态文本。
	fixed := caseW + prevW + nextW + replaceBtnW + allW + closeW + gap*5 + 18
	avail := width - pad*2 - fixed
	qW, rW := int32(240), int32(220)
	if avail < qW+rW+20 {
		// 窄窗口：两框按比例压缩（最小 96px），保证所有按钮与关闭键仍在面板内
		const minFlex = int32(96) * 2
		flex := avail - 20
		if flex < minFlex {
			flex = minFlex
		}
		qW = flex * 55 / 100
		if qW < 96 {
			qW = 96
		}
		rW = flex - qW
		if rW < 96 {
			rW = 96
			qW = flex - rW
			if qW < 96 {
				qW = 96
			}
		}
	}

	x := pad
	f.query.SetBounds(x, top, qW, ctrlH)
	x += qW + gap
	f.btnCase.SetBounds(x, top, caseW, ctrlH)
	x += caseW + gap
	f.btnPrev.SetBounds(x, top, prevW, ctrlH)
	x += prevW + gap
	f.btnNext.SetBounds(x, top, nextW, ctrlH)
	x += nextW + 18
	f.replaceBox.SetBounds(x, top, rW, ctrlH)
	x += rW + gap
	f.btnReplace.SetBounds(x, top, replaceBtnW, ctrlH)
	x += replaceBtnW + gap
	f.btnAll.SetBounds(x, top, allW, ctrlH)
	x += allW + gap

	closeX := width - pad - closeW
	if closeX < x {
		closeX = x
	}
	f.btnClose.SetBounds(closeX, top, closeW, ctrlH)

	statusW := closeX - gap - x
	if statusW < 0 {
		statusW = 0
	}
	f.status.SetBounds(x, top, statusW, ctrlH)
}

// Show 显示查找条（已显示时仅切换焦点并重新锚定搜索起点）
func (f *findBar) show(focusReplace bool, width int32) {
	f.searchGen++
	fresh := !f.visible
	f.visible = true
	f.panel.SetVisible(true)
	f.layout(width)
	if fresh {
		f.onLayoutChange()
	}

	// 搜索起点 = 当前光标位置，保证"打开查找条再输入"总是从用户所在处向前找
	r, c := f.area.GetCursorPosition()
	f.openRow, f.openCol = r-1, c-1

	// 编辑器与查找输入框不能同时持有键盘焦点，否则未处理的按键（Tab/撤销）会漏入正文
	f.area.SetFocused(false)
	if focusReplace {
		f.query.SetFocused(false)
		f.replaceBox.SetFocused(true)
	} else {
		f.replaceBox.SetFocused(false)
		f.query.SetFocused(true)
	}

	if f.query.GetText() != "" {
		f.searchFromOrigin()
	}
	f.redraw()
}

// Hide 隐藏查找条并交还焦点给编辑器
func (f *findBar) hide() {
	if !f.visible {
		return
	}
	f.searchGen++
	f.visible = false
	f.hasMatch = false
	f.panel.SetVisible(false)
	f.query.SetFocused(false)
	f.replaceBox.SetFocused(false)
	f.status.SetText("")
	f.area.SetFocused(true)
	f.area.ScrollCursorIntoView()
	f.onLayoutChange()
	f.redraw()
}

// focusQuery / focusReplace 在按钮操作后恢复输入框焦点，并让编辑器让出键盘
func (f *findBar) focusQuery() {
	f.area.SetFocused(false)
	f.replaceBox.SetFocused(false)
	f.query.SetFocused(true)
	f.redraw()
}

func (f *findBar) focusReplace() {
	f.area.SetFocused(false)
	f.query.SetFocused(false)
	f.replaceBox.SetFocused(true)
	f.redraw()
}

// blurInputs 让查找/替换输入框让出键盘（用户点击编辑器时调用）
func (f *findBar) blurInputs() {
	f.query.SetFocused(false)
	f.replaceBox.SetFocused(false)
}

// contains 判断屏幕坐标是否落在查找条内
func (f *findBar) contains(x, y int32) bool {
	if !f.visible {
		return false
	}
	b := f.panel.GetBounds()
	return x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
}

// scheduleSearch 对查找输入做防抖：大文档下避免每敲一个字符就全量扫描一次。
// 只有最后一次调度（searchGen 未变化）才会真正执行搜索。
func (f *findBar) scheduleSearch() {
	f.searchGen++
	gen := f.searchGen
	if f.win == nil {
		f.searchFromOrigin()
		return
	}
	f.win.Schedule(120*time.Millisecond, func() {
		if gen == f.searchGen {
			f.searchFromOrigin()
		}
	})
}

// cancelPendingSearch 作废尚未触发的防抖任务（立即操作优先）。
func (f *findBar) cancelPendingSearch() {
	f.searchGen++
}

// searchFromOrigin 从打开查找条时的光标位置开始查找（用于输入/切换大小写后的增量定位）
func (f *findBar) searchFromOrigin() {
	f.searchGen++ // 任何立即搜索都会作废仍在等待的防抖任务
	q := f.query.GetText()
	if q == "" {
		f.hasMatch = false
		f.status.SetText("")
		f.redraw()
		return
	}
	m, ok := f.area.FindNext(q, f.openRow, f.openCol, true, f.caseSensitive)
	if !ok {
		f.hasMatch = false
		f.status.SetText("无匹配")
		f.redraw()
		return
	}
	f.applyMatch(m)
}

// find 查找下一个/上一个匹配（从当前匹配继续，自动回绕）
func (f *findBar) find(forward bool) {
	f.cancelPendingSearch()
	q := f.query.GetText()
	if q == "" {
		return
	}

	fromRow, fromCol := f.area.GetCursorPosition()
	fromRow--
	fromCol--

	if f.hasMatch && f.selectionIsCurrentMatch() {
		if forward {
			fromRow, fromCol = f.current.EndRow, f.current.EndCol
		} else {
			fromRow, fromCol = f.current.StartRow, f.current.StartCol
		}
	}

	m, ok := f.area.FindNext(q, fromRow, fromCol, forward, f.caseSensitive)
	if !ok {
		f.hasMatch = false
		f.status.SetText("无匹配")
		f.redraw()
		return
	}
	f.applyMatch(m)
}

// replaceOne 若当前匹配仍是选区则替换，然后自动跳到下一个匹配
func (f *findBar) replaceOne() {
	f.cancelPendingSearch()
	q := f.query.GetText()
	if q == "" {
		return
	}
	if !f.hasMatch || !f.selectionIsCurrentMatch() {
		f.find(true)
		if !f.hasMatch {
			return
		}
	}

	if f.area.ReplaceCurrentSelection(q, f.replaceBox.GetText(), f.caseSensitive) {
		f.hasMatch = false
		f.find(true)
		f.status.SetText("已替换 1 处")
		f.redraw()
	}
}

// replaceAll 全部替换（整体作为一步撤销）
func (f *findBar) replaceAll() {
	f.cancelPendingSearch()
	q := f.query.GetText()
	if q == "" {
		return
	}
	n := f.area.ReplaceAll(q, f.replaceBox.GetText(), f.caseSensitive)
	f.hasMatch = false
	if n == 0 {
		f.status.SetText("无匹配")
	} else {
		f.status.SetText(fmt.Sprintf("已替换 %d 处", n))
	}
	f.redraw()
}

// applyMatch 选中匹配并滚动进视口
func (f *findBar) applyMatch(m goui.Match) {
	f.current = m
	f.hasMatch = true
	f.area.SetSelection(m.StartRow, m.StartCol, m.EndRow, m.EndCol)
	f.area.ScrollCursorIntoView()
	f.status.SetText(fmt.Sprintf("第 %d 行", m.StartRow+1))
	f.redraw()
}

// selectionIsCurrentMatch 判断编辑器当前选区是否仍是最近一次命中的匹配
func (f *findBar) selectionIsCurrentMatch() bool {
	sr, sc, er, ec, ok := f.area.GetSelectionRange()
	return ok && sr == f.current.StartRow && sc == f.current.StartCol &&
		er == f.current.EndRow && ec == f.current.EndCol
}
