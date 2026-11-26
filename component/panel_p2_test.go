package component

import (
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
)

// p2Overlay 是自定义命中规则的覆盖层：bounds 很大，但只占据 pointIn 返回 true 的点。
type p2Overlay struct {
	BaseComponent
	pointIn func(x, y int32) bool
}

func (o *p2Overlay) Render(canvas *render.Canvas) {}
func (o *p2Overlay) OnEvent(evt event.Event) bool { return false }
func (o *p2Overlay) HitTest(x, y int32) bool {
	return o.pointIn == nil || o.pointIn(x, y)
}

// Panel.GetChildAt 让实现了 HitTester 的覆盖层只在自己声明的范围内遮住下层组件；
// 未实现 HitTester 的组件仍以 bounds 为准。
func TestP2_PanelGetChildAtHonorsHitTester(t *testing.T) {
	under := NewButton("under")
	under.SetBounds(0, 0, 100, 100)
	overlay := &p2Overlay{BaseComponent: NewBaseComponent()}
	overlay.SetBounds(0, 0, 100, 100)
	overlay.pointIn = func(_, y int32) bool { return y < 10 }
	root := NewPanel()
	root.SetBounds(0, 0, 100, 100)
	root.Add(under)
	root.Add(overlay)

	if got := root.GetChildAt(50, 5); got != Component(overlay) {
		t.Fatalf("point claimed by the overlay resolved to %T, want the overlay", got)
	}
	if got := root.GetChildAt(50, 20); got != Component(under) {
		t.Fatalf("point outside the overlay's HitTest resolved to %T, want the component underneath", got)
	}

	// 覆盖层展开后遮住一切；隐藏后不再参与
	overlay.pointIn = func(_, _ int32) bool { return true }
	if got := root.GetChildAt(50, 20); got != Component(overlay) {
		t.Fatalf("fully claimed overlay resolved to %T, want the overlay", got)
	}
	overlay.SetVisible(false)
	if got := root.GetChildAt(50, 5); got != Component(under) {
		t.Fatalf("hidden overlay resolved to %T, want the component underneath", got)
	}
	overlay.SetVisible(true)

	// 事件分发不受命中规则影响：覆盖层不消费时事件照旧按自顶向下到达下层组件
	overlay.pointIn = func(_, _ int32) bool { return true }
	if !root.OnEvent(p2Down(50, 20)) || !under.IsPressed() {
		t.Fatalf("event dispatch must not consult HitTester: the button underneath must still get the press")
	}

	// 未实现 HitTester 的相邻组件：bounds 重叠时后加入者在上层
	plain := NewLabel("plain")
	plain.SetBounds(0, 0, 100, 100)
	root.Add(plain)
	if got := root.GetChildAt(50, 20); got != Component(plain) {
		t.Fatalf("bounds-only component resolved to %T, want the top-most label", got)
	}
}
