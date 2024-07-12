package serialize

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

// JSONCoder 实现Coder的所有接口
// conn 是由构建函数传入，通常是TCP socket
// decode 使用encoding/json模块中的Decoder解码
// encode 使用encoding/json模块中的Encoder编码
// buffer 是带缓冲的Writer，防止输入阻塞
type JSONCoder struct {
	conn   io.ReadWriteCloser
	buffer *bufio.Writer
	decode *json.Decoder
	encode *json.Encoder
}

var _ Coder = (*JSONCoder)(nil)

func NewJSONCoder(conn io.ReadWriteCloser) Coder {
	buffer := bufio.NewWriter(conn)
	return &JSONCoder{
		conn:   conn,
		buffer: buffer,
		decode: json.NewDecoder(conn),
		encode: json.NewEncoder(buffer),
	}
}

func (J JSONCoder) Close() error {
	//TODO implement me
	return J.conn.Close()
}

func (J JSONCoder) ReadHeader(header *Header) error {
	//TODO implement me
	return J.decode.Decode(header)
}

func (J JSONCoder) ReadBody(i interface{}) error {
	//TODO implement me
	return J.decode.Decode(i)
}

func (J JSONCoder) Write(header *Header, i interface{}) error {
	//TODO implement me
	defer func() {
		if err := J.buffer.Flush(); err != nil {
			_ = J.conn.Close()
		}
	}()

	if err := J.encode.Encode(header); err != nil {
		log.Println("dRPC coder: json encode header error:", err)
		return err
	}
	if err := J.encode.Encode(i); err != nil {
		log.Println("dRPC coder: json encode body error:", err)
		return err
	}
	return nil
}
