// Package handler 实现游戏消息处理层，负责消息反序列化、调用 service、序列化响应发送。
// Handler 不包含任何业务逻辑，仅作为网关层与业务层之间的桥梁。
//
// 核心设计：
//   - Handler 从 conn 提取 PlayerID，反序列化 body 为请求结构体
//   - 调用 service 方法，传入业务参数
//   - 根据 service 返回的 GameError 设置响应的 Code 字段
//   - 通过 conn.Send 发送响应
package handler

import (
	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/service"
)

// Handler 消息处理器管理器，组合所有子模块的 handler，
// 提供统一的 Register 方法将所有消息路由注册到网关路由器。
type Handler struct {
	auth    *AuthHandler    // 登录/创建角色处理器
	dungeon *DungeonHandler // 地下城处理器
	combat  *CombatHandler  // 战斗处理器
	equip   *EquipHandler   // 装备处理器
	pvp     *PvpHandler     // PvP处理器
	pet     *PetHandler     // 宠物处理器
	shop    *ShopHandler    // 商店处理器
	trade   *TradeHandler   // 交易行处理器
	skill   *SkillHandler   // 技能处理器
	rank    *RankHandler    // 排行榜处理器
	attr    *AttrHandler    // 属性分配处理器
}

// New 创建消息处理器管理器实例。
// 参数为各业务模块的 service 接口实现，handler 通过这些接口调用业务逻辑。
// pvpPenalty/pvpHonor/redNameThreshold 为 PvP 模块配置参数。
// resourceLookup 为资源查找回调，用于在场景中查找资源对象（由 GameManager 提供）。
// enemiesLookup 为仇人列表查找回调，用于获取玩家的仇人列表（由 GameManager 提供）。
func New(
	playerSvc service.PlayerService,
	dungeonSvc service.DungeonService,
	combatSvc service.CombatService,
	equipSvc service.EquipService,
	pvpSvc service.PvpService,
	petSvc service.PetService,
	shopSvc service.ShopService,
	tradeSvc service.TradeService,
	skillSvc service.SkillService,
	rankSvc service.RankService,
	pvpPenalty float64,
	pvpHonor int32,
	redNameThreshold int32,
	resourceLookup func(uint64) *model.Resource,
	enemiesLookup func(uint64) []uint64,
) *Handler {
	return &Handler{
		auth:    NewAuthHandler(playerSvc),
		dungeon: NewDungeonHandler(dungeonSvc, playerSvc),
		combat:  NewCombatHandler(combatSvc, playerSvc, resourceLookup),
		equip:   NewEquipHandler(equipSvc),
		pvp:     NewPvpHandler(pvpSvc, playerSvc, pvpPenalty, pvpHonor, redNameThreshold, enemiesLookup),
		pet:     NewPetHandler(petSvc),
		shop:    NewShopHandler(shopSvc, playerSvc),
		trade:   NewTradeHandler(tradeSvc, playerSvc),
		skill:   NewSkillHandler(skillSvc),
		rank:    NewRankHandler(rankSvc),
		attr:    NewAttrHandler(playerSvc),
	}
}

// Register 将所有消息ID与对应的处理函数注册到网关路由器。
// 调用此方法后，网关收到对应消息时会自动路由到相应的 handler 方法。
func (h *Handler) Register(router *gateway.Router) {
	// 登录/角色模块
	router.Register(1001, h.auth.HandleLogin)       // 登录请求
	router.Register(1003, h.auth.HandleCreatePlayer) // 创建角色

	// 地下城模块
	router.Register(1101, h.dungeon.HandleEnterDungeon)  // 进入地下城
	router.Register(1103, h.dungeon.HandleLeaveDungeon)  // 离开地下城
	router.Register(1107, h.dungeon.HandleLayerTeleport) // 层间传送

	// 战斗模块
	router.Register(1201, h.combat.HandleAttack)         // 普通攻击
	router.Register(1205, h.combat.HandleSkillCast)      // 释放技能
	router.Register(1209, h.combat.HandleCollectResource) // 采集资源

	// 装备模块
	router.Register(1301, h.equip.HandleStrengthen) // 装备强化
	router.Register(1303, h.equip.HandleEnchant)    // 装备附魔
	router.Register(1305, h.equip.HandleWear)       // 穿戴装备
	router.Register(1307, h.equip.HandleUnload)     // 卸下装备
	router.Register(1309, h.equip.HandleForge)      // 锻造合成

	// PvP模块
	router.Register(1401, h.pvp.HandlePvpAttack)  // PvP攻击
	router.Register(1404, h.pvp.HandleBountyHunt) // 悬赏追杀
	router.Register(1406, h.pvp.HandleRevenge)    // 复仇

	// 宠物模块
	router.Register(1601, h.pet.HandleSummon)  // 召唤宠物
	router.Register(1603, h.pet.HandleRecall)   // 收回宠物
	router.Register(1604, h.pet.HandleLevelUp)  // 宠物升级
	router.Register(1605, h.pet.HandleEvolve)   // 宠物进阶
	router.Register(1607, h.pet.HandleExplore)  // 宠物探险
	router.Register(1609, h.pet.HandleCompose)  // 宠物合成

	// 商店模块
	router.Register(1801, h.shop.HandleShopList) // 查询商店
	router.Register(1803, h.shop.HandleShopBuy)  // 购买商品

	// 交易行模块
	router.Register(1701, h.trade.HandleTradeList)    // 查询交易行
	router.Register(1703, h.trade.HandleTradePublish) // 上架商品
	router.Register(1705, h.trade.HandleTradeBuy)     // 购买商品
	router.Register(1707, h.trade.HandleTradeCancel)  // 取消上架

	// 技能模块
	router.Register(1901, h.skill.HandleSkillLevelUp) // 技能升级
	router.Register(1903, h.skill.HandleSkillReset)   // 技能重置

	// 排行榜模块
	router.Register(2101, h.rank.HandleRankingList) // 查询排行榜

	// 属性分配模块
	router.Register(2001, h.attr.HandleAttrAssign) // 分配属性点
}
