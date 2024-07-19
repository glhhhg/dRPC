/*
服务端的功能进行真正地处理，通过分析完Header后处理客户端调用的 Method。通过使用反射reflect获取程序运行时
客户端请求的方法Method中的各类信息，包括方法名称、传入参数、返回值等等（使用reflect.Method类型）。
对 net/rpc 而言，一个函数需要能够被远程调用，需要满足如下五个条件：

	the method's type is exported.
	the method is exported.
	the method has two arguments, both exported (or builtin) types.
	the method's second argument is a pointer.
	the method has return type error.
service.go中的代码根据上述的五个条件来实现.
*/

package server

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

/*
methodType 结构体中：

	method 是指方法本身
	ArgType 是第一个参数的类型
	ReplyType 是第二个参数的类型
	numCalls 统计方法的调用次数
*/
type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64
}

func (m *methodType) NumCalls() uint64 {
	// 采用原子操作确保多线程的安全性，读取到的numCalls都是最新值
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value

	// argv 可能是指针或者值
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) NewReply() reflect.Value {
	// reply 必须是指针
	reply := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		reply.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		reply.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return reply
}

/*
service 结构体中：

	name 对应反射的结构体的名称，比如 WaitGroup 等等
	typ 对应反射的结构体的类型
	rcvr 反射的对象本身
	method 存储映射的结构体的所有符合条件的方法
*/
type service struct {
	name   string
	typ    reflect.Type
	rcvr   reflect.Value
	method map[string]*methodType
}

// newService 针对rcvr实例创建一个service实例，同时注册其满足条件的方法
func newService(rcvr interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	/*
		reflect.Indirect(s.rcvr)：这个函数用于获取reflect.Value的具体值。
		如果s.rcvr是一个指针，reflect.Indirect会返回它指向的值；
		如果它已经是一个值类型，reflect.Indirect直接返回这个值。
		简言之，这一步确保你得到的是一个值，而不是一个指针。
	*/
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.name) {
		/*
			ast.IsExported是ast包中的一个函数，用于判断一个标识符（例如变量名、函数名）是否是导出的。
			根据Go语言的约定，以大写字母开头的标识符是导出的，可以在包外访问；
			以小写字母开头的标识符是未导出的，仅在包内访问。
		*/
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethod()
	return s
}

/*
registerMethod 必须满足下面的两个条件：

	the method has two arguments, both exported (or builtin) types.
	the method has return type error.
*/
func (s *service) registerMethod() {
	s.method = make(map[string]*methodType)
	for i := range s.typ.NumMethod() {
		method := s.typ.Method(i)
		mType := method.Type

		// 传参为两个，包括自己的话就是三个，返回值只能是一个error
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

// isExportedOrBuiltinType 返回argType是否是已导出的或者是内建的
func isExportedOrBuiltinType(argType reflect.Type) bool {
	return ast.IsExported(argType.Name()) || argType.PkgPath() == ""
	// t.PkgPath()：返回定义类型的包路径。如果类型是内建类型或未命名类型，返回空字符串。
}

// call 通过反射值调用方法
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValue := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if err := returnValue[0].Interface(); err != nil {
		return err.(error)
	}
	return nil
}
