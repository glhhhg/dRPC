/*
根据net/rpc中客户端调用可以归纳，每一次call包含
	1. method's type
	2. methods itself
	3. methods two argument, second argument is a pointer
	4. method's return type is error
例如：func (t* T) MethodName(argType T1, replyType *T2) error {}
结构体Call 用来承载上述的要素，其中Done表示当调用结束时，会调用call.done()通知调用方。
客户端调用支持异步
*/

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"rpc_test/codec"
	"rpc_test/server"
	"sync"
	"time"
)

type Call struct {
	Seq           uint64
	ServiceMethod string // 与server.go一样，形式为 service.method
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}

/*
Client代表一个RPC客户端。单个 Client 可能有多个相关联的调用，一个Client可能同时被多个goroutine使用。
Client结构体中：
	cc 是消息的编解码器，和服务端类似，消息的序列化及反序列化。
	sending 互斥锁，防止出现多个请求报文混淆。
	seq 用于给发送的请求编号，每个请求拥有唯一编号。
	pending 用于存储未处理完的请求的哈希表，键是编号，值是Call对象。
	losing 和shutdown 任意一个值置为true，则表示Client处于不可用的状态，
但是closing是用户主动关闭的，即调用 Close() 方法，而shutdown置为 true一般是有错误发生。
*/

type Client struct {
	cc       codec.Codec
	opt      *server.Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *server.Option) (client *Client, err error)

var _ io.Closer = (*Client)(nil)
var ErrorShutDown error = errors.New("connection is closed")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrorShutDown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !(client.shutdown || client.closing)
}

// registerCall 将参数 call 添加到 client.pending 中，并更新 client.seq。
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrorShutDown
	}
	call.Seq = client.seq
	client.pending[client.seq] = call
	client.seq++
	return call.Seq, nil
}

// removeCall 根据 seq，从 client.pending 中移除对应的 call，并返回。
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminateCalls 服务端或客户端发生错误时调用，将shutdown设置为true，且将错误信息通知所有pending状态的call。
func (client *Client) terminateCall(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		// 读取消息头出错，结束处理
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}

		call := client.removeCall(h.Seq)

		switch {
		// 服务端返回的call不存在，可能是因为服务端已经处理过了或者是返回的消息出错
		case call == nil:
			err = client.cc.ReadBody(nil)
		// 消息头部错误
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		// 读取消息体Body到call.Reply中进一步处理
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body: " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCall(err)
}

// NewClient 创建Client对象，同时与服务端协商好协议Option
func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invaild codec type %s", opt.CodecType)
		log.Println("rpc client: codec error: ", err)
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(f codec.Codec, opt *server.Option) *Client {
	client := &Client{
		seq:     1, // seq starts with 1, 0 means invalid call
		cc:      f,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

// parseOptions 解析Option字段，以便后续Dial建立连接
func parseOptions(opts ...*server.Option) (*server.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return server.DefaultOption, nil
	}
	if len(opts) > 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = server.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = server.DefaultOption.CodecType
	}
	return opt, nil
}

// dialTimeout 与服务端建立连接时超时
func dialTimeout(f newClientFunc, network, address string, opts ...*server.Option) (*Client, error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectionTimeout)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{client, err}
	}()
	// 超时时间限制为0
	if opt.ConnectionTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}

	select {
	case result := <-ch:
		return result.client, result.err
	case <-time.After(opt.ConnectionTimeout):
		return nil, fmt.Errorf("rpc client: connection timeout: expect within %s", opt.ConnectionTimeout)
	}
}

// Dial 用户传入服务端地址，创建 Client 实例
func Dial(network, address string, opts ...*server.Option) (client *Client, err error) {
	return dialTimeout(NewClient, network, address, opts...)
}

// send 客户端发送请求
func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	// register call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare for header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// encode and send request
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		// if call is nil, it means Write failed, so we don't need to remove if nil
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

/*
Go 和 Call 是客户端暴露给用户的两个RPC服务调用接口，Go是一个异步接口，返回call实例。
Call 是对 Go 的封装，阻塞call.Done，等待响应返回，是一个同步接口。
*/
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case call := <-call.Done:
		return call.Error
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	}
}
