package main

import "log"

func main() {
	tun, err := CreateTUN("utun2", 1500)
	if err != nil {
		log.Println("[err]:", err)
	}
	log.Println("[tun]:", tun)
	for {
	}
}
