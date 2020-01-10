
BFD协议的golang实现

### 开源出来，或许发现写的不够好的地方可以修改，实现的更好.


```
///////////////////////////// bfd帧结构  ///////////////////////////
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
```



使用例子
```go 

// 定义回调函数
/*
 * 目标ip, 上一个状态, 当前状态
 */
func callBackBFDState(ipAddr string, preState, curState int) error {
	fmt.Println("ipAddr:", ipAddr, ",preState:", preState, ",curState:", curState)
	return nil
}


func test(){
    family := syscall.AF_INET // 默认ipv4
    local := "0.0.0.0"
    
    passive := false          // 是否是被动模式
    rxInterval := 400  		  // 接收速率400 毫秒
    txInterval := 400  		  // 发送速率400 毫秒
    detectMult := 1           // 报文最大失效的个数
    
    // 启动
    control := NewControl(local, family)
    // 添加监测
    remote1 := "192.168.1.244"  // 远程ip
    control.AddSession(remote1, passive, rxInterval, txInterval, detectMult, callBackBFDState)
    
    // 添加监测2
    remote2 := "192.168.1.185"  // 远程ip2
    control.AddSession(remote2, passive, rxInterval, txInterval, detectMult, callBackBFDState)
    
    
    fmt.Println("sleep 30 second...")
    time.Sleep(time.Second * 30)
    
  
    // 删除监测
    fmt.Println("del session ...")
    control.DelSession(remote1)
    control.DelSession(remote2)
}
``````
