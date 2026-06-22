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
	"go.mongodb.org/mongo-driver/v2/bson"
	mongoOpts "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	jwtSecret = "hero-quest-secret-key"
	serverURL = "ws://localhost:8088/ws"
	playerID  = uint64(200099)
	mongoURI  = "mongodb://root:hero_quest_mongo_2026@127.0.0.1:27017"
	mongoDB   = "hero_quest"
)

func main() {
	fmt.Println("=== 宠物召唤后连接存活测试 ===")

	// 预埋宠物
	petUID := insertPet()
	fmt.Printf("宠物 pet_uid=%d\n", petUID)

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
	recv(conn)
	fmt.Println("登录成功")

	// 进入地下城
	send(conn, 1101, map[string]int{"layer": 1})
	recv(conn)
	fmt.Println("进入地下城成功")

	// 召唤宠物
	fmt.Println("召唤宠物...")
	send(conn, 1601, map[string]interface{}{"pet_uid": petUID})
	mid, body := recv(conn)
	fmt.Printf("PetSummonResp: MsgID=%d body=%s\n", mid, string(body))

	// 发送移动消息
	fmt.Println("发送Move...")
	err = send(conn, 1501, map[string]interface{}{"x": 50.0, "y": 50.0})
	if err != nil {
		fmt.Printf("Move发送失败: %v\n", err)
		return
	}
	fmt.Println("Move发送成功")

	// 尝试读取 PlayerMove 广播
	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, data, rerr := conn.Read(rctx)
	rcancel()
	if rerr != nil {
		fmt.Printf("读取Move广播失败: %v\n", rerr)
	} else {
		rmid, rbody := decodeFrame(data)
		fmt.Printf("收到广播: MsgID=%d body=%s\n", rmid, string(rbody))
	}

	// 发送 AutoBattle
	fmt.Println("发送AutoBattle...")
	err = send(conn, 1211, map[string]interface{}{"enable": true})
	if err != nil {
		fmt.Printf("AutoBattle发送失败: %v\n", err)
		return
	}
	fmt.Println("AutoBattle发送成功")

	// 心跳检测，每秒一次
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		err := send(conn, 9002, map[string]interface{}{"timestamp": time.Now().UnixMilli()})
		if err != nil {
			fmt.Printf("[t=%ds] 连接已死: %v\n", i+1, err)
			return
		}
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

func insertPet() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _ := mongoOpts.Connect(options.Client().ApplyURI(mongoURI))
	defer client.Disconnect(ctx)
	db := client.Database(mongoDB)
	petsCol := db.Collection("player_pets")
	petsCol.DeleteMany(ctx, bson.D{{Key: "player_id", Value: playerID}})
	countersCol := db.Collection("counters")
	var counter struct{ Seq uint64 `bson:"seq"` }
	result := countersCol.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: "pet_uid"}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: uint64(1)}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
	if err := result.Decode(&counter); err != nil {
		counter.Seq = uint64(time.Now().UnixNano() % 1000000)
	}
	petUID := counter.Seq
	petsCol.InsertOne(ctx, bson.D{
		{Key: "player_id", Value: playerID}, {Key: "pet_uid", Value: petUID},
		{Key: "pet_id", Value: int32(100)}, {Key: "name", Value: "小火龙"},
		{Key: "level", Value: int32(1)}, {Key: "quality", Value: int32(0)},
		{Key: "exp", Value: int32(0)}, {Key: "type", Value: int32(0)},
		{Key: "skills", Value: []bson.D{}}, {Key: "exploring", Value: false},
		{Key: "attack", Value: int32(25)}, {Key: "defense", Value: int32(6)},
		{Key: "hp", Value: int32(115)},
	})
	return petUID
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
	if int(bodyLen) > len(data)-4 { bodyLen = uint16(len(data) - 4) }
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
