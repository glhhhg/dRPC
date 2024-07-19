/*
客户端中的 Option 结构体主要用来指定客户端序列化与反序列化方式，在本次实现中默认的方式是Gob
但是为了简单，客户端固定采用JSON编码Option，后续的header和body的编码方式由Option中的CodeType指定
服务端首先使用JSON解码Option，然后通过Option的CodeType解码剩余的内容，在本次实现中主要是Gob解码
| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
为了测试简便，代码中DefaultOption 设置默认的序列化方式，DefaultServer 为默认的Server对象
DefaultAccept 为默认的接受tcp监听的函数。

服务端从接收到请求到回复一共以下几个步骤：
	第一步，根据入参类型，将请求的 body 反序列化；
	第二步，调用 service.call，完成方法调用；
	第三步，将 reply 序列化为字节流，构造响应报文，返回。

超时处理
	客户端：
		1. 与服务端建立连接时超时
		2. 生成请求报文时超时
		3. 等待服务端处理时超时
		4. 接收服务端响应报文时超时
	服务端：
		1. 读取客户端请求报文时超时
		2. 生成响应报文时超时
		3. 调用请求的方法，处理报文超时
	超时设置在Option字段中。
*/

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"rpc_test/codec"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber       int           // 用来区分一次客户端的请求
	CodecType         codec.Type    // 用来指定客户端序列化与反序列化方式
	ConnectionTimeout time.Duration // 超时的时间限制
	HandlerTimeout    time.Duration
}

// DefaultOption 设置默认的序列化方式 Gob，默认超时时间为10s
var DefaultOption = &Option{
	MagicNumber:       MagicNumber,
	CodecType:         codec.GobType,
	ConnectionTimeout: 10 * time.Second,
}

// Server 服务端结构体
type Server struct {
	serviceMap sync.Map // 多线程安全的Map
}

// Register 服务端Server注册服务rcvr
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc service already defined: " + s.name)
	}
	return nil
}

// Register 默认Server情况下的register
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// findService 服务端查找服务，ServiceMethod 的构成是 "Service.Method"
func (server *Server) findService(serviceMethod string) (s *service, mtype *methodType, err error) {
	index := strings.LastIndex(serviceMethod, ".")
	if index < 0 {
		err = errors.New("rpc server: service/method request wrong-formed: " + serviceMethod)
		return nil, nil, err
	}
	serviceName, methodName := serviceMethod[:index], serviceMethod[index+1:]
	svc, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service: " + serviceName)
		return nil, nil, err
	}
	s = svc.(*service)
	mtype = s.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method: " + methodName)
		return nil, nil, err
	}
	return s, mtype, nil
}

// NewServer 构造一个新的Server对象
func NewServer() *Server {
	return &Server{}
}

// DefaultServer 默认的Server对象
var DefaultServer = NewServer()

// Accept for循环等待 socket 连接建立，并开启子协程处理
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			return
		}
		go server.ServerConn(conn)
	}
}

// DefaultAccept 默认的Accept函数
func DefaultAccept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

/*
ServeConn 首先使用json.NewDecoder反序列化得到Option实例，检查MagicNumber和CodeType
然后根据CodeType得到对应的消息编解码器，接下来的处理交给serverCodec
*/

func (server *Server) ServerConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()

	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x\n", opt.MagicNumber)
		return
	}
	/*
		构造一个确定序列化方式的构造函数，默认是Gob序列化方式，使用了codec中的NewCodecFuncMap表
		得到的变量f 是一个构造函数，构造一个opt.CodecType序列化方式的Codec对象
	*/
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s\n", opt.CodecType)
		return
	}
	server.serverCodec(f(conn))
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct {
}{}

/*
读取请求 readRequest
处理请求 handleRequest
回复请求 sendResponse
*/
func (server *Server) serverCodec(f codec.Codec) {
	sending := new(sync.Mutex) // 互斥锁，处理并发
	wg := new(sync.WaitGroup)  // 确保并发程序执行完毕

	for {
		req, err := server.readRequest(f)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(f, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(f, req, sending, wg, 10*time.Second)
	}
	wg.Wait()
	_ = f.Close()
}

// request 存储了客户端每一次发送的所有数据，包括Header和Body
type request struct {
	h           *codec.Header // 请求消息的Header
	argv, reply reflect.Value // 请求消息的传参和返回值
	mtype       *methodType   // 客户端所请求方法的类型
	svc         *service      // 客户端请求的服务
}

// 读取请求消息的头部信息
func (server *Server) readRequestHeader(c codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := c.ReadHeader(&h); err != nil {
		if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("rpc server: read header error: ", err)
		}
		return nil, err
	}
	return &h, nil
}

/*
readRequest 通过newArgv()和newReply()两个方法创建出两个入参实例，然后通过f.ReadBody()
将请求报文反序列化为第一个入参 argv，在这里同样需要注意argv可能是值类型，也可能是指针类型。
*/
func (server *Server) readRequest(f codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(f)
	if err != nil {
		return nil, err
	}

	req := &request{h: h}
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.reply = req.mtype.NewReply()

	// argv必须是指针
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = f.ReadBody(argvi); err != nil {
		log.Println("rpc server: read argv error: ", err)
		return req, err
	}
	return req, nil
}

func (server *Server) sendResponse(f codec.Codec, h *codec.Header, body interface{},
	sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := f.Write(h, body); err != nil {
		log.Println("rpc server: write response error: ", err)
	}
}

func (server *Server) handleRequest(f codec.Codec, req *request, sending *sync.Mutex,
	wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{}) // 服务端调用Call时的超时处理通道
	sent := make(chan struct{})   // 服务端发送响应报文时的超时处理通道
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.reply)
		called <- struct{}{} // 可能超时
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(f, req.h, invalidRequest, sending)
			sent <- struct{}{} // 可能超时
			return
		}

		server.sendResponse(f, req.h, req.reply.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0 { // 设置的超时时间限制时0
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: requset handle timeout: expect within %s", timeout)
		server.sendResponse(f, req.h, invalidRequest, sending)
	case <-called:
		<-sent
	}
}
