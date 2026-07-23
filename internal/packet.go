package gobfd

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

/*

///////////////////////////////// 帧结构  ///////////////////////////

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
*/

var (
	//
	PACKET_DEBUG_MSG = "\n|--------------------------------------------------\n" +
		"| Vers: %d Diag: %d State: %d Poll: %d Final: %d\n" +
		"| CPI: %d Auth: %d Demand: %d Multi: %d DetectMult: %d\n" +
		"| Length: %d MyDisc: %d YourDisc: %d\n" +
		"| TxInterval: %d RxInterval: %d EchoRxInterval: %d\n" +
		"|--------------------------------------------------"

	// 认证auth
	auth = &layers.BFDAuthHeader{
		AuthType:       layers.BFDAuthTypeKeyedMD5,
		KeyID:          2,
		SequenceNumber: 5,
		Data: []byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
		},
	}
)

// //////////////////////////////////// Decode解码 ///////////////////////////////////////////////////////////
func DecodePacket(packetBytes []byte) (*layers.BFD, error) {
	var pbfd *layers.BFD
	//var err error
	p := gopacket.NewPacket(packetBytes, layers.LayerTypeBFD, gopacket.Default)
	if p.ErrorLayer() != nil {
		// 解包失败
		slogger.Errorf("Failed to decode packet  %v", p.ErrorLayer().Error())
		return pbfd, errors.New("decode packet error")
	}

	// 确保包包含这层:
	//    Application Layer = BFD.
	err := checkLayers(p, []gopacket.LayerType{
		layers.LayerTypeBFD,
	})
	if err != nil {
		return pbfd, err
	}

	pbfd, ok := p.ApplicationLayer().(*layers.BFD)
	if !ok {
		// 没有BFD协议层
		logger.Error("No BFD layer type found in packet")
		return pbfd, errors.New("No BFD layer type found in packet")
	}
	if err := validate(pbfd); err != nil {
		return pbfd, err
	}

	// 解码成功
	slogger.Debugf(PACKET_DEBUG_MSG, pbfd.Diagnostic, pbfd.State, pbfd.Poll, pbfd.Final,
		pbfd.ControlPlaneIndependent, pbfd.AuthPresent,
		pbfd.Demand, pbfd.Multipoint, pbfd.DetectMultiplier, pbfd.Length(),
		pbfd.MyDiscriminator, pbfd.YourDiscriminator, pbfd.DesiredMinTxInterval,
		pbfd.RequiredMinRxInterval, pbfd.RequiredMinEchoRxInterval)

	return pbfd, nil
}

func checkLayers(p gopacket.Packet, want []gopacket.LayerType) error {
	layers := p.Layers()
	if len(layers) < len(want) {
		slogger.Errorf(" Number of layers mismatch: got %d want %d", len(layers), len(want))
		return errors.New("len layers < len want")
	}
	for i, l := range want {
		if l == gopacket.LayerTypePayload {
			// done matching layers
			continue
		}
		if layers[i].LayerType() != l {
			msg := fmt.Sprintf("  Layer %d mismatch: got %v want %v", i,
				layers[i].LayerType(), l)
			logger.Error(msg)
			return errors.New(msg)
		}
	}
	return nil
}

func validate(pbfd *layers.BFD) error {
	if pbfd.Version != 1 {
		logger.Error("Unsupported BFD protocol version")
		return errors.New("Unsupported BFD protocol version")
	}
	// 其他校验,待添加Auth等认证
	if pbfd.AuthPresent {
		if pbfd.AuthHeader == nil {
			logger.Error("auth header error")
			return errors.New("auth header error")
		}
		if pbfd.AuthHeader.AuthType != auth.AuthType {
			logger.Error("auth type error!")
			return errors.New("auth type error")
		}
		if pbfd.AuthHeader.KeyID != auth.KeyID {
			logger.Error("auth key id error")
			return errors.New("auth key id error")
		}
		if string(pbfd.AuthHeader.Data) != string(auth.Data) {
			logger.Error("auth header data error")
			return errors.New("auth header data error")
		}
	}

	return nil
}

// ///////////////////////////////////// Encode编码 ////////////////////////////////////////////////////
func EncodePacket(Version layers.BFDVersion,
	Diagnostic layers.BFDDiagnostic,
	State layers.BFDState,
	Poll bool,
	Final bool,
	ControlPlaneIndependent bool,
	AuthPresent bool,
	Demand bool,
	Multipoint bool,
	DetectMultiplier layers.BFDDetectMultiplier,
	MyDiscriminator layers.BFDDiscriminator,
	YourDiscriminator layers.BFDDiscriminator,
	DesiredMinTxInterval layers.BFDTimeInterval,
	RequiredMinRxInterval layers.BFDTimeInterval,
	RequiredMinEchoRxInterval layers.BFDTimeInterval,
	AuthHeader *layers.BFDAuthHeader) []byte {

	pExpectedBFD := &layers.BFD{
		BaseLayer: layers.BaseLayer{
			Contents: []byte{
				0x20, 0x40, 0x05, 0x18, 0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x0f, 0x42, 0x40,
				0x00, 0x0f, 0x42, 0x40, 0x00, 0x00, 0x00, 0x00,
			},
			Payload: nil,
		},
		Version:                   Version,
		Diagnostic:                Diagnostic,
		State:                     State,
		Poll:                      Poll,
		Final:                     Final,
		ControlPlaneIndependent:   ControlPlaneIndependent,
		AuthPresent:               AuthPresent,
		Demand:                    Demand,
		Multipoint:                Multipoint,
		DetectMultiplier:          DetectMultiplier,
		MyDiscriminator:           MyDiscriminator,
		YourDiscriminator:         YourDiscriminator,
		DesiredMinTxInterval:      DesiredMinTxInterval,
		RequiredMinRxInterval:     RequiredMinRxInterval,
		RequiredMinEchoRxInterval: RequiredMinEchoRxInterval,
		AuthHeader:                AuthHeader,
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}
	err := pExpectedBFD.SerializeTo(buf, opts)
	if err != nil {
		slogger.Errorf("serial error: %s" + err.Error())
		return []byte{}
	}

	//fmt.Println(buf.Bytes())
	return buf.Bytes()
}

/////////////////////////////////////// Echo 报文编解码 ////////////////////////////////////////////////////

// Echo 报文格式 (RFC 5880 Echo 模式, 载荷由实现自定义):
//
//	0                   1                   2                   3
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                    My Discriminator                          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                    Timestamp (高 32 位, 纳秒)                 |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                    Timestamp (低 32 位, 纳秒)                 |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// MyDiscriminator: 发送方的本地标识符, 用于匹配回送报文与对应的会话
// Timestamp:       发送时刻(纳秒), 用于计算往返时延 RTT

const EchoPacketLen = 12

// EncodeEchoPacket 编码 Echo 报文
func EncodeEchoPacket(discriminator uint32, timestampNs int64) []byte {
	buf := make([]byte, EchoPacketLen)
	binary.BigEndian.PutUint32(buf[0:4], discriminator)
	binary.BigEndian.PutUint64(buf[4:12], uint64(timestampNs))
	return buf
}

// DecodeEchoPacket 解码 Echo 报文
func DecodeEchoPacket(data []byte) (discriminator uint32, timestampNs int64, err error) {
	if len(data) < EchoPacketLen {
		return 0, 0, errors.New("echo packet too short")
	}
	discriminator = binary.BigEndian.Uint32(data[0:4])
	timestampNs = int64(binary.BigEndian.Uint64(data[4:12]))
	return discriminator, timestampNs, nil
}
