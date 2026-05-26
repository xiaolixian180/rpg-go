package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/eventbus"
	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/boss"
	"hero-quest/internal/service/combat"
	"hero-quest/internal/service/dungeon"
	"hero-quest/internal/service/equip"
	"hero-quest/internal/service/pet"
	"hero-quest/internal/service/player"
	"hero-quest/internal/service/pvp"
	"hero-quest/internal/service/rank"
	"hero-quest/internal/service/shop"
	"hero-quest/internal/service/skill"
	"hero-quest/internal/service/trade"
	"hero-quest/pkg/auth"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// Handler 消息处理器管理器
type Handler struct {
	world   service.World
	bus     *eventbus.Bus
	auth    *AuthHandler
	dungeon *DungeonHandler
	combat  *CombatHandler
	equip   *EquipHandler
	pvp     *PvpHandler
	pet     *PetHandler
	shop    *ShopHandler
	trade   *TradeHandler
	skill   *SkillHandler
	rank    *RankHandler
	attr    *AttrHandler
}

// New 创建消息处理器管理器实例
func New(
	world service.World,
	playerSvc player.PlayerService,
	dungeonSvc dungeon.DungeonService,
	combatSvc combat.CombatService,
	bossSvc boss.BossService,
	equipSvc equip.EquipService,
	pvpSvc pvp.PvpService,
	petSvc pet.PetService,
	shopSvc shop.ShopService,
	tradeSvc trade.TradeService,
	skillSvc skill.SkillService,
	rankSvc rank.RankService,
	bus *eventbus.Bus,
	jwtMgr *auth.JWTManager,
) *Handler {
	return &Handler{
		world:   world,
		bus:     bus,
		auth:    NewAuthHandler(world, playerSvc, jwtMgr),
		dungeon: NewDungeonHandler(world, dungeonSvc),
		combat:  NewCombatHandler(world, combatSvc, bossSvc, bus),
		equip:   NewEquipHandler(world, equipSvc),
		pvp:     NewPvpHandler(world, pvpSvc),
		pet:     NewPetHandler(petSvc),
		shop:    NewShopHandler(world, shopSvc),
		trade:   NewTradeHandler(world, tradeSvc),
		skill:   NewSkillHandler(world, skillSvc),
		rank:    NewRankHandler(rankSvc),
		attr:    NewAttrHandler(world, playerSvc),
	}
}

// Register 将所有消息ID与对应的处理函数注册到网关路由器
func (h *Handler) Register(router *gateway.Router) {
	router.Register(protocol.MsgIDLogin, h.auth.HandleLogin)
	router.Register(protocol.MsgIDCreatePlayer, h.auth.HandleCreatePlayer)

	router.Register(protocol.MsgIDEnterDungeon, h.dungeon.HandleEnterDungeon)
	router.Register(protocol.MsgIDLeaveDungeon, h.dungeon.HandleLeaveDungeon)
	router.Register(protocol.MsgIDLayerTeleport, h.dungeon.HandleLayerTeleport)

	router.Register(protocol.MsgIDAttack, h.combat.HandleAttack)
	router.Register(protocol.MsgIDSkillCast, h.combat.HandleSkillCast)
	router.Register(protocol.MsgIDCollectResource, h.combat.HandleCollectResource)

	router.Register(protocol.MsgIDEquipStrengthen, h.equip.HandleStrengthen)
	router.Register(protocol.MsgIDEquipEnchant, h.equip.HandleEnchant)
	router.Register(protocol.MsgIDEquipWear, h.equip.HandleWear)
	router.Register(protocol.MsgIDEquipUnload, h.equip.HandleUnload)
	router.Register(protocol.MsgIDForge, h.equip.HandleForge)

	router.Register(protocol.MsgIDPvpAttack, h.pvp.HandlePvpAttack)
	router.Register(protocol.MsgIDBountyHunt, h.pvp.HandleBountyHunt)
	router.Register(protocol.MsgIDRevenge, h.pvp.HandleRevenge)

	router.Register(protocol.MsgIDMove, h.dungeon.HandleMove)

	router.Register(protocol.MsgIDPetSummon, h.pet.HandleSummon)
	router.Register(protocol.MsgIDPetRecall, h.pet.HandleRecall)
	router.Register(protocol.MsgIDPetLevelUp, h.pet.HandleLevelUp)
	router.Register(protocol.MsgIDPetEvolve, h.pet.HandleEvolve)
	router.Register(protocol.MsgIDPetExplore, h.pet.HandleExplore)
	router.Register(protocol.MsgIDPetCompose, h.pet.HandleCompose)

	router.Register(protocol.MsgIDShopList, h.shop.HandleShopList)
	router.Register(protocol.MsgIDShopBuy, h.shop.HandleShopBuy)

	router.Register(protocol.MsgIDTradeList, h.trade.HandleTradeList)
	router.Register(protocol.MsgIDTradePublish, h.trade.HandleTradePublish)
	router.Register(protocol.MsgIDTradeBuy, h.trade.HandleTradeBuy)
	router.Register(protocol.MsgIDTradeCancel, h.trade.HandleTradeCancel)

	router.Register(protocol.MsgIDSkillLevelUp, h.skill.HandleSkillLevelUp)
	router.Register(protocol.MsgIDSkillReset, h.skill.HandleSkillReset)

	router.Register(protocol.MsgIDRankingList, h.rank.HandleRankingList)

	router.Register(protocol.MsgIDAttrAssign, h.attr.HandleAttrAssign)
}

// onlinePlayer 获取内存中的在线玩家
func onlinePlayer(world service.World, conn *gateway.Conn) *model.Player {
	p := world.GetOnlinePlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: errors.ErrNotLogin.Code})
		return nil
	}
	return p
}

// connCtx 从连接中提取追踪上下文
func connCtx(conn *gateway.Conn) context.Context {
	return conn.Context()
}

// handleReq 泛型请求处理，自动携带 trace context
func handleReq[T any](world service.World, conn *gateway.Conn, body []byte, fn func(ctx context.Context, player *model.Player, req T)) {
	var req T
	if err := json.Unmarshal(body, &req); err != nil {
		logger.TWarn(connCtx(conn), "请求参数解析失败", "err", err)
		return
	}
	player := onlinePlayer(world, conn)
	if player == nil {
		return
	}
	fn(connCtx(conn), player, req)
}
