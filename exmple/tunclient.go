package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/jun/tuntap"
	"github.com/jun/tuntap/util"
)

const (
	CLIENT_IP = "10.0.0.30"
)

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
	exeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", CLIENT_IP))
	// make all traffic via tun0
	// ip -4 route add s所有的流量都走 10.0.0.30 网卡
	// 全部流量走网关
	// exeCmd(fmt.Sprintf("ip -4 route add 0.0.0.0/1 via %s dev tun0", CLIENT_IP))
	// https://github.com/jun/tuntap
	// exeCmd(fmt.Sprintf("ip -4 route add 192.168.3.1/24 via %s dev tun0", CLIENT_IP))
	// 查看路由有没有加上：route -n
	exeCmd(fmt.Sprintf("ip -4 route add 106.15.180.58/32 via %s dev tun0", CLIENT_IP))
}

func main() {
	// 必须写ip
	//lip := flag.String("l", "localhost", "client local ip")
	rip := flag.String("r", "139.219.6.50", "server remote ip")
	flag.Parse()

	tun, err := tuntap.Open("tun0", tuntap.DevTun)
	checkError(err)
	switch runtime.GOOS {
	case "linux":
		setTunLinux()
	default:
		fmt.Println("OS NOT supported")
		os.Exit(1)
	}

	//laddr, err := net.ResolveUDPAddr("udp", *lip+":0")
	checkError(err)
	raddr, err := net.ResolveUDPAddr("udp", *rip+":9826")
	checkError(err)
	fmt.Println("[laddr]", "local address", "[raddr]", raddr)
	// laddr is local address
	conn, err := net.DialUDP("udp", nil, raddr)
	checkError(err)
	defer conn.Close()

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
			n, err = conn.Write(buf[:n])
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
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("udp Read error:", err)
			continue
		}
		fmt.Sprintf("[tun client receive from dest] receive %d bytes, from %s to %s, \n", n, util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
		// 4.将conn的数据写入tun，并通过tun发送到物理网卡上
		n, err = tun.Write(buf[:n])
		if err != nil {
			fmt.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}
}
