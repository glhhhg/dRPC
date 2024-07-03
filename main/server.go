package main

import (
	"dPRC/main/methods"
	"dPRC/registry"
	"dPRC/server"
	"flag"
	"fmt"
	"log"
	"net"
)

func displayHelp() {
	fmt.Println("Usage dprc server:  -l IPADDRESS -p PORT")
	fmt.Println("\t-l\tserver listen IPv4 or IPv6 address")
	fmt.Println("\t-p\tserver listen port number")
	fmt.Println("\t-h\thelp message")
}

func main() {
	help := flag.Bool("h", false, "display help")
	ip := flag.String("l", "0.0.0.0", "server ip address")
	port := flag.Int("p", 12000, "server port")
	flag.Parse()

	if *help {
		displayHelp()
		return
	}

	address := fmt.Sprintf("%s:%d", *ip, *port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("server failed to listen: %v", err)
	}
	s := server.NewServer()

	var f methods.Function
	if err = s.Register(&f); err != nil {
		log.Fatalf("server failed to register method: %v", err)
	}
	registry.Heartbeat("http://192.168.1.10:8080/",
		"192.168.1.11:12000", 0)
	s.Accept(lis)
}
