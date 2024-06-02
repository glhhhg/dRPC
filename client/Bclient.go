package client

import (
	"dPRC/option"
	"dPRC/registry"
	"io"
	"reflect"
	"sync"
)

type BClient struct {
	d       registry.Discovery
	mode    registry.BalanceMode
	opt     *option.Option
	mu      sync.Mutex
	clients map[string]*Client
}

var _ io.Closer = (*BClient)(nil)

func (xc *BClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	for key, clt := range xc.clients {
		_ = clt.Close()
		delete(xc.clients, key)
	}
	return nil
}

// NewBClient 的构造函数需要传入三个参数，服务发现实例Discovery、负载均衡模式SelectMode
// 以及协议选项Option。使用clients 保存创建成功的 Client 实例
func NewBClient(d registry.Discovery, mode registry.BalanceMode, opts ...*option.Option) *BClient {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil
	}
	return &BClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

// dial 检查xc.clients是否有缓存的Client，如果有，检查是否是可用状态，如果是则返回缓存的 Client;
// 如果不可用，则从缓存中删除。如果没有返回缓存的Client，则说明需要创建新的Client，缓存并返回。
func (xc *BClient) dial(rpcAddr string) (*Client, error) {
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
		clt, err = Dial("tcp", rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = clt
	}
	return clt, nil
}

func (xc *BClient) call(rpcAddr string, serviceMethod string,
	args, reply interface{}) error {
	clt, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return clt.Call(serviceMethod, args, reply)
}

func (xc *BClient) Call(serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, serviceMethod, args, reply)
}

// Broadcast 将请求广播到注册中心所有的服务实例，如果任意一个实例发生错误，返回错误；
// 如果调用成功，返回一个结果
// 为了提升性能，请求是并发的。
// 并发情况下需要使用互斥锁保证 error 和 reply 能被正确赋值。
// 借助 context.WithCancel 确保有错误发生时，快速失败。
func (xc *BClient) Broadcast(serviceMethod string,
	args, reply interface{}) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil // if reply is nil, don't need to set value

	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()

			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err // if call failed, cancel calls
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
