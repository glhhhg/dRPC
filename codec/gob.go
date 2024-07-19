/*
codec 包实现了RPC消息序列化与反序列化的，其中提供实现JSON与Gob两种实现
gob.go 实现了Codec接口，采用Gob序列化方式
conn 是由构建函数传入，通常是TCP socket，decode、encode使用gob模块中的方法
buffer 是带缓冲的Writer，防止输入阻塞
通过NewGobCodec 构造函数得到gob发放实现的序列化或者反序列化消息
*/

package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn   io.ReadWriteCloser
	buffer *bufio.Writer
	decode *gob.Decoder
	encode *gob.Encoder
}

func (g *GobCodec) Close() error {
	//TODO implement me
	return g.conn.Close()
}

func (g *GobCodec) ReadHeader(header *Header) error {
	//TODO implement me
	return g.decode.Decode(header)
}

func (g *GobCodec) ReadBody(i interface{}) error {
	//TODO implement me
	return g.decode.Decode(i)
}

func (g *GobCodec) Write(header *Header, i interface{}) error {
	//TODO implement me
	defer func() {
		err := g.buffer.Flush()
		if err != nil {
			_ = g.Close()
		}
	}()

	if err := g.encode.Encode(header); err != nil {
		log.Println("rpc codec: gob error encoding header: ", err)
		return err
	}
	if err := g.encode.Encode(i); err != nil {
		log.Println("rpc codec: gob error encoding body: ", err)
		return err
	}
	return nil
}

// NewGobCodec 构造函数
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn:   conn,
		buffer: buf,
		decode: gob.NewDecoder(conn),
		encode: gob.NewEncoder(buf),
	}
}
