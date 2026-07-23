// gobfd 命令行使用示例
//
// 启动一个 BFD 控制实例, 对指定 remote ip 进行链路检测, 并在状态变化时打印日志。
//
// 用法:
//
//	go run ./cmd/gobfd -local 0.0.0.0 -remote 192.168.1.244 -rx 400 -tx 400 -mult 1
//	go run ./cmd/gobfd -remote 192.168.1.244 -remote 192.168.1.185
//	go run ./cmd/gobfd -remote 192.168.1.244 -echo 100   # 启用 Echo 模式
//	go run ./cmd/gobfd -remote 192.168.1.244 -demand     # 启用 Demand 模式
//
// 参数:
//
//	-local    本地绑定ip, 默认 0.0.0.0
//	-family   协议家族, 4=ipv4(默认), 6=ipv6
//	-remote   对端ip, 可多次指定以同时检测多个目标
//	-passive  是否被动模式, 默认 false
//	-rx       接收间隔(毫秒), 默认 400
//	-tx       发送间隔(毫秒), 默认 400
//	-mult     报文最大失效个数, 默认 1
//	-echo     Echo 报文发送间隔(毫秒), >0 启用 RFC 5880 Echo 模式, 默认 0(关闭)
//	-demand   是否启用 RFC 5880 Demand 模式(低开销), 默认 false
//	-duration 运行时长(秒), <=0 表示一直运行, 默认 0
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WLmutou/gobfd"
)

// callBackBFDState BFD 会话状态变化回调
// ipAddr:  目标对端ip
// preState: 变化前状态
// curState: 变化后状态
func callBackBFDState(ipAddr string, preState, curState int) error {
	fmt.Printf("[BFD] remote=%s state: %s -> %s\n", ipAddr, stateName(preState), stateName(curState))
	return nil
}

func stateName(s int) string {
	switch s {
	case gobfd.StateAdminDown:
		return "AdminDown"
	case gobfd.StateDown:
		return "Down"
	case gobfd.StateInit:
		return "Init"
	case gobfd.StateUp:
		return "Up"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// stringSliceFlag 支持 -remote 多次指定
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	local := flag.String("local", "0.0.0.0", "本地绑定ip")
	family := flag.Int("family", 4, "协议家族: 4=ipv4, 6=ipv6")
	passive := flag.Bool("passive", false, "是否被动模式")
	rx := flag.Int("rx", 400, "接收间隔(毫秒)")
	tx := flag.Int("tx", 400, "发送间隔(毫秒)")
	mult := flag.Int("mult", 1, "报文最大失效个数")
	echo := flag.Int("echo", 0, "Echo 报文发送间隔(毫秒), >0 启用 RFC 5880 Echo 模式")
	demand := flag.Bool("demand", false, "是否启用 RFC 5880 Demand 模式(低开销)")
	duration := flag.Int("duration", 0, "运行时长(秒), <=0 表示一直运行")

	var remotes stringSliceFlag
	flag.Var(&remotes, "remote", "对端ip, 可多次指定")
	flag.Parse()

	if len(remotes) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one -remote is required")
		flag.Usage()
		os.Exit(1)
	}

	var fam int
	switch *family {
	case 4:
		fam = syscall.AF_INET
	case 6:
		fam = syscall.AF_INET6
	default:
		fmt.Fprintf(os.Stderr, "error: invalid family %d (use 4 or 6)\n", *family)
		os.Exit(1)
	}

	// 创建并启动 BFD 控制实例 (内部已启动后台收发 goroutine)
	control := gobfd.NewControl(*local, fam)

	// 添加监测会话
	for _, remote := range remotes {
		if *echo > 0 {
			control.AddEchoSession(remote, *passive, *rx, *tx, *mult, *echo, callBackBFDState)
			fmt.Printf("[BFD] added echo session: local=%s -> remote=%s (rx=%dms tx=%dms mult=%d echo=%dms passive=%v)\n",
				*local, remote, *rx, *tx, *mult, *echo, *passive)
		} else if *demand {
			control.AddDemandSession(remote, *passive, *rx, *tx, *mult, callBackBFDState)
			fmt.Printf("[BFD] added demand session: local=%s -> remote=%s (rx=%dms tx=%dms mult=%d passive=%v)\n",
				*local, remote, *rx, *tx, *mult, *passive)
		} else {
			control.AddSession(remote, *passive, *rx, *tx, *mult, callBackBFDState)
			fmt.Printf("[BFD] added session: local=%s -> remote=%s (rx=%dms tx=%dms mult=%d passive=%v)\n",
				*local, remote, *rx, *tx, *mult, *passive)
		}
	}

	fmt.Printf("[BFD] local server listening on %s:%d (control) / %d (echo), waiting for state changes...\n",
		*local, gobfd.ControlPort, gobfd.EchoPort)

	// 等待退出信号或定时结束
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if *duration > 0 {
		select {
		case <-sigCh:
		case <-time.After(time.Duration(*duration) * time.Second):
		}
	} else {
		<-sigCh
	}

	// 清理: 删除所有监测会话
	fmt.Println("[BFD] shutting down, deleting sessions...")
	for _, remote := range remotes {
		_ = control.DelSession(remote)
	}
	fmt.Println("[BFD] exited.")
}
