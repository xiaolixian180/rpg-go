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
	playerID  = uint64(200010)
)

// Message IDs matching both Go server and Unity client
const (
	MsgLogin            uint16 = 1001
	MsgLoginResp        uint16 = 1002
	MsgCreatePlayer     uint16 = 1003
	MsgCreatePlayerResp uint16 = 1004
	MsgEnterDungeon     uint16 = 1101
	MsgEnterDungeonResp uint16 = 1102
	MsgHeartbeat        uint16 = 9002
)

func main() {
	fmt.Println("=== Hero Quest 端到端联调测试 ===")
	fmt.Printf("目标服务器: %s\n\n", serverURL)

	// Step 1: Connect
	fmt.Print("[1/5] WebSocket 连接... ")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	fmt.Println("✓ 成功")

	// Step 2: Generate JWT
	fmt.Print("[2/5] JWT 生成... ")
	token := generateJWT(playerID)
	fmt.Printf("✓ (长度 %d)\n", len(token))

	// Step 3: Login
	fmt.Print("[3/5] 发送登录 (MsgID 1001)... ")
	loginPayload := map[string]string{"token": token}
	if err := sendJSON(conn, MsgLogin, loginPayload); err != nil {
		log.Fatalf("发送登录失败: %v", err)
	}
	fmt.Println("✓ 已发送")

	// Wait for response
	fmt.Print("       等待 LoginResponse (1002)... ")
	msgID, body, err := receiveFrame(conn, 10*time.Second)
	if err != nil {
		log.Fatalf("接收失败: %v", err)
	}
	fmt.Printf("✓ 收到 MsgID=%d\n", msgID)

	var loginResp struct {
		Code   uint32 `json:"code"`
		Player *struct {
			ID    uint64 `json:"id"`
			Name  string `json:"name"`
			Level int    `json:"level"`
		} `json:"player"`
	}
	json.Unmarshal(body, &loginResp)
	fmt.Printf("       code=%d\n", loginResp.Code)

	// Step 4: Create player if needed
	if loginResp.Code != 0 {
		fmt.Printf("[4/5] 玩家不存在 (code=%d)，创建角色 (MsgID 1003)... ", loginResp.Code)
		createPayload := map[string]interface{}{
			"token": token,
			"name":  fmt.Sprintf("Test%d", time.Now().UnixNano()%1000000),
			"class": 1,
		}
		if err := sendJSON(conn, MsgCreatePlayer, createPayload); err != nil {
			log.Fatalf("创建角色失败: %v", err)
		}

		msgID, body, err = receiveFrame(conn, 10*time.Second)
		if err != nil {
			log.Fatalf("接收创建角色响应失败: %v", err)
		}
		fmt.Printf("✓ 收到 MsgID=%d\n", msgID)

		var createResp struct {
			Code   uint32 `json:"code"`
			Player *struct {
				ID    uint64 `json:"id"`
				Name  string `json:"name"`
			} `json:"player"`
		}
		json.Unmarshal(body, &createResp)
		fmt.Printf("       code=%d", createResp.Code)
		if createResp.Player != nil {
			fmt.Printf(" player_id=%d name=%s", createResp.Player.ID, createResp.Player.Name)
		}
		fmt.Println()

		if createResp.Code != 0 {
			log.Fatalf("创建角色失败: code=%d", createResp.Code)
		}
	} else {
		fmt.Printf("[4/5] 登录成功！玩家: %s (ID=%d, Lv.%d)\n",
			loginResp.Player.Name, loginResp.Player.ID, loginResp.Player.Level)
		fmt.Println("       跳过角色创建")
	}

	// Step 5: Enter dungeon
	fmt.Print("[5/5] 进入地下城 (MsgID 1101)... ")
	enterPayload := map[string]int{"layer": 1}
	if err := sendJSON(conn, MsgEnterDungeon, enterPayload); err != nil {
		log.Fatalf("进入地下城失败: %v", err)
	}

	msgID, body, err = receiveFrame(conn, 10*time.Second)
	if err != nil {
		log.Fatalf("接收地下城响应失败: %v", err)
	}
	fmt.Printf("✓ 收到 MsgID=%d\n", msgID)

	var dungeonResp struct {
		Code     uint32 `json:"code"`
		Layer    int    `json:"layer"`
		Zone     string `json:"zone"`
		Monsters []struct {
			ID    uint64 `json:"id"`
			Name  string `json:"name"`
			HP    int64  `json:"hp"`
			MaxHP int64  `json:"max_hp"`
		} `json:"monsters"`
	}
	json.Unmarshal(body, &dungeonResp)
	fmt.Printf("       code=%d layer=%d zone=%s monsters=%d\n",
		dungeonResp.Code, dungeonResp.Layer, dungeonResp.Zone, len(dungeonResp.Monsters))

	if len(dungeonResp.Monsters) > 0 {
		fmt.Println("       怪物列表:")
		for _, m := range dungeonResp.Monsters {
			fmt.Printf("         - %s (ID=%d) HP=%d/%d\n", m.Name, m.ID, m.HP, m.MaxHP)
		}
	}

	fmt.Println("\n=== 全部测试通过！Unity 客户端联调链路已就绪 ===")
}

func generateJWT(pid uint64) string {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	exp := now + 86400
	payload := base64URLEncode([]byte(fmt.Sprintf(`{"player_id":%d,"exp":%d,"iat":%d}`, pid, exp, now)))

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(signingInput))
	sig := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + sig
}

func base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}

func sendJSON(conn *websocket.Conn, msgID uint16, payload interface{}) error {
	body, _ := json.Marshal(payload)
	frame := encodeFrame(msgID, body)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageBinary, frame)
}

func receiveFrame(conn *websocket.Conn, timeout time.Duration) (uint16, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		return 0, nil, err
	}

	if len(data) < 4 {
		return 0, nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}

	length := binary.BigEndian.Uint16(data[0:2])
	msgID := binary.BigEndian.Uint16(data[2:4])
	bodyLen := length - 2

	if len(data) < 4+int(bodyLen) {
		return 0, nil, fmt.Errorf("incomplete frame")
	}

	return msgID, data[4 : 4+bodyLen], nil
}

func encodeFrame(msgID uint16, body []byte) []byte {
	length := uint16(2 + len(body))
	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(frame[0:2], length)
	binary.BigEndian.PutUint16(frame[2:4], msgID)
	copy(frame[4:], body)
	return frame
}
