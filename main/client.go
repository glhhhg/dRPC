package main

import (
	"dPRC/client"
	"dPRC/registry"
	"log"
)

func main() {
	dis := registry.NewRegistryDiscovery("http://192.168.1.10:8080/registry", 0)
	clt := client.NewBClient(dis, registry.RandomSelect)

	var reply float64
	if err := clt.Call("Circle.Area", Circle{5.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Call Circle.Area:", reply)

	if err := clt.Call("Circle.GetRadius", Circle{15.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Call Circle.Radius:", reply)

	if err := clt.Call("Circle.Length", Circle{5.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Call Circle.Length:", reply)

	// -----------------------------------------------------------------------------------
	if err := clt.Broadcast("Circle.Area", Circle{5.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Broadcast Circle.Area:", reply)

	if err := clt.Broadcast("Circle.GetRadius", Circle{15.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Broadcast Circle.Radius:", reply)

	if err := clt.Broadcast("Circle.Length", Circle{5.0}, &reply); err != nil {
		log.Fatal(err)
	}
	log.Println("Broadcast Circle.Length:", reply)
}
