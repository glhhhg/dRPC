package main

import (
	"dPRC/registry"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
)

func displayHelp() {
	fmt.Println("Usage dprc registry:  -i IPADDRESS -p PORT")
	fmt.Println("\t-i\tregistry IPv4 or IPv6 address")
	fmt.Println("\t-p\tregistry port number")
	fmt.Println("\t-h\thelp message")
}
func main() {
	help := flag.Bool("h", false, "display help")
	ip := flag.String("i", "127.0.0.1", "ip address")
	port := flag.Int("p", 8080, "port")
	flag.Parse()

	if *help {
		displayHelp()
		return
	}

	address := fmt.Sprintf("%s:%d", *ip, *port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("registry failed to listen: %v", err)
		return
	}
	reg := registry.NewRegistry(0)
	reg.HandleHTTP("/registry")
	_ = http.Serve(lis, nil)
}
