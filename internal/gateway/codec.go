package gateway

import (
	"encoding/binary"
	"io"
)

// encodeMessage 将消息ID和消息体编码为二进制格式。
// 编码格式：[2字节长度(大端)][2字节消息ID(大端)][body]
// 长度字段值为 消息ID字节数(2) + body字节数。
func encodeMessage(msgID uint16, body []byte) []byte {
	length := uint16(2 + len(body))
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(buf[0:2], length)
	binary.BigEndian.PutUint16(buf[2:4], msgID)
	copy(buf[4:], body)
	return buf
}

// decodeMessage 将二进制数据解码为消息ID和消息体。
// 解码格式：[2字节长度(大端)][2字节消息ID(大端)][body]
// 数据不足4字节或长度字段与实际数据不符时返回 ErrUnexpectedEOF。
func decodeMessage(data []byte) (uint16, []byte, error) {
	if len(data) < 4 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	length := binary.BigEndian.Uint16(data[0:2])
	msgID := binary.BigEndian.Uint16(data[2:4])
	body := data[4:]
	if int(length) > len(body) {
		return 0, nil, io.ErrUnexpectedEOF
	}
	if int(length) < len(body) {
		body = body[:length]
	}
	return msgID, body, nil
}
