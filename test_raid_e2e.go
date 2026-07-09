//go:build ignore

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
	"time"

	"github.com/coder/websocket"
)

const (
	jwtSecret = "hero-quest-secret-key"
	serverURL = "ws://localhost:8088/ws"
	playerID  = uint64(200010)
)

const (
	MsgLogin               uint16 = 1001
	MsgLoginResp           uint16 = 1002
	MsgRaidEnter           uint16 = 2401
	MsgRaidEnterResp       uint16 = 2402
	MsgRaidLeave           uint16 = 2403
	MsgRaidLeaveResp       uint16 = 2404
	MsgRaidLootOpen        uint16 = 2409
	MsgRaidLootOpenResp    uint16 = 2410
	MsgRaidLootPickup      uint16 = 2411
	MsgRaidLootPickupResp  uint16 = 2412
	MsgRaidExtract         uint16 = 2406
	MsgRaidExtractResp     uint16 = 2407
	MsgRaidMapList         uint16 = 2421
	MsgRaidMapListResp     uint16 = 2422
	MsgRaidTimer           uint16 = 2417
	MsgRaidDeath           uint16 = 2416
	MsgRaidExtractProgress uint16 = 2408
	MsgMove                uint16 = 1501
)

func main() {
	fmt.Println("=== 搜打撤模式 E2E 测试 ===")
	fmt.Printf("目标服务器: %s\n\n", serverURL)
	passed := 0
	failed := 0

	check := func(name string, ok bool, detail string) {
		if ok {
			fmt.Printf("  ✓ %s\n", name)
			if detail != "" {
				fmt.Printf("    %s\n", detail)
			}
			passed++
		} else {
			fmt.Printf("  ✗ %s — %s\n", name, detail)
			failed++
		}
	}

	// 1. Connect
	fmt.Println("[1] WebSocket 连接")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	check("连接成功", true, "")

	// 2. Login
	fmt.Println("[2] 登录")
	token := generateJWT(playerID)
	sendJSON(conn, MsgLogin, map[string]string{"token": token})
	msgID, body := recvFrame(conn)
	var loginResp struct {
		Code   uint32 `json:"code"`
		Player *struct {
			Name  string `json:"name"`
			Level int32  `json:"level"`
		} `json:"player"`
	}
	json.Unmarshal(body, &loginResp)
	check("登录成功", msgID == MsgLoginResp && loginResp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d name=%s level=%d", msgID, loginResp.Code, loginResp.Player.Name, loginResp.Player.Level))

	// 3. Query raid map list
	fmt.Println("[3] 查询战局地图列表")
	sendJSON(conn, MsgRaidMapList, map[string]interface{}{})
	msgID, body = recvFrame(conn)
	var mapListResp struct {
		Code uint32 `json:"code"`
		Maps []struct {
			TemplateID int32  `json:"template_id"`
			Name       string `json:"name"`
			Duration   int64  `json:"duration"`
			ZoneCount  int32  `json:"zone_count"`
			LootTier   int32  `json:"loot_tier"`
		} `json:"maps"`
	}
	json.Unmarshal(body, &mapListResp)
	check("地图列表", msgID == MsgRaidMapListResp && mapListResp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d maps=%d", msgID, mapListResp.Code, len(mapListResp.Maps)))
	for _, m := range mapListResp.Maps {
		fmt.Printf("    地图 %d: %s (时长%ds, %d区域, 掉落T%d)\n",
			m.TemplateID, m.Name, m.Duration, m.ZoneCount, m.LootTier)
	}

	// 4. Enter raid (map 1 = 翠绿密林)
	fmt.Println("[4] 进入战局（翠绿密林）")
	sendJSON(conn, MsgRaidEnter, map[string]int32{"map_id": 1})
	msgID, body = recvFrame(conn)
	var raidResp struct {
		Code    uint32 `json:"code"`
		MapID   uint64 `json:"map_id"`
		MapName string `json:"map_name"`
		Duration int64 `json:"duration"`
		Monsters []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
			Hp   int64  `json:"hp"`
		} `json:"monsters"`
		Zones []struct {
			ID         int32  `json:"id"`
			Name       string `json:"name"`
			PvPEnabled bool   `json:"pvp_enabled"`
		} `json:"zones"`
		ExtractionPoints []struct {
			ID              int32   `json:"id"`
			X               float64 `json:"x"`
			Y               float64 `json:"y"`
			ExtractDuration int32   `json:"extract_duration"`
		} `json:"extraction_points"`
		LootContainers []struct {
			ID uint64  `json:"id"`
			X  float64 `json:"x"`
			Y  float64 `json:"y"`
		} `json:"loot_containers"`
	}
	json.Unmarshal(body, &raidResp)
	check("进入战局", msgID == MsgRaidEnterResp && raidResp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d map=%s duration=%ds",
			msgID, raidResp.Code, raidResp.MapName, raidResp.Duration))
	fmt.Printf("    怪物: %d, 区域: %d, 撤离点: %d, 容器: %d\n",
		len(raidResp.Monsters), len(raidResp.Zones),
		len(raidResp.ExtractionPoints), len(raidResp.LootContainers))
	for _, z := range raidResp.Zones {
		fmt.Printf("    区域 %d: %s (PvP=%v)\n", z.ID, z.Name, z.PvPEnabled)
	}

	// 5. Try to enter again (should fail - already in raid)
	fmt.Println("[5] 重复进入战局（应失败）")
	sendJSON(conn, MsgRaidEnter, map[string]int32{"map_id": 2})
	msgID, body = recvFrame(conn)
	var doubleResp struct{ Code uint32 `json:"code"` }
	json.Unmarshal(body, &doubleResp)
	check("重复进入被拒绝", msgID == MsgRaidEnterResp && doubleResp.Code != 0,
		fmt.Sprintf("msgID=%d code=%d", msgID, doubleResp.Code))

	// 6. Open a loot container
	if len(raidResp.LootContainers) > 0 {
		containerID := raidResp.LootContainers[0].ID
		fmt.Printf("[6] 打开战利品容器 %d\n", containerID)
		sendJSON(conn, MsgRaidLootOpen, map[string]uint64{"container_id": containerID})
		msgID, body = recvFrame(conn)
		var lootResp struct {
			Code  uint32 `json:"code"`
			Items []struct {
				Index   int32  `json:"index"`
				Name    string `json:"name"`
				Count   int32  `json:"count"`
				Quality int32  `json:"quality"`
			} `json:"items"`
		}
		json.Unmarshal(body, &lootResp)
		check("打开容器", msgID == MsgRaidLootOpenResp && lootResp.Code == 0,
			fmt.Sprintf("msgID=%d code=%d items=%d", msgID, lootResp.Code, len(lootResp.Items)))
		for _, item := range lootResp.Items {
			fmt.Printf("    #%d %s x%d (品质%d)\n", item.Index, item.Name, item.Count, item.Quality)
		}

		// 7. Pick up first item
		if len(lootResp.Items) > 0 {
			fmt.Println("[7] 拾取第一个物品")
			sendJSON(conn, MsgRaidLootPickup, map[string]int32{"item_index": 0})
			msgID, body = recvFrame(conn)
			var pickupResp struct{ Code uint32 `json:"code"` }
			json.Unmarshal(body, &pickupResp)
			check("拾取物品", msgID == MsgRaidLootPickupResp && pickupResp.Code == 0,
				fmt.Sprintf("msgID=%d code=%d", msgID, pickupResp.Code))
		}
	}

	// 8. Move to extraction point then attempt extraction
	if len(raidResp.ExtractionPoints) > 0 {
		pointID := raidResp.ExtractionPoints[0].ID
		// Move player near extraction point (90, 90) for map 1
		fmt.Println("[8] 移动到撤离点附近")
		sendJSON(conn, MsgMove, map[string]float64{"x": 90.0, "y": 90.0})
		time.Sleep(200 * time.Millisecond) // let server process the move

		fmt.Printf("[9] 尝试撤离 (撤离点 %d)\n", pointID)
		sendJSON(conn, MsgRaidExtract, map[string]int32{"point_id": pointID})
		msgID, body = recvFrame(conn)
		var extractResp struct {
			Code  uint32 `json:"code"`
			Timer int32  `json:"timer"`
		}
		json.Unmarshal(body, &extractResp)
		check("开始撤离", msgID == MsgRaidExtractResp && extractResp.Code == 0,
			fmt.Sprintf("msgID=%d code=%d timer=%ds", msgID, extractResp.Code, extractResp.Timer))
	}

	// 10. Leave raid (abandon)
	fmt.Println("[10] 离开战局（放弃）")
	sendJSON(conn, MsgRaidLeave, map[string]interface{}{})
	msgID, body = recvFrame(conn)
	var leaveResp struct{ Code uint32 `json:"code"` }
	json.Unmarshal(body, &leaveResp)
	check("离开战局", msgID == MsgRaidLeaveResp && leaveResp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d", msgID, leaveResp.Code))

	// 11. Re-enter raid (烈焰矿洞)
	fmt.Println("[11] 再次进入战局（烈焰矿洞）")
	sendJSON(conn, MsgRaidEnter, map[string]int32{"map_id": 2})
	msgID, body = recvFrame(conn)
	var raid2Resp struct {
		Code    uint32 `json:"code"`
		MapName string `json:"map_name"`
	}
	json.Unmarshal(body, &raid2Resp)
	check("进入烈焰矿洞", msgID == MsgRaidEnterResp && raid2Resp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d map=%s", msgID, raid2Resp.Code, raid2Resp.MapName))

	// 12. Leave again
	fmt.Println("[12] 离开第二次战局")
	sendJSON(conn, MsgRaidLeave, map[string]interface{}{})
	msgID, body = recvFrame(conn)
	var leave2Resp struct{ Code uint32 `json:"code"` }
	json.Unmarshal(body, &leave2Resp)
	check("离开第二次战局", msgID == MsgRaidLeaveResp && leave2Resp.Code == 0,
		fmt.Sprintf("msgID=%d code=%d", msgID, leave2Resp.Code))

	// Summary
	fmt.Printf("\n=== 测试结果: %d 通过, %d 失败 ===\n", passed, failed)
	if failed > 0 {
		fmt.Println("❌ 存在失败的测试")
	} else {
		fmt.Println("✅ 全部通过")
	}
}

func generateJWT(pid uint64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"player_id":%d,"exp":%d}`,
		pid, time.Now().Add(24*time.Hour).Unix())))
	sig := hmac.New(sha256.New, []byte(jwtSecret))
	sig.Write([]byte(header + "." + payload))
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig.Sum(nil))
}

func sendJSON(conn *websocket.Conn, msgID uint16, v interface{}) {
	body, _ := json.Marshal(v)
	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(frame[0:2], uint16(2+len(body)))
	binary.BigEndian.PutUint16(frame[2:4], msgID)
	copy(frame[4:], body)
	ctx := context.Background()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		log.Fatalf("发送 msgID=%d 失败: %v", msgID, err)
	}
}

func recvFrame(conn *websocket.Conn) (uint16, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		log.Fatalf("接收失败: %v", err)
	}
	if len(data) < 4 {
		log.Fatalf("帧太短: %d bytes", len(data))
	}
	msgID := binary.BigEndian.Uint16(data[2:4])
	return msgID, data[4:]
}
