package gobfd

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// 回调函数
type CallbackFunc func(ipAddr string, preState, curState int) error

type Control struct {
	Local    string
	Family   int // 协议家族: ipv4, ipv6,
	RxQueue  chan *RxData
	sessions []*Session
}

func NewControl(local string, family int) *Control {
	tmpControl := &Control{
		Local:  local,
		Family: family,

		RxQueue: make(chan *RxData, 0),
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
	nsession := NewSession(
		c.Local,
		remote,
		c.Family,
		passive,
		rxInterval*1000,
		txInterval*1000,
		detectMult,
		f,
	)
	slogger.Debugf("Creating BFD session for remote %s.", remote)
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
		f,
	)
	slogger.Debugf("Creating BFD Echo session for remote %s (echoInterval=%dms).", remote, echoInterval)
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
		f,
	)
	slogger.Debugf("Creating BFD Demand session for remote %s.", remote)
	c.sessions = append(c.sessions, nsession)
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
