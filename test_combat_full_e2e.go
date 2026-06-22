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

// wsMsg 代表从 WebSocket 读取一条消息
type wsMsg struct {
	msgID uint16
	body  []byte
}

const (
	jwtSecret = "hero-quest-secret-key"
	serverURL = "ws://localhost:8088/ws"
	playerID  = uint64(200099)
	mongoURI  = "mongodb://root:hero_quest_mongo_2026@127.0.0.1:27017"
	mongoDB   = "hero_quest"
)

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
	MsgPlayerDie        uint16 = 1207
	MsgAutoBattle       uint16 = 1211
	MsgAutoBattleResp   uint16 = 1212
	MsgMove             uint16 = 1501
	MsgPetSummon        uint16 = 1601
	MsgPetSummonResp    uint16 = 1602
)

type testResult struct {
	name   string
	passed bool
	detail string
}

var results []testResult

func record(name string, passed bool, detail string) {
	results = append(results, testResult{name, passed, detail})
	if passed {
		fmt.Printf("  ✓ %s: %s\n", name, detail)
	} else {
		fmt.Printf("  ✗ %s: %s\n", name, detail)
	}
}

type damageMsg struct {
	TargetID uint64 `json:"target_id"`
	Damage   int64  `json:"damage"`
	PetDmg   int64  `json:"pet_damage"`
	PetCrit  bool   `json:"pet_crit"`
	PetDead  bool   `json:"pet_dead"`
	ExpGain  int64  `json:"exp_gain"`
	GoldGain int64  `json:"gold_gain"`
	CurrHP   int64  `json:"curr_hp"`
	IsDead   bool   `json:"is_dead"`
	LevelUp  bool   `json:"level_up"`
	NewLevel int32  `json:"new_level"`
}

// msgCh 是后台读取协程使用的消息通道
var msgCh = make(chan wsMsg, 256)

// startReader 启动后台 WebSocket 读取协程，所有消息都通过 msgCh 传递
func startReader(conn *websocket.Conn) {
	go func() {
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				fmt.Printf("  [reader] 连接读取结束: %v\n", err)
				close(msgCh)
				return
			}
			msgID, body := decodeFrame(data)
			msgCh <- wsMsg{msgID, body}
		}
	}()
}

// recvBlocking 阻塞等待一条消息（最多 10 秒）
func recvBlocking() (uint16, []byte) {
	select {
	case m, ok := <-msgCh:
		if !ok {
			log.Fatal("连接已关闭")
		}
		return m.msgID, m.body
	case <-time.After(10 * time.Second):
		log.Fatal("等待消息超时(10s)")
		return 0, nil
	}
}

// recvTimeout 在指定超时时间内等待一条消息，超时返回 error
func recvTimeout(d time.Duration) (uint16, []byte, error) {
	select {
	case m, ok := <-msgCh:
		if !ok {
			return 0, nil, fmt.Errorf("连接已关闭")
		}
		return m.msgID, m.body, nil
	case <-time.After(d):
		return 0, nil, fmt.Errorf("timeout")
	}
}

// drainAndCollect 在指定时间内持续读取消息，将怪物攻击消息收集到列表中
func drainAndCollect(label string, duration time.Duration, monsterAtkList *[]damageMsg) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining > 500*time.Millisecond {
			remaining = 500 * time.Millisecond
		}
		mid, bdy, err := recvTimeout(remaining)
		if err != nil {
			continue
		}
		if mid == MsgDamage {
			var dmg damageMsg
			json.Unmarshal(bdy, &dmg)
			if dmg.TargetID == playerID {
				*monsterAtkList = append(*monsterAtkList, dmg)
				fmt.Printf("    [怪物AI] damage=%d curr_hp=%d\n", dmg.Damage, dmg.CurrHP)
			}
		}
	}
}

func main() {
	fmt.Println("=== Hero Quest 战斗系统全面联调测试 ===")
	fmt.Printf("目标服务器: %s\n", serverURL)
	fmt.Printf("测试玩家ID: %d\n\n", playerID)

	// ── Step 0: 预埋宠物 ──
	fmt.Println("[准备] 预埋测试宠物到 MongoDB...")
	petUID := insertTestPet()
	fmt.Printf("  ✓ 宠物就绪 pet_uid=%d\n\n", petUID)

	// ── Step 1: WebSocket 连接 + 登录 ──
	fmt.Println("[1/6] 连接 & 登录")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// 启动后台读取协程（关键修复：避免 context 超时导致 coder/websocket 关闭连接）
	startReader(conn)

	token := generateJWT(playerID)
	sendJSON(conn, MsgLogin, map[string]string{"token": token})
	msgID, body := recvBlocking()
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
		fmt.Printf("  创建角色... ")
		sendJSON(conn, MsgCreatePlayer, map[string]interface{}{
			"token": token, "name": fmt.Sprintf("TestWarrior%d", time.Now().UnixNano()%100000), "class": 0,
		})
		msgID, body = recvBlocking()
		assertMsgID(msgID, MsgCreatePlayerResp, "CreatePlayerResp")
		var cr struct{ Code uint32 `json:"code"` }
		json.Unmarshal(body, &cr)
		if cr.Code != 0 {
			log.Fatalf("创建角色失败: code=%d", cr.Code)
		}
		fmt.Printf("✓\n")
	}
	record("登录", true, fmt.Sprintf("player=%s", loginResp.Player.Name))

	// ── Step 2: 进入地下城 ──
	fmt.Println("\n[2/6] 进入地下城 (layer=1)")
	sendJSON(conn, MsgEnterDungeon, map[string]int{"layer": 1})
	msgID, body = recvBlocking()
	assertMsgID(msgID, MsgEnterDungeonResp, "EnterDungeonResp")

	var dungeonResp struct {
		Code     uint32 `json:"code"`
		Layer    int    `json:"layer"`
		Zone     string `json:"zone"`
		Monsters []struct {
			ID    uint64  `json:"id"`
			Name  string  `json:"name"`
			HP    int64   `json:"hp"`
			MaxHP int64   `json:"max_hp"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
		} `json:"monsters"`
	}
	json.Unmarshal(body, &dungeonResp)
	if dungeonResp.Code != 0 {
		log.Fatalf("进入地下城失败: code=%d", dungeonResp.Code)
	}
	record("进入地下城", true, fmt.Sprintf("layer=%d zone=%s monsters=%d",
		dungeonResp.Layer, dungeonResp.Zone, len(dungeonResp.Monsters)))

	if len(dungeonResp.Monsters) == 0 {
		log.Fatal("没有怪物，无法继续测试")
	}
	targetID := dungeonResp.Monsters[0].ID
	fmt.Printf("  目标怪物: %s (ID=%d) HP=%d/%d pos=(%.1f,%.1f)\n",
		dungeonResp.Monsters[0].Name, targetID,
		dungeonResp.Monsters[0].HP, dungeonResp.Monsters[0].MaxHP,
		dungeonResp.Monsters[0].X, dungeonResp.Monsters[0].Y)

	// 立即移动到怪物附近（让怪物AI有时间检测玩家）
	mx, my := dungeonResp.Monsters[0].X, dungeonResp.Monsters[0].Y
	moveX, moveY := mx-3, my-3
	if moveX < 0 { moveX = mx + 3 }
	if moveY < 0 { moveY = my + 3 }
	fmt.Printf("  移动到怪物附近 (%.1f, %.1f) → 怪物在 (%.1f, %.1f)\n", moveX, moveY, mx, my)
	sendJSON(conn, MsgMove, map[string]interface{}{"x": moveX, "y": moveY})
	// 排空 PlayerMove 广播
	recvTimeout(1 * time.Second)

	// 全局怪物攻击收集（跨步骤累积）
	var monsterAtkList []damageMsg

	// ── Step 3: 普通攻击 ──
	fmt.Println("\n[3/6] 普通攻击测试")
	sendJSON(conn, MsgAttack, map[string]interface{}{"target_id": targetID, "skill_id": 0})
	msgID, body = recvBlocking()
	if msgID == MsgDamage {
		var dmg struct {
			Damage  int64 `json:"damage"`
			CurrHP  int64 `json:"curr_hp"`
			IsDead  bool  `json:"is_dead"`
			IsCrit  bool  `json:"is_crit"`
			IsDodge bool  `json:"is_dodge"`
		}
		json.Unmarshal(body, &dmg)
		record("普通攻击", dmg.Damage > 0 || dmg.IsDodge,
			fmt.Sprintf("damage=%d curr_hp=%d dead=%v crit=%v dodge=%v",
				dmg.Damage, dmg.CurrHP, dmg.IsDead, dmg.IsCrit, dmg.IsDodge))
	} else {
		record("普通攻击", false, fmt.Sprintf("unexpected MsgID=%d", msgID))
	}
	// 排空攻击后可能的怪物AI消息
	drainAndCollect("attack", 1*time.Second, &monsterAtkList)

	// ── Step 4: 技能释放 ──
	fmt.Println("\n[4/6] 技能释放测试 (战士技能ID=1)")
	sendJSON(conn, MsgSkillCast, map[string]interface{}{
		"skill_id": 1, "target_id": targetID, "x": 50.0, "y": 50.0,
	})
	msgID, body = recvBlocking()
	if msgID == MsgSkillEffect {
		var eff struct {
			SkillID int32 `json:"skill_id"`
			Targets []struct {
				Damage int64 `json:"damage"`
				IsDead bool  `json:"is_dead"`
			} `json:"targets"`
		}
		json.Unmarshal(body, &eff)
		totalDmg := int64(0)
		for _, t := range eff.Targets {
			totalDmg += t.Damage
		}
		record("技能释放", totalDmg > 0 || len(eff.Targets) > 0,
			fmt.Sprintf("skill=%d targets=%d total_damage=%d", eff.SkillID, len(eff.Targets), totalDmg))
	} else if msgID == MsgDamage {
		var dmg struct {
			Damage int64 `json:"damage"`
			IsDead bool  `json:"is_dead"`
		}
		json.Unmarshal(body, &dmg)
		record("技能释放", true, fmt.Sprintf("damage=%d dead=%v", dmg.Damage, dmg.IsDead))
	} else {
		record("技能释放", false, fmt.Sprintf("unexpected MsgID=%d", msgID))
	}
	// 排空技能后可能的怪物AI消息
	drainAndCollect("skill", 1*time.Second, &monsterAtkList)

	// ── Step 4.5: 怪物AI观察窗口 ──
	// 玩家已在 step 2 移动到怪物附近，怪物AI需要 ~3-5 秒从 Idle→Alert→Chase→Attack
	// 此处等待 5 秒，专门收集怪物AI的攻击推送
	fmt.Println("\n[4.5] 怪物AI观察窗口 (等待5秒，收集怪物攻击)...")
	fmt.Printf("  已累计收到 %d 条怪物攻击\n", len(monsterAtkList))
	drainAndCollect("monster_ai_watch", 5*time.Second, &monsterAtkList)
	fmt.Printf("  观察窗口结束，累计收到 %d 条怪物攻击\n", len(monsterAtkList))

	// ── Step 5: 召唤宠物 + 自动战斗 ──
	fmt.Println("\n[5/6] 召唤宠物 + 自动战斗 (含宠物AI协同 & 怪物AI)")

	// 5a. 召唤宠物
	sendJSON(conn, MsgPetSummon, map[string]interface{}{"pet_uid": petUID})
	msgID, body = recvBlocking()
	assertMsgID(msgID, MsgPetSummonResp, "PetSummonResp")
	var petResp struct {
		Code uint32 `json:"code"`
		Pet  *struct {
			Name  string `json:"name"`
			Level int32  `json:"level"`
			Type  int32  `json:"type"`
		} `json:"pet"`
	}
	json.Unmarshal(body, &petResp)
	petSummoned := petResp.Code == 0 && petResp.Pet != nil
	if petSummoned {
		record("宠物召唤", true, fmt.Sprintf("pet=%s lv=%d type=%d",
			petResp.Pet.Name, petResp.Pet.Level, petResp.Pet.Type))
	} else {
		record("宠物召唤", false, fmt.Sprintf("code=%d", petResp.Code))
	}

	// 5b. 开启自动战斗
	sendJSON(conn, MsgAutoBattle, map[string]interface{}{"enable": true})
	fmt.Println("  开启自动战斗，收集推送 (10s)...")

	var autoDmgList []damageMsg
	var healList []damageMsg
	abEnabled := false

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining > 2*time.Second {
			remaining = 2 * time.Second
		}
		mid, bdy, err := recvTimeout(remaining)
		if err != nil {
			continue
		}
		switch mid {
		case MsgAutoBattleResp:
			var ab struct {
				Code   uint32 `json:"code"`
				Enable bool   `json:"enable"`
			}
			json.Unmarshal(bdy, &ab)
			abEnabled = ab.Enable
		case MsgDamage:
			var dmg damageMsg
			json.Unmarshal(bdy, &dmg)
			if dmg.Damage < 0 {
				healList = append(healList, dmg)
			} else if dmg.TargetID == playerID {
				monsterAtkList = append(monsterAtkList, dmg)
			} else {
				autoDmgList = append(autoDmgList, dmg)
			}
		case MsgPlayerDie:
			fmt.Println("  ⚠ 玩家死亡")
		}
	}

	record("自动战斗开启", abEnabled, fmt.Sprintf("enable=%v", abEnabled))

	// 分析结果
	autoCount := len(autoDmgList)
	monsterCount := len(monsterAtkList)

	totalPetDmg := int64(0)
	for _, d := range autoDmgList {
		totalPetDmg += d.PetDmg
	}

	record("自动战斗伤害", autoCount > 0, fmt.Sprintf("收到 %d 条自动攻击推送", autoCount))
	for i, d := range autoDmgList {
		if i < 3 {
			fmt.Printf("    [auto#%d] target=%d damage=%d pet_dmg=%d exp=%d gold=%d\n",
				i+1, d.TargetID, d.Damage, d.PetDmg, d.ExpGain, d.GoldGain)
		}
	}

	if petSummoned {
		record("宠物AI协同伤害", totalPetDmg > 0, fmt.Sprintf("宠物总伤害=%d", totalPetDmg))
	}

	record("怪物AI攻击玩家", monsterCount > 0, fmt.Sprintf("收到 %d 条怪物攻击推送", monsterCount))
	for i, d := range monsterAtkList {
		if i < 3 {
			fmt.Printf("    [monster#%d] damage=%d curr_hp=%d dead=%v\n",
				i+1, d.Damage, d.CurrHP, d.IsDead)
		}
	}

	if len(healList) > 0 {
		record("辅助宠物治疗", true, fmt.Sprintf("收到 %d 条治疗推送", len(healList)))
	}

	// 关闭自动战斗（忽略连接已断开的情况）
	disableBody, _ := json.Marshal(map[string]interface{}{"enable": false})
	disableFrame := encodeFrame(MsgAutoBattle, disableBody)
	writeCtx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer writeCancel()
	conn.Write(writeCtx, websocket.MessageBinary, disableFrame)

	// ── Step 6: 汇总 ──
	fmt.Println("\n[6/6] 测试结果汇总")
	fmt.Println("───────────────────────────────────────")
	passed, failed := 0, 0
	for _, r := range results {
		if r.passed {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("  通过: %d  失败: %d  总计: %d\n", passed, failed, len(results))
	if failed > 0 {
		fmt.Println("\n  失败项:")
		for _, r := range results {
			if !r.passed {
				fmt.Printf("    ✗ %s: %s\n", r.name, r.detail)
			}
		}
	}
	fmt.Println("\n=== 战斗系统联调测试完成 ===")
}

// ── MongoDB ──
func insertTestPet() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongoOpts.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB 连接失败: %v", err)
	}
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
	petDoc := bson.D{
		{Key: "player_id", Value: playerID}, {Key: "pet_uid", Value: petUID},
		{Key: "pet_id", Value: int32(100)}, {Key: "name", Value: "小火龙"},
		{Key: "level", Value: int32(1)}, {Key: "quality", Value: int32(0)},
		{Key: "exp", Value: int32(0)}, {Key: "type", Value: int32(0)},
		{Key: "skills", Value: []bson.D{}}, {Key: "exploring", Value: false},
		{Key: "attack", Value: int32(25)}, {Key: "defense", Value: int32(6)},
		{Key: "hp", Value: int32(115)},
	}
	if _, err := petsCol.InsertOne(ctx, petDoc); err != nil {
		log.Fatalf("插入宠物失败: %v", err)
	}
	return petUID
}

// ── 工具函数 ──
func generateJWT(pid uint64) string {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	payload := base64URLEncode([]byte(fmt.Sprintf(`{"player_id":%d,"exp":%d,"iat":%d}`, pid, now+86400, now)))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64URLEncode(mac.Sum(nil))
}

func base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.TrimRight(s, "=")
}

func sendJSON(conn *websocket.Conn, msgID uint16, payload interface{}) {
	body, _ := json.Marshal(payload)
	frame := encodeFrame(msgID, body)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		log.Printf("发送失败 (MsgID=%d): %v", msgID, err)
	}
}

func decodeFrame(data []byte) (uint16, []byte) {
	if len(data) < 4 {
		log.Fatalf("frame too short: %d bytes", len(data))
	}
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

func assertMsgID(got, expected uint16, name string) {
	if got != expected {
		log.Printf("期望 %s (MsgID=%d)，收到 MsgID=%d", name, expected, got)
	}
}
