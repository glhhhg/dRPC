package main

import (
	"dPRC/server"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
)

type Circle struct {
	R float64
}

func (c Circle) Area(circle Circle, area *float64) error {
	*area = math.Pi * circle.R * circle.R
	return nil
}

func (c Circle) GetRadius(circle Circle, radius *float64) error {
	*radius = circle.R
	return nil
}
func (c Circle) Length(circle Circle, l *float64) error {
	*l = circle.R * 2 * math.Pi
	return nil
}

func main() {
	ip := flag.String("l", "127.0.0.1", "server ip address")
	port := flag.Int("p", 8080, "server port")
	flag.Parse()

	address := fmt.Sprintf("%s:%d", *ip, *port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("server failed to listen: %v", err)
	}
	s := server.NewServer()

	var c Circle
	if err = s.Register(c); err != nil {
		log.Fatalf("server failed to register serve: %v", err)
	}
	s.Accept(lis)
}
