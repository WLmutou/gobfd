package gobfd

import (
	"fmt"
	"github.com/google/gopacket/layers"
	"testing"
)

func TestEncodePacket(t *testing.T) {
	txByte := EncodePacket(VERSION,
		layers.BFDDiagnosticNone,
		layers.BFDStateDown,
		false,
		false,
		false,
		false,
		false,
		false,
		5,
		1,
		0,
		1000000,
		1000000,
		0,
		auth)
	lbfd, err := DecodePacket(txByte)
	if err != nil {
		// if decode the package is error
		t.Errorf("encode and decode error!")
	}
	fmt.Println("bfd version:", lbfd.Version)
	if lbfd.Version != VERSION {
		t.Errorf("decode version error!")
	}
}

func TestEncodeDecodeEchoPacket(t *testing.T) {
	discr := uint32(0x12345678)
	ts := int64(1609459200000000000) // 2021-01-01 00:00:00 UTC

	pkt := EncodeEchoPacket(discr, ts)
	if len(pkt) != EchoPacketLen {
		t.Fatalf("echo packet len = %d, want %d", len(pkt), EchoPacketLen)
	}

	gotDiscr, gotTs, err := DecodeEchoPacket(pkt)
	if err != nil {
		t.Fatalf("decode echo packet error: %v", err)
	}
	if gotDiscr != discr {
		t.Errorf("discriminator = 0x%x, want 0x%x", gotDiscr, discr)
	}
	if gotTs != ts {
		t.Errorf("timestamp = %d, want %d", gotTs, ts)
	}
}

func TestDecodeEchoPacketTooShort(t *testing.T) {
	_, _, err := DecodeEchoPacket([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Error("expected error for short echo packet, got nil")
	}
}
