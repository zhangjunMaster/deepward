package main

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2019 WireGuard LLC. All Rights Reserved.
 */

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/zhangjunMaster/deepward/util"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

const (
	TUN_IP    = "10.0.0.8"
	OUTTER_IP = "139.219.6.50"
)

const utunControlName = "com.apple.net.utun_control"

// _CTLIOCGINFO value derived from /usr/include/sys/{kern_control,ioccom}.h
const _CTLIOCGINFO = (0x40000000 | 0x80000000) | ((100 & 0x1fff) << 16) | uint32(byte('N'))<<8 | 3

// sockaddr_ctl specifeid in /usr/include/sys/kern_control.h
type sockaddrCtl struct {
	scLen      uint8
	scFamily   uint8
	ssSysaddr  uint16
	scID       uint32
	scUnit     uint32
	scReserved [5]uint32
}

type NativeTun struct {
	name        string
	tunFile     *os.File
	errors      chan error
	routeSocket int
}

var sockaddrCtlSize uintptr = 32

// 监听路由，使一直存在着,不影响创建tun
func (tun *NativeTun) routineRouteListener(tunIfindex int) {
	//defer close(tun.events)

	data := make([]byte, os.Getpagesize())
	for {
	retry:
		n, err := unix.Read(tun.routeSocket, data)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.EINTR {
				goto retry
			}
			tun.errors <- err
			return
		}

		if n < 14 {
			continue
		}

		if data[3 /* type */] != unix.RTM_IFINFO {
			continue
		}
		ifindex := int(*(*uint16)(unsafe.Pointer(&data[12 /* ifindex */])))
		if ifindex != tunIfindex {
			continue
		}
	}
}

func CreateTUN(name string, mtu int) (*NativeTun, error) {
	ifIndex := -1
	if name != "utun" {
		_, err := fmt.Sscanf(name, "utun%d", &ifIndex)
		if err != nil || ifIndex < 0 {
			return nil, fmt.Errorf("Interface name must be utun[0-9]*")
		}
	}
	// 1.@@@这个类似于我们的内核与用户交互的文件
	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2)

	if err != nil {
		fmt.Println("[CreateTUN unix.Socket err]:", err)
		return nil, err
	}

	var ctlInfo = &struct {
		ctlID   uint32
		ctlName [96]byte
	}{}

	// 2.@@@ 驱动名称
	copy(ctlInfo.ctlName[:], []byte(utunControlName))

	// 3.@@@ 用户与内核态的交互
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),                      // fd是UDP的文件，用户的文件字符
		uintptr(_CTLIOCGINFO),            // _CTLIOCGINFO是驱动
		uintptr(unsafe.Pointer(ctlInfo)), //
	)

	if errno != 0 {
		return nil, fmt.Errorf("_CTLIOCGINFO: %v", errno)
	}

	sc := sockaddrCtl{
		scLen:     uint8(sockaddrCtlSize),
		scFamily:  unix.AF_SYSTEM,
		ssSysaddr: 2,
		scID:      ctlInfo.ctlID,
		scUnit:    uint32(ifIndex) + 1,
	}

	scPointer := unsafe.Pointer(&sc)

	// 做什么用的？
	_, _, errno = unix.RawSyscall(
		unix.SYS_CONNECT,
		uintptr(fd),
		uintptr(scPointer),
		uintptr(sockaddrCtlSize),
	)

	if errno != 0 {
		return nil, fmt.Errorf("SYS_CONNECT: %v", errno)
	}
	// 4.@@@转为非阻塞
	err = syscall.SetNonblock(fd, true)
	if err != nil {
		fmt.Println("[SetNonblock err]:", err)
		return nil, err
	}
	// 5.@@@生成 fd 文件，os.NewFile，以文件描述符 fd 为基础
	tun, err := CreateTUNFromFile(os.NewFile(uintptr(fd), ""), mtu)
	if err != nil {
		fmt.Println("[CreateTUNFromFile err]:", err)
	}
	return tun, err
}

// 从文件中生成TUN
func CreateTUNFromFile(file *os.File, mtu int) (*NativeTun, error) {
	tun := &NativeTun{
		tunFile: file,
		errors:  make(chan error, 5),
	}
	// 1.name: utun[0-9] 网卡名称
	name, err := tun.Name()
	fmt.Println("[name]:", name)

	if err != nil {
		fmt.Println("[tun.Name] err:", err)
		tun.tunFile.Close()
		return nil, err
	}
	// 2.通过网卡名称，获取网卡的Index
	tunIfindex, err := func() (int, error) {
		// net.InterfaceByName 通过网卡名获取网卡
		iface, err := net.InterfaceByName(name)
		if err != nil {
			fmt.Println("[net.InterfaceByName]:", err)
			return -1, err
		}
		return iface.Index, nil
	}()
	if err != nil {
		tun.tunFile.Close()
		return nil, err
	}
	// http://blog.chinaunix.net/uid-25324849-id-213155.html
	// 通信域为AF_ROUTE，它只能支持原始套接字，只有超级用户才能创建这个套接字
	// sockfd=socket(AF_ROUTE,SOCK_RAW,0);
	// 路由套接字,网卡的静态路由
	// 3.网卡静态路由
	tun.routeSocket, err = unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, unix.AF_UNSPEC)
	if err != nil {
		fmt.Println("[CreateTUNFromFile unix.Socket]:", err)
		tun.tunFile.Close()
		return nil, err
	}
	//  根据index,监听路由
	go tun.routineRouteListener(tunIfindex)

	if mtu > 0 {
		err = tun.setMTU(mtu)
		if err != nil {
			tun.Close()
			return nil, err
		}
	}

	return tun, nil
}

func (tun *NativeTun) operateOnFd(fn func(fd uintptr)) {
	sysconn, err := tun.tunFile.SyscallConn()
	if err != nil {
		tun.errors <- fmt.Errorf("unable to find sysconn for tunfile: %s", err.Error())
		return
	}
	err = sysconn.Control(fn)
	if err != nil {
		tun.errors <- fmt.Errorf("unable to control sysconn for tunfile: %s", err.Error())
	}
}

func (tun *NativeTun) Name() (string, error) {
	var ifName struct {
		name [16]byte
	}
	ifNameSize := uintptr(16)

	var errno syscall.Errno
	tun.operateOnFd(func(fd uintptr) {
		_, _, errno = unix.Syscall6(
			unix.SYS_GETSOCKOPT,
			fd,
			2, /* #define SYSPROTO_CONTROL 2 */
			2, /* #define UTUN_OPT_IFNAME 2 */
			uintptr(unsafe.Pointer(&ifName)),
			uintptr(unsafe.Pointer(&ifNameSize)), 0)
	})

	if errno != 0 {
		return "", fmt.Errorf("SYS_GETSOCKOPT: %v", errno)
	}

	tun.name = string(ifName.name[:ifNameSize-1])
	return tun.name, nil
}

func (tun *NativeTun) File() *os.File {
	return tun.tunFile
}

func (tun *NativeTun) Read(buff []byte) (int, error) {
	select {
	case err := <-tun.errors:
		return 0, err
	default:
		n, err := tun.tunFile.Read(buff)
		if n < 4 {
			return 0, err
		}
		return n - 4, err
	}
}

func (tun *NativeTun) Write(buff []byte) (int, error) {

	// reserve space for header

	// add packet information header

	buff[0] = 0x00
	buff[1] = 0x00
	buff[2] = 0x00

	if buff[4]>>4 == ipv6.Version {
		buff[3] = unix.AF_INET6
	} else {
		buff[3] = unix.AF_INET
	}

	// write

	return tun.tunFile.Write(buff)
}

func (tun *NativeTun) Flush() error {
	// TODO: can flushing be implemented by buffering and using sendmmsg?
	return nil
}

func (tun *NativeTun) Close() error {
	var err2 error
	err1 := tun.tunFile.Close()
	if tun.routeSocket != -1 {
		unix.Shutdown(tun.routeSocket, unix.SHUT_RDWR)
		err2 = unix.Close(tun.routeSocket)
	}
	if err1 != nil {
		return err1
	}
	return err2
}

func (tun *NativeTun) setMTU(n int) error {

	// open datagram socket

	var fd int
	// AF_INET:典型的TCP/IP四层模型的通信过程
	fd, err := unix.Socket(
		unix.AF_INET,
		unix.SOCK_DGRAM,
		0,
	)

	if err != nil {
		return err
	}

	defer unix.Close(fd)

	// do ioctl call

	var ifr [32]byte
	copy(ifr[:], tun.name)
	*(*uint32)(unsafe.Pointer(&ifr[unix.IFNAMSIZ])) = uint32(n)
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.SIOCSIFMTU),
		uintptr(unsafe.Pointer(&ifr[0])),
	)

	if errno != 0 {
		return fmt.Errorf("failed to set MTU on %s", tun.name)
	}

	return nil
}

func (tun *NativeTun) MTU() (int, error) {

	// open datagram socket
	// AF_INET:目的就是使用 IPv4 进行通信。
	// SOCK_DGRAM分是数据包,是udp协议网络编程
	fd, err := unix.Socket(
		unix.AF_INET,
		unix.SOCK_DGRAM,
		0,
	)

	if err != nil {
		return 0, err
	}

	defer unix.Close(fd)

	// do ioctl call

	var ifr [64]byte
	copy(ifr[:], tun.name)
	// 系统调用 SYS_IOCTL
	// 获取接口MTU
	// SIOCxxx: 系统的实现
	// MTU限制的是数据链路层的payload，也就是上层协议的大小，例如IP，ICMP等
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.SIOCGIFMTU),
		uintptr(unsafe.Pointer(&ifr[0])),
	)
	if errno != 0 {
		return 0, fmt.Errorf("failed to get MTU on %s", tun.name)
	}

	return int(*(*int32)(unsafe.Pointer(&ifr[16]))), nil
}

func main() {
	tun, err := CreateTUN("utun2", 1500)
	if err != nil {
		fmt.Println("[err]:", err)
	}
	fmt.Println("[tun]:", tun)

	// 2.打洞，发送握手信息
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 35520} // 注意端口必须固定
	dstAddr := &net.UDPAddr{IP: net.ParseIP(OUTTER_IP), Port: 9826}
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		fmt.Println("[Listen UDP err]:", err)
	}

	defer conn.Close()

	var n int
	if n, err = conn.WriteTo([]byte("我是打洞消息"), dstAddr); err != nil {
		log.Println("send handshake:", err)
	}
	fmt.Println("[conn write 我是打洞消息]", n)

	go func() {
		for {
			time.Sleep(10 * time.Second)
			if n, err = conn.WriteTo([]byte("from ["+TUN_IP+"]"), dstAddr); err != nil {
				log.Println("send msg fail", err)
			}
			fmt.Println("[打洞] dst:", OUTTER_IP, "[打洞数据]:", n)
		}
	}()

	fmt.Println("[tun server] Waiting IP Packet from UDP")

	// 1.将conn的数据，先给虚拟网卡，虚拟网卡再转给物理网卡
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
			n, err = conn.WriteTo(buf[:n], dstAddr)
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
		n, err = tun.Write(buf[:n])
		if err != nil {
			fmt.Println("[tun client write to tun] udp write error:", err)
			continue
		}
		fmt.Printf("[tun client write to tun] write %d bytes to tun interface\n", n)
	}
	for {
	}
}

// sysctl -w net.inet.ip.forwarding=1 路由转发
// ifconfig utun2 10.10.10.8 10.10.10.8 设置ip
// 设置NAT https://www.xarg.org/2017/07/set-up-internet-sharing-on-mac-osx-using-command-line-tools/
// /etc/pf.conf:
// nat on en0 from utun2:network to any -> (en0)
//# Disable PF if it was enabled before
//sudo pfctl -d
//# Enable PF and load the config
//sudo pfctl -e -f /etc/pf.conf

//https://blog.chionlab.moe/2016/02/01/use-pf-on-osx/
//https://apple.stackexchange.com/questions/192089/how-can-i-setup-my-mac-os-x-yosemite-as-an-internet-gateway
//pfctl -s nat 是否起作用
