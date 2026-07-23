package gobfd

import (
	"fmt"
	"github.com/google/gopacket/layers"
	"math/rand"
	"net"

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

	clientDone chan bool  // true: down
	clientQuit chan bool  // true: 退出

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
		AuthType:            true,  //  是否需要认证
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
				time.Sleep(time.Duration(int(interval)) * time.Microsecond )
				continue
			}
			s.conn = conn
			s.clientDone = make(chan bool)
			// 启动检测
			go s.DetectFailure()

		case <- s.clientQuit:
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
		RequiredMinEchoRxInterval,
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
					fmt.Printf("Time since last packet: %d ms; Detect Time: %d ms \n", (time.Now().UnixNano()/1e6 - s.LastRxPacketTime), int64(s.asyncDetectTime) / 1000)

				}
			}

			time.Sleep(time.Millisecond / 10) // 这里等待时间, 如果太短,cpu占用就大,等待时长,最后的结果不是很准

		}
	}
}
