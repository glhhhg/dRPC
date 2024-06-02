package main

import (
	"dPRC/client"
	"log"
)

type Func struct {
	Num1, Num2 int
}

func main() {
	address := "127.0.0.1:12000"
	clt, err := client.Dial("tcp", address)
	if err != nil {
		log.Fatalf("client dial server error: %v", err)
		return
	}
	defer func() {
		_ = clt.Close()
	}()

	var reply int
	_ = clt.Call("Func.Add", Func{Num1: 10, Num2: 20}, &reply)
	log.Printf("client call reply: %d", reply)
}
