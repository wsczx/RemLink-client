package core

import (
	"sync"
)

// eventBus 简单的 fan-out 事件总线：一个生产者、多消费者（通常宿主单消费者）。
// 缓冲事件避免生产者阻塞；消费者退出时调用返回的 cancel 释放订阅。
// 每个 kind 缓存最近一次事件，新订阅者订阅时立即重放快照（避免订阅前事件丢失）。
type eventBus struct {
	mu   sync.Mutex
	subs []chan Event
	buf  int
	last map[string]Event // 按 kind 缓存最近一次事件，供新订阅者重放
}

func newEventBus(buf int) *eventBus {
	if buf < 1 {
		buf = 1
	}
	return &eventBus{buf: buf, last: map[string]Event{}}
}

// sub 注册一个订阅者，返回事件通道与取消函数；并立即重放各 kind 最近一次事件，
// 使订阅者无需关心订阅时机（典型场景：Connect 已在内部 emit 首批事件后才订阅）。
func (b *eventBus) sub() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, b.buf)
	b.subs = append(b.subs, ch)
	// 重放快照（按 map 迭代顺序无所谓，订阅者按自身逻辑处理）
	for _, e := range b.last {
		select {
		case ch <- e:
		default:
		}
	}
	return ch
}

// publish 向所有订阅者广播一个事件（非阻塞：慢消费者会丢事件，避免状态机被拖死），
// 同时更新各 kind 的最近缓存，供后续订阅者重放。
func (b *eventBus) publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if e.Kind != "" {
		b.last[e.Kind] = e
	}
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// close 关闭所有订阅通道（Disconnect/进程退出时调用）。
func (b *eventBus) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	b.last = map[string]Event{}
}
