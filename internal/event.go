package gobfd

import (
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

// SessionMode BFD 会话模式
type SessionMode int

const (
	ModeAsync    SessionMode = iota // 异步模式
	ModeDemand                      // Demand 模式 (RFC 5880 Section 6.5)
	ModeEcho                        // Echo 模式 (RFC 5880 Section 6.4)
	ModeMultihop                    // Multihop 模式 (RFC 5883)
)

func (m SessionMode) String() string {
	switch m {
	case ModeAsync:
		return "Async"
	case ModeDemand:
		return "Demand"
	case ModeEcho:
		return "Echo"
	case ModeMultihop:
		return "Multihop"
	default:
		return "Unknown"
	}
}

// BfdEvent BFD 会话状态变更事件
// 用于向上层协议(OSPF/BGP等)通知 BFD 会话状态变化
type BfdEvent struct {
	Remote      string      // 对端 IP 地址
	Local       string      // 本地 IP 地址
	Family      int         // 协议家族 (AF_INET / AF_INET6)
	Mode        SessionMode // 会话模式
	PreState    int         // 变化前状态 (StateXXX 常量)
	CurState    int         // 变化后状态 (StateXXX 常量)
	Timestamp   time.Time   // 事件发生时间
	Discr       uint32      // 本地 Discriminator
	RemoteDiscr uint32      // 对端 Discriminator
}

// EventListener BFD 事件监听器接口
// 上层协议(OSPF/BGP等)可实现此接口来接收 BFD 事件通知
type EventListener interface {
	OnBfdEvent(event BfdEvent)
}

// EventChan 基于通道的事件订阅类型
// 上层应用可通过通道异步接收 BFD 事件
type EventChan <-chan BfdEvent

type eventSubscriber struct {
	listener EventListener
	channel  chan BfdEvent
}

// EventDispatcher BFD 事件分发器
// 支持多个订阅者同时监听事件，异步分发确保不阻塞 BFD 状态机
type EventDispatcher struct {
	subscribers []eventSubscriber
	mu          sync.RWMutex
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{}
}

// Subscribe 订阅 BFD 事件(接口方式)
// listener: 实现了 EventListener 接口的对象
func (d *EventDispatcher) Subscribe(listener EventListener) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, sub := range d.subscribers {
		if sub.listener == listener {
			return
		}
	}

	d.subscribers = append(d.subscribers, eventSubscriber{
		listener: listener,
	})
}

// SubscribeChan 订阅 BFD 事件(通道方式)
// 返回一个只读通道，事件会发送到该通道
// bufSize: 通道缓冲区大小，建议 >= 16 避免慢消费者阻塞
func (d *EventDispatcher) SubscribeChan(bufSize int) EventChan {
	ch := make(chan BfdEvent, bufSize)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.subscribers = append(d.subscribers, eventSubscriber{
		channel: ch,
	})

	return ch
}

// Unsubscribe 取消订阅(接口方式)
func (d *EventDispatcher) Unsubscribe(listener EventListener) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, sub := range d.subscribers {
		if sub.listener == listener {
			d.subscribers = append(d.subscribers[:i], d.subscribers[i+1:]...)
			return
		}
	}
}

// UnsubscribeChan 取消订阅(通道方式)
func (d *EventDispatcher) UnsubscribeChan(ch EventChan) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, sub := range d.subscribers {
		if sub.channel == ch {
			close(sub.channel)
			d.subscribers = append(d.subscribers[:i], d.subscribers[i+1:]...)
			return
		}
	}
}

// Dispatch 分发事件到所有订阅者
// 异步分发，不会阻塞调用者
func (d *EventDispatcher) Dispatch(event BfdEvent) {
	d.mu.RLock()
	subscribers := make([]eventSubscriber, len(d.subscribers))
	copy(subscribers, d.subscribers)
	d.mu.RUnlock()

	for _, sub := range subscribers {
		go func(s eventSubscriber) {
			if s.listener != nil {
				s.listener.OnBfdEvent(event)
			} else if s.channel != nil {
				select {
				case s.channel <- event:
				default:
				}
			}
		}(sub)
	}
}

// GetSessionMode 根据会话获取模式
func GetSessionMode(s *Session) SessionMode {
	if s.Multihop {
		return ModeMultihop
	}
	if s.EchoEnabled {
		return ModeEcho
	}
	if s.DemandMode {
		return ModeDemand
	}
	return ModeAsync
}

// BuildBfdEvent 构建 BFD 事件
func BuildBfdEvent(s *Session, preState, curState layers.BFDState) BfdEvent {
	return BfdEvent{
		Remote:      s.Remote,
		Local:       s.Local,
		Family:      s.Family,
		Mode:        GetSessionMode(s),
		PreState:    int(preState),
		CurState:    int(curState),
		Timestamp:   time.Now(),
		Discr:       uint32(s.LocalDiscr),
		RemoteDiscr: uint32(s.RemoteDiscr),
	}
}
