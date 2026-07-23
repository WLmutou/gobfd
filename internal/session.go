package gobfd

import (
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"

	"github.com/google/gopacket/layers"

	"time"
)

const (
	defaultDetectMult = 3
	SourcePortMin     = 49152
	SourcePortMax     = 65535

	VERSION = 1

	// Default timers
	DesiredMinTXInterval = 1000000 // Minimum initial value

	ControlPlaneIndependent = false // Control Plane Independent

	DemandMode                = false // Demand Mode
	MULTIPOINT                = false // Multipoint
	RequiredMinEchoRxInterval = 0     //  Do not support echo packet
)

type Session struct {
	conn *net.UDPConn

	clientDone chan bool // true: down
	clientQuit chan bool // true: 退出

	// 回调状态
	callFunc CallbackFunc

	// BFD session
	Local      string
	Remote     string
	Family     int
	Passive    bool
	RxInterval int
	TxInterval int

	// As per 6.8.1 State Variables
	State       layers.BFDState
	RemoteState layers.BFDState
	LocalDiscr  layers.BFDDiscriminator
	RemoteDiscr layers.BFDDiscriminator
	LocalDiag   layers.BFDDiagnostic

	desiredMinTxInterval  uint32
	requiredMinRxInterval uint32
	remoteMinRxInterval   uint32

	DemandMode       bool // 异步模式和demand模式, 异步互相发送, demand模式只有在需要的时候才发送BFD control packet
	RemoteDemandMode bool
	DetectMult       uint8 //报文最大失效的个数, layers.BFDDetectMultiplier,
	AuthType         bool
	RcvAuthSeq       int
	XmitAuthSeq      int64
	AuthSeqKnown     bool

	// State Variables beyond those defined in RFC 5880
	asyncTxInterval      uint32
	finalAsyncTxInterval uint32 // layers.BFDTimeInterval
	LastRxPacketTime     int64  // 为了存储最后获取包的时间(毫秒)
	asyncDetectTime      uint32 // 也是毫秒
	finalAsyncDetectTime uint32 //
	PollSequence         bool
	remoteDetectMult     uint32 //layers.BFDDetectMultiplier
	remoteMinTxInterval  uint32 //layers.BFDTimeInterval
	txPackets            *layers.BFD

	// Echo 模式相关字段 (RFC 5880 Section 6.4)
	EchoEnabled               bool   // 是否启用 Echo 模式
	echoInterval              uint32 // Echo 报文发送间隔(微秒)
	echoDetectTime            uint32 // Echo 检测超时时间(微秒)
	requiredMinEchoRxInterval uint32 // 本端支持的最小 Echo 接收间隔(微秒), 通告给对端
	lastEchoRxTime            int64  // 最近一次收到 Echo 回送的时间(纳秒)
	echoConn                  *net.UDPConn
	echoQuit                  chan bool // 退出 Echo 循环
}

func NewSession(local, remote string, family int, passive bool,
	rxInterval, txInterval, detectMult int, f CallbackFunc) *Session {

	if detectMult <= 0 {
		detectMult = defaultDetectMult
	}

	rand.Seed(time.Now().UnixNano())

	tmpSess := &Session{
		clientDone: make(chan bool),
		clientQuit: make(chan bool),
		echoQuit:   make(chan bool, 1), // buffered, 避免 DelSession 阻塞
		callFunc:   f,
		Local:      local,
		Remote:     remote,
		Family:     family,
		Passive:    passive,
		RxInterval: rxInterval,
		TxInterval: txInterval,
		//
		State:       layers.BFDStateDown,
		RemoteState: layers.BFDStateDown,
		LocalDiscr:  layers.BFDDiscriminator(rand.Int63n(4294967295)), // 32-bit
		RemoteDiscr: 0,
		LocalDiag:   layers.BFDDiagnosticNone,
		//desiredMinTxInterval:  DesiredMinTXInterval,
		//requiredMinRxInterval:  uint32(rxInterval), //layers.BFDTimeInterval(rxInterval),
		remoteMinRxInterval: 1,
		DemandMode:          DemandMode,
		RemoteDemandMode:    false,
		DetectMult:          uint8(detectMult), //layers.BFDDetectMultiplier(detectMult),
		AuthType:            true,              //  是否需要认证
		RcvAuthSeq:          0,
		XmitAuthSeq:         rand.Int63n(4294967295), // 32-bit
		AuthSeqKnown:        false,
		//
		asyncTxInterval: DesiredMinTXInterval,
		PollSequence:    false,
	}

	tmpSess.setDesiredMinTxInterval(DesiredMinTXInterval)
	tmpSess.setRequiredMinRxInterval(uint32(rxInterval))

	go tmpSess.sessionLoop()

	return tmpSess
}

// NewEchoSession 创建一个启用 Echo 模式的 BFD 会话
// echoInterval: Echo 报文发送间隔(毫秒), >0 时启用 Echo
func NewEchoSession(local, remote string, family int, passive bool,
	rxInterval, txInterval, detectMult, echoInterval int, f CallbackFunc) *Session {

	tmpSess := NewSession(local, remote, family, passive, rxInterval, txInterval, detectMult, f)

	if echoInterval <= 0 {
		return tmpSess
	}

	tmpSess.EchoEnabled = true
	tmpSess.echoInterval = uint32(echoInterval * 1000)                 // 转为微秒
	tmpSess.echoDetectTime = uint32(detectMult) * tmpSess.echoInterval // detectMult * echoInterval
	// 通告对端本端支持的最小 Echo 接收间隔
	tmpSess.requiredMinEchoRxInterval = tmpSess.echoInterval
	// 初始化为当前时间, 避免一启动就误判超时
	tmpSess.lastEchoRxTime = time.Now().UnixNano()

	go tmpSess.echoLoop()

	return tmpSess
}

func (s *Session) sessionLoop() {
	slogger.Infof("setting up UDP client for %s:%d", s.Remote, CONTROL_PORT)

	conn, err := NewClient(s.Local, s.Remote, s.Family)
	if err != nil {
		logger.Error("loop new client close client chan")
		s.clientDone <- true
	} else {
		s.conn = conn
	}

	var interval float64
	for {
		if s.DetectMult == 1 {
			// 如果bfd.DetectMult == 1, 那间隔必须不能超过 90% 和必须有不能小于75% 间隔
			interval = float64(s.asyncTxInterval) * (rand.Float64()*0.75 + 0.15)
		} else {
			interval = float64(s.asyncTxInterval) * (1 - (rand.Float64() * 0) + 0.25)
		}

		select {
		case <-s.clientDone:
			//fmt.Println("new client ...")
			conn, err := NewClient(s.Local, s.Remote, s.Family)
			if err != nil {
				s.closeConn()
				time.Sleep(time.Duration(int(interval)) * time.Microsecond)
				continue
			}
			s.conn = conn
			s.clientDone = make(chan bool)
			// 启动检测
			go s.DetectFailure()

		case <-s.clientQuit:
			// 执行退出
			s.closeConn()
			return

		default:
			if !((s.RemoteDiscr == 0 && s.Passive) ||
				(s.remoteMinRxInterval == 0) ||
				(!s.PollSequence &&
					(s.RemoteDemandMode == true &&
						s.State == layers.BFDStateUp &&
						s.RemoteState == layers.BFDStateUp))) {
				// 判断是否应该主动发包
				s.TxPacket(false)
			}
			time.Sleep(time.Duration(int(interval)) * time.Microsecond / 10) // 决定发包速度
		}

	}
}

// 处理received接收到的包
func (s *Session) RxPacket(p *layers.BFD) {
	//fmt.Println("====================== session rx packet ===================")
	if p.AuthPresent && !s.AuthType {
		logger.Error("Received packet with authentication while no authentication is configured locally")
		return
	}

	if !p.AuthPresent && s.AuthType {
		logger.Error("Received packet without authentication while authentication is configured locally")
		return
	}
	if p.AuthPresent != s.AuthType {
		logger.Error("Authenticated packet received, not supported!")
		return
	}

	// 设置远程的bfd.RemoteDiscr 为 My Discriminator.
	s.RemoteDiscr = p.MyDiscriminator

	// 设置远程状态
	s.RemoteState = p.State

	s.RemoteDemandMode = p.Demand

	//
	s.setRemoteMinRxInterval(uint32(p.RequiredMinRxInterval))

	//
	s.setRemoteDetectMult(uint32(p.DetectMultiplier))

	//
	s.setRemoteMinTxInterval(uint32(p.DesiredMinTxInterval))

	if s.State == layers.BFDStateAdminDown {
		slogger.Warnf("Received packet from %s while in Admin Down state", s.Remote)
		return
	}

	if p.State == layers.BFDStateAdminDown {
		if s.State != layers.BFDStateDown {
			s.LocalDiag = layers.BFDDiagnosticNeighborSignalDown
			// 状态变化,执行回调函数
			go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateDown))

			s.State = layers.BFDStateDown
			s.desiredMinTxInterval = DesiredMinTXInterval
			slogger.Errorf("BFD remote %s signaled going ADMIN_DOWN", s.Remote)

		}
	} else {
		if s.State == layers.BFDStateDown {
			if p.State == layers.BFDStateDown {
				// 状态变化,执行回调函数
				go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateInit))

				s.State = layers.BFDStateInit
				slogger.Errorf("BFD session with %s going to INIT state", s.Remote)

			} else if p.State == layers.BFDStateInit {
				// 状态变化,执行回调函数
				go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateUp))

				s.State = layers.BFDStateUp
				s.setDesiredMinTxInterval(uint32(s.TxInterval))
				slogger.Errorf("BFD session with %s going to UP state", s.Remote)
			}
		} else if s.State == layers.BFDStateInit {
			if p.State == layers.BFDStateInit || p.State == layers.BFDStateUp {
				// 状态变化,执行回调函数
				go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateUp))

				s.State = layers.BFDStateUp
				s.setDesiredMinTxInterval(uint32(s.TxInterval))
				slogger.Errorf("BFD session with %s going to UP state", s.Remote)
			}
		} else {
			if p.State == layers.BFDStateDown {
				s.LocalDiag = layers.BFDDiagnosticNeighborSignalDown
				// 状态变化,执行回调函数
				go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateDown))

				s.State = layers.BFDStateDown
				slogger.Errorf("BFD remote %s signaled going DOWN", s.Remote)

			}
		}
	}

	// If a BFD Control packet is received with the Poll (P) bit set to 1,
	// the receiving system MUST transmit a BFD Control packet with the Poll
	//  (P) bit clear and the Final (F) bit set as soon as practicable, ...
	if p.Poll {
		slogger.Infof("Received packet with Poll (P) bit set from %s, sending packet with Final (F) bit set", s.Remote)
		s.TxPacket(true)
	}

	// When the system sending the Poll sequence receives a packet with
	// Final, the Poll Sequence is terminated
	if p.Final {
		slogger.Infof("Received packet with Final (F) bit set from %s, ending Poll Sequence", s.Remote)
		s.PollSequence = false
		if s.finalAsyncTxInterval > 0 {
			slogger.Infof("Increasing Tx Interval from %d to %d now that Poll Sequence has ended", s.asyncTxInterval, s.finalAsyncTxInterval)
			s.asyncTxInterval = s.finalAsyncTxInterval
			s.finalAsyncTxInterval = 0
		}
		if s.finalAsyncDetectTime > 0 {
			slogger.Infof("Increasing Detect Time from %d to %d now that Poll Sequence has ended.", s.asyncDetectTime, s.finalAsyncDetectTime)
			s.asyncDetectTime = s.finalAsyncDetectTime
			s.finalAsyncDetectTime = 0
		}
	}

	s.LastRxPacketTime = time.Now().UnixNano() / 1e6 // 毫秒

}

/*
	(Version
	Diagnostic
	State
	Poll
	Final
	ControlPlaneIndependent
	AuthPresent
	Demand
	Multipoint
	DetectMultiplier
	MyDiscriminator
	YourDiscriminator
	DesiredMinTxInterval
	RequiredMinRxInterval
	RequiredMinEchoRxInterval
	AuthHeader)
*/

// 将要target发送的包
func (s *Session) TxPacket(final bool) {
	//fmt.Println("tx packet...", s.conn.LocalAddr().String())
	var demand bool
	if s.DemandMode && s.State == layers.BFDStateUp && s.RemoteState == layers.BFDStateUp {
		demand = true
	} else {
		demand = false
	}

	var poll bool
	if !final {
		poll = s.PollSequence
	} else {
		poll = false
	}

	var tmpAuth *layers.BFDAuthHeader
	if s.AuthType {
		tmpAuth = auth
	} else {
		tmpAuth = nil
	}

	txByte := EncodePacket(VERSION,
		s.LocalDiag,
		s.State,
		poll,
		final,
		ControlPlaneIndependent,
		s.AuthType,
		demand,
		MULTIPOINT,
		layers.BFDDetectMultiplier(s.DetectMult),
		s.LocalDiscr,
		s.RemoteDiscr,
		layers.BFDTimeInterval(s.desiredMinTxInterval),
		layers.BFDTimeInterval(s.requiredMinRxInterval),
		layers.BFDTimeInterval(s.requiredMinEchoRxInterval),
		tmpAuth)

	_, err := s.conn.Write(txByte)
	if err != nil {
		//log.Println(err.Error())
		logger.Debug("send byte to udp server error:" + err.Error())
		s.closeConn()
		return
	}
	return
}

func (s *Session) restartTxPackets() {
	//fmt.Println("restart close client chan")
	s.closeConn()
}

func (s *Session) closeConn() {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	s.conn.Close()
	close(s.clientDone)

}

// 计算探测时间"""Calculate the BFD Detection Time"""
func (s *Session) calcDetectTime(detectMult, rxInterval, txInterval uint32) (ret uint32) {
	if detectMult == 0 && rxInterval == 0 && txInterval == 0 {
		slogger.Debugf("BFD Detection Time calculation not possible values detect_mult: %d rx_interval: %d tx_interval: %d", detectMult, rxInterval, txInterval)
		return 0
	}
	if rxInterval > txInterval {
		ret = detectMult * rxInterval
	} else {
		ret = detectMult * txInterval
	}

	//slogger.Debugf("BFD Detection Time calculated using detect_mult: %d rx_interval: %d tx_interval: %d" , detectMult, rxInterval, txInterval)
	return
}

func (s *Session) setRemoteDetectMult(value uint32) {
	if value == s.remoteDetectMult {
		return
	}
	s.asyncDetectTime = s.calcDetectTime(uint32(value), uint32(s.requiredMinRxInterval), uint32(s.remoteDetectMult))
	s.remoteDetectMult = value
}

func (s *Session) setRemoteMinTxInterval(value uint32) {
	if value == s.remoteMinTxInterval {
		return
	}
	s.asyncDetectTime = s.calcDetectTime(uint32(s.remoteDetectMult), uint32(s.requiredMinRxInterval), uint32(value))
	s.remoteMinRxInterval = value
}

func (s *Session) setRemoteMinRxInterval(value uint32) {
	if value == s.remoteMinRxInterval {
		return
	}
	oldTxInterval := s.asyncTxInterval
	if value > s.desiredMinTxInterval {
		s.asyncTxInterval = value
	} else {
		s.asyncTxInterval = s.desiredMinTxInterval
	}

	if s.asyncTxInterval < oldTxInterval {
		// restart tx packets
		s.restartTxPackets()
	}
	s.remoteMinRxInterval = value
}

func (s *Session) setRequiredMinRxInterval(value uint32) {
	if value == s.requiredMinRxInterval {
		return
	}
	detectTime := s.calcDetectTime(uint32(s.remoteDetectMult), uint32(value), uint32(s.remoteMinRxInterval))
	if value < s.requiredMinRxInterval && s.State == layers.BFDStateUp {
		s.finalAsyncDetectTime = detectTime
	} else {
		s.asyncDetectTime = detectTime
	}
	s.requiredMinRxInterval = value
	s.PollSequence = true

}

func (s *Session) setDesiredMinTxInterval(value uint32) {
	if value == s.desiredMinTxInterval {
		return
	}
	var txInterval uint32
	if value > s.remoteMinRxInterval {
		txInterval = value
	} else {
		txInterval = s.remoteMinRxInterval
	}

	if value > s.desiredMinTxInterval && s.State == layers.BFDStateUp {
		s.finalAsyncDetectTime = txInterval
	} else {
		s.asyncTxInterval = value
	}
	s.desiredMinTxInterval = value
	s.PollSequence = true
}

// 发送超时失败
func (s *Session) DetectFailure() {
	for {
		select {
		case <-s.clientDone:
			return
		default:
			if !(s.DemandMode || s.asyncDetectTime == 0) {
				if (s.State == layers.BFDStateInit || s.State == layers.BFDStateUp) &&
					((time.Now().UnixNano()/1e6 - s.LastRxPacketTime) > (int64(s.asyncDetectTime) / 1000)) {

					// 状态变化,执行回调函数
					go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateDown))

					s.State = layers.BFDStateDown
					s.LocalDiag = layers.BFDDiagnosticTimeExpired
					s.setDesiredMinTxInterval(DesiredMinTXInterval)

					slogger.Errorf("Detected BFD remote %s going DOWN ", s.Remote)

					slogger.Infof("Time since last packet: %d ms; Detect Time: %d ms ", (time.Now().UnixNano()/1e6 - s.LastRxPacketTime), int64(s.asyncDetectTime)/1000)

					//fmt.Printf("Detected BFD remote %s going DOWN \n", s.Remote)
					fmt.Printf("Time since last packet: %d ms; Detect Time: %d ms \n", (time.Now().UnixNano()/1e6 - s.LastRxPacketTime), int64(s.asyncDetectTime)/1000)

				}
			}

			time.Sleep(time.Millisecond / 10) // 这里等待时间, 如果太短,cpu占用就大,等待时长,最后的结果不是很准

		}
	}
}

//////////////////////////////// Echo 模式 (RFC 5880 Section 6.4) ////////////////////////////////
//
// Echo 模式下, 本端主动向对端 ECHO_PORT 发送 Echo 报文, 对端在网络层直接将报文
// 原样环回。本端通过检测 Echo 回送报文的到达情况来判断转发路径的健康度, 并可计算
// 往返时延 RTT。若在 echoDetectTime 内未收到回送, 则认为路径故障, 会话状态转为 Down。

// echoLoop Echo 模式主循环: 周期性发送 Echo 报文并检测超时
func (s *Session) echoLoop() {
	slogger.Infof("setting up BFD Echo client for %s:%d", s.Remote, ECHO_PORT)

	conn, err := NewEchoClient(s.Local, s.Remote, s.Family)
	if err != nil {
		logger.Error("echo client setup error: " + err.Error())
		return
	}
	defer conn.Close()
	s.echoConn = conn

	// 启动回送报文读取 goroutine
	go s.echoReadLoop(conn)

	ticker := time.NewTicker(time.Duration(s.echoInterval) * time.Microsecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.echoQuit:
			slogger.Infof("BFD Echo loop for %s stopped", s.Remote)
			return
		case <-ticker.C:
			// 仅在会话 Up 时发送 Echo, 避免在协商完成前发包
			if s.State != layers.BFDStateUp {
				continue
			}

			now := time.Now().UnixNano()
			pkt := EncodeEchoPacket(uint32(s.LocalDiscr), now)
			if _, err := conn.Write(pkt); err != nil {
				logger.Debug("echo send error: " + err.Error())
				continue
			}

			// 检测 Echo 超时
			last := atomic.LoadInt64(&s.lastEchoRxTime)
			elapsedUs := (now - last) / 1000 // 微秒
			if elapsedUs > int64(s.echoDetectTime) {
				slogger.Errorf("BFD Echo timeout for %s, declaring DOWN (elapsed=%d us, detect=%d us)",
					s.Remote, elapsedUs, s.echoDetectTime)

				// 状态变化, 执行回调函数
				go s.callFunc(s.Remote, int(s.State), int(layers.BFDStateDown))

				s.State = layers.BFDStateDown
				s.LocalDiag = layers.BFDDiagnosticTimeExpired
				s.setDesiredMinTxInterval(DesiredMinTXInterval)
			}
		}
	}
}

// echoReadLoop 读取 Echo 回送报文, 更新最近接收时间并计算 RTT
func (s *Session) echoReadLoop(conn *net.UDPConn) {
	buf := make([]byte, 64)
	for {
		select {
		case <-s.echoQuit:
			return
		default:
		}
		// 设置读超时, 以便周期性检查 echoQuit
		conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}
		discr, sendTs, err := DecodeEchoPacket(buf[:n])
		if err != nil {
			continue
		}
		// 仅匹配本会话标识符的回送报文
		if discr != uint32(s.LocalDiscr) {
			continue
		}
		now := time.Now().UnixNano()
		atomic.StoreInt64(&s.lastEchoRxTime, now)
		rttMs := (now - sendTs) / 1e6
		slogger.Debugf("Echo reply from %s, RTT=%d ms", s.Remote, rttMs)
	}
}
