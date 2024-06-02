package server

import (
	"dPRC/option"
	"dPRC/serialize"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Server 服务端结构体
type Server struct {
	serviceMap sync.Map // 多线程安全的Map
}

// NewServer 构造一个新的Server对象
func NewServer() *Server {
	return &Server{}
}

// Register 服务端Server注册服务rcvr，将rcvr的所有符合协议的方法注册到服务端
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc service already defined: " + s.name)
	}
	return nil
}

// findService 服务端查找服务，查找服务端是否有注册好的ServiceMethod服务
// ServiceMethod的构成是 "Service.Method"
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

// Accept for循环等待tcp socket连接建立，与客户端建立起链接，并开启子协程处理
func (server *Server) Accept(lis net.Listener) {
	log.Println("rpc server accept listening on", lis.Addr().Network(), lis.Addr())
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			return
		}
		go server.ServerConn(conn)
	}
}

// ServerConn 首先反序列化得到Option实例，检查MagicNumber和CodeType
// 然后根据CodeType得到对应的消息编解码器，接下来的处理交给serverHandler
func (server *Server) ServerConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()

	coder := serialize.NewJSONCoder(conn)

	var h serialize.Header
	if err := coder.ReadHeader(&h); err != nil {
	}

	var opt option.Option
	if err := coder.ReadBody(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	if opt.MagicNumber != option.MagicNumber {
		log.Printf("rpc server: invalid magic number %x\n", opt.MagicNumber)
		return
	}

	server.serverHandler(coder, opt.HandlerTimeout)
}

// request 存储了客户端每一次发送的所有数据，包括Header和Body
type request struct {
	h           *serialize.Header // 请求消息的Header
	argv, reply reflect.Value     // 请求消息的传参和返回值
	mtype       *methodType       // 客户端所请求方法的类型
	svc         *service          // 客户端请求的服务
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct {
}{}

// serverHandler 处理客户端发送的请求，包括以下三步
// readRequest读取请求
// handleRequest处理请求
// sendResponse回复请求
func (server *Server) serverHandler(f serialize.Coder, timeout time.Duration) {
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
		go server.handleRequest(f, req, sending, wg, timeout)
	}
	wg.Wait()
	_ = f.Close()
}

// readRequestHeader 读取请求消息的头部信息
func (server *Server) readRequestHeader(c serialize.Coder) (*serialize.Header, error) {
	var h serialize.Header
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
func (server *Server) readRequest(f serialize.Coder) (*request, error) {
	h, err := server.readRequestHeader(f)
	if err != nil {
		return nil, err
	}

	req := &request{h: h}
	log.Println("rpc server: read request header: client request:", h.ServiceMethod)
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.reply = req.mtype.newReply()

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

func (server *Server) sendResponse(f serialize.Coder, h *serialize.Header, body interface{},
	sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	log.Printf("rpc server: sending response: %s\n", body)
	if err := f.Write(h, body); err != nil {
		log.Println("rpc server: write response error: ", err)
	}
}

func (server *Server) handleRequest(f serialize.Coder, req *request, sending *sync.Mutex,
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
