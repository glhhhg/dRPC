package main

import (
	"dPRC/registry"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
)

func main() {
	ip := flag.String("i", "127.0.0.1", "ip address")
	port := flag.Int("p", 8080, "port")
	flag.Parse()
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
