# gobfd

BFD（Bidirectional Forwarding Detection）协议的 Go 语言实现，支持 RFC 5880 标准的控制报文模式和 Echo 模式。

## 特性

- **RFC 5880 兼容**：完整实现 BFD 控制报文协议
- **Echo 模式**：支持 RFC 5880 Section 6.4 的 Echo 辅助检测模式
- **多会话管理**：单个 Control 实例可管理多个 BFD 检测会话
- **IPv4/IPv6**：支持双协议栈
- **状态回调**：会话状态变化时触发自定义回调函数

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

### 类型

```go
// CallbackFunc 会话状态变化回调函数
type CallbackFunc func(ipAddr string, preState, curState int) error

// Control BFD 控制实例, 负责管理多个检测会话的收发包与状态机
// 通过 NewControl 创建, 不应直接构造
type Control struct{ ... }
```

### 方法

```go
// NewControl 创建并启动 BFD 控制实例
func NewControl(local string, family int) *Control

// AddSession 添加控制报文模式会话
func (c *Control) AddSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc)

// AddEchoSession 添加 Echo 模式会话
// echoInterval: Echo 报文发送间隔(毫秒), >0 启用 Echo
func (c *Control) AddEchoSession(remote string, passive bool, rxInterval, txInterval, detectMult, echoInterval int, f CallbackFunc)

// DelSession 删除会话
func (c *Control) DelSession(remote string) error
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

# 多个目标
go run ./cmd/gobfd -remote 192.168.1.244 -remote 192.168.1.185

# 自定义参数
go run ./cmd/gobfd -local 0.0.0.0 -remote 192.168.1.244 -rx 400 -tx 400 -mult 1

# IPv6
go run ./cmd/gobfd -family 6 -remote fe80::1

# 限时运行
go run ./cmd/gobfd -remote 192.168.1.244 -duration 30
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
| `-duration` | 运行时长(秒)，<=0 一直运行 | 0 |

## 测试

```bash
# 运行所有测试
go test -v ./...

# 仅运行根包测试
go test -v .

# 仅运行 Echo 相关测试
go test -v -run 'Echo' ./...
```

## 项目结构

```
gobfd/
├── api.go              # 对外公共 API
├── api_test.go         # API 单元测试
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── gobfd/
│       └── main.go     # 命令行示例
└── internal/           # 内部实现
    ├── control.go      # 控制层
    ├── control_test.go
    ├── log.go          # 日志
    ├── packet.go       # 报文编解码
    ├── packet_test.go
    ├── session.go      # 会话状态机
    └── transport.go    # 传输层(UDP服务器/Echo反射器)
```
