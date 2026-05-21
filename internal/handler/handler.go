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
) *Handler {
	return &Handler{
		world:   world,
		bus:     bus,
		auth:    NewAuthHandler(world, playerSvc),
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
	router.Register(1001, h.auth.HandleLogin)
	router.Register(1003, h.auth.HandleCreatePlayer)

	router.Register(1101, h.dungeon.HandleEnterDungeon)
	router.Register(1103, h.dungeon.HandleLeaveDungeon)
	router.Register(1107, h.dungeon.HandleLayerTeleport)

	router.Register(1201, h.combat.HandleAttack)
	router.Register(1205, h.combat.HandleSkillCast)
	router.Register(1209, h.combat.HandleCollectResource)

	router.Register(1301, h.equip.HandleStrengthen)
	router.Register(1303, h.equip.HandleEnchant)
	router.Register(1305, h.equip.HandleWear)
	router.Register(1307, h.equip.HandleUnload)
	router.Register(1309, h.equip.HandleForge)

	router.Register(1401, h.pvp.HandlePvpAttack)
	router.Register(1404, h.pvp.HandleBountyHunt)
	router.Register(1406, h.pvp.HandleRevenge)

	router.Register(1501, h.dungeon.HandleMove)

	router.Register(1601, h.pet.HandleSummon)
	router.Register(1603, h.pet.HandleRecall)
	router.Register(1604, h.pet.HandleLevelUp)
	router.Register(1605, h.pet.HandleEvolve)
	router.Register(1607, h.pet.HandleExplore)
	router.Register(1609, h.pet.HandleCompose)

	router.Register(1801, h.shop.HandleShopList)
	router.Register(1803, h.shop.HandleShopBuy)

	router.Register(1701, h.trade.HandleTradeList)
	router.Register(1703, h.trade.HandleTradePublish)
	router.Register(1705, h.trade.HandleTradeBuy)
	router.Register(1707, h.trade.HandleTradeCancel)

	router.Register(1901, h.skill.HandleSkillLevelUp)
	router.Register(1903, h.skill.HandleSkillReset)

	router.Register(2101, h.rank.HandleRankingList)

	router.Register(2001, h.attr.HandleAttrAssign)
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
