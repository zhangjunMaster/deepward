package p2p

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/spf13/viper"
	"github.com/zhangjunMaster/deepward/deepcrypt"
)

type P2P struct {
	TunIP   string
	DstAddr *net.UDPAddr
	Conn    *net.UDPConn
	TUNPRK  string
	PEERPUK string
}

func GenerateP2P(listenPort int, tunIP string, dstPort int, dstIP string) (*P2P, error) {
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: listenPort} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(dstIP), Port: dstPort}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		fmt.Println("[Listen UDP err]:", err)
		return nil, err
	}
	fmt.Println("[dstAddr]:", dstAddr)
	return &P2P{
		TunIP:   tunIP,
		DstAddr: dstAddr,
		Conn:    conn,
		TUNPRK:  viper.GetString("TUN.PRK"),
		PEERPUK: viper.GetString("PEER.PUK"),
	}, nil
}

func (p *P2P) PingPong() error {
	// conn
	n, err := p.Conn.WriteTo([]byte("ping"), p.DstAddr)
	if err != nil {
		log.Println("send handshake:", err)
	}
	fmt.Println("[conn write ping]", n)
	if err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			//tunipdecrypt := deepcrypt.EncryptAES([]byte(p.TunIP), []byte("1234567899876543"))
			if _, err = p.Conn.WriteTo([]byte(p.TunIP), p.DstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			fmt.Println("[ping] dst:", p.DstAddr)
		}
	}()
	return err
}

func (p *P2P) ExchangeAesKey() (string, error) {
	aesKey := deepcrypt.Generate128Key(16)
	peerPuk := p.PEERPUK
	// use puk encrypt aeskey
	fmt.Println("[ExchangeAesKey]", "[aesKey]", aesKey, "[peerPuk]:", peerPuk)
	eKey, err := deepcrypt.Encrypt([]byte(aesKey), peerPuk)
	if err != nil {
		return "", err
	}
	// lable,exchange key {0 0}
	lable := []byte{0, 0}
	// concat
	var buffer bytes.Buffer
	buffer.Write(lable)
	buffer.Write(eKey)
	data := buffer.Bytes()
	fmt.Println("[ExchangeAesKey data]:", data)
	// exchage aeskey

	n, err := p.Conn.WriteTo(data, p.DstAddr)
	if err != nil {
		log.Println("send handshake:", err)
	}
	fmt.Println("[conn write ping]", n)
	if err != nil {
		return "", err
	}
	return aesKey, nil
}

func (p *P2P) DecrptKey(eKey []byte) ([]byte, error) {
	if eKey[0] != 0 || eKey[1] != 0 {
		return nil, nil
	}
	fmt.Println("[eKey[2:]]:", eKey[2:])
	dKey, err := deepcrypt.Decrypt(eKey[2:], p.TUNPRK)
	if err != nil {
		return nil, err
	}
	return dKey, err
}

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
