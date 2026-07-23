package gobfd

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func CallBackBFDState(ip string, pre, cur int) error {
	fmt.Println(ip, pre, cur)
	return nil
}

func TestNewControl(t *testing.T) {
	family := syscall.AF_UNSPEC

	var local, remote string
	if len(os.Args) > 2 {
		args := os.Args[1:]
		local = args[0]
		remote = args[1]
	} else {
		local = "192.168.43.103"
		remote = "192.168.43.244"
	}
	passive := false
	rxInterval := 400 // 毫秒
	txInterval := 400 // 毫秒
	detectMult := 1

	control := NewControl(local, family)
	control.Run()

	fmt.Println("add bfd check session  remote ip: ", remote)
	control.AddSession(remote, passive, rxInterval, txInterval, detectMult, CallBackBFDState)

	fmt.Println("local bfd server running at port: ", CONTROL_PORT)
	time.Sleep(time.Second * 30)
}
