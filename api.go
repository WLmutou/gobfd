package gobfd

import (
	internalbfd "github.com/WLmutou/gobfd/internal"
)

// BFD 会话状态常量 (RFC 5880)
// 在回调函数 CallbackFunc 中作为 preState / curState 返回
const (
	StateAdminDown = 0 // 管理Down
	StateDown      = 1 // Down
	StateInit      = 2 // 初始化
	StateUp        = 3 // Up
)

// BFD 使用的 UDP 端口 (RFC 5880)
const (
	ControlPort = internalbfd.CONTROL_PORT // BFD Control 报文端口 (3784)
	EchoPort    = internalbfd.ECHO_PORT    // BFD Echo 报文端口 (3785)
)

// CallbackFunc BFD 会话状态变化时触发的回调函数类型
// ipAddr:   目标对端ip
// preState: 变化前的状态 (取值为 StateXXX 常量)
// curState: 变化后的状态 (取值为 StateXXX 常量)
type CallbackFunc = internalbfd.CallbackFunc

// Control BFD 控制实例, 负责管理多个检测会话的收发包与状态机
type Control = internalbfd.Control

// NewControl 创建并启动一个 BFD 控制实例
// local:  本地绑定ip (如 "0.0.0.0")
// family: 协议家族, syscall.AF_INET(ipv4) 或 syscall.AF_INET6(ipv6)
func NewControl(local string, family int) *Control {
	return internalbfd.NewControl(local, family)
}
