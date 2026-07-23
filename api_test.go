package gobfd

import (
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/gopacket/layers"
)

// 验证导出的状态常量与 RFC 5880 / gopacket layers.BFDState 定义一致
func TestStateConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"StateAdminDown", StateAdminDown, int(layers.BFDStateAdminDown)},
		{"StateDown", StateDown, int(layers.BFDStateDown)},
		{"StateInit", StateInit, int(layers.BFDStateInit)},
		{"StateUp", StateUp, int(layers.BFDStateUp)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// 全局共享 Control 实例, 避免多个测试同时监听 3784 端口
var (
	globalControl *Control
	controlOnce   sync.Once
)

func getControl() *Control {
	controlOnce.Do(func() {
		globalControl = NewControl("127.0.0.1", syscall.AF_INET)
	})
	return globalControl
}

func TestNewControl(t *testing.T) {
	c := getControl()
	if c == nil {
		t.Fatal("NewControl returned nil")
	}
	if c.Local != "127.0.0.1" {
		t.Errorf("Control.Local = %q, want %q", c.Local, "127.0.0.1")
	}
	if c.Family != syscall.AF_INET {
		t.Errorf("Control.Family = %d, want %d", c.Family, syscall.AF_INET)
	}
	if c.RxQueue == nil {
		t.Error("Control.RxQueue is nil")
	}
}

// 127.0.0.2 同属 loopback, DialUDP 可达; 但 server 监听在 127.0.0.1, 不会收到发往 127.0.0.2 的包
const testRemote = "127.0.0.2"

func TestAddSession(t *testing.T) {
	c := getControl()
	cb := func(ipAddr string, pre, cur int) error { return nil }

	// 添加会话不应 panic
	c.AddSession(testRemote, false, 400, 400, 1, cb)

	// 清理, 避免影响其他测试
	if err := c.DelSession(testRemote); err != nil {
		t.Fatalf("cleanup DelSession error: %v", err)
	}
}

func TestDelSession(t *testing.T) {
	c := getControl()
	cb := func(ipAddr string, pre, cur int) error { return nil }
	c.AddSession(testRemote, false, 400, 400, 1, cb)

	if err := c.DelSession(testRemote); err != nil {
		t.Errorf("DelSession error: %v", err)
	}

	// 重复删除已不存在的 session 不应报错
	if err := c.DelSession(testRemote); err != nil {
		t.Errorf("DelSession again error: %v", err)
	}
}

func TestDelSessionNotExist(t *testing.T) {
	c := getControl()

	if err := c.DelSession("127.0.0.99"); err != nil {
		t.Errorf("DelSession non-existent returned error: %v", err)
	}
}

func TestCallbackFunc(t *testing.T) {
	var gotIP string
	var gotPre, gotCur int

	var cb CallbackFunc = func(ipAddr string, pre, cur int) error {
		gotIP = ipAddr
		gotPre = pre
		gotCur = cur
		return nil
	}

	if err := cb(testRemote, StateDown, StateUp); err != nil {
		t.Fatalf("callback returned error: %v", err)
	}
	if gotIP != testRemote || gotPre != StateDown || gotCur != StateUp {
		t.Errorf("callback got ip=%q pre=%d cur=%d", gotIP, gotPre, gotCur)
	}
}

func TestEchoPortConstant(t *testing.T) {
	if ControlPort != 3784 {
		t.Errorf("ControlPort = %d, want 3784", ControlPort)
	}
	if EchoPort != 3785 {
		t.Errorf("EchoPort = %d, want 3785", EchoPort)
	}
}

func TestAddEchoSession(t *testing.T) {
	c := getControl()
	cb := func(ipAddr string, pre, cur int) error { return nil }

	// 添加 Echo 会话不应 panic
	c.AddEchoSession(testRemote, false, 400, 400, 1, 100, cb)

	// 清理, 验证删除 Echo 会话不阻塞
	if err := c.DelSession(testRemote); err != nil {
		t.Fatalf("cleanup DelSession error: %v", err)
	}
}

func TestDelEchoSessionNotBlock(t *testing.T) {
	// 验证删除已退出的 Echo 会话不会因 echoQuit 阻塞
	c := getControl()
	cb := func(ipAddr string, pre, cur int) error { return nil }
	c.AddEchoSession("127.0.0.3", false, 400, 400, 1, 50, cb)

	done := make(chan struct{})
	go func() {
		_ = c.DelSession("127.0.0.3")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DelSession on echo session blocked")
	}
}
