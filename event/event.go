// Package event 提供事件系统实现
package event

import (
	"sync"
	"time"
)

// EventType 事件类型
type EventType int

const (
	// 鼠标事件
	EventMouseMove EventType = iota
	EventMouseDown
	EventMouseUp
	EventMouseClick
	EventMouseDoubleClick
	EventMouseWheel

	// 键盘事件
	EventKeyDown
	EventKeyUp
	EventKeyPress

	// 窗口事件
	EventPaint
	EventResize
	EventClose
	EventFocus
	EventBlur
	EventDpiChanged // 窗口所在显示器 DPI 变化

	// 输入法 (IME) 事件
	EventImeStart  // 开始组合
	EventImeUpdate // 组合串更新（预编辑文本）
	EventImeCommit // 组合上屏（最终文本）
	EventImeEnd    // 组合结束/取消

	// 自定义事件
	EventCustom
)

// Event 表示一个事件
type Event struct {
	Type      EventType
	Timestamp int64
	Data      interface{}
}

// MouseData 鼠标事件数据
type MouseData struct {
	X, Y   int32
	Button MouseButton
	Delta  int32 // 滚轮增量
	Ctrl   bool  // Ctrl键是否按下
	Shift  bool  // Shift键是否按下
}

// MouseButton 鼠标按钮
type MouseButton int

const (
	MouseButtonNone MouseButton = iota
	MouseButtonLeft
	MouseButtonRight
	MouseButtonMiddle
)

// KeyData 键盘事件数据
type KeyData struct {
	KeyCode  int
	Char     rune
	Shift    bool
	Ctrl     bool
	Alt      bool
	Repeated bool
}

// ImeData 输入法组合事件数据
type ImeData struct {
	Text string
}

// DpiData 窗口 DPI 变化数据
type DpiData struct {
	OldDPI uint32
	NewDPI uint32
}

// ResizeData 窗口大小改变数据
type ResizeData struct {
	Width, Height int32
	OldWidth      int32
	OldHeight     int32
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now().UnixNano() / 1e6,
		Data:      data,
	}
}

// Handler 事件处理函数类型
type Handler func(Event)

// Bus 事件总线接口
type Bus interface {
	Subscribe(eventType EventType) <-chan Event
	SubscribeHandler(eventType EventType, handler Handler) func()
	Publish(event Event)
	Close()
}

// EventBus 事件总线实现
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
	handlers    map[EventType][]Handler
	bufferSize  int
	closed      bool
}

// NewEventBus 创建新的事件总线
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventBus{
		subscribers: make(map[EventType][]chan Event),
		handlers:    make(map[EventType][]Handler),
		bufferSize:  bufferSize,
	}
}

// Subscribe 订阅事件类型，返回事件通道
func (eb *EventBus) Subscribe(eventType EventType) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		ch := make(chan Event, 1)
		close(ch)
		return ch
	}

	ch := make(chan Event, eb.bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}

// SubscribeHandler 使用处理函数订阅事件，返回取消订阅函数
func (eb *EventBus) SubscribeHandler(eventType EventType, handler Handler) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return func() {}
	}

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	index := len(eb.handlers[eventType]) - 1

	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		if index < len(eb.handlers[eventType]) {
			eb.handlers[eventType][index] = nil
		}
	}
}

// Publish 发布事件
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	if eb.closed {
		eb.mu.RUnlock()
		return
	}

	// 复制handlers避免死锁（handler中可能调用Close）
	handlers := make([]Handler, len(eb.handlers[event.Type]))
	copy(handlers, eb.handlers[event.Type])

	// 发送到通道订阅者
	for _, ch := range eb.subscribers[event.Type] {
		select {
		case ch <- event:
		default:
			// 缓冲区满，丢弃事件
		}
	}
	eb.mu.RUnlock()

	// 在锁外调用处理函数（避免死锁）
	for _, handler := range handlers {
		if handler != nil {
			handler(event)
		}
	}
}

// Close 关闭事件总线
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}

	eb.closed = true

	// 关闭所有订阅通道
	for _, channels := range eb.subscribers {
		for _, ch := range channels {
			close(ch)
		}
	}

	eb.subscribers = nil
	eb.handlers = nil
}

// Dispatcher 事件分发器
type Dispatcher struct {
	handlers map[EventType][]Handler
	mu       sync.RWMutex
}

// NewDispatcher 创建新的事件分发器
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[EventType][]Handler),
	}
}

// On 注册事件处理器
func (d *Dispatcher) On(eventType EventType, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Off 移除所有该类型的事件处理器
func (d *Dispatcher) Off(eventType EventType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.handlers, eventType)
}

// Dispatch 分发事件
func (d *Dispatcher) Dispatch(event Event) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	handlers, ok := d.handlers[event.Type]
	if !ok || len(handlers) == 0 {
		return false
	}

	for _, handler := range handlers {
		handler(event)
	}
	return true
}

// HasHandler 检查是否有该类型的事件处理器
func (d *Dispatcher) HasHandler(eventType EventType) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	handlers, ok := d.handlers[eventType]
	return ok && len(handlers) > 0
}
