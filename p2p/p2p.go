package p2p

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/zhangjunMaster/deepward/deepcrypt"
)

func PingPong(listenPort int, tunIP string, dstPort int, dstIP string) (*net.UDPConn, error) {
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: listenPort} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(dstIP), Port: dstPort}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		fmt.Println("[Listen UDP err]:", err)
		return nil, err
	}
	fmt.Println("[dstAddr]:", dstAddr)
	//defer conn.Close()

	var n int
	pingpong := deepcrypt.EncryptAES([]byte("ping"), []byte("1234567899876543"))
	if n, err = conn.WriteTo(pingpong, dstAddr); err != nil {
		log.Println("send handshake:", err)
	}
	fmt.Println("[conn write ping]", n)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			tunipdecrypt := deepcrypt.EncryptAES([]byte(tunIP), []byte("1234567899876543"))
			if _, err = conn.WriteTo(tunipdecrypt, dstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			fmt.Println("[ping] dst:", dstIP)
		}
	}()
	return conn, nil
}
