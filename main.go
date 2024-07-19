/*
该main.go 文件主要用来测试客户端与服务端之间rpc通信
一个函数需要能够被远程调用，需要满足如下五个条件：
1. the method's type is exported.
2. the method is exported.
3. the method has two arguments, both exported (or builtin) types.
4. the method's second argument is a pointer.
5. the method has return type error.
*/
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"rpc_test/registry"
	"rpc_test/server"
	"rpc_test/xclient"
	"sync"
	"time"
)

// 定义结构体 Foo 和方法 Sum

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

// 启动注册中心
func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

// 启动服务端
func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	s := server.NewServer()
	_ = s.Register(&foo)
	registry.Heartbeat(registryAddr, l.Addr().String(), 0)
	wg.Done()
	s.Accept(l)
}

// printLog 便于在 Call 或 Broadcast 之后统一打印成功或失败的日志
func printLog(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string,
	args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod,
			args.Num1, args.Num2, reply)
	}
}

func call(registry string) {
	r := xclient.NewRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(r, xclient.RandomSelect, nil)
	defer func() {
		_ = xc.Close()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			printLog(xc, context.Background(), "call", "Foo.Sum",
				&Args{
					Num1: i,
					Num2: i * 10,
				})
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string) {
	r := xclient.NewRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(r, xclient.RandomSelect, nil)
	defer func() {
		_ = xc.Close()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			printLog(xc, context.Background(), "broadcast", "Foo.Sum",
				&Args{
					Num1: i,
					Num2: i * 10,
				})
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			printLog(xc, ctx, "broadcast", "Foo.Sleep",
				&Args{
					Num1: i,
					Num2: i * 10,
				})
		}(i)
	}
	wg.Wait()
}

// 构造参数，发送 RPC 请求，并打印结果
func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/rpc-test/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}
