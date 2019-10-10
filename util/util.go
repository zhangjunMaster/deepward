package util

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
)

var lable2byte = map[string][]byte{
	"ecc": []byte{0, 0},
	"aes": []byte{0, 1},
}

func Concat(src1 []byte, src2 []byte) []byte {
	var buffer bytes.Buffer
	buffer.Write(src1)
	buffer.Write(src2)
	data := buffer.Bytes()
	return data
}

func GenMsg(lable string, data []byte) []byte {
	lableByte := lable2byte[lable]
	return Concat(lableByte, data)
}

func CheckError(err error) {
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}

func ExeCmd(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Printf("execute %s error:%v", cmd, err)
		os.Exit(1)
	}
	log.Println(string(out))
}

func SetTunClientLinux(TUN_IP string, ALLOW_IP string) {
	log.Println("[TUN_IP]", TUN_IP, "[ALLOW_IP]", ALLOW_IP)
	ExeCmd("ip link set dev tun0 up")
	ExeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", TUN_IP))
	ExeCmd(fmt.Sprintf("ip -4 route add %s/32 via %s dev tun0", ALLOW_IP, TUN_IP))
}

func SetTunServerLinux(TUN_IP string) {
	ExeCmd("ip link set dev tun0 up")
	ExeCmd(fmt.Sprintf("ip addr add %s/24 dev tun0", TUN_IP))
	// 把符合条件的用公网ip出去
	ExeCmd("iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -o eth0 -j MASQUERADE")
	ExeCmd("echo '1' > /proc/sys/net/ipv4/ip_forward")
}

func Filter(allow_port string, port string) bool {
	if allow_port == "*" {
		return true
	}
	exp := regexp.MustCompile(port)
	result := exp.MatchString(allow_port)
	return result
}
