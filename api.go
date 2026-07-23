package gobfd

import (
	internalbfd "github.com/WLmutou/gobfd/internal"
	"github.com/google/gopacket/layers"
)

// BFD 会话状态常量 (RFC 5880)
// 在回调函数 CallbackFunc 中作为 preState / curState 返回
const (
	StateAdminDown = 0 // 管理Down
	StateDown      = 1 // Down
	StateInit      = 2 // 初始化
	StateUp        = 3 // Up
)

// BFD 使用的 UDP 端口 (RFC 5880 / RFC 5883)
const (
	ControlPort         = internalbfd.CONTROL_PORT          // BFD Control 报文端口 (3784)
	EchoPort            = internalbfd.ECHO_PORT             // BFD Echo 报文端口 (3785)
	MultihopControlPort = internalbfd.MULTIHOP_CONTROL_PORT // BFD Multihop 端口 (4784, RFC 5883)
)

// BFD 模式常量
const (
	ModeAsync    = false // 异步模式: 双方周期性发送 Control 报文
	ModeDemand   = true  // Demand 模式: 会话 Up 后停止周期性发送, 通过 Poll/Final 交互验证连通性
	ModeMultihop = true  // Multihop 模式: 使用端口 4784, TTL=255, 用于检测非直连路径 (RFC 5883)
)

// SessionMode BFD 会话模式
type SessionMode = internalbfd.SessionMode

const (
	ModeSessionAsync    SessionMode = iota // 异步模式
	ModeSessionDemand                      // Demand 模式 (RFC 5880 Section 6.5)
	ModeSessionEcho                        // Echo 模式 (RFC 5880 Section 6.4)
	ModeSessionMultihop                    // Multihop 模式 (RFC 5883)
)

// BFD 认证类型常量 (RFC 5880)
// 用于在创建会话时指定认证方式
const (
	AuthTypeNone                = layers.BFDAuthTypeNone                // 无认证
	AuthTypePassword            = layers.BFDAuthTypePassword            // 简单密码认证
	AuthTypeKeyedMD5            = layers.BFDAuthTypeKeyedMD5            // Keyed MD5 认证
	AuthTypeMeticulousKeyedMD5  = layers.BFDAuthTypeMeticulousKeyedMD5  // Meticulous Keyed MD5 认证
	AuthTypeKeyedSHA1           = layers.BFDAuthTypeKeyedSHA1           // Keyed SHA1 认证
	AuthTypeMeticulousKeyedSHA1 = layers.BFDAuthTypeMeticulousKeyedSHA1 // Meticulous Keyed SHA1 认证
)

// AuthType BFD 认证类型
type AuthType = layers.BFDAuthType

// BfdEvent BFD 会话状态变更事件
// 用于向上层协议(OSPF/BGP等)通知 BFD 会话状态变化
type BfdEvent = internalbfd.BfdEvent

// EventListener BFD 事件监听器接口
// 上层协议(OSPF/BGP等)可实现此接口来接收 BFD 事件通知
type EventListener = internalbfd.EventListener

// EventChan 基于通道的事件订阅类型
// 上层应用可通过通道异步接收 BFD 事件
type EventChan = internalbfd.EventChan

// CallbackFunc BFD 会话状态变化时触发的回调函数类型
// ipAddr:   目标对端ip
// preState: 变化前的状态 (取值为 StateXXX 常量)
// curState: 变化后的状态 (取值为 StateXXX 常量)
type CallbackFunc = internalbfd.CallbackFunc

// Control BFD 控制实例, 负责管理多个检测会话的收发包与状态机
type Control = internalbfd.Control

// AuthConfig BFD 认证配置
// 用于在创建会话时配置认证参数
type AuthConfig struct {
	AuthType AuthType // 认证类型 (AuthTypeKeyedMD5, AuthTypeKeyedSHA1等)
	KeyID    uint8    // 密钥ID (1-255)
	Key      string   // 密钥字符串
}

// NewControl 创建并启动一个 BFD 控制实例
// local:  本地绑定ip (如 "0.0.0.0")
// family: 协议家族, syscall.AF_INET(ipv4) 或 syscall.AF_INET6(ipv6)
func NewControl(local string, family int) *Control {
	return internalbfd.NewControl(local, family)
}
