// Simple use of the deepward package that prints packets received by the interface.
package main

import (
	"encoding/binary"
	"log"
	"os"

	"github.com/zhangjunMaster/deepward"
	"github.com/zhangjunMaster/deepward/util"
)

func main() {
	if len(os.Args) != 3 {
		log.Println("syntax:", os.Args[0], "tun|tap", "<device name>")
		return
	}

	var typ deepward.DevKind
	switch os.Args[1] {
	case "tun":
		typ = deepward.DevTun
	case "tap":
		typ = deepward.DevTap
	default:
		log.Println("Unknown device type", os.Args[1])
		return
	}

	tun, err := deepward.Open(os.Args[2], typ)
	if err != nil {
		log.Println("Error opening tun/tap device:", err)
		return
	}

	log.Println("Listening on", string(tun.Name()))
	buf := make([]byte, 1522)
	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("Read error:", err)
		} else {
			if util.IsIPv4(buf) {
				log.Printf("%d bytes from iface, IHL:%02X, TTL:%d\n", n, buf[0], buf[8])
				log.Printf("from %s to %s\n", util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
				log.Printf("protocol %02x checksum %02x\n", util.IPv4Protocol(buf), binary.BigEndian.Uint16(buf[22:24]))
				if util.IPv4Protocol(buf) == util.ICMP {
					srcip := make([]byte, 4)
					copy(srcip, buf[12:16])
					copy(buf[12:16], buf[16:20])
					copy(buf[16:20], srcip)
					buf[20] = 0x00
					buf[21] = 0x00
					buf[22] = 0x00
					buf[23] = 0x00
					cksum := util.Checksum(buf[20:n])
					log.Printf("my checksum:%02x\n", uint16(cksum))
					buf[22] = byte((cksum & 0xff00) >> 8)
					buf[23] = byte(cksum & 0xff)
					log.Printf("rsp: from %s to %s\n", util.IPv4Source(buf).String(), util.IPv4Destination(buf).String())
					_, err = tun.Write(buf)
					if err != nil {
						log.Println(err)
					}
				}
			}
		}
	}
}
