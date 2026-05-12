// Package service - 游戏管理器
// GameManager 是薄协调层，组合所有 service，持有在线玩家和地下城的内存状态，
// 提供 OnLogin/OnLogout 生命周期管理，以及 RegisterHandlers 消息路由注册。
// GameManager 知道 gateway.Conn 的存在（仅用于消息发送），但所有业务逻辑委托给各 service。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 游戏管理器 ====================

// GameManager 游戏管理器（薄协调层），组合所有 service，
// 持有在线玩家 map、地下城 map、Boss map 的内存状态。
// 业务逻辑由各 service 实现，GameManager 只负责：
//   - 生命周期管理（OnLogin/OnLogout）
//   - 内存状态维护（players/dungeons/bosses）
//   - 消息反序列化 → 调用 service → 协议序列化 → 发送响应
type GameManager struct {
	// 组合的所有 service
	playerSvc  PlayerService  // 环家服务
	dungeonSvc DungeonService // 地下城服务
	combatSvc  CombatService  // 战斗服务
	bossSvc    BossService    // Boss服务
	equipSvc   EquipService   // 装备服务
	pvpSvc     PvpService     // PvP服务
	petSvc     PetService     // 宠物服务
	shopSvc    ShopService    // 商店服务
	tradeSvc   TradeService   // 交易行服务
	skillSvc   SkillService   // 技能服务
	rankSvc    RankService    // 排行榜服务

	// 内存状态
	players  map[uint64]*model.Player    // 在线玩家映射表，key为玩家ID
	dungeons map[int32]*model.DungeonLayer // 地下城实例映射表，key为层数
	bosses   map[uint64]*model.Boss      // 当前存活的Boss映射表，key为Boss实例ID

	// 配置参数
	maxLayer         int32   // 地下城最大层数
	pvpPenalty       float64 // PvP击杀金币掠夺比例
	pvpHonor         int32   // PvP击杀荣誉值奖励
	redNameThreshold int32   // 红名杀戮值阈值
	skillResetCost   int64   // 技能重置金币消耗

	// 网关消息中心（仅用于消息发送）
	hub *gateway.Hub

	// 并发保护
	mu sync.RWMutex // 保护 players/dungeons/bosses 的读写锁
}

// NewGameManager 创建并初始化游戏管理器实例
// 参数：
//   - playerSvc ~ rankSvc: 各业务 service 实例
//   - hub: 网关消息中心
//   - maxLayer: 地下城最大层数
//   - pvpPenalty: PvP金币掠夺比例
//   - pvpHonor: PvP击杀荣誉值奖励
//   - redNameThreshold: 红名杀戮值阈值
//   - skillResetCost: 技能重置金币消耗
func NewGameManager(
	playerSvc PlayerService,
	dungeonSvc DungeonService,
	combatSvc CombatService,
	bossSvc BossService,
	equipSvc EquipService,
	pvpSvc PvpService,
	petSvc PetService,
	shopSvc ShopService,
	tradeSvc TradeService,
	skillSvc SkillService,
	rankSvc RankService,
	hub *gateway.Hub,
	maxLayer int32,
	pvpPenalty float64,
	pvpHonor int32,
	redNameThreshold int32,
	skillResetCost int64,
) *GameManager {
	gm := &GameManager{
		playerSvc:        playerSvc,
		dungeonSvc:       dungeonSvc,
		combatSvc:        combatSvc,
		bossSvc:          bossSvc,
		equipSvc:         equipSvc,
		pvpSvc:           pvpSvc,
		petSvc:           petSvc,
		shopSvc:          shopSvc,
		tradeSvc:         tradeSvc,
		skillSvc:         skillSvc,
		rankSvc:          rankSvc,
		players:          make(map[uint64]*model.Player),
		dungeons:         make(map[int32]*model.DungeonLayer),
		bosses:           make(map[uint64]*model.Boss),
		hub:              hub,
		maxLayer:         maxLayer,
		pvpPenalty:       pvpPenalty,
		pvpHonor:         pvpHonor,
		redNameThreshold: redNameThreshold,
		skillResetCost:   skillResetCost,
	}
	gm.initDungeons()
	return gm
}

// initDungeons 初始化所有地下城层实例
// 从第1层到maxLayer层逐一创建DungeonLayer实例
// 每隔10层标记为Boss层
func (gm *GameManager) initDungeons() {
	for i := int32(1); i <= gm.maxLayer; i++ {
		gm.dungeons[i] = &model.DungeonLayer{
			Layer:     i,
			IsBoss:    model.IsBossLayer(i),
			Monsters:  make(map[uint64]*model.Monster),
			Players:   make(map[uint64]*model.Player),
			Resources: make(map[uint64]*model.Resource),
		}
	}
}

// ==================== 生命周期管理 ====================

// OnLogin 处理玩家登录，是玩家进入游戏的核心入口
// 流程：
//  1. 调用 PlayerService.Login 加载玩家数据
//  2. 检查内存中是否已存在（断线重连场景）
//  3. 将玩家加入内存映射表
func (gm *GameManager) OnLogin(playerID uint64) (*model.Player, error) {
	ctx := context.Background()

	gm.mu.Lock()
	defer gm.mu.Unlock()

	// 玩家已在内存中（断线重连场景），直接标记在线
	if p, ok := gm.players[playerID]; ok {
		p.Online = true
		return p, nil
	}

	// 调用玩家服务加载玩家数据
	p, gameErr := gm.playerSvc.Login(ctx, playerID)
	if gameErr != nil {
		return nil, fmt.Errorf("player login: %s", gameErr.Error())
	}

	// 将玩家加入内存映射表
	gm.players[playerID] = p
	return p, nil
}

// OnLogout 处理玩家登出，是玩家离开游戏的清理入口
// 流程：
//  1. 若玩家在地下城中，从对应层的玩家表中移除
//  2. 调用 PlayerService.Logout 保存数据并清理缓存
//  3. 从内存映射表中删除玩家对象
func (gm *GameManager) OnLogout(playerID uint64) {
	ctx := context.Background()

	gm.mu.Lock()
	defer gm.mu.Unlock()

	p, ok := gm.players[playerID]
	if !ok {
		return
	}

	// 从地下城层中移除玩家
	if p.Layer > 0 {
		if d, ok := gm.dungeons[p.Layer]; ok {
			d.Mu().Lock()
			delete(d.Players, playerID)
			d.Mu().Unlock()
		}
	}

	// 调用玩家服务执行登出逻辑（保存数据、清理缓存）
	gm.playerSvc.Logout(ctx, playerID)

	// 从内存映射表中删除玩家对象
	delete(gm.players, playerID)
}

// ==================== 内存状态查询 ====================

// GetPlayer 根据玩家ID获取内存中的在线玩家对象
func (gm *GameManager) GetPlayer(playerID uint64) *model.Player {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.players[playerID]
}

// GetDungeon 根据层数获取地下城层实例
func (gm *GameManager) GetDungeon(layer int32) *model.DungeonLayer {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.dungeons[layer]
}

// GetBoss 根据BossID获取Boss实例
func (gm *GameManager) GetBoss(bossID uint64) *model.Boss {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.bosses[bossID]
}

// AddBoss 添加Boss实例到内存
func (gm *GameManager) AddBoss(boss *model.Boss) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.bosses[boss.ID] = boss
}

// RemoveBoss 从内存中移除Boss实例
func (gm *GameManager) RemoveBoss(bossID uint64) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	delete(gm.bosses, bossID)
}

// AllPlayers 返回所有在线玩家（只读引用，调用方不应修改map结构）
func (gm *GameManager) AllPlayers() map[uint64]*model.Player {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.players
}

// ==================== 消息处理器 ====================

// HandleLogin 处理客户端登录请求消息
// 流程：反序列化 → OnLogin → 发送响应
func (gm *GameManager) HandleLogin(conn *gateway.Conn, body []byte) {
	var req protocol.C2SLogin
	if err := json.Unmarshal(body, &req); err != nil {
		logger.Warn("登录请求反序列化失败", "err", err)
		return
	}

	player, err := gm.OnLogin(conn.PlayerID)
	if err != nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: 1})
		return
	}

	conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{
		Code:   0,
		Player: playerToProto(player),
	})
}

// HandleEnterDungeon 处理玩家进入地下城请求
func (gm *GameManager) HandleEnterDungeon(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEnterDungeon
	if err := json.Unmarshal(body, &req); err != nil {
		logger.Warn("进入地下城请求反序列化失败", "err", err)
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: 1})
		return
	}

	// 调用地下城服务校验
	ctx := context.Background()
	_, gameErr := gm.dungeonSvc.Enter(ctx, p.ID, req.Layer, p.MaxLayer, gm.maxLayer)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: gameErr.Code})
		return
	}

	// 更新玩家当前所在层
	p.Layer = req.Layer

	// 将玩家加入目标层的玩家表
	dungeon := gm.dungeons[req.Layer]
	dungeon.Mu().Lock()
	dungeon.Players[p.ID] = p

	// 收集当前层所有怪物信息
	monsters := make([]protocol.MonsterData, 0)
	for _, m := range dungeon.Monsters {
		monsters = append(monsters, protocol.MonsterData{
			ID: m.ID, Name: m.Name, Hp: m.Hp, MaxHp: m.MaxHp, X: m.X, Y: m.Y,
		})
	}

	// 收集当前层所有玩家信息
	players := make([]protocol.PlayerBrief, 0)
	for _, pl := range dungeon.Players {
		players = append(players, protocol.PlayerBrief{
			ID: pl.ID, Name: pl.Name, Class: pl.Class, Level: pl.Level,
			Hp: pl.Hp, MaxHp: pl.MaxHp, X: pl.X, Y: pl.Y,
		})
	}

	// 收集当前层所有资源信息
	resources := make([]protocol.ResourceData, 0)
	for _, r := range dungeon.Resources {
		resources = append(resources, protocol.ResourceData{
			ID: r.ID, Type: r.Type, Name: r.Name, X: r.X, Y: r.Y, Harvested: r.Harvested,
		})
	}
	dungeon.Mu().Unlock()

	conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{
		Code:     0,
		Layer:    req.Layer,
		Zone:     model.GetZoneByLayer(req.Layer),
		Monsters: monsters,
		Players:  players,
		Resources: resources,
	})
}

// HandleLeaveDungeon 处理玩家离开地下城请求
func (gm *GameManager) HandleLeaveDungeon(conn *gateway.Conn, body []byte) {
	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{Code: 1})
		return
	}

	ctx := context.Background()
	gameErr := gm.dungeonSvc.Leave(ctx, p)
	if gameErr != nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{Code: gameErr.Code})
		return
	}

	// 从地下城层中移除玩家
	gm.mu.Lock()
	if d, ok := gm.dungeons[p.Layer]; ok {
		d.Mu().Lock()
		delete(d.Players, p.ID)
		d.Mu().Unlock()
	}
	gm.mu.Unlock()

	conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{Code: 0})
}

// HandleAttack 处理玩家PvE攻击请求
func (gm *GameManager) HandleAttack(conn *gateway.Conn, body []byte) {
	var req protocol.C2SAttack
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()

	// 调用战斗服务计算伤害
	result, gameErr := gm.combatSvc.Attack(ctx, p, req.TargetID, req.SkillID)
	if gameErr != nil {
		return
	}

	// 查找目标：先在当前层怪物表中查找
	if dungeon, ok := gm.dungeons[p.Layer]; ok {
		dungeon.Mu().Lock()
		if m, ok := dungeon.Monsters[req.TargetID]; ok {
			m.Hp -= result.Damage
			result.CurrHp = m.Hp
			result.IsDead = m.Hp <= 0

			// 怪物死亡后从该层移除
			if result.IsDead {
				delete(dungeon.Monsters, req.TargetID)
				result.ExpGain = m.ExpReward
				result.GoldGain = m.GoldReward
			}
			dungeon.Mu().Unlock()

			conn.Send(protocol.MsgIDDamage, &protocol.S2CDamage{
				TargetID: req.TargetID, Damage: result.Damage,
				CurrHp: result.CurrHp, IsDead: result.IsDead,
			})

			// 怪物死亡处理：奖励经验和金币
			if result.IsDead {
				p.Exp += result.ExpGain
				p.Gold += result.GoldGain
			}
			return
		}
		dungeon.Mu().Unlock()
	}

	// 当前层无匹配怪物，查找全局Boss表
	gm.mu.Lock()
	if boss, ok := gm.bosses[req.TargetID]; ok {
		boss.Mu().Lock()
		boss.Hp -= result.Damage
		result.CurrHp = boss.Hp
		result.IsDead = boss.Hp <= 0
		boss.Mu().Unlock()
		gm.mu.Unlock()

		conn.Send(protocol.MsgIDDamage, &protocol.S2CDamage{
			TargetID: req.TargetID, Damage: result.Damage,
			CurrHp: result.CurrHp, IsDead: result.IsDead,
		})

		// Boss死亡处理
		if result.IsDead {
			gm.handleBossDie(p, boss)
		}
		return
	}
	gm.mu.Unlock()
}

// handleBossDie 处理Boss死亡（调用BossService + 广播 + 移除Boss）
func (gm *GameManager) handleBossDie(killer *model.Player, boss *model.Boss) {
	ctx := context.Background()

	// 调用Boss服务处理死亡逻辑（掉落、通关、冷却）
	bossResult, gameErr := gm.bossSvc.OnDie(ctx, killer, boss)
	if gameErr != nil {
		logger.Error("Boss死亡处理失败", "boss_id", boss.ID, "err", gameErr)
		return
	}

	// 将DropItem转为协议格式
	protoDrops := make([]protocol.DropItem, len(bossResult.Drops))
	for i, d := range bossResult.Drops {
		protoDrops[i] = protocol.DropItem{
			ItemID: d.ItemID, Name: d.Name, Quality: d.Quality, Count: d.Count,
		}
	}

	// 广播Boss死亡消息
	gm.hub.Broadcast(protocol.MsgIDBossDie, &protocol.S2CBossDie{
		BossID: boss.ID, Drops: protoDrops,
	})

	// 稀有掉落全服播报
	if bossResult.Message != "" {
		gm.hub.Broadcast(protocol.MsgIDBroadcast, &protocol.S2CBroadcast{
			Type:    1,
			Content: bossResult.Message,
		})
	}

	// 从全局Boss表中移除已死亡的Boss实例
	gm.RemoveBoss(boss.ID)
}

// HandleEquipStrengthen 处理装备强化请求
func (gm *GameManager) HandleEquipStrengthen(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEquipStrengthen
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()

	// 调用装备服务执行强化逻辑
	result, gameErr := gm.equipSvc.Strengthen(ctx, p.ID, req.Slot)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{Code: gameErr.Code})
		return
	}

	// 扣减金币
	p.Gold -= result.CostGold

	conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{
		Code:     0,
		Slot:     result.Slot,
		NewLevel: result.NewLevel,
		CostGold: result.CostGold,
		IsSuccess: result.IsSuccess,
	})
}

// HandlePvpAttack 处理PvP攻击请求
func (gm *GameManager) HandlePvpAttack(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPvpAttack
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	attacker := gm.GetPlayer(conn.PlayerID)
	target := gm.GetPlayer(req.TargetID)
	if attacker == nil || target == nil {
		return
	}

	ctx := context.Background()

	// 调用PvP服务执行攻击逻辑
	result, gameErr := gm.pvpSvc.Attack(ctx, attacker, target, req.SkillID, gm.pvpPenalty, gm.pvpHonor, gm.redNameThreshold)
	if gameErr != nil {
		return
	}

	// 广播PvP战斗结果
	gm.hub.Broadcast(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
		AttackerID: result.AttackerID,
		TargetID:   result.TargetID,
		Damage:     result.Damage,
		GoldGain:   result.GoldGain,
		HonorGain:  result.HonorGain,
		IsDead:     result.IsDead,
	})

	// 红名播报
	if result.IsRedName {
		gm.hub.Broadcast(protocol.MsgIDBroadcast, &protocol.S2CBroadcast{
			Type:    2,
			Content: result.RedMsg,
		})
	}
}

// HandleMove 处理玩家移动请求
func (gm *GameManager) HandleMove(conn *gateway.Conn, body []byte) {
	var req protocol.C2SMove
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	// 更新玩家服务端坐标
	p.X = req.X
	p.Y = req.Y

	// 广播给同层其他玩家
	gm.hub.BroadcastToLayer(p.Layer, protocol.MsgIDPlayerMove, &protocol.S2CPlayerMove{
		PlayerID: p.ID, X: req.X, Y: req.Y,
	}, conn.ID)
}

// HandleSkillCast 处理技能释放请求
func (gm *GameManager) HandleSkillCast(conn *gateway.Conn, body []byte) {
	var req protocol.C2SSkillCast
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()

	// 调用战斗服务执行技能释放
	result, gameErr := gm.combatSvc.SkillCast(ctx, p, req.SkillID, req.TargetID, req.X, req.Y)
	if gameErr != nil {
		return
	}

	// 转换为协议格式
	protoTargets := make([]protocol.DamageInfo, len(result.Targets))
	for i, t := range result.Targets {
		protoTargets[i] = protocol.DamageInfo{
			TargetID: t.TargetID, Damage: t.Damage, CurrHp: t.CurrHp, IsDead: t.IsDead,
		}
	}

	conn.Send(protocol.MsgIDSkillEffect, &protocol.S2CSkillEffect{
		CasterID: result.CasterID,
		SkillID:  result.SkillID,
		Targets:  protoTargets,
		X:        result.X,
		Y:        result.Y,
	})
}

// HandleCollectResource 处理采集资源请求
func (gm *GameManager) HandleCollectResource(conn *gateway.Conn, body []byte) {
	var req protocol.C2SCollectResource
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	// 在当前层查找资源
	dungeon, ok := gm.dungeons[p.Layer]
	if !ok {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{Code: 1})
		return
	}

	dungeon.Mu().Lock()
	resource, ok := dungeon.Resources[req.ResourceID]
	if !ok {
		dungeon.Mu().Unlock()
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{Code: 1})
		return
	}

	ctx := context.Background()
	result, gameErr := gm.combatSvc.CollectResource(ctx, p.ID, resource)
	dungeon.Mu().Unlock()

	if gameErr != nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
		Code:       0,
		ResourceID: result.ResourceID,
		ItemID:     result.ItemID,
		ItemName:   result.ItemName,
		Count:      result.Count,
	})
}

// HandleEquipEnchant 处理装备附魔请求
func (gm *GameManager) HandleEquipEnchant(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEquipEnchant
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.equipSvc.Enchant(ctx, p.ID, req.Slot, req.MaterialID)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEquipEnchantResp, &protocol.S2CEquipEnchantResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDEquipEnchantResp, &protocol.S2CEquipEnchantResp{
		Code:     0,
		Slot:     result.Slot,
		AttrName: result.AttrName,
		AttrVal:  result.AttrVal,
	})
}

// HandleEquipWear 处理穿戴装备请求
func (gm *GameManager) HandleEquipWear(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEquipWear
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	gameErr := gm.equipSvc.Wear(ctx, p.ID, req.Slot, req.EquipID)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{Code: 0, Slot: req.Slot})
}

// HandleEquipUnload 处理卸下装备请求
func (gm *GameManager) HandleEquipUnload(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEquipUnload
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	gameErr := gm.equipSvc.Unload(ctx, p.ID, req.Slot)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEquipUnloadResp, &protocol.S2CEquipUnloadResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDEquipUnloadResp, &protocol.S2CEquipUnloadResp{Code: 0, Slot: req.Slot})
}

// HandleForge 处理锻造合成请求
func (gm *GameManager) HandleForge(conn *gateway.Conn, body []byte) {
	var req protocol.C2SForge
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.equipSvc.Forge(ctx, p.ID, req.RecipeID, req.Materials)
	if gameErr != nil {
		conn.Send(protocol.MsgIDForgeResp, &protocol.S2SForgeResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDForgeResp, &protocol.S2SForgeResp{
		Code:       0,
		ResultID:   result.ResultID,
		ResultName: result.ResultName,
		Quality:    result.Quality,
	})
}

// HandlePetSummon 处理召唤宠物请求
func (gm *GameManager) HandlePetSummon(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetSummon
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	pet, gameErr := gm.petSvc.Summon(ctx, p.ID, req.PetUID)
	if gameErr != nil {
		conn.Send(protocol.MsgIDPetSummonResp, &protocol.S2CPetSummonResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDPetSummonResp, &protocol.S2CPetSummonResp{
		Code: 0,
		Pet:  petToProto(pet),
	})
}

// HandlePetRecall 处理收回宠物请求
func (gm *GameManager) HandlePetRecall(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetRecall
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	gm.petSvc.Recall(ctx, p.ID, req.PetUID)
}

// HandlePetLevelUp 处理宠物升级请求
func (gm *GameManager) HandlePetLevelUp(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetLevelUp
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.petSvc.LevelUp(ctx, p.ID, req.PetUID, 100) // 宠物升级消耗100金币
	if gameErr != nil {
		conn.Send(protocol.MsgIDPetLevelUp, &protocol.S2CPetLevelUp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDPetLevelUp, &protocol.S2CPetLevelUp{
		Code:   0,
		PetUID: result.PetUID,
		Level:  result.Level,
	})
}

// HandlePetEvolve 处理宠物进阶请求
func (gm *GameManager) HandlePetEvolve(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetEvolve
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.petSvc.Evolve(ctx, p.ID, req.PetUID)
	if gameErr != nil {
		conn.Send(protocol.MsgIDPetEvolveResp, &protocol.S2CPetEvolveResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDPetEvolveResp, &protocol.S2CPetEvolveResp{
		Code:       0,
		PetUID:     result.PetUID,
		NewPetID:   result.NewPetID,
		NewQuality: result.NewQuality,
	})
}

// HandlePetExplore 处理宠物探险派遣请求
func (gm *GameManager) HandlePetExplore(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetExplore
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.petSvc.Explore(ctx, p.ID, req.PetUID, req.Duration)
	if gameErr != nil {
		conn.Send(protocol.MsgIDPetExploreResp, &protocol.S2CPetExploreResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDPetExploreResp, &protocol.S2CPetExploreResp{
		Code:    0,
		PetUID:  result.PetUID,
		EndTime: result.EndTime,
	})
}

// HandlePetCompose 处理宠物合成请求
func (gm *GameManager) HandlePetCompose(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPetCompose
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.petSvc.Compose(ctx, p.ID, req.PetUIDs)
	if gameErr != nil {
		conn.Send(protocol.MsgIDPetComposeResp, &protocol.S2CPetComposeResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDPetComposeResp, &protocol.S2CPetComposeResp{
		Code:     0,
		ResultID: result.ResultID,
		PetID:    result.PetID,
		Quality:  result.Quality,
	})
}

// HandleShopList 处理商店列表查询请求
func (gm *GameManager) HandleShopList(conn *gateway.Conn, body []byte) {
	var req protocol.C2SShopList
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	ctx := context.Background()
	items, gameErr := gm.shopSvc.List(ctx, req.Type)
	if gameErr != nil {
		conn.Send(protocol.MsgIDShopListResp, &protocol.S2SShopListResp{Code: gameErr.Code})
		return
	}

	// 转换为协议格式
	protoItems := make([]protocol.ShopItem, len(items))
	for i, item := range items {
		protoItems[i] = protocol.ShopItem{
			ID: item.ID, Name: item.Name, Price: item.Price,
			Stock: item.Stock, RequireLevel: item.RequireLevel,
		}
	}

	conn.Send(protocol.MsgIDShopListResp, &protocol.S2SShopListResp{
		Code:  0,
		Items: protoItems,
	})
}

// HandleShopBuy 处理商店购买请求
func (gm *GameManager) HandleShopBuy(conn *gateway.Conn, body []byte) {
	var req protocol.C2SShopBuy
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	_, gameErr := gm.shopSvc.Buy(ctx, p, req.ItemID, req.Count, 0) // 默认金币购买
	if gameErr != nil {
		conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2SShopBuyResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2SShopBuyResp{
		Code:   0,
		ItemID: req.ItemID,
		Count:  req.Count,
	})
}

// HandleTradeList 处理交易行列表查询请求
func (gm *GameManager) HandleTradeList(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeList
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.tradeSvc.List(ctx, req.Category, req.Page)
	if gameErr != nil {
		conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{Code: gameErr.Code})
		return
	}

	// 转换为协议格式
	protoItems := make([]protocol.TradeItem, len(result.Items))
	for i, order := range result.Items {
		protoItems[i] = protocol.TradeItem{
			OrderID:         order.ID,
			SellerID:        order.SellerID,
			EquipID:         order.EquipID,
			Quality:         order.Quality,
			StrengthenLevel: order.StrengthenLevel,
			Price:           order.Price,
		}
	}

	conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{
		Code:  0,
		Items: protoItems,
		Total: result.Total,
	})
}

// HandleTradePublish 处理交易行上架请求
func (gm *GameManager) HandleTradePublish(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradePublish
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.tradeSvc.Publish(ctx, p.ID, req.Slot, req.Price)
	if gameErr != nil {
		conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{
		Code:    0,
		OrderID: result.OrderID,
	})
}

// HandleTradeBuy 处理交易行购买请求
func (gm *GameManager) HandleTradeBuy(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeBuy
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	_, gameErr := gm.tradeSvc.Buy(ctx, p, req.OrderID)
	if gameErr != nil {
		conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{Code: 0, OrderID: req.OrderID})
}

// HandleTradeCancel 处理交易行取消上架请求
func (gm *GameManager) HandleTradeCancel(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeCancel
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	gameErr := gm.tradeSvc.Cancel(ctx, p.ID, req.OrderID)
	if gameErr != nil {
		logger.Warn("取消交易行上架失败", "player_id", p.ID, "order_id", req.OrderID, "err", gameErr)
	}
}

// HandleSkillLevelUp 处理技能升级请求
func (gm *GameManager) HandleSkillLevelUp(conn *gateway.Conn, body []byte) {
	var req protocol.C2SSkillLevelUp
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.skillSvc.LevelUp(ctx, p.ID, req.SkillID, p.AttrPoints)
	if gameErr != nil {
		conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2SSkillLevelUpResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2SSkillLevelUpResp{
		Code:     0,
		SkillID:  result.SkillID,
		NewLevel: result.NewLevel,
	})
}

// HandleSkillReset 处理技能重置请求
func (gm *GameManager) HandleSkillReset(conn *gateway.Conn, body []byte) {
	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()

	// 校验金币是否充足
	if p.Gold < gm.skillResetCost {
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{Code: errors.ErrGoldNotEnough.Code})
		return
	}

	// 扣减金币
	p.Gold -= gm.skillResetCost

	result, gameErr := gm.skillSvc.Reset(ctx, p.ID, gm.skillResetCost)
	if gameErr != nil {
		// 重置失败，退还金币
		p.Gold += gm.skillResetCost
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{
		Code:         0,
		RefundPoints: result.RefundPoints,
	})
}

// HandleAttrAssign 处理属性分配请求
func (gm *GameManager) HandleAttrAssign(conn *gateway.Conn, body []byte) {
	var req protocol.C2SAttrAssign
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	gameErr := gm.playerSvc.AssignAttr(ctx, p.ID, req.Attr, req.Val)
	if gameErr != nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
		Code:       0,
		Attr:       req.Attr,
		Val:        req.Val,
		AttrPoints: p.AttrPoints,
	})
}

// HandleRankingList 处理排行榜查询请求
func (gm *GameManager) HandleRankingList(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRankingList
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	ctx := context.Background()
	rankings, gameErr := gm.rankSvc.GetRanking(ctx, req.Type)
	if gameErr != nil {
		conn.Send(protocol.MsgIDRankingListResp, &protocol.S2CRankingListResp{Code: gameErr.Code})
		return
	}

	// 转换为协议格式
	protoRankings := make([]protocol.RankingItem, len(rankings))
	for i, r := range rankings {
		protoRankings[i] = protocol.RankingItem{
			Rank: r.Rank, PlayerID: r.PlayerID, Name: r.Name, Value: r.Value,
		}
	}

	conn.Send(protocol.MsgIDRankingListResp, &protocol.S2CRankingListResp{
		Code:     0,
		Type:     req.Type,
		Rankings: protoRankings,
	})
}

// HandlePvpBountyHunt 处理悬赏追杀请求
func (gm *GameManager) HandlePvpBountyHunt(conn *gateway.Conn, body []byte) {
	var req protocol.C2SBountyHunt
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	hunter := gm.GetPlayer(conn.PlayerID)
	target := gm.GetPlayer(req.TargetID)
	if hunter == nil || target == nil {
		return
	}

	ctx := context.Background()
	result, gameErr := gm.pvpSvc.BountyHunt(ctx, hunter, target)
	if gameErr != nil {
		return
	}

	conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{
		TargetID:  result.TargetID,
		GoldGain:  result.GoldGain,
		HonorGain: result.HonorGain,
	})
}

// HandlePvpRevenge 处理复仇请求
func (gm *GameManager) HandlePvpRevenge(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRevenge
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	// 简化：仇人列表由GameManager维护，此处传空map
	gameErr := gm.pvpSvc.Revenge(ctx, p.ID, req.TargetID, nil)
	if gameErr != nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{Code: gameErr.Code})
		return
	}

	conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{Code: 0, TargetID: req.TargetID})
}

// HandleLayerTeleport 处理层间传送请求
func (gm *GameManager) HandleLayerTeleport(conn *gateway.Conn, body []byte) {
	var req protocol.C2SLayerTeleport
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p := gm.GetPlayer(conn.PlayerID)
	if p == nil {
		return
	}

	ctx := context.Background()
	_, gameErr := gm.dungeonSvc.Teleport(ctx, p.ID, req.TargetLayer, p.MaxLayer, gm.maxLayer)
	if gameErr != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: gameErr.Code})
		return
	}

	// 从旧层移除，加入新层
	if p.Layer > 0 {
		if oldDungeon, ok := gm.dungeons[p.Layer]; ok {
			oldDungeon.Mu().Lock()
			delete(oldDungeon.Players, p.ID)
			oldDungeon.Mu().Unlock()
		}
	}

	p.Layer = req.TargetLayer
	if newDungeon, ok := gm.dungeons[req.TargetLayer]; ok {
		newDungeon.Mu().Lock()
		newDungeon.Players[p.ID] = p
		newDungeon.Mu().Unlock()
	}

	conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{
		Code:  0,
		Layer: req.TargetLayer,
		Zone:  model.GetZoneByLayer(req.TargetLayer),
	})
}

// ==================== 消息路由注册 ====================

// RegisterHandlers 注册所有游戏消息处理器到路由器
// 将各消息ID与对应的处理函数绑定，当网关收到对应消息时自动路由到处理函数
func (gm *GameManager) RegisterHandlers(router *gateway.Router) {
	// 登录模块
	router.Register(protocol.MsgIDLogin, gm.HandleLogin)

	// 地下城模块
	router.Register(protocol.MsgIDEnterDungeon, gm.HandleEnterDungeon)
	router.Register(protocol.MsgIDLeaveDungeon, gm.HandleLeaveDungeon)
	router.Register(protocol.MsgIDLayerTeleport, gm.HandleLayerTeleport)

	// 战斗模块
	router.Register(protocol.MsgIDAttack, gm.HandleAttack)
	router.Register(protocol.MsgIDSkillCast, gm.HandleSkillCast)
	router.Register(protocol.MsgIDCollectResource, gm.HandleCollectResource)

	// 装备模块
	router.Register(protocol.MsgIDEquipStrengthen, gm.HandleEquipStrengthen)
	router.Register(protocol.MsgIDEquipEnchant, gm.HandleEquipEnchant)
	router.Register(protocol.MsgIDEquipWear, gm.HandleEquipWear)
	router.Register(protocol.MsgIDEquipUnload, gm.HandleEquipUnload)
	router.Register(protocol.MsgIDForge, gm.HandleForge)

	// PvP模块
	router.Register(protocol.MsgIDPvpAttack, gm.HandlePvpAttack)
	router.Register(protocol.MsgIDBountyHunt, gm.HandlePvpBountyHunt)
	router.Register(protocol.MsgIDRevenge, gm.HandlePvpRevenge)

	// 移动模块
	router.Register(protocol.MsgIDMove, gm.HandleMove)

	// 宠物模块
	router.Register(protocol.MsgIDPetSummon, gm.HandlePetSummon)
	router.Register(protocol.MsgIDPetRecall, gm.HandlePetRecall)
	router.Register(protocol.MsgIDPetLevelUp, gm.HandlePetLevelUp)
	router.Register(protocol.MsgIDPetEvolve, gm.HandlePetEvolve)
	router.Register(protocol.MsgIDPetExplore, gm.HandlePetExplore)
	router.Register(protocol.MsgIDPetCompose, gm.HandlePetCompose)

	// 交易行模块
	router.Register(protocol.MsgIDTradeList, gm.HandleTradeList)
	router.Register(protocol.MsgIDTradePublish, gm.HandleTradePublish)
	router.Register(protocol.MsgIDTradeBuy, gm.HandleTradeBuy)
	router.Register(protocol.MsgIDTradeCancel, gm.HandleTradeCancel)

	// 商店模块
	router.Register(protocol.MsgIDShopList, gm.HandleShopList)
	router.Register(protocol.MsgIDShopBuy, gm.HandleShopBuy)

	// 技能模块
	router.Register(protocol.MsgIDSkillLevelUp, gm.HandleSkillLevelUp)
	router.Register(protocol.MsgIDSkillReset, gm.HandleSkillReset)

	// 属性模块
	router.Register(protocol.MsgIDAttrAssign, gm.HandleAttrAssign)

	// 排行榜模块
	router.Register(protocol.MsgIDRankingList, gm.HandleRankingList)
}

// ==================== 辅助方法 ====================

// playerToProto 将 model.Player 转为 protocol.PlayerData
func playerToProto(p *model.Player) protocol.PlayerData {
	return protocol.PlayerData{
		ID: p.ID, Name: p.Name, Class: p.Class, Level: p.Level,
		Exp: p.Exp, Gold: p.Gold, Honor: p.Honor, KillValue: p.KillValue,
		Str: p.Str, Agi: p.Agi, Int: p.Int, Con: p.Con,
		AttrPoints: p.AttrPoints, MaxLayer: p.MaxLayer,
	}
}

// petToProto 将 model.Pet 转为 protocol.PetData
func petToProto(p *model.Pet) protocol.PetData {
	return protocol.PetData{
		UID: p.UID, PetID: p.PetID, Name: p.Name,
		Level: p.Level, Quality: p.Quality, Type: p.Type, Skills: p.Skills,
	}
}