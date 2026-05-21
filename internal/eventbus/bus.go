// Package eventbus 提供异步进程内事件总线，
// 用于解耦各 service 之间的通知逻辑（如 Boss 死亡 → 掉落/播报/排行榜）。
// 通过 worker goroutine 池实现异步分发，避免阻塞发布者。
package eventbus

import (
	"sync"

	"hero-quest/pkg/logger"
)

// Event 事件类型，所有事件均可作为 Event 传递
type Event any

// Handler 事件处理函数
type Handler func(e Event)

// Bus 异步进程内事件总线，通过 worker 池并发分发事件
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	ch       chan eventWrap
	quit     chan struct{}
	wg       sync.WaitGroup
}

// eventWrap 事件包装，携带 topic 和重试次数
type eventWrap struct {
	topic string
	event Event
	retry int
}

// New 创建异步事件总线实例，启动 4 个 worker goroutine
func New() *Bus {
	b := &Bus{
		handlers: make(map[string][]Handler),
		ch:       make(chan eventWrap, 1024),
		quit:     make(chan struct{}),
	}
	for i := 0; i < 4; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	return b
}

// Subscribe 订阅指定 topic 的事件
func (b *Bus) Subscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

// Publish 异步发布事件到指定 topic。
// 如果通道已满则丢弃事件并记录警告日志，避免阻塞发布者。
func (b *Bus) Publish(topic string, event Event) {
	select {
	case b.ch <- eventWrap{topic: topic, event: event}:
	default:
		logger.Warn("event bus channel full, dropping event", "topic", topic)
	}
}

// Close 关闭事件总线，停止所有 worker 并等待退出
func (b *Bus) Close() {
	close(b.quit)
	b.wg.Wait()
}

// worker 从事件通道消费事件并分发到对应的处理器
func (b *Bus) worker() {
	defer b.wg.Done()
	for {
		select {
		case <-b.quit:
			return
		case ew := <-b.ch:
			b.dispatch(ew)
		}
	}
}

// dispatch 将事件分发到所有订阅了该 topic 的处理器，
// 每个处理器独立执行，panic 不会影响其他处理器
func (b *Bus) dispatch(ew eventWrap) {
	b.mu.RLock()
	handlers := b.handlers[ew.topic]
	b.mu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("event handler panic", "topic", ew.topic, "err", r)
				}
			}()
			h(ew.event)
		}()
	}
}
