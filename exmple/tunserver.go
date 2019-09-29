package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/zhangjunMaster/deepward"
)

const (
	SERVER_IP = "10.0.0.2"
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
	// 1.启动 tun0网卡
	exeCmd("ip link set dev tun0 up")
	// 2.在tun0 虚拟网卡上添加ip
	exeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", SERVER_IP))
	// 3.表示来自 10.0.0.0/24 的数据包的来源改成 eth0 的 ip 发出去
	// 使用ifconfig配置ens32/eth0
	// 查看有没有 nat: iptables -t nat -L
	exeCmd("iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -o eth0 -j MASQUERADE")
	// 4.服务器开启路由转发
	// 查看是否开启路由转发：cat /proc/sys/net/ipv4/ip_forward
	exeCmd("echo '1' > /proc/sys/net/ipv4/ip_forward")
}

func main() {
	tun, err := deepward.Open("tun0", deepward.DevTun)
	checkError(err)
	switch runtime.GOOS {
	case "linux":
		setTunLinux()
	default:
		fmt.Println("OS NOT supported")
		os.Exit(1)
	}
	// 1.监听 udp端口 9826
	addr, err := net.ResolveUDPAddr("udp", ":9826")
	checkError(err)
	conn, err := net.ListenUDP("udp", addr)
	checkError(err)
	defer conn.Close()
	raddr := &net.UDPAddr{}
	fmt.Println("[tun server] Waiting IP Packet from UDP")
	go func() {
		buf := make([]byte, 10000)
		for {
			// 2.从连接中 conn中读取数据
			n, fromAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				fmt.Println("ReadFromUDP error:", err)
				continue
			}
			raddr = fromAddr
			fmt.Printf("[tun server receive from client 1] receive %d bytes from %s\n", n, fromAddr.String())
			// 3.写到tun网卡中
			n, _ = tun.Write(buf[:n])
			fmt.Printf("[tun server write to tun 2] write %d bytes to tun interface\n", n)
		}
	}()

	buf := make([]byte, 10000)
	for {
		// 1.tun网卡转发后，会有数据到 tun虚拟网卡中，再从tun网卡中读数据
		n, err := tun.Read(buf)
		if err != nil {
			fmt.Println("[tun server read from tun 3] run read error:", err)
			continue
		}
		// 2.将数据conn写到 raddr中
		n, err = conn.WriteTo(buf[:n], raddr)
		// n, err = conn.Write(buf[:n])
		fmt.Printf("[tun server send to client 4] write %d bytes to udp network\n", n)
	}
}
