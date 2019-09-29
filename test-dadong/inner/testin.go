package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

var tag string

const HAND_SHAKE_MSG = "我是打洞消息"

func main() {
	// 当前进程标记字符串,便于显示
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 9826} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP("139.219.6.50"), Port: 9826}
	// 开始打洞
	fmt.Println("[srcAddr]:", srcAddr, "[dstAddr]:", dstAddr)
	bidirectionHole(srcAddr, dstAddr)
}

func parseAddr(addr string) net.UDPAddr {
	t := strings.Split(addr, ":")
	port, _ := strconv.Atoi(t[1])
	return net.UDPAddr{
		IP:   net.ParseIP(t[0]),
		Port: port,
	}
}

func bidirectionHole(srcAddr *net.UDPAddr, anotherAddr *net.UDPAddr) {
	conn, err := net.DialUDP("udp", srcAddr, anotherAddr)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(conn.LocalAddr().String())

	defer conn.Close()
	// 向另一个peer发送一条udp消息(对方peer的nat设备会丢弃该消息,非法来源),用意是在自身的nat设备打开一条可进入的通道,这样对方peer就可以发过来udp消息
	if _, err = conn.Write([]byte(HAND_SHAKE_MSG)); err != nil {
		log.Println("send handshake:", err)
	}
	go func() {
		for {
			time.Sleep(10 * time.Second)
			if _, err = conn.Write([]byte("from srcAddr")); err != nil {
				log.Println("send msg fail", err)
			}
			fmt.Println("[开始打洞]")
		}
	}()
	for {
		data := make([]byte, 1024)
		n, _, err := conn.ReadFromUDP(data)
		if err != nil {
			log.Printf("error during read: %s\n", err)
		} else {
			log.Printf("收到数据:%s\n", data[:n])
		}
	}
}
