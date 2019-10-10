package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/songgao/water"
	"github.com/zhangjunMaster/deepward/util"
)

var (
	TUN_IP      = "10.1.0.10"
	VISIT_IP    = "192.168.2.97"
	INNER_IP    = "192.168.2.154"
	INNER_PORT  = 50879
	LISTEN_PORT = 9826
)

func exeCmd(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Printf("execute %s error:%v", cmd, err)
		os.Exit(1)
	}
	log.Println(string(out))
}

func setTunDarwin() {
	exeCmd(fmt.Sprintf("ifconfig utun1 inet %s/24 %s up", TUN_IP, TUN_IP))
	exeCmd(fmt.Sprintf("route -n add 0.0.0.0/1 %s", TUN_IP))
	exeCmd(fmt.Sprintf("route -n add 128.0.0.0/1 %s", TUN_IP))
}

//sudo ifconfig utun1 10.1.0.10 10.1.0.20 up
//sudo route -n add -net 192.168.2.97/32 10.1.0.10

//route delete 192.168.2.97/32 10.1.0.10 删除
//route -n add -net 192.168.3.51 10.1.0.10  访问192.168.3.51走10.1.0.10
//route -n add -net en0 -netmask 255.255.255.0 10.1.0.10
// netstat -rn 查看路由表

func main() {
	tun, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Interface Name: %s\n", tun.Name())
	// 设置脚本
	setTunDarwin()
	// 2.打洞，发送握手信息
	// https://blog.csdn.net/rankun1/article/details/78027027
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: LISTEN_PORT} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(INNER_IP), Port: INNER_PORT}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		log.Println("[Listen UDP err]:", err)
	}
	log.Println("[dstAddr]:", dstAddr)
	defer conn.Close()

	var n int
	if n, err = conn.WriteTo([]byte("我是打洞消息"), dstAddr); err != nil {
		log.Println("send handshake:", err)
	}
	log.Println("[conn write 我是打洞消息]", n)

	go func() {
		for {
			time.Sleep(10 * time.Second)
			if _, err = conn.WriteTo([]byte("from ["+TUN_IP+"]"), dstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			log.Println("[打洞] dst:", INNER_IP)
		}
	}()

	log.Println("[tun client] Waiting IP buf from tun interface")

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
			// 加密
			//payload := util.IPv4Payload(buf)

			// 2.将接收的数据通过conn发送出去
			n, err = conn.WriteTo(buf[:n], dstAddr)
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
		log.Printf("[conn 收到数据]:%s\n", buf[:n])
		log.Printf("[tun client receive from conn] receive %d bytes from %s\n", n, fromAddr.String())
		// 4.将conn的数据写入tun，并通过tun发送到物理网卡上
		n, err = tun.Write(buf[:n])
		if err != nil {
			log.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		log.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}

}
