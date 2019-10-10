package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/viper"
	"github.com/zhangjunMaster/deepward/config"
	"github.com/zhangjunMaster/deepward/deepcrypt"
	"github.com/zhangjunMaster/deepward/p2p"
	"github.com/zhangjunMaster/deepward/tun"
	"github.com/zhangjunMaster/deepward/util"
	"golang.org/x/net/ipv4"
)

var (
	TUN_IP     string
	ALLOW_IP   string
	DST_IP     string
	DST_PORT   int
	PORT       int
	ALLOW_PORT string
)

// 部署在2.159上作为client

func init() {
	if err := config.Init("config.yaml"); err != nil {
		panic(err)
	}
	TUN_IP = viper.GetString("TUN.IP")
	ALLOW_IP = viper.GetString("TUN.ALLOW_IP")
	PORT = viper.GetInt("TUN.PORT")
	DST_IP = viper.GetString("PEER.IP")
	DST_PORT = viper.GetInt("PEER.PORT")
	ALLOW_PORT = viper.GetString("TUN.ALLOW_PORT")
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
	util.CheckError(err)
	util.SetTunServerLinux(TUN_IP)

	// 设置环境
	util.SetTunServerLinux(TUN_IP)
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

			dstPort := strconv.Itoa(int(util.IPv4DestinationPort(ddata)))
			if !util.Filter(ALLOW_PORT, dstPort) {
				fmt.Printf("[forbidden]: ALLOW_PORT %s , port %s", ALLOW_PORT, dstPort)
				continue
			}

			n, err = tun.Write(ddata)
			if err != nil {
				fmt.Println("[tun client write to tun] udp write error:", err)
				continue
			}
			fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
		}
	}
}
