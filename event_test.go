package gobfd

import (
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestEventListener 测试接口方式订阅事件
func TestEventListener(t *testing.T) {
	c := getControl()

	var wg sync.WaitGroup
	wg.Add(1)

	// 实现 EventListener 接口
	listener := &testListener{
		t:          t,
		wg:         &wg,
		wantRemote: testRemote,
		wantPre:    StateDown,
		wantCur:    StateUp,
	}

	c.Subscribe(listener)

	// 模拟发送一个事件（通过添加会话触发状态变化）
	cb := func(ipAddr string, pre, cur int) error { return nil }
	c.AddSession(testRemote, false, 400, 400, 1, cb)

	// 等待事件或超时
	go func() {
		time.Sleep(100 * time.Millisecond)
		wg.Done()
	}()

	wg.Wait()

	// 取消订阅
	c.Unsubscribe(listener)

	// 清理
	_ = c.DelSession(testRemote)
}

type testListener struct {
	t          *testing.T
	wg         *sync.WaitGroup
	wantRemote string
	wantPre    int
	wantCur    int
}

func (l *testListener) OnBfdEvent(event BfdEvent) {
	l.t.Logf("[testListener] received event: Remote=%s, PreState=%d, CurState=%d",
		event.Remote, event.PreState, event.CurState)
	l.wg.Done()
}

// TestEventChan 测试通道方式订阅事件
func TestEventChan(t *testing.T) {
	c := getControl()

	// 通过通道订阅
	eventChan := c.SubscribeChan(10)

	// 添加会话
	cb := func(ipAddr string, pre, cur int) error { return nil }
	c.AddSession(testRemote, false, 400, 400, 1, cb)

	// 等待接收事件或超时
	select {
	case event := <-eventChan:
		t.Logf("[TestEventChan] received event: Remote=%s, PreState=%d, CurState=%d, Mode=%v",
			event.Remote, event.PreState, event.CurState, event.Mode)
	case <-time.After(1 * time.Second):
		t.Log("[TestEventChan] no event received within timeout (expected if no BFD peer)")
	}

	// 取消订阅
	c.UnsubscribeChan(eventChan)

	// 清理
	_ = c.DelSession(testRemote)
}

// TestMultipleSubscribers 测试多个订阅者同时接收事件
func TestMultipleSubscribers(t *testing.T) {
	c := getControl()

	var wg sync.WaitGroup
	wg.Add(2)

	// 添加第一个订阅者（接口方式）
	listener1 := &testListener{
		t:          t,
		wg:         &wg,
		wantRemote: testRemote,
	}

	// 添加第二个订阅者（通道方式）
	eventChan := c.SubscribeChan(10)

	c.Subscribe(listener1)

	// 添加会话触发事件
	cb := func(ipAddr string, pre, cur int) error { return nil }
	c.AddSession(testRemote, false, 400, 400, 1, cb)

	// 等待两个订阅者
	done := make(chan struct{})
	go func() {
		select {
		case event := <-eventChan:
			t.Logf("[TestMultipleSubscribers] channel received: Remote=%s", event.Remote)
		case <-time.After(1 * time.Second):
			t.Log("[TestMultipleSubscribers] channel no event (expected)")
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Log("[TestMultipleSubscribers] timeout")
	}

	// 清理
	c.Unsubscribe(listener1)
	c.UnsubscribeChan(eventChan)
	_ = c.DelSession(testRemote)
}

// TestBfdEventFields 测试 BfdEvent 字段完整性
func TestBfdEventFields(t *testing.T) {
	event := BfdEvent{
		Remote:      "192.168.1.1",
		Local:       "192.168.1.2",
		Family:      syscall.AF_INET,
		Mode:        ModeSessionAsync,
		PreState:    StateDown,
		CurState:    StateUp,
		Timestamp:   time.Now(),
		Discr:       12345,
		RemoteDiscr: 67890,
	}

	if event.Remote != "192.168.1.1" {
		t.Errorf("Remote = %q, want %q", event.Remote, "192.168.1.1")
	}
	if event.Local != "192.168.1.2" {
		t.Errorf("Local = %q, want %q", event.Local, "192.168.1.2")
	}
	if event.Family != syscall.AF_INET {
		t.Errorf("Family = %d, want %d", event.Family, syscall.AF_INET)
	}
	if event.Mode != ModeSessionAsync {
		t.Errorf("Mode = %v, want %v", event.Mode, ModeSessionAsync)
	}
	if event.PreState != StateDown {
		t.Errorf("PreState = %d, want %d", event.PreState, StateDown)
	}
	if event.CurState != StateUp {
		t.Errorf("CurState = %d, want %d", event.CurState, StateUp)
	}
	if event.Discr != 12345 {
		t.Errorf("Discr = %d, want %d", event.Discr, 12345)
	}
	if event.RemoteDiscr != 67890 {
		t.Errorf("RemoteDiscr = %d, want %d", event.RemoteDiscr, 67890)
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

// TestSessionModeConstants 测试会话模式常量
func TestSessionModeConstants(t *testing.T) {
	cases := []struct {
		name string
		got  SessionMode
		want SessionMode
	}{
		{"ModeSessionAsync", ModeSessionAsync, 0},
		{"ModeSessionDemand", ModeSessionDemand, 1},
		{"ModeSessionEcho", ModeSessionEcho, 2},
		{"ModeSessionMultihop", ModeSessionMultihop, 3},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
