package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const (
	jwtSecret = "hero-quest-secret-key"
	serverURL = "ws://localhost:8088/ws"
	playerID  = uint64(200099)
)

func main() {
	fmt.Println("=== 最小连接存活测试 ===")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// 登录
	token := generateJWT(playerID)
	send(conn, 1001, map[string]string{"token": token})
	mid, body := recv(conn)
	fmt.Printf("LoginResp: MsgID=%d body=%s\n", mid, string(body))

	// 进入地下城
	send(conn, 1101, map[string]int{"layer": 1})
	mid, body = recv(conn)
	fmt.Printf("EnterDungeonResp: MsgID=%d\n", mid)

	// 每隔1秒发一个心跳，持续10秒，看连接是否存活
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		err := send(conn, 9002, map[string]interface{}{"timestamp": time.Now().UnixMilli()})
		if err != nil {
			fmt.Printf("[t=%ds] 连接已死: %v\n", i+1, err)
			return
		}
		// 尝试读取心跳响应（1秒超时）
		rctx, rcancel := context.WithTimeout(context.Background(), 1*time.Second)
		_, data, rerr := conn.Read(rctx)
		rcancel()
		if rerr != nil {
			fmt.Printf("[t=%ds] 心跳无响应: %v\n", i+1, rerr)
		} else {
			rmid, _ := decodeFrame(data)
			fmt.Printf("[t=%ds] 心跳响应 MsgID=%d ✓\n", i+1, rmid)
		}
	}
	fmt.Println("连接存活 10 秒 ✓")
}

func generateJWT(pid uint64) string {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	payload := b64([]byte(fmt.Sprintf(`{"player_id":%d,"exp":%d,"iat":%d}`, pid, now+86400, now)))
	input := header + "." + payload
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(input))
	return input + "." + b64(mac.Sum(nil))
}

func b64(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.TrimRight(s, "=")
}

func send(conn *websocket.Conn, msgID uint16, payload interface{}) error {
	body, _ := json.Marshal(payload)
	frame := encodeFrame(msgID, body)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageBinary, frame)
}

func recv(conn *websocket.Conn) (uint16, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		log.Fatalf("接收失败: %v", err)
	}
	return decodeFrame(data)
}

func decodeFrame(data []byte) (uint16, []byte) {
	length := binary.BigEndian.Uint16(data[0:2])
	msgID := binary.BigEndian.Uint16(data[2:4])
	bodyLen := length - 2
	if int(bodyLen) > len(data)-4 {
		bodyLen = uint16(len(data) - 4)
	}
	return msgID, data[4 : 4+bodyLen]
}

func encodeFrame(msgID uint16, body []byte) []byte {
	length := uint16(2 + len(body))
	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(frame[0:2], length)
	binary.BigEndian.PutUint16(frame[2:4], msgID)
	copy(frame[4:], body)
	return frame
}
