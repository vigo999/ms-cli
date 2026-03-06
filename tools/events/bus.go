package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event 事件接口
type Event interface {
	Topic() string
	Timestamp() time.Time
	Data() interface{}
}

// baseEvent 基础事件实现
type baseEvent struct {
	topic     string
	timestamp time.Time
	data      interface{}
}

func (e *baseEvent) Topic() string      { return e.topic }
func (e *baseEvent) Timestamp() time.Time { return e.timestamp }
func (e *baseEvent) Data() interface{}  { return e.data }

// NewEvent 创建新事件
func NewEvent(topic string, data interface{}) Event {
	return &baseEvent{
		topic:     topic,
		timestamp: time.Now(),
		data:      data,
	}
}

// EventHandler 事件处理函数
type EventHandler func(event Event)

// EventBus 事件总线接口
type EventBus interface {
	// Publish 发布事件
	Publish(event Event) error

	// Subscribe 订阅事件
	Subscribe(topic string, handler EventHandler) error

	// Unsubscribe 取消订阅
	Unsubscribe(topic string, handler EventHandler) error

	// SubscribeOnce 只订阅一次
	SubscribeOnce(topic string, handler EventHandler) error

	// Close 关闭总线
	Close() error
}

// DefaultEventBus 默认事件总线实现
type DefaultEventBus struct {
	handlers   map[string][]*handlerInfo
	handlersMu sync.RWMutex
	closed     bool
	closeCh    chan struct{}
}

type handlerInfo struct {
	handler EventHandler
	once    bool
}

// NewEventBus 创建新的事件总线
func NewEventBus() EventBus {
	return &DefaultEventBus{
		handlers: make(map[string][]*handlerInfo),
		closeCh:  make(chan struct{}),
	}
}

// Publish 发布事件
func (b *DefaultEventBus) Publish(event Event) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	b.handlersMu.RLock()
	handlers := b.handlers[event.Topic()]
	b.handlersMu.RUnlock()

	// 复制处理器列表以避免在迭代时修改
	var toRemove []*handlerInfo
	for _, h := range handlers {
		h.handler(event)
		if h.once {
			toRemove = append(toRemove, h)
		}
	}

	// 移除一次性处理器
	if len(toRemove) > 0 {
		b.handlersMu.Lock()
		b.removeHandlers(event.Topic(), toRemove)
		b.handlersMu.Unlock()
	}

	return nil
}

// Subscribe 订阅事件
func (b *DefaultEventBus) Subscribe(topic string, handler EventHandler) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	b.handlersMu.Lock()
	defer b.handlersMu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], &handlerInfo{
		handler: handler,
		once:    false,
	})
	return nil
}

// Unsubscribe 取消订阅
func (b *DefaultEventBus) Unsubscribe(topic string, handler EventHandler) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	b.handlersMu.Lock()
	defer b.handlersMu.Unlock()

	handlers := b.handlers[topic]
	for i, h := range handlers {
		// 比较函数指针（近似比较）
		if fmt.Sprintf("%p", h.handler) == fmt.Sprintf("%p", handler) {
			b.handlers[topic] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
	return nil
}

// SubscribeOnce 只订阅一次
func (b *DefaultEventBus) SubscribeOnce(topic string, handler EventHandler) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	b.handlersMu.Lock()
	defer b.handlersMu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], &handlerInfo{
		handler: handler,
		once:    true,
	})
	return nil
}

// Close 关闭总线
func (b *DefaultEventBus) Close() error {
	if b.closed {
		return nil
	}

	b.closed = true
	close(b.closeCh)

	b.handlersMu.Lock()
	b.handlers = make(map[string][]*handlerInfo)
	b.handlersMu.Unlock()

	return nil
}

// removeHandlers 移除指定的处理器
func (b *DefaultEventBus) removeHandlers(topic string, toRemove []*handlerInfo) {
	handlers := b.handlers[topic]
	var newHandlers []*handlerInfo

	for _, h := range handlers {
		found := false
		for _, tr := range toRemove {
			if h == tr {
				found = true
				break
			}
		}
		if !found {
			newHandlers = append(newHandlers, h)
		}
	}

	b.handlers[topic] = newHandlers
}

// AsyncEventBus 异步事件总线
type AsyncEventBus struct {
	*DefaultEventBus
	eventCh chan Event
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewAsyncEventBus 创建异步事件总线
func NewAsyncEventBus(bufferSize int) EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	bus := &AsyncEventBus{
		DefaultEventBus: &DefaultEventBus{
			handlers: make(map[string][]*handlerInfo),
			closeCh:  make(chan struct{}),
		},
		eventCh: make(chan Event, bufferSize),
		ctx:     ctx,
		cancel:  cancel,
	}
	bus.start()
	return bus
}

// start 启动处理循环
func (b *AsyncEventBus) start() {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case event := <-b.eventCh:
				b.DefaultEventBus.Publish(event)
			case <-b.ctx.Done():
				return
			case <-b.closeCh:
				return
			}
		}
	}()
}

// Publish 异步发布事件
func (b *AsyncEventBus) Publish(event Event) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	select {
	case b.eventCh <- event:
		return nil
	case <-b.ctx.Done():
		return fmt.Errorf("event bus is closed")
	default:
		return fmt.Errorf("event bus is full")
	}
}

// Close 关闭异步总线
func (b *AsyncEventBus) Close() error {
	b.cancel()
	b.wg.Wait()
	return b.DefaultEventBus.Close()
}

// 预定义的事件主题
const (
	// PermissionRequestedTopic 权限请求事件
	PermissionRequestedTopic = "permission:requested"
	// PermissionRespondedTopic 权限响应事件
	PermissionRespondedTopic = "permission:responded"
	// ToolExecutionStartedTopic 工具执行开始
	ToolExecutionStartedTopic = "tool:execution:started"
	// ToolExecutionCompletedTopic 工具执行完成
	ToolExecutionCompletedTopic = "tool:execution:completed"
)
