package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/spf13/viper"
	"github.com/zhangjunMaster/deepward/config"
	"github.com/zhangjunMaster/deepward/p2p"
	"github.com/zhangjunMaster/deepward/tun"
	"github.com/zhangjunMaster/deepward/util"
)

//const (
//	TUN_IP   = "10.0.0.30"
//	ALLOW_IP = "192.168.3.51"
//	DST_IP = "124.193.68.147"
//)

var (
	TUN_IP   string
	ALLOW_IP string
	DST_IP   string
	DST_PORT int
	PORT     int
)

// 部署在2.159上作为client
func checkError(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func exeCmd(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		fmt.Printf("execute %s error:%v", cmd, err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func setTunLinux() {
	exeCmd("ip link set dev tun0 up")
	exeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", TUN_IP))
	exeCmd(fmt.Sprintf("ip -4 route add %s/32 via %s dev tun0", ALLOW_IP, TUN_IP))
}

func init() {
	if err := config.Init("config.yaml"); err != nil {
		panic(err)
	}
	TUN_IP = viper.GetString("TUN.IP")
	ALLOW_IP = viper.GetString("TUN.ALLOW_IP")
	DST_IP = viper.GetString("PEER.IP")
	DST_PORT = viper.GetInt("PEER.PORT")
	PORT = viper.GetInt("TUN.PORT")
}

func main() {

	// 1.开启虚拟网卡
	tun, err := tun.Open("tun0", tun.DevTun)
	checkError(err)
	setTunLinux()

	// 2.p2p打洞，发送握手信息
	// https://blog.csdn.net/rankun1/article/details/78027027
	conn, err := p2p.PingPong(PORT, TUN_IP, DST_PORT, DST_IP)
	defer conn.Close()

	// 3.tun接收和发送消息
	fmt.Println("[tun client] Waiting IP Packet from tun interface")
	dstAddr := &net.UDPAddr{IP: net.ParseIP(DST_IP), Port: DST_PORT}

	go func() {
		buf := make([]byte, 10000)
		for {
			// 1.tun接收来自物理网卡的数据
			n, err := tun.Read(buf)
			if err != nil {
				fmt.Println("tun Read error:", err)
				continue
			}
			fmt.Printf("[tun client receive from local] receive %d bytes, from %s to %s, \n", n, util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
			// 加密
			//payload := util.IPv4Payload(buf)

			// 2.将接收的数据通过conn发送出去
			n, err = conn.WriteTo(buf[:n], dstAddr)
			if err != nil {
				fmt.Println("udp write error:", err)
				continue
			}
			fmt.Printf("[tun client conn send to dest] write %d bytes to udp network\n", n)
		}
	}()

	buf := make([]byte, 10000)
	for {
		// 3.conn连接中读取 buf
		n, fromAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("udp Read error:", err)
			continue
		}
		fmt.Printf("[conn 收到数据]:%s\n", buf[:n])
		fmt.Printf("[tun client receive from conn] receive %d bytes from %s\n", n, fromAddr.String())
		// 4.将conn的数据写入tun，并通过tun发送到物理网卡上
		n, err = tun.Write(buf[:n])
		if err != nil {
			fmt.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}
}
