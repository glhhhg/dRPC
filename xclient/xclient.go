/*
一个面向用户暴露的支持负载均衡的客户端 XClient
*/

package xclient

import (
	"context"
	"io"
	"reflect"
	"rpc_test/client"
	"rpc_test/server"
	"sync"
)

type XClient struct {
	d       Discovery
	mode    SelectMode
	opt     *server.Option
	mu      sync.Mutex
	clients map[string]*client.Client
}

var _ io.Closer = (*XClient)(nil)

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	for key, clt := range xc.clients {
		_ = clt.Close()
		delete(xc.clients, key)
	}
	return nil
}

/*
NewXClient 的构造函数需要传入三个参数，服务发现实例Discovery、负载均衡模式SelectMode
以及协议选项Option。使用clients 保存创建成功的 Client 实例，
*/
func NewXClient(d Discovery, mode SelectMode, opt *server.Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*client.Client),
	}
}

/*
dial 检查xc.clients是否有缓存的Client，如果有，检查是否是可用状态，如果是则返回缓存的 Client;
如果不可用，则从缓存中删除。如果没有返回缓存的Client，则说明需要创建新的Client，缓存并返回。
*/
func (xc *XClient) dial(rpcAddr string) (*client.Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	clt, ok := xc.clients[rpcAddr]
	if ok && !clt.IsAvailable() {
		_ = clt.Close()
		delete(xc.clients, rpcAddr)
		clt = nil
	}
	if clt == nil {
		var err error
		clt, err = client.Dial("tcp", rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = clt
	}
	return clt, nil
}

func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string,
	args, reply interface{}) error {
	clt, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return clt.Call(ctx, serviceMethod, args, reply)
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

/*
Broadcast 将请求广播到注册中心所有的服务实例，如果任意一个实例发生错误，返回错误；如果调用成功，返回一个结果
 1. 为了提升性能，请求是并发的。
 2. 并发情况下需要使用互斥锁保证 error 和 reply 能被正确赋值。
 3. 借助 context.WithCancel 确保有错误发生时，快速失败。
*/
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string,
	args, reply interface{}) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil // if reply is nil, don't need to set value
	ctx, cancel := context.WithCancel(ctx)

	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()

			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel() // if call failed, cancel calls
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
