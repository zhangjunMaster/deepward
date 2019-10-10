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
	"github.com/zhangjunMaster/deepward/deepcrypt"
	"github.com/zhangjunMaster/deepward/util"
	"golang.org/x/net/ipv4"
)

// 部署在内网 2.159，为 6.50 打洞
//const (
//	TUN_IP    = "10.0.0.2"
//	OUTTER_IP = "139.219.6.50"
//)

var (
	TUN_IP      string
	LISTEN_PORT int
	OUTTER_IP   string
	OUTTER_PORT int
)

func checkError(err error) {
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}

func exeCmd(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Printf("execute %s error:%v", cmd, err)
		os.Exit(1)
	}
	log.Println(string(out))
}

func setTunServerLinux() {
	exeCmd("ip link set dev tun0 up")
	exeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", TUN_IP))
	// 把符合条件的用公网ip出去
	exeCmd("iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -o eth0 -j MASQUERADE")
	exeCmd("echo '1' > /proc/sys/net/ipv4/ip_forward")
}

func checkErr(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func init() {
	if err := config.Init("config.yaml"); err != nil {
		panic(err)
	}
	TUN_IP = viper.GetString("TUN.IP")
	LISTEN_PORT = viper.GetInt("TUN.LISTEN_PORT")
	OUTTER_IP = viper.GetString("TUN.OUTTER_IP")
	OUTTER_PORT = viper.GetInt("TUN.OUTTER_PORT")

}

func main() {
	// 1.开启虚拟网卡
	tun, err := deepward.Open("tun0", deepward.DevTun)
	checkError(err)
	setTunServerLinux()

	// 2.打洞，发送握手信息
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: LISTEN_PORT} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(OUTTER_IP), Port: OUTTER_PORT}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		log.Println("[Listen UDP err]:", err)
	}
	log.Println("[listen]:", srcAddr)
	defer conn.Close()

	var n int
	pingpong := deepcrypt.EncryptAES([]byte("pingpong"), []byte("1234567899876543"))
	if n, err = conn.WriteTo(pingpong, dstAddr); err != nil {
		log.Println("send handshake:", err)
	}
	log.Println("[conn write 我是打洞消息]", n)

	go func() {
		for {
			time.Sleep(10 * time.Second)
			tunipdata := deepcrypt.EncryptAES([]byte("from ["+TUN_IP+"]"), []byte("1234567899876543"))
			if n, err = conn.WriteTo(tunipdata, dstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			log.Println("[打洞] dst:", OUTTER_IP, "[打洞数据]:", n)
		}
	}()

	log.Println("[tun server] Waiting IP Packet from UDP")

	// 1.将conn的数据，先给虚拟网卡，虚拟网卡再转给物理网卡
	go func() {
		buf := make([]byte, 10000)
		for {
			// 1.tun接收来自物理网卡的数据
			n, err := tun.Read(buf)
			if err != nil {
				log.Println("tun Read error:", err)
				continue
			}
			log.Printf("[tun client receive from local] receive %d bytes, from %s to %s, \n", n, util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
			// 2.将接收的数据通过conn发送出去, util.IPv4Destination(buf).String()  => tun ip => endpoint
			// n, err = conn.WriteTo(buf[:n], dstAddr) 原始写法
			edata := deepcrypt.EncryptAES(buf[:n], []byte("1234567899876543"))
			n, err = conn.WriteTo(edata, dstAddr)

			if err != nil {
				log.Println("udp write error:", err)
				continue
			}
			log.Printf("[tun client conn send to dest] write %d bytes to udp network\n", n)
		}
	}()

	buf := make([]byte, 10000)
	for {
		// 3.conn连接中读取 buf
		n, fromAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("udp Read error:", err)
			continue
		}
		log.Printf("[conn 收到数据  before]:%s\n", buf[:n])
		//  fromAddr.String() => endpoint ip
		log.Printf("[tun client receive from conn] receive %d bytes from %s\n", n, fromAddr.String())
		log.Println("[3 fromAddr]:", fromAddr)
		log.Printf("[3 tun  receive from remote] receive %d bytes, from %s to %s, \n", n, util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
		// only ip(exclude port)
		header, _ := ipv4.ParseHeader(buf)
		log.Printf("[4header]: %+v \n", header)

		port := util.IPv4DestinationPort(buf)
		log.Printf("----[5 tun payload header port]  %d \n", port)

		//payload, err := ipv4.ReadFrom(buf)
		//if err != nil {
		//	log.Println("[5 payload] err", err)
		//}
		//log.Println("[5 payload] %+v", payload)
		// 4.将conn的数据写入tun，并通过tun发送到物理网卡上

		// n, err = tun.Write(buf[:n]) 原始写法
		ddata := deepcrypt.DecryptAES(buf[:n], []byte("1234567899876543"))
		log.Printf("[conn 收到数据  after]:%s\n", ddata)
		n, err = tun.Write(ddata)
		if err != nil {
			log.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		log.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}
}
