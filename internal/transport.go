package gobfd

import (
	"fmt"
	"math/rand"
	"net"
	"syscall"
	"time"

	"github.com/google/gopacket/layers"
)

const (
	CONTROL_PORT = 3784
	ECHO_PORT    = 3785 // RFC 5880: BFD Echo 报文使用的 UDP 端口
)

type RxData struct {
	Data *layers.BFD
	Addr string
}

type Client struct {
	Transport int
}

func RandInt(min, max int) int {
	if min >= max || min == 0 || max == 0 {
		return max
	}
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func NewClient(local, remote string, family int) (*net.UDPConn, error) {
	var conn *net.UDPConn
	var err error
	var udpAddr *net.UDPAddr
	var rudpAddr *net.UDPAddr
	srcPort := RandInt(SourcePortMin, SourcePortMax)
	addr := fmt.Sprintf("%s:%d", local, srcPort)
	serAddr := fmt.Sprintf("%s:%d", remote, CONTROL_PORT)
	if family == syscall.AF_INET6 {
		// ipv6
		udpAddr, _ = net.ResolveUDPAddr("udp6", addr)
		rudpAddr, _ = net.ResolveUDPAddr("udp6", serAddr)
	} else {
		// ipv4
		udpAddr, _ = net.ResolveUDPAddr("udp4", addr)
		rudpAddr, _ = net.ResolveUDPAddr("udp4", serAddr)
	}
	conn, err = net.DialUDP("udp", udpAddr, rudpAddr)
	if err != nil {
		return conn, err
	}
	return conn, nil
}

/////////////////////// server /////////////////////

type Server struct {
	Addr     string
	listener *net.UDPConn // udp conn

	Family  int
	RxQueue chan *RxData
}

func NewServer(addr string, family int, rx chan *RxData) *Server {
	return &Server{
		Addr:    addr,
		Family:  family,
		RxQueue: rx,
	}
}

func (s *Server) Start() error {
	logger.Debug(" server run start...")

	var udpAddr *net.UDPAddr
	if s.Family == syscall.AF_INET6 {
		// ipv6
		udpAddr, err := net.ResolveUDPAddr("udp6", s.Addr)
		if err != nil {
			logger.Error("ResolveUDPAddr err: " + err.Error())
			return err
		}
		s.listener, err = net.ListenUDP("udp6", udpAddr)
		if err != nil {
			logger.Error("ListenUDP err:" + err.Error())
			return err
		}
	} else {
		// ipv4
		udpAddr, err := net.ResolveUDPAddr("udp4", s.Addr)
		if err != nil {
			logger.Error("ResolveUDPAddr err: " + err.Error())
			return err
		}
		s.listener, err = net.ListenUDP("udp4", udpAddr)
		if err != nil {
			logger.Error("ListenUDP err:" + err.Error())
			return err
		}
	}

	defer s.listener.Close()

	slogger.Infof("udp server run at:", udpAddr.String())

	s.Loop()

	return nil
}

func (s *Server) Loop() {
	for {
		data := make([]byte, 1024)
		n, udpConn, err := s.listener.ReadFromUDP(data)
		if err != nil {
			logger.Error("read from udp error:" + err.Error())
			continue
		}

		bfdPk, err := DecodePacket(data[:n])
		if err != nil {
			continue
		}
		rxData := &RxData{Data: bfdPk, Addr: udpConn.String()}
		s.RxQueue <- rxData
	}
}

/////////////////////// echo client /////////////////////

// NewEchoClient 创建到对端 ECHO_PORT 的 UDP 连接, 用于发送/接收 Echo 报文
func NewEchoClient(local, remote string, family int) (*net.UDPConn, error) {
	srcPort := RandInt(SourcePortMin, SourcePortMax)
	addr := fmt.Sprintf("%s:%d", local, srcPort)
	serAddr := fmt.Sprintf("%s:%d", remote, ECHO_PORT)

	var udpAddr, rudpAddr *net.UDPAddr
	var err error
	if family == syscall.AF_INET6 {
		udpAddr, _ = net.ResolveUDPAddr("udp6", addr)
		rudpAddr, _ = net.ResolveUDPAddr("udp6", serAddr)
	} else {
		udpAddr, _ = net.ResolveUDPAddr("udp4", addr)
		rudpAddr, _ = net.ResolveUDPAddr("udp4", serAddr)
	}
	conn, err := net.DialUDP("udp", udpAddr, rudpAddr)
	if err != nil {
		return conn, err
	}
	return conn, nil
}

/////////////////////// echo server (reflector) /////////////////////

// EchoServer BFD Echo 反射服务器
// RFC 5880: 接收 Echo 报文后在网络层直接将报文原样回送给发送方,
// 用于检测转发路径的实际时延和健康度
type EchoServer struct {
	Addr     string
	listener *net.UDPConn
	Family   int
}

func NewEchoServer(addr string, family int) *EchoServer {
	return &EchoServer{
		Addr:   addr,
		Family: family,
	}
}

func (s *EchoServer) Start() error {
	slogger.Debugf("Setting up echo reflector on %s:%d", s.Addr, ECHO_PORT)

	var udpAddr *net.UDPAddr
	var err error
	if s.Family == syscall.AF_INET6 {
		udpAddr, err = net.ResolveUDPAddr("udp6", s.Addr)
	} else {
		udpAddr, err = net.ResolveUDPAddr("udp4", s.Addr)
	}
	if err != nil {
		logger.Error("echo ResolveUDPAddr err: " + err.Error())
		return err
	}
	s.listener, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Error("echo ListenUDP err:" + err.Error())
		return err
	}
	defer s.listener.Close()

	slogger.Infof("BFD Echo reflector running at %s", udpAddr.String())

	s.reflectLoop()
	return nil
}

// reflectLoop 原样回送收到的 Echo 报文给发送方
func (s *EchoServer) reflectLoop() {
	for {
		data := make([]byte, 1024)
		n, raddr, err := s.listener.ReadFromUDP(data)
		if err != nil {
			logger.Error("echo read from udp error:" + err.Error())
			continue
		}
		// 将报文原样回送给发送方, 模拟网络层环回
		_, _ = s.listener.WriteToUDP(data[:n], raddr)
	}
}
