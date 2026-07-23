package gobfd

import (
	"fmt"
	"strings"

	"github.com/google/gopacket/layers"
	"go.uber.org/zap"
)

// 回调函数
type CallbackFunc func(ipAddr string, preState, curState int) error

type Control struct {
	Local           string
	Family          int // 协议家族: ipv4, ipv6,
	RxQueue         chan *RxData
	sessions        []*Session
	eventDispatcher *EventDispatcher
}

func NewControl(local string, family int) *Control {
	tmpControl := &Control{
		Local:           local,
		Family:          family,
		RxQueue:         make(chan *RxData, 0),
		eventDispatcher: NewEventDispatcher(),
	}
	tmpControl.Run()
	return tmpControl
}

////// 添加需要检测的实例 ///////
/*
 * local: 本地ip(0.0.0.0)
 * remote: 对端ip
 * family: AF_INET4, AF_INET6
 * Passive: 是否被动模式
 * rxInterval: 接收间隔(输入毫秒单位),
 * txInterval: 发送间隔(输入毫秒单位)
 * detectMult:  报文最大失效的个数
 * f: 回调函数
 */
func (c *Control) AddSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc) {
	nsession := NewSessionWithOptions(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		false,
		0,
		false,
		c.eventDispatcher,
		f,
	)
	slogger.Debugf("Creating BFD session for remote %s.", remote)
	c.sessions = append(c.sessions, nsession)
}

func (c *Control) AddSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int,
	authType layers.BFDAuthType, authKeyID uint8, authKey string, f CallbackFunc) {
	nsession := NewSessionWithAuth(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		false,
		0,
		false,
		c.eventDispatcher,
		f,
		authType,
		authKeyID,
		authKey,
	)
	slogger.Debugf("Creating BFD session with auth for remote %s (authType=%v).", remote, authType)
	c.sessions = append(c.sessions, nsession)
}

////// 添加需要检测的实例 (启用 Echo 模式) ///////
/*
 * remote: 对端ip
 * passive: 是否被动模式
 * rxInterval: 接收间隔(输入毫秒单位),
 * txInterval: 发送间隔(输入毫秒单位)
 * detectMult:  报文最大失效的个数
 * echoInterval: Echo 报文发送间隔(毫秒), >0 启用 Echo
 * f: 回调函数
 */
func (c *Control) AddEchoSession(remote string, passive bool, rxInterval, txInterval, detectMult, echoInterval int, f CallbackFunc) {
	nsession := NewEchoSession(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		echoInterval,
		c.eventDispatcher,
		f,
	)
	slogger.Debugf("Creating BFD Echo session for remote %s (echoInterval=%dms).", remote, echoInterval)
	c.sessions = append(c.sessions, nsession)
}

func (c *Control) AddEchoSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult, echoInterval int,
	authType layers.BFDAuthType, authKeyID uint8, authKey string, f CallbackFunc) {
	nsession := NewEchoSessionWithAuth(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		echoInterval,
		c.eventDispatcher,
		f,
		authType,
		authKeyID,
		authKey,
	)
	slogger.Debugf("Creating BFD Echo session with auth for remote %s (echoInterval=%dms, authType=%v).", remote, echoInterval, authType)
	c.sessions = append(c.sessions, nsession)
}

////// 添加需要检测的实例 (启用 Demand 模式) ///////
/*
 * remote: 对端ip
 * passive: 是否被动模式
 * rxInterval: 接收间隔(输入毫秒单位),
 * txInterval: 发送间隔(输入毫秒单位)
 * detectMult:  报文最大失效的个数
 * f: 回调函数
 */
func (c *Control) AddDemandSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc) {
	nsession := NewDemandSession(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		c.eventDispatcher,
		f,
	)
	slogger.Debugf("Creating BFD Demand session for remote %s.", remote)
	c.sessions = append(c.sessions, nsession)
}

func (c *Control) AddDemandSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int,
	authType layers.BFDAuthType, authKeyID uint8, authKey string, f CallbackFunc) {
	nsession := NewDemandSessionWithAuth(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		c.eventDispatcher,
		f,
		authType,
		authKeyID,
		authKey,
	)
	slogger.Debugf("Creating BFD Demand session with auth for remote %s (authType=%v).", remote, authType)
	c.sessions = append(c.sessions, nsession)
}

// //// 添加需要检测的实例 (启用 Multihop 模式) ///////
// Multihop 模式(RFC 5883): 使用 UDP 端口 4784, 发送方设置 TTL=255,
// 用于检测非直连的多跳路径
func (c *Control) AddMultihopSession(remote string, passive bool, rxInterval, txInterval, detectMult int, f CallbackFunc) {
	nsession := NewMultihopSession(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		c.eventDispatcher,
		f,
	)
	slogger.Debugf("Creating BFD Multihop session for remote %s.", remote)
	c.sessions = append(c.sessions, nsession)
}

func (c *Control) AddMultihopSessionWithAuth(remote string, passive bool, rxInterval, txInterval, detectMult int,
	authType layers.BFDAuthType, authKeyID uint8, authKey string, f CallbackFunc) {
	nsession := NewMultihopSessionWithAuth(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		c.eventDispatcher,
		f,
		authType,
		authKeyID,
		authKey,
	)
	slogger.Debugf("Creating BFD Multihop session with auth for remote %s (authType=%v).", remote, authType)
	c.sessions = append(c.sessions, nsession)
}

////// 事件订阅接口 ///////

func (c *Control) Subscribe(listener EventListener) {
	c.eventDispatcher.Subscribe(listener)
}

func (c *Control) SubscribeChan(bufSize int) EventChan {
	return c.eventDispatcher.SubscribeChan(bufSize)
}

func (c *Control) Unsubscribe(listener EventListener) {
	c.eventDispatcher.Unsubscribe(listener)
}

func (c *Control) UnsubscribeChan(ch EventChan) {
	c.eventDispatcher.UnsubscribeChan(ch)
}

// //// 删除某个需要检测的实例  /////
func (c *Control) DelSession(remote string) error {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("Del session error:", zap.Any("err:", err))
			return
		}
	}()

	for i, session := range c.sessions {
		if session.Remote == remote {
			session.clientQuit <- true // 执行退出
			if session.EchoEnabled {
				session.echoQuit <- true // 退出 Echo 循环
			}
			c.sessions = append(c.sessions[:i], c.sessions[i+1:]...) // 删除session
		}
	}

	return nil
}

// 处理接收到的包
func (c *Control) processPackets(rxdt *RxData) {
	slogger.Debugf("Received a new packet from %s.", rxdt.Addr)

	bfdPack := rxdt.Data
	if bfdPack.YourDiscriminator > 0 {
		for _, session := range c.sessions {
			if session.LocalDiscr == bfdPack.YourDiscriminator {
				session.RxPacket(bfdPack)
				return
			}
		}
	} else {
		for _, session := range c.sessions {
			//log.Println("session remote:", session.Remote, ", packat addr:", rxdt.Addr)
			addrIp := strings.Split(rxdt.Addr, ":")[0]
			if session.Remote == addrIp {
				session.RxPacket(bfdPack)
				return
			}
		}
	}

	slogger.Infof("Dropping packet from %s as it doesnt match any configured remote.", rxdt.Addr)
}

func (c *Control) initServer() {
	slogger.Debugf("Setting up udp server on %s:%d", c.Local, CONTROL_PORT)
	addr := fmt.Sprintf("%s:%d", c.Local, CONTROL_PORT)
	s := NewServer(addr, c.Family, c.RxQueue)
	go s.Start()

	slogger.Debugf("Setting up multihop udp server on %s:%d", c.Local, MULTIHOP_CONTROL_PORT)
	multihopAddr := fmt.Sprintf("%s:%d", c.Local, MULTIHOP_CONTROL_PORT)
	multihopS := NewServerWithPort(multihopAddr, c.Family, c.RxQueue, MULTIHOP_CONTROL_PORT, true)
	go multihopS.Start()

	// 启动 Echo 反射服务器, 用于回送对端发来的 Echo 报文 (RFC 5880)
	echoAddr := fmt.Sprintf("%s:%d", c.Local, ECHO_PORT)
	echoS := NewEchoServer(echoAddr, c.Family)
	go echoS.Start()
}

func (c *Control) backgroundRun() {
	c.initServer()
	logger.Warn("BFD Daemon fully configured.")
	for {
		select {
		case rxData := <-c.RxQueue:
			c.processPackets(rxData)
		}
	}
}

func (c *Control) Run() {
	logger.Info("run...")
	go c.backgroundRun()
}
