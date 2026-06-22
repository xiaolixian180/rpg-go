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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	jwtSecret  = "hero-quest-secret-key"
	serverURL  = "ws://localhost:8088/ws"
	playerID   = uint64(200099)
	mongoURI   = "mongodb://root:hero_quest_mongo_2026@127.0.0.1:27017"
	mongoDB    = "hero_quest"
)

// ── Message IDs ──────────────────────────────────────────────
const (
	MsgLogin            uint16 = 1001
	MsgLoginResp        uint16 = 1002
	MsgCreatePlayer     uint16 = 1003
	MsgCreatePlayerResp uint16 = 1004
	MsgEnterDungeon     uint16 = 1101
	MsgEnterDungeonResp uint16 = 1102

	MsgAttack           uint16 = 1201
	MsgDamage           uint16 = 1202
	MsgSkillCast        uint16 = 1205
	MsgSkillEffect      uint16 = 1206
	MsgAutoBattle       uint16 = 1211
	MsgAutoBattleResp   uint16 = 1212

	MsgPetSummon        uint16 = 1601
	MsgPetSummonResp    uint16 = 1602
	MsgPetRecall        uint16 = 1603
	MsgPetRecallResp    uint16 = 1611
	MsgPetLevelUp       uint16 = 1604
)

func main() {
	fmt.Println("=== Hero Quest 战斗 + 宠物 联调测试 ===")
	fmt.Printf("目标服务器: %s\n\n", serverURL)

	// ── Step 0: 在 MongoDB 中预埋一只宠物 ──────────────────────
	fmt.Print("[0/7] 预埋宠物到 MongoDB... ")
	petUID := insertTestPet()
	fmt.Printf("✓ pet_uid=%d\n", petUID)

	// ── Step 1: WebSocket 连接 ────────────────────────────────
	fmt.Print("[1/7] WebSocket 连接... ")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	fmt.Println("✓")

	// ── Step 2: JWT + Login ───────────────────────────────────
	fmt.Print("[2/7] 登录... ")
	token := generateJWT(playerID)
	sendJSON(conn, MsgLogin, map[string]string{"token": token})
	msgID, body := recvFrame(conn)
	assertMsgID(msgID, MsgLoginResp, "LoginResp")

	var loginResp struct {
		Code   uint32 `json:"code"`
		Player *struct {
			ID    uint64 `json:"id"`
			Name  string `json:"name"`
			Level int    `json:"level"`
		} `json:"player"`
	}
	json.Unmarshal(body, &loginResp)

	if loginResp.Code != 0 {
		// 玩家不存在 → 创建
		fmt.Printf("code=%d, 创建角色... ", loginResp.Code)
		sendJSON(conn, MsgCreatePlayer, map[string]interface{}{
			"token": token, "name": fmt.Sprintf("Warrior%d", time.Now().UnixNano()%100000), "class": 0,
		})
		msgID, body = recvFrame(conn)
		assertMsgID(msgID, MsgCreatePlayerResp, "CreatePlayerResp")
		var cr struct{ Code uint32 `json:"code"` }
		json.Unmarshal(body, &cr)
		if cr.Code != 0 {
			log.Fatalf("创建角色失败: code=%d", cr.Code)
		}
		fmt.Printf("✓ code=%d\n", cr.Code)
	} else {
		fmt.Printf("✓ code=0 player=%s lv=%d\n", loginResp.Player.Name, loginResp.Player.Level)
	}

	// ── Step 3: 进入地下城 ────────────────────────────────────
	fmt.Print("[3/7] 进入地下城 (layer=1)... ")
	sendJSON(conn, MsgEnterDungeon, map[string]int{"layer": 1})
	msgID, body = recvFrame(conn)
	assertMsgID(msgID, MsgEnterDungeonResp, "EnterDungeonResp")

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
	if dungeonResp.Code != 0 {
		log.Fatalf("进入地下城失败: code=%d", dungeonResp.Code)
	}
	fmt.Printf("✓ layer=%d zone=%s monsters=%d\n", dungeonResp.Layer, dungeonResp.Zone, len(dungeonResp.Monsters))

	if len(dungeonResp.Monsters) == 0 {
		fmt.Println("       等待怪物刷新 (30s)...")
		time.Sleep(31 * time.Second)
		fmt.Println("       继续测试")
	}

	// 选取第一个怪物作为攻击目标
	var targetID uint64
	if len(dungeonResp.Monsters) > 0 {
		targetID = dungeonResp.Monsters[0].ID
		fmt.Printf("       攻击目标: %s (ID=%d) HP=%d/%d\n",
			dungeonResp.Monsters[0].Name, targetID,
			dungeonResp.Monsters[0].HP, dungeonResp.Monsters[0].MaxHP)
	}

	// ── Step 4: 普通攻击 ──────────────────────────────────────
	fmt.Print("[4/7] 普通攻击 (MsgID 1201)... ")
	if targetID > 0 {
		sendJSON(conn, MsgAttack, map[string]interface{}{
			"target_id": targetID,
			"skill_id":  0,
		})
		msgID, body = recvFrame(conn)
		assertMsgID(msgID, MsgDamage, "Damage")

		var dmg struct {
			TargetID uint64 `json:"target_id"`
			Damage   int64  `json:"damage"`
			CurrHP   int64  `json:"curr_hp"`
			IsDead   bool   `json:"is_dead"`
			ExpGain  int64  `json:"exp_gain"`
			GoldGain int64  `json:"gold_gain"`
			LevelUp  bool   `json:"level_up"`
			PetDmg   int64  `json:"pet_damage"`
		}
		json.Unmarshal(body, &dmg)
		fmt.Printf("✓ damage=%d curr_hp=%d dead=%v exp=%d gold=%d\n",
			dmg.Damage, dmg.CurrHP, dmg.IsDead, dmg.ExpGain, dmg.GoldGain)
	} else {
		fmt.Println("跳过 (无怪物)")
	}

	// ── Step 5: 技能释放 ──────────────────────────────────────
	fmt.Print("[5/7] 技能释放 (MsgID 1205, 战士技能ID=1)... ")
	if targetID > 0 {
		sendJSON(conn, MsgSkillCast, map[string]interface{}{
			"skill_id":  1,
			"target_id": targetID,
			"x":         50.0,
			"y":         50.0,
		})
		msgID, body = recvFrame(conn)

		if msgID == MsgSkillEffect {
			var eff struct {
				CasterID uint64 `json:"caster_id"`
				SkillID  int32  `json:"skill_id"`
				Targets  []struct {
					TargetID uint64 `json:"target_id"`
					Damage   int64  `json:"damage"`
					IsDead   bool   `json:"is_dead"`
				} `json:"targets"`
			}
			json.Unmarshal(body, &eff)
			totalDmg := int64(0)
			for _, t := range eff.Targets {
				totalDmg += t.Damage
			}
			fmt.Printf("✓ skill=%d targets=%d total_damage=%d\n",
				eff.SkillID, len(eff.Targets), totalDmg)
		} else if msgID == MsgDamage {
			// 技能也可能返回 Damage 格式
			var dmg struct {
				Damage int64 `json:"damage"`
				IsDead bool  `json:"is_dead"`
			}
			json.Unmarshal(body, &dmg)
			fmt.Printf("✓ (damage格式) damage=%d dead=%v\n", dmg.Damage, dmg.IsDead)
		} else {
			fmt.Printf("收到 MsgID=%d (body=%s)\n", msgID, string(body[:min(len(body), 200)]))
		}
	} else {
		fmt.Println("跳过 (无怪物)")
	}

	// ── Step 6: 召唤宠物 ──────────────────────────────────────
	fmt.Printf("[6/7] 召唤宠物 (MsgID 1601, pet_uid=%d)... ", petUID)
	sendJSON(conn, MsgPetSummon, map[string]interface{}{"pet_uid": petUID})
	msgID, body = recvFrame(conn)
	assertMsgID(msgID, MsgPetSummonResp, "PetSummonResp")

	var petResp struct {
		Code uint32 `json:"code"`
		Pet  *struct {
			UID     uint64 `json:"uid"`
			PetID   int32  `json:"pet_id"`
			Name    string `json:"name"`
			Level   int32  `json:"level"`
			Quality int32  `json:"quality"`
			Type    int32  `json:"type"`
		} `json:"pet"`
	}
	json.Unmarshal(body, &petResp)
	if petResp.Code == 0 && petResp.Pet != nil {
		fmt.Printf("✓ pet=%s lv=%d quality=%d type=%d\n",
			petResp.Pet.Name, petResp.Pet.Level, petResp.Pet.Quality, petResp.Pet.Type)
	} else {
		fmt.Printf("code=%d", petResp.Code)
		if petResp.Pet != nil {
			fmt.Printf(" pet=%s", petResp.Pet.Name)
		}
		fmt.Println()
	}

	// ── Step 7: 带宠物的自动战斗 ───────────────────────────────
	fmt.Print("[7/7] 开启自动战斗 (MsgID 1211)... ")
	sendJSON(conn, MsgAutoBattle, map[string]interface{}{"enable": true})
	msgID, body = recvFrame(conn)
	assertMsgID(msgID, MsgAutoBattleResp, "AutoBattleResp")

	var abResp struct {
		Code   uint32 `json:"code"`
		Enable bool   `json:"enable"`
	}
	json.Unmarshal(body, &abResp)
	if abResp.Code == 0 {
		fmt.Printf("✓ enable=%v\n", abResp.Enable)

		// 等待自动战斗 tick (每 2s 一次)
		fmt.Print("       等待自动战斗 tick (3s)... ")
		time.Sleep(3 * time.Second)

		// 尝试读取自动战斗推送的 Damage 消息
		dmgCount := 0
		for i := 0; i < 3; i++ {
			msgID, body, err := tryRecvFrame(conn, 1500*time.Millisecond)
			if err != nil {
				break
			}
			if msgID == MsgDamage {
				var dmg struct {
					Damage   int64 `json:"damage"`
					PetDmg   int64 `json:"pet_damage"`
					ExpGain  int64 `json:"exp_gain"`
					GoldGain int64 `json:"gold_gain"`
				}
				json.Unmarshal(body, &dmg)
				dmgCount++
				fmt.Printf("\n       [auto] damage=%d pet_damage=%d exp=%d gold=%d",
					dmg.Damage, dmg.PetDmg, dmg.ExpGain, dmg.GoldGain)
			}
		}
		fmt.Println()
		if dmgCount > 0 {
			fmt.Printf("       ✓ 收到 %d 条自动战斗伤害推送\n", dmgCount)
		} else {
			fmt.Println("       (未收到伤害推送，可能怪物已死亡)")
		}
	} else {
		fmt.Printf("code=%d\n", abResp.Code)
	}

	fmt.Println("\n=== 战斗 + 宠物联调测试完成 ===")
}

// ── MongoDB: 预埋测试宠物 ────────────────────────────────────
func insertTestPet() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB 连接失败: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(mongoDB)

	// 获取下一个 pet_uid
	countersCol := db.Collection("counters")
	var counter struct {
		Seq uint64 `bson:"seq"`
	}
	result := countersCol.FindOneAndUpdate(
		ctx,
		bson.D{{Key: "_id", Value: "pet_uid"}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: uint64(1)}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)
	if err := result.Decode(&counter); err != nil {
		counter.Seq = uint64(time.Now().UnixNano() % 1000000)
	}
	petUID := counter.Seq

	// 插入小火龙 (PetID=100, Type=0 Attack, Quality=0 White)
	petsCol := db.Collection("player_pets")
	petDoc := bson.D{
		{Key: "player_id", Value: playerID},
		{Key: "pet_uid", Value: petUID},
		{Key: "pet_id", Value: int32(100)},
		{Key: "name", Value: "小火龙"},
		{Key: "level", Value: int32(1)},
		{Key: "quality", Value: int32(0)},
		{Key: "exp", Value: int32(0)},
		{Key: "type", Value: int32(0)},
		{Key: "skills", Value: []bson.D{}},
		{Key: "exploring", Value: false},
		{Key: "attack", Value: int32(25)},
		{Key: "defense", Value: int32(6)},
		{Key: "hp", Value: int32(115)},
	}
	_, err = petsCol.InsertOne(ctx, petDoc)
	if err != nil {
		// 可能已存在，尝试查询
		var existing struct {
			PetUID uint64 `bson:"pet_uid"`
		}
		err2 := petsCol.FindOne(ctx, bson.D{
			{Key: "player_id", Value: playerID},
		}).Decode(&existing)
		if err2 == nil {
			return existing.PetUID
		}
		log.Fatalf("插入宠物失败: %v", err)
	}

	return petUID
}

// ── 工具函数 ─────────────────────────────────────────────────
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

func sendJSON(conn *websocket.Conn, msgID uint16, payload interface{}) {
	body, _ := json.Marshal(payload)
	frame := encodeFrame(msgID, body)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		log.Fatalf("发送失败 (MsgID=%d): %v", msgID, err)
	}
}

func recvFrame(conn *websocket.Conn) (uint16, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		log.Fatalf("接收失败: %v", err)
	}
	return decodeFrame(data)
}

func tryRecvFrame(conn *websocket.Conn, timeout time.Duration) (uint16, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return 0, nil, err
	}
	msgID, body := decodeFrame(data)
	return msgID, body, nil
}

func decodeFrame(data []byte) (uint16, []byte) {
	if len(data) < 4 {
		log.Fatalf("frame too short: %d bytes", len(data))
	}
	length := binary.BigEndian.Uint16(data[0:2])
	msgID := binary.BigEndian.Uint16(data[2:4])
	bodyLen := length - 2
	if len(data) < 4+int(bodyLen) {
		log.Fatalf("incomplete frame")
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

func assertMsgID(got, expected uint16, name string) {
	if got != expected {
		log.Fatalf("期望 %s (MsgID=%d)，收到 MsgID=%d", name, expected, got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
