package gobfd

import (
	"fmt"
	"github.com/google/gopacket/layers"
	"math/rand"
	"net"
	"syscall"
	"time"
)

const (
	CONTROL_PORT = 3784
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
	conn, err = net.DialUDP("udp",  udpAddr, rudpAddr)
	if err != nil {
		return  conn, err
	}
	return conn, nil
}


/////////////////////// server /////////////////////

type Server struct {
	Addr     string
	listener *net.UDPConn // udp conn

	Family    int
	RxQueue   chan *RxData
}

func NewServer(addr string, family int, rx chan *RxData) *Server {
	return &Server{
		Addr:      addr,
		Family: family,
		RxQueue:   rx,
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

func (s *Server) Loop()  {
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
		rxData := &RxData{Data:bfdPk, Addr: udpConn.String()}
		s.RxQueue <- rxData
	}
}



