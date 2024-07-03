package main

import (
	"dPRC/client"
	"log"
)

func main() {
	clt, err := client.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = clt.Close()
	}()
	var r float64
	if err = clt.Call("Circle.Area", struct {
		R float64
	}{1}, &r); err != nil {
		panic(err)
	}
	log.Println(r)
}
