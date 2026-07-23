package main

import (
	"context"
	"fmt"
	"syscall"

	"github.com/WLmutou/gobfd"
	"github.com/osrg/gobgp/v3/api"
	"google.golang.org/grpc"
)

// BFD事件处理器，直接操作BGP Peer
type BGPIntegration struct {
	gobgpClient api.GobgpApiClient
}

func (b *BGPIntegration) OnBfdEvent(event gobfd.BfdEvent) {
	switch event.CurState {
	case gobfd.StateDown:
		fmt.Printf("[BFD] Peer %s DOWN, disabling in BGP\n", event.Remote)
		b.gobgpClient.DisablePeer(context.Background(), &api.DisablePeerRequest{
			Address: event.Remote,
		})
	case gobfd.StateUp:
		fmt.Printf("[BFD] Peer %s UP, enabling in BGP\n", event.Remote)
		b.gobgpClient.EnablePeer(context.Background(), &api.EnablePeerRequest{
			Address: event.Remote,
		})
	}
}

func main() {
	// 1. 连接 GoBGP 的 gRPC
	conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
	client := api.NewGobgpApiClient(conn)

	// 2. 启动 gobfd
	bfdControl := gobfd.NewControl("0.0.0.0", syscall.AF_INET)

	// 3. 订阅 BFD 事件，并绑定 BGP 动作
	handler := &BGPIntegration{gobgpClient: client}
	bfdControl.Subscribe(handler)

	// 4. 添加 BFD 会话
	bfdControl.AddSession("192.168.1.244", false, 400, 400, 3, nil)

	// 保持运行
	select {}
}
