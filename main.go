package main

import (
	"dPRC/server"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
)

type Func struct {
	Num1, Num2 int
}

func (f Func) Add(n Func, rely *int) error {
	*rely = n.Num1 + n.Num2
	return nil
}
func (f Func) Sub12(n Func, rely *int) error {
	*rely = n.Num1 - n.Num2
	return nil
}
func (f Func) Sub21(n Func, rely *int) error {
	*rely = n.Num2 - n.Num1
	return nil
}
func (f Func) Mul(n Func, rely *int) error {
	*rely = n.Num1 * n.Num2
	return nil
}
func (f Func) Div12(n Func, rely *int) error {
	if n.Num2 == 0 {
		return errors.New("division by zero")
	}
	*rely = n.Num1 / n.Num2
	return nil
}
func (f Func) Div21(n Func, rely *int) error {
	if n.Num1 == 0 {
		return errors.New("division by zero")
	}
	*rely = n.Num2 / n.Num1
	return nil
}
func (f Func) Exp12(n Func, rely *int) error {
	*rely = int(math.Pow(float64(n.Num1), float64(n.Num2)))
	return nil
}
func (f Func) Exp21(n Func, rely *int) error {
	*rely = int(math.Pow(float64(n.Num2), float64(n.Num1)))
	return nil
}
func main() {
	ip := flag.String("i", "127.0.0.1", "server ip address")
	port := flag.Int("p", 8080, "server port")
	flag.Parse()
	address := fmt.Sprintf("%s:%d", *ip, *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("dRPC server listener error: %v", err)
		return
	}
	s := server.NewServer()
	var f Func
	if err := s.Register(f); err != nil {
		log.Fatalf("dRPC server register error: %v", err)
		return
	}
	s.Accept(listener)
}
