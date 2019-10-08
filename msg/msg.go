package msg

import (
	"fmt"
	"net"

	"github.com/zhangjunMaster/deepward/tun"
	"github.com/zhangjunMaster/deepward/util"
)

func Send(tun *tun.Interface, conn *net.UDPConn, dstAddr *net.UDPAddr) {
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
}

func Receive(tun *tun.Interface, conn *net.UDPConn, dstAddr *net.UDPAddr) {
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
