package serialize

import "io"

// Header 消息头结构体
// ServiceMethod 调用服务和方法的名称，格式为：Service.Method
// Seq 客户端调用序号，用于区分不同的调用
// Error 错误消息
type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}

// Coder 消息序列化与反序列化的接口
// ReadHeader 用于读取消息的头部信息，并将读取的结果存储到给定的Header变量中
// ReadBody 用于读取消息的主体部分，并将读取的结果存储到给定的接口类型变量中
// Write 用于将消息的头部信息和主体部分写入到数据流中
// 读取或者写入消息时如果出错都会返回Error
type Coder interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}
