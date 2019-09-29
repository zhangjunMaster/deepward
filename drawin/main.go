package main

import "fmt"

func main() {
	tun, err := CreateTUN("utun2", 1500)
	if err != nil {
		fmt.Println("[err]:", err)
	}
	fmt.Println("[tun]:", tun)
	for {
	}
}
