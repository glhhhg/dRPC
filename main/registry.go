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
	fmt.Println("Usage dprc registry:  -l IPADDRESS -p PORT")
	fmt.Println("\t-l\tregistry listen IPv4 or IPv6 address")
	fmt.Println("\t-p\tregistry listen port number")
	fmt.Println("\t-h\thelp message")
}
func main() {
	help := flag.Bool("h", false, "display help")
	ip := flag.String("l", "0.0.0.0", "ip address")
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
	reg.HandleHTTP("/")
	_ = http.Serve(lis, nil)
}
