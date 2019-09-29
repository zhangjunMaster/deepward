package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/viper"
	"github.com/zhangjunMaster/deepward"
	"github.com/zhangjunMaster/deepward/config"
	"github.com/zhangjunMaster/deepward/util"
)

//const (
//	TUN_IP   = "10.0.0.30"
//	VISIT_IP = "192.168.3.51"
//	INNER_IP = "124.193.68.147"
//)

var (
	TUN_IP      string
	VISIT_IP    string
	INNER_IP    string
	INNER_PORT  int
	LISTEN_PORT int
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
	exeCmd(fmt.Sprintf("ip -4 route add %s/32 via %s dev tun0", VISIT_IP, TUN_IP))
}

func init() {
	if err := config.Init(""); err != nil {
		panic(err)
	}
	TUN_IP = viper.GetString("TUN.IP")
	VISIT_IP = viper.GetString("TUN.VISIT_IP")
	INNER_IP = viper.GetString("TUN.INNER_IP")
	INNER_PORT = viper.GetInt("TUN.INNER_PORT")
	LISTEN_PORT = viper.GetInt("TUN.LISTEN_PORT")
}

func main() {

	// 1.开启虚拟网卡
	tun, err := deepward.Open("tun0", deepward.DevTun)
	checkError(err)
	setTunLinux()

	// 2.打洞，发送握手信息
	// https://blog.csdn.net/rankun1/article/details/78027027
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: LISTEN_PORT} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(INNER_IP), Port: INNER_PORT}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		fmt.Println("[Listen UDP err]:", err)
	}
	fmt.Println("[dstAddr]:", dstAddr)
	defer conn.Close()

	var n int
	if n, err = conn.WriteTo([]byte("我是打洞消息"), dstAddr); err != nil {
		log.Println("send handshake:", err)
	}
	fmt.Println("[conn write 我是打洞消息]", n)

	go func() {
		for {
			time.Sleep(10 * time.Second)
			if _, err = conn.WriteTo([]byte("from ["+TUN_IP+"]"), dstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			fmt.Println("[打洞] dst:", INNER_IP)
		}
	}()

	fmt.Println("[tun client] Waiting IP Packet from tun interface")
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
