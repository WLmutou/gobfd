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
