# gobfd

BFD（Bidirectional Forwarding Detection）协议的 Go 语言实现，支持 RFC 5880 标准的控制报文模式、Echo 模式、Demand 模式，以及 RFC 5883 的 Multihop 模式，并提供认证机制和事件通知功能。

## 特性

- **RFC 5880 兼容**：完整实现 BFD 控制报文协议
- **Echo 模式**：支持 RFC 5880 Section 6.4 的 Echo 辅助检测模式
- **Demand 模式**：支持 RFC 5880 Section 6.5 的低开销 Demand 模式
- **Multihop 模式**：支持 RFC 5883 的多跳路径检测
- **认证机制**：支持 RFC 5880 的认证方式（Password、Keyed MD5、Keyed SHA1），防止报文欺骗和攻击
- **多会话管理**：单个 Control 实例可管理多个 BFD 检测会话
- **IPv4/IPv6**：支持双协议栈
- **状态回调**：会话状态变化时触发自定义回调函数
- **事件通知机制**：为上层协议(OSPF/BGP等)提供会话状态变更事件通知，支持接口方式和通道方式两种订阅模式

## BFD 帧结构

```
///////////////////////////// BFD Control 帧结构  ///////////////////////////
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|Vers |  Diag   |Sta|P|F|C|A|D|M|  Detect Mult  |    Length     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       My Discriminator                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Your Discriminator                       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Desired Min TX Interval                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                   Required Min RX Interval                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Required Min Echo RX Interval                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

```
///////////////////////////// BFD Echo 帧结构  ///////////////////////////
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    My Discriminator                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Timestamp (高 32 位, 纳秒)                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Timestamp (低 32 位, 纳秒)                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

```
///////////////////////////// BFD 认证头部结构  ///////////////////////////
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Auth Type    | Key ID | Data Len |        Reserved           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Sequence Number                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Authentication Data ...                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

## 安装

```bash
go get github.com/WLmutou/gobfd
```

## 快速开始

### 基础用法（控制报文模式）

```go
package main

import (
	"fmt"
	"syscall"
	"time"

	"github.com/WLmutou/gobfd"
)

func callBackBFDState(ipAddr string, preState, curState int) error {
	fmt.Printf("[BFD] %s state: %s -> %s\n", ipAddr, stateName(preState), stateName(curState))
	return nil
}

func stateName(s int) string {
	switch s {
	case gobfd.StateAdminDown:
		return "AdminDown"
	case gobfd.StateDown:
		return "Down"
	case gobfd.StateInit:
		return "Init"
	case gobfd.StateUp:
		return "Up"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

func main() {
	family := syscall.AF_INET
	local := "0.0.0.0"

	passive := false
	rxInterval := 400
	txInterval := 400
	detectMult := 1

	control := gobfd.NewControl(local, family)

	control.AddSession("192.168.1.244", passive, rxInterval, txInterval, detectMult, callBackBFDState)
	control.AddSession("192.168.1.185", passive, rxInterval, txInterval, detectMult, callBackBFDState)

	fmt.Println("running for 30 seconds...")
	time.Sleep(time.Second * 30)

	control.DelSession("192.168.1.244")
	control.DelSession("192.168.1.185")
}
```

### Echo 模式

Echo 模式是一种辅助检测模式，本端发送 Echo 报文，对端在网络层直接环回，用于检测转发路径的实际时延和健康度。

```go
func main() {
	control := gobfd.NewControl("0.0.0.0", syscall.AF_INET)

	// 启用 Echo 模式，echoInterval > 0
	control.AddEchoSession("192.168.1.244", false, 400, 400, 1, 100, callBackBFDState)

	time.Sleep(time.Second * 30)
	control.DelSession("192.168.1.244")
}
```

### 事件通知机制（与上层协议联动）

BFD 的价值在于为 OSPF、BGP 等路由协议提供快速故障通知。gobfd 提供了两种订阅方式：

#### 方式一：接口方式（EventListener）

```go
// 实现 EventListener 接口
type MyRouteProtocol struct{}

func (m *MyRouteProtocol) OnBfdEvent(event gobfd.BfdEvent) {
	fmt.Printf("[OSPF] BFD event: %s -> %s for %s\n", 
		stateName(event.PreState), stateName(event.CurState), event.Remote)
	
	if event.CurState == gobfd.StateDown {
		// 触发路由收敛，切换到备用路径
		fmt.Println("Triggering route convergence...")
	}
}

func main() {
	control := gobfd.NewControl("0.0.0.0", syscall.AF_INET)
	
	// 订阅事件
	ospf := &MyRouteProtocol{}
	control.Subscribe(ospf)
	
	// 添加会话
	control.AddSession("192.168.1.244", false, 400, 400, 1, callBackBFDState)
	
	time.Sleep(time.Second * 30)
	
	// 取消订阅
	control.Unsubscribe(ospf)
}
```

#### 方式二：通道方式（EventChan）

```go
func main() {
	control := gobfd.NewControl("0.0.0.0", syscall.AF_INET)
	
	// 通过通道订阅，缓冲区大小为 10
	eventChan := control.SubscribeChan(10)
	
	// 启动事件处理协程
	go func() {
		for event := range eventChan {
			fmt.Printf("[BGP] BFD state change: %s -> %s for %s (mode=%v)\n",
				stateName(event.PreState), stateName(event.CurState),
				event.Remote, event.Mode)
		}
	}()
	
	control.AddSession("192.168.1.244", false, 400, 400, 1, callBackBFDState)
	
	time.Sleep(time.Second * 30)
	
	// 取消订阅
	control.UnsubscribeChan(eventChan)
}
```

### 认证机制

BFD 认证机制用于防止报文欺骗和攻击，支持 RFC 5880 定义的多种认证类型。

```go
func main() {
	control := gobfd.NewControl("0.0.0.0", syscall.AF_INET)

	// 使用 Keyed MD5 认证
	control.AddSessionWithAuth("192.168.1.244", false, 400, 400, 1,
		gobfd.AuthTypeKeyedMD5, 1, "mysecretkey", callBackBFDState)

	// 使用 Keyed SHA1 认证
	control.AddSessionWithAuth("192.168.1.185", false, 400, 400, 1,
		gobfd.AuthTypeKeyedSHA1, 2, "anotherkey", callBackBFDState)

	// Demand 模式 + 认证
	control.AddDemandSessionWithAuth("192.168.1.200", false, 400, 400, 1,
		gobfd.AuthTypePassword, 1, "password123", callBackBFDState)

	time.Sleep(time.Second * 30)
}
```

## API 参考

### 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `StateAdminDown` | 0 | 管理 Down 状态 |
| `StateDown` | 1 | Down 状态 |
| `StateInit` | 2 | Init 状态 |
| `StateUp` | 3 | Up 状态 |
| `ControlPort` | 3784 | BFD Control 报文 UDP 端口 |
| `EchoPort` | 3785 | BFD Echo 报文 UDP 端口 |
| `MultihopControlPort` | 4784 | BFD Multihop 端口 (RFC 5883) |
| `AuthTypeNone` | 0 | 无认证 |
| `AuthTypePassword` | 1 | 密码认证 |
| `AuthTypeKeyedMD5` | 2 | Keyed MD5 认证 |
| `AuthTypeMeticulousKeyedMD5` | 3 | Meticulous Keyed MD5 认证 |
| `AuthTypeKeyedSHA1` | 4 | Keyed SHA1 认证 |
| `AuthTypeMeticulousKeyedSHA1` | 5 | Meticulous Keyed SHA1 认证 |

### 类型

```go
// CallbackFunc 会话状态变化回调函数
type CallbackFunc func(ipAddr string, preState, curState int) error

// Control BFD 控制实例, 负责管理多个检测会话的收发包与状态机
// 通过 NewControl 创建, 不应直接构造
type Control struct{ ... }

// SessionMode BFD 会话模式
type SessionMode int

const (
	ModeSessionAsync   // 异步模式
	ModeSessionDemand  // Demand 模式 (RFC 5880 Section 6.5)
	ModeSessionEcho    // Echo 模式 (RFC 5880 Section 6.4)
	ModeSessionMultihop // Multihop 模式 (RFC 5883)
)

// AuthType BFD 认证类型 (RFC 5880)
type AuthType int

// BfdEvent BFD 会话状态变更事件
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
type EventListener interface {
	OnBfdEvent(event BfdEvent)
}

// EventChan 基于通道的事件订阅类型
type EventChan <-chan BfdEvent

// AuthConfig BFD 认证配置
type AuthConfig struct {
	AuthType AuthType // 认证类型 (AuthTypeKeyedMD5, AuthTypeKeyedSHA1等)
	KeyID    uint8    // 密钥ID (1-255)
	Key      string   // 密钥字符串
}
```

### 方法

```go
// NewControl 创建并启动 BFD 控制实例
func NewControl(local string, family int) *Control

// AddSession 添加控制报文模式会话
func (c *Control) AddSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc)

// AddSessionWithAuth 添加带认证的控制报文模式会话
func (c *Control) AddSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int, authType AuthType, keyID uint8, key string, f CallbackFunc)

// AddEchoSession 添加 Echo 模式会话
// echoInterval: Echo 报文发送间隔(毫秒), >0 启用 Echo
func (c *Control) AddEchoSession(remote string, passive bool, rxInterval, txInterval, detectMult, echoInterval int, f CallbackFunc)

// AddEchoSessionWithAuth 添加带认证的 Echo 模式会话
func (c *Control) AddEchoSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult, echoInterval int, authType AuthType, keyID uint8, key string, f CallbackFunc)

// AddDemandSession 添加 Demand 模式会话 (RFC 5880 Section 6.5)
func (c *Control) AddDemandSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc)

// AddDemandSessionWithAuth 添加带认证的 Demand 模式会话
func (c *Control) AddDemandSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int, authType AuthType, keyID uint8, key string, f CallbackFunc)

// AddMultihopSession 添加 Multihop 模式会话 (RFC 5883)
func (c *Control) AddMultihopSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc)

// AddMultihopSessionWithAuth 添加带认证的 Multihop 模式会话
func (c *Control) AddMultihopSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int, authType AuthType, keyID uint8, key string, f CallbackFunc)

// DelSession 删除会话
func (c *Control) DelSession(remote string) error

// Subscribe 通过接口方式订阅 BFD 事件
func (c *Control) Subscribe(listener EventListener)

// SubscribeChan 通过通道方式订阅 BFD 事件
func (c *Control) SubscribeChan(bufSize int) EventChan

// Unsubscribe 取消接口方式订阅
func (c *Control) Unsubscribe(listener EventListener)

// UnsubscribeChan 取消通道方式订阅
func (c *Control) UnsubscribeChan(ch EventChan)
```

## Echo 模式工作原理

```
主机A (Echo发送端)                    主机B (Echo反射端)
    |                                     |
    |-- Echo报文 --> 3785端口 ----------->|
    |                       原样环回       |
    |<-- Echo回送 <-- 3785端口 <-----------|
    |                                     |
    检测: echoDetectTime内未收到回送 => Down
    度量: 通过Timestamp计算RTT
```

- **发送端**：向对端 3785 端口发送带时间戳的 Echo 报文
- **反射端**：收到 Echo 报文后原样回送给发送方（通过 EchoServer 实现）
- **超时检测**：若在 `detectMult * echoInterval` 时间内未收到回送，会话状态转为 Down
- **RTT 计算**：通过报文中的 Timestamp 计算往返时延

## 命令行工具

项目提供了可运行的命令行示例：

```bash
# 基础用法
go run ./cmd/gobfd -remote 192.168.1.244

# 启用 Echo 模式
go run ./cmd/gobfd -remote 192.168.1.244 -echo 100

# 启用 Demand 模式
go run ./cmd/gobfd -remote 192.168.1.244 -demand

# 启用 Multihop 模式
go run ./cmd/gobfd -remote 192.168.1.244 -multihop

# 启用认证 (Keyed MD5)
go run ./cmd/gobfd -remote 192.168.1.244 -auth-type 2 -auth-key mysecretkey -auth-keyid 1

# 启用认证 (Keyed SHA1)
go run ./cmd/gobfd -remote 192.168.1.244 -auth-type 4 -auth-key mysecretkey -auth-keyid 1

# Demand 模式 + 认证
go run ./cmd/gobfd -remote 192.168.1.244 -demand -auth-type 2 -auth-key mysecretkey

# 多个目标
go run ./cmd/gobfd -remote 192.168.1.244 -remote 192.168.1.185

# 自定义参数
go run ./cmd/gobfd -local 0.0.0.0 -remote 192.168.1.244 -rx 400 -tx 400 -mult 1

# IPv6
go run ./cmd/gobfd -family 6 -remote fe80::1

# 限时运行
go run ./cmd/gobfd -remote 192.168.1.244 -duration 30

# 启用事件通知演示模式
go run ./cmd/gobfd -remote 192.168.1.244 -event
```

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-local` | 本地绑定 IP | 0.0.0.0 |
| `-family` | 协议家族: 4=IPv4, 6=IPv6 | 4 |
| `-remote` | 对端 IP，可多次指定 | 必填 |
| `-passive` | 是否被动模式 | false |
| `-rx` | 接收间隔(毫秒) | 400 |
| `-tx` | 发送间隔(毫秒) | 400 |
| `-mult` | 报文最大失效个数 | 1 |
| `-echo` | Echo 发送间隔(毫秒)，>0 启用 | 0 |
| `-demand` | 是否启用 Demand 模式 | false |
| `-multihop` | 是否启用 Multihop 模式 | false |
| `-auth-type` | 认证类型: 0=None, 1=Password, 2=KeyedMD5, 3=MeticulousKeyedMD5, 4=KeyedSHA1, 5=MeticulousKeyedSHA1 | 0 |
| `-auth-key` | 认证密钥字符串 | "" |
| `-auth-keyid` | 认证密钥 ID (1-255) | 1 |
| `-event` | 是否启用事件通知演示模式 | false |
| `-duration` | 运行时长(秒)，<=0 一直运行 | 0 |

## 测试

```bash
# 运行所有测试
go test -v ./...

# 仅运行根包测试
go test -v .

# 仅运行 Echo 相关测试
go test -v -run 'Echo' ./...

# 仅运行认证相关测试
go test -v -run 'Auth' ./...

# 仅运行事件通知相关测试
go test -v -run 'Event' ./...
```

## 项目结构

```
gobfd/
├── api.go              # 对外公共 API
├── api_test.go         # API 单元测试
├── auth_test.go        # 认证功能测试
├── event_test.go       # 事件通知测试
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── gobfd/
│       └── main.go     # 命令行示例
└── internal/           # 内部实现
    ├── control.go      # 控制层
    ├── control_test.go
    ├── event.go        # 事件通知系统(与上层协议联动)
    ├── log.go          # 日志
    ├── packet.go       # 报文编解码
    ├── packet_test.go
    ├── session.go      # 会话状态机
    └── transport.go    # 传输层(UDP服务器/Echo反射器)
```
