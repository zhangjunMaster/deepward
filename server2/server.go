package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/spf13/viper"
	"github.com/zhangjunMaster/deepward/config"
	"github.com/zhangjunMaster/deepward/deepcrypt"
	"github.com/zhangjunMaster/deepward/p2p"
	"github.com/zhangjunMaster/deepward/tun"
	"github.com/zhangjunMaster/deepward/util"
)

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
	// 把符合条件的用公网ip出去
	exeCmd("iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -o eth0 -j MASQUERADE")
	exeCmd("echo '1' > /proc/sys/net/ipv4/ip_forward")
}

func init() {
	if err := config.Init("config.yaml"); err != nil {
		panic(err)
	}
	TUN_IP = viper.GetString("TUN.IP")
	ALLOW_IP = viper.GetString("TUN.ALLOW_IP")
	PORT = viper.GetInt("TUN.PORT")
	DST_IP = viper.GetString("PEER.IP")
	DST_PORT = viper.GetInt("PEER.PORT")
}

func main() {

	// 1.开启虚拟网卡
	tun, err := tun.Open("tun0", tun.DevTun)
	checkError(err)
	setTunLinux()

	// 2.p2p打洞，发送握手信息
	// https://blog.csdn.net/rankun1/article/details/78027027
	fmt.Println("[pingpong]", PORT, TUN_IP, DST_PORT, DST_IP)
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
			edata := deepcrypt.EncryptAES(buf[:n], []byte("1234567899876543"))
			n, err = conn.WriteTo(edata, dstAddr)
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
		ddata := deepcrypt.DecryptAES(buf[:n], []byte("1234567899876543"))
		fmt.Printf("[conn 收到数据  after]:%s\n", ddata)
		n, err = tun.Write(ddata)
		if err != nil {
			fmt.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}
}
