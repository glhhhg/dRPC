package client

import (
	"dPRC/option"
	"dPRC/serialize"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Call 表示一个活跃的RPC客户端实例
// Seq 客户端调用的序号，用于区分不同调用
// Args 调用方法的传参
// Reply 调用方法的返回值
// Error 调用时如果出错会设置Error
// Done 为了支持并发调用，Call结构体中添加了一个字段 Done，
// 的类型是chan *Call，当调用结束时，会调用call.done()通知调用方
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

// Client 支持并发的客户端实例
// seq 用于给发送的请求进行编号，每个请求拥有唯一编号。
// cc 是消息的编解码器，处理消息的序列化及反序列化。
// header 请求消息的头部，包含了要调用的方法
// pending 用于存储未处理完的请求，键是seq，值是Call实例
// sending 互斥锁，防止出现多个请求报文混淆。
// closing和shutdown 任意一个值置为true，则表示Client处于不可用的状态，
// 但是closing是用户主动关闭的，即调用Close() 方法，
// 而shutdown置为true一般是有错误发生。
type Client struct {
	seq      uint64
	cc       serialize.Coder
	opt      *option.Option
	header   serialize.Header
	pending  map[uint64]*Call
	sending  sync.Mutex
	mu       sync.Mutex
	closing  bool
	shutdown bool
}

var _ io.Closer = (*Client)(nil)
var ErrorShutDown = errors.New("connection is closed")

func (client *Client) Close() error {
	//TODO implement me
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

// registerCall 将参数Call实例添加到client.pending中，并更新client.seq
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	// 不采用IsAvailable防止死锁
	if client.closing || client.shutdown {
		return 0, ErrorShutDown
	}
	call.Seq = client.seq
	client.pending[client.seq] = call
	client.seq++
	return call.Seq, nil
}

// removeCall 根据seq，从client.pending中移除对应的call，并返回。
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminateCalls 服务端或客户端发生错误时调用，将shutdown设置为true，
// 且将错误信息通知所有pending状态的call。
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

// receive 客户端处理服务端处理过后返回的请求
func (client *Client) receive() {
	var err error = nil
	for err == nil {
		var h serialize.Header
		// 读取消息头出错，结束处理
		if err = client.cc.ReadHeader(&h); err != nil {
			log.Println("rpc client: receive header error:", err.Error())
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
		default:
			err = client.cc.ReadBody(call.Reply)
			log.Printf("rpc client: receive from server: header %v, body %s", h, call.Reply)
			if err != nil {
				call.Error = errors.New("reading body: " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCall(err)
}

// newClient 创建一个客户端实例并向服务端发送Option字段协商好协议
// 通过receive并发地处理服务端处理过后返回的请求
func newClient(conn net.Conn, opt *option.Option) *Client {
	c := serialize.NewJSONCoder(conn)

	var h serialize.Header
	if err := c.Write(&h, opt); err != nil {
		_ = c.Close()
		_ = conn.Close()
		return nil
	}
	client := &Client{
		seq:     1, // seq starts with 1, 0 means invalid call
		cc:      c,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

// parseOptions 解析Option字段，方便与服务端建立连接建立连接
func parseOptions(opts ...*option.Option) (*option.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return option.DefaultOption, nil
	}
	if len(opts) > 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = option.DefaultOption.MagicNumber
	return opt, nil
}

// dialTimeout 超时机制下客户端与服务端建立连接
func dialTimeout(network, address string, opts ...*option.Option) (*Client, error) {
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

	ch := make(chan *Client)
	go func() {
		client := newClient(conn, opt)
		ch <- client
	}()
	// 超时时间限制为0
	if opt.ConnectionTimeout == 0 {
		result := <-ch
		return result, nil
	}

	select {
	case result := <-ch:
		return result, nil
	case <-time.After(opt.ConnectionTimeout):
		return nil, fmt.Errorf("rpc client: connection timeout: "+
			"expect within %s", opt.ConnectionTimeout)
	}
}

// Dial 用户传入服务端地址，创建Client实例并处理服务端返回的数据
func Dial(network, address string, opts ...*option.Option) (client *Client, err error) {
	log.Println("rpc client: dial with:", network, address)
	return dialTimeout(network, address, opts...)
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

// Go 和 Call 是客户端暴露给用户的两个RPC服务调用接口，Go是一个异步接口，返回call实例。
// Call 是对 Go 的封装，阻塞call.Done，等待响应返回，是一个同步接口。
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

// Call 和 Go 是客户端暴露给用户的两个RPC服务调用接口，Go是一个异步接口，返回call实例。
// Call 是对 Go 的封装，阻塞call.Done，等待响应返回，是一个同步接口。
func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	log.Printf("rpc client: call method %s(args %s, reply %s)", serviceMethod, args, reply)
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case call := <-call.Done:
		return call.Error
	case <-time.After(client.opt.HandlerTimeout):
		client.removeCall(call.Seq)
		return errors.New(fmt.Sprintf("rpc client: handle timeout: expect within %s",
			client.opt.HandlerTimeout))
	}
}
