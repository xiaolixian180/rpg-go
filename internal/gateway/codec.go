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
	// length = 2(msgID) + bodyLen，需要扣除 msgID 的 2 字节得到实际 body 长度
	bodyLen := int(length) - 2
	if bodyLen < 0 || bodyLen > len(body) {
		return 0, nil, io.ErrUnexpectedEOF
	}
	body = body[:bodyLen]
	return msgID, body, nil
}
