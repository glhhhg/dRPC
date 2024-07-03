package option

import "time"

// Option 字段定义dRPC协议中的一些重要指标
// MagicNumber 用来标识这是一个dRPC请求
// ConnectionTimeout 指定建立连接时的超时的时间限制
// HandlerTimeout 指定处理服务调用时的超时的时间限制
type Option struct {
	MagicNumber       int
	ConnectionTimeout time.Duration
	HandlerTimeout    time.Duration
}

const MagicNumber = 0x3bef5c

// DefaultOption 设置默认请求序号和默认超时时间为10s
var DefaultOption = &Option{
	MagicNumber:       MagicNumber,
	ConnectionTimeout: 10 * time.Second,
	HandlerTimeout:    10 * time.Second,
}
