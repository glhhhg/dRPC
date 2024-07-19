/*
codec 包实现了RPC消息序列化与反序列化的，其中提供实现JSON与Gob两种实现
codec.go 提供了消息序列化与反序列化的接口，其内嵌一个io.Close接口，
方法ReadHeader 用于读取消息的头部信息，并将读取的结果存储到给定的‘Header变量中；
方法ReadBody 用于读取消息的主体部分，并将读取的结果存储到给定的接口类型变量中；
方法Write 用于将消息的头部信息和主体部分写入到数据流中。三者均包括错误信息error
gob.go 提供了Gob（Go binary）的序列化与反序列化方法，你可以根据自己的需求完成JSON方法的实现
*/

package codec

import "io"

// Header 消息头结构体
type Header struct {
	ServiceMethod string // 调用服务和方法的名称，格式为：Service.Method
	Seq           uint64 // 客户端调用序列，用于区分不同的调用
	Error         string // 错误消息
}

// Codec 消息序列化与反序列化的接口
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// NewCodecFunc Codec对象的构造函数
type NewCodecFunc func(writer io.ReadWriteCloser) Codec
type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

// NewCodecFuncMap 两种序列化方式的构造函数
var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
