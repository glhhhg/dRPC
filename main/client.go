package main

import (
	"dPRC/client"
	"dPRC/main/methods"
	"dPRC/registry"
	"flag"
	"fmt"
	"log"
)

func displayHelp() {
	fmt.Println("Usage dprc client:  -i IPADDRESS -p PORT")
	fmt.Println("\t-i\tclient call registry's IPv4 or IPv6 address")
	fmt.Println("\t-p\tclient call registry's port number")
	fmt.Println("\t-h\thelp message")
}

func main() {
	help := flag.Bool("h", false, "display help")
	ip := flag.String("i", "", "client call registry ip address")
	port := flag.Int("p", 0, "client call registry port")
	flag.Parse()
	if *help {
		displayHelp()
		return
	}

	address := fmt.Sprintf("http://%s:%d/", *ip, *port)
	dis := registry.NewRegistryDiscovery(address, 0)

	var reply methods.Reply
	var args = methods.Args{
		A: 1, B: 2,
	}
	for i := range 10 {
		clt := client.NewBClient(dis, registry.RandomSelect)
		if err := clt.Call("Function.Add", args, &reply); err != nil {
			log.Println(err)
			return
		}
		fmt.Printf("client%d: Function.Add reslut %d \n", i, reply)
	}

}
