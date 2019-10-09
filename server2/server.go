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
	"golang.org/x/net/ipv4"
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

func parseHeader(buf []byte, n int, fromAddr *net.UDPAddr) {
	fmt.Println("--------------\n")
	fmt.Printf("[1 tun  receive from remote] receive %d bytes, from %s to %s, \n", n, util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
	header, _ := ipv4.ParseHeader(buf)
	fmt.Printf("[2header]: %+v \n", header)
	fmt.Printf("[3 tun payload header port]  %d \n", util.IPv4DestinationPort(buf))
	fmt.Printf("[4 from address]:%+v\n", fromAddr)
	fmt.Printf("[5 dst ip]: %+v", util.IPv4Destination(buf))
	fmt.Println("--------------\n")
}

func main() {
	// 1.开启虚拟网卡
	tun, err := tun.Open("tun0", tun.DevTun)
	checkError(err)
	setTunLinux()

	// 2.p2p打洞，发送握手信息
	// https://blog.csdn.net/rankun1/article/details/78027027
	p, err := p2p.GenerateP2P(PORT, TUN_IP, DST_PORT, DST_IP)
	defer p.Conn.Close()
	err = p.PingPong()

	// 3.tun接收和发送消息
	fmt.Println("[tun client] Waiting IP Packet from tun interface")
	var aesKey []byte
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
			if len(aesKey) == 0 {
				fmt.Println("[no aes key]")
				continue
			}

			edata := deepcrypt.EncryptAES(buf[:n], aesKey)
			data := util.GenMsg("aes", edata)

			n, err = p.Conn.WriteTo(data, p.DstAddr)
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
		n, fromAddr, err := p.Conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("udp Read error:", err)
			continue
		}
		// ecc decrypt
		if buf[0] == 0 && buf[1] == 0 {
			newAesKey, err := p.DecrptKey(buf[:n])
			if err != nil {
				fmt.Println("[DecrptKey error]:", err)
				continue
			}
			if newAesKey == nil && len(aesKey) == 0 {
				fmt.Println("[no aes key]")
				continue
			} else if len(newAesKey) != 0 {
				aesKey = newAesKey
			}
		}

		// 4.将conn的数据写入tun，并通过tun发送到物理网卡上
		// aes decrypt
		if len(aesKey) == 0 {
			fmt.Println("no aes key")
			continue
		}
		if buf[0] == 0 && buf[1] == 1 {
			fmt.Println("[aesKey]:", aesKey)
			ddata := deepcrypt.DecryptAES(buf[2:n], aesKey)
			// decrypt is right
			parseHeader(buf[2:n], n, fromAddr)
			n, err = tun.Write(ddata)
			if err != nil {
				fmt.Println("[tun client write to tun] udp write error:", err)
				continue
			}
			fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
		}
	}
}
