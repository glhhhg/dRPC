package main

import (
	"dPRC/client"
	"dPRC/registry"
	"errors"
	"fmt"
	"math"
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
	/*ip:port address from cmd*/
	//ip := flag.String("i", "127.0.0.1", "server ip address")
	//port := flag.Int("p", 8080, "server port")
	//flag.Parse()
	//address := fmt.Sprintf("%s:%d", *ip, *port)

	/*normal server end*/
	//listener, err := net.Listen("tcp", address)
	//if err != nil {
	//	log.Fatalf("dRPC server listener error: %v", err)
	//	return
	//}
	//s := server.NewServer()
	//var f Func
	//if err := s.Register(f); err != nil {
	//	log.Fatalf("dRPC server register error: %v", err)
	//	return
	//}
	//s.Accept(listener)

	/*normal client end*/
	//clt, _ := client.Dial("tcp", "127.0.0.1:12000")
	//var r int
	//clt.Call("Func.Add", &Func{1, 1}, &r)
	//fmt.Println(r)

	/*registry end*/
	//lis, err := net.Listen("tcp", address)
	//if err != nil {
	//	panic(err)
	//}
	//reg := registry.NewRegistry(time.Minute * 5)
	//reg.HandleHTTP("/rpc-test/registry")
	//_ = http.Serve(lis, nil)

	/*server with heartbeat*/
	//listener, err := net.Listen("tcp", address)
	//if err != nil {
	//	log.Fatalf("dRPC server listener error: %v", err)
	//	return
	//}
	//s := server.NewServer()
	//var f Func
	//if err := s.Register(f); err != nil {
	//	log.Fatalf("dRPC server register error: %v", err)
	//	return
	//}
	//registry.Heartbeat("http://127.0.0.1:12000/rpc-test/registry", address, 0)
	//s.Accept(listener)

	/*balance and load client end*/
	d := registry.NewRegistryDiscovery("http://127.0.0.1:12000/rpc-test/registry", 0)
	bc := client.NewBClient(d, registry.RandomSelect)
	defer bc.Close()

	var r int
	bc.Call("Func.Add", Func{1, 1}, &r)
	fmt.Println("Call: ", r)
	bc.Broadcast("Func.Add", Func{1, 1}, &r)
	fmt.Println("Broadcast: ", r)
}
