package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hero-quest/internal/cache"
	"hero-quest/internal/database"
	"hero-quest/internal/eventbus"
	"hero-quest/internal/gateway"
	"hero-quest/internal/handler"
	"hero-quest/internal/protocol"
	"hero-quest/internal/rbac"
	"hero-quest/internal/repo"
	"hero-quest/internal/scheduler"
	"hero-quest/internal/service"
	"hero-quest/internal/service/boss"
	"hero-quest/internal/service/chat"
	"hero-quest/internal/service/combat"
	"hero-quest/internal/service/dungeon"
	"hero-quest/internal/service/equip"
	"hero-quest/internal/service/pet"
	"hero-quest/internal/service/player"
	"hero-quest/internal/service/pvp"
	"hero-quest/internal/service/rank"
	"hero-quest/internal/service/shop"
	"hero-quest/internal/service/skill"
	"hero-quest/internal/service/team"
	"hero-quest/internal/service/trade"
	"hero-quest/pkg/auth"
	"hero-quest/pkg/config"
	"hero-quest/pkg/logger"
)

func main() {
	cfgPath := flag.String("c", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Printf("配置校验失败: %v\n", err)
		os.Exit(1)
	}

	logger.Init("info")
	logger.Info("hero-quest 服务器启动中...")

	// 数据库
	db, err := database.New(database.Config{
		Host: cfg.DB.Host, Port: cfg.DB.Port, User: cfg.DB.User,
		Password: cfg.DB.Password, DBName: cfg.DB.DBName,
		MaxIdle: cfg.DB.MaxIdle, MaxOpen: cfg.DB.MaxOpen,
	})
	if err != nil {
		logger.Error("连接数据库失败", "err", err)
		os.Exit(1)
	}
	if err := db.InitSchema(); err != nil {
		logger.Error("初始化数据库表结构失败", "err", err)
		os.Exit(1)
	}

	// RBAC
	enforcer, err := rbac.New(rbac.Config{
		Host: cfg.DB.Host, Port: cfg.DB.Port, User: cfg.DB.User,
		Password: cfg.DB.Password, DBName: cfg.DB.DBName,
	})
	if err != nil {
		logger.Error("初始化权限系统失败", "err", err)
		os.Exit(1)
	}
	if err := rbac.InitDefaultPolicies(enforcer); err != nil {
		logger.Error("初始化默认权限策略失败", "err", err)
		os.Exit(1)
	}

	// MongoDB
	mdb, err := database.NewMongo(cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		logger.Error("连接 MongoDB 失败", "err", err)
		os.Exit(1)
	}

	// Redis
	rdb, err := cache.New(cache.Config{
		Host: cfg.Redis.Host, Port: cfg.Redis.Port,
		Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	})
	if err != nil {
		logger.Error("连接 Redis 失败", "err", err)
		os.Exit(1)
	}

	// Repos
	playerRepo := repo.NewPlayerRepo(db)
	mysqlEquipRepo := repo.NewEquipRepo(db)                  // MySQL 原始实现，作为混合模式的底层
	equipRepo := repo.NewMongoEquipRepo(mysqlEquipRepo, mdb) // 混合：MySQL 主记录 + MongoDB 附魔
	petRepo := repo.NewMongoPetRepo(mdb)                     // 纯 MongoDB
	tradeRepo := repo.NewTradeRepo(db)
	shopRepo := repo.NewShopRepo(db)
	skillRepo := repo.NewMongoSkillRepo(mdb) // 纯 MongoDB
	cacheRepo := repo.NewCacheRepo(rdb)
	invRepo := repo.NewMongoInventoryRepo(mdb) // 纯 MongoDB

	// Services（不依赖 World 的服务）
	playerSvc := player.NewPlayerService(playerRepo, cacheRepo)
	equipSvc := equip.NewEquipService(equipRepo, invRepo)
	petSvc := pet.NewPetService(petRepo)
	shopSvc := shop.NewShopService(shopRepo, invRepo)
	skillSvc := skill.NewSkillService(skillRepo)
	rankSvc := rank.NewRankService(cacheRepo, playerRepo)

	// Gateway
	router := gateway.NewRouter()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	gw := gateway.New(addr, router)
	if cfg.Server.MaxRequestPerSec > 0 {
		gw.SetRateLimit(cfg.Server.MaxRequestPerSec)
	}

	// GameConfig（需在依赖配置的服务之前构建）
	gameCfg := service.GameConfig{
		MaxLayer:          int32(cfg.Game.MaxDungeonLayer),
		PvpGoldPenalty:    cfg.Game.PvpGoldPenalty,
		PvpHonorGain:      cfg.Game.PvpHonorGain,
		RedNameThreshold:  cfg.Game.RedNameThreshold,
		SkillResetCost:    cfg.Game.SkillResetCost,
		TeleportCost:      cfg.Game.TeleportCost,
		SkillMultiplier:   cfg.Game.SkillMultiplier,
		InvincibleSec:     cfg.Game.InvincibleSec,
		PetLevelUpCost:    cfg.Game.PetLevelUpCost,
		BountyGoldPerKill: cfg.Game.BountyGoldPerKill,
		BountyHonorGain:   cfg.Game.BountyHonorGain,
		BossDropPurple:    cfg.Game.BossDropPurple,
		BossDropOrange:    cfg.Game.BossDropOrange,
		PageSize:          cfg.Game.PageSize,
	}
	gm := service.NewGameManager(playerSvc, playerRepo, equipRepo, gw.Hub(), gameCfg)

	// 依赖 GameConfig 的服务
	bossSvc := boss.NewBossService(cacheRepo, playerRepo, gameCfg)
	pvpSvc := pvp.NewPvpService(enforcer, gameCfg)

	// 依赖 World 的服务（需在 GameManager 之后创建）
	tradeSvc := trade.NewTradeService(tradeRepo, equipRepo, playerRepo, gm)
	combatSvc := combat.NewCombatService(gm, playerSvc)
	dungeonSvc := dungeon.NewDungeonService(enforcer, gm, playerRepo)

	// 组队 / 聊天（无持久化，纯内存）
	teamSvc := team.NewTeamService()
	chatSvc := chat.NewChatService()

	// 事件总线 — 解耦 Boss 死亡等跨模块事件
	bus := eventbus.New()
	bus.Subscribe(eventbus.TopicBossDie, func(e eventbus.Event) {
		evt := e.(*eventbus.BossDieEvent)
		// 全服播报Boss被击杀
		gw.Hub().Broadcast(protocol.MsgIDBroadcast, &protocol.S2CBroadcast{
			Type: 1, Content: fmt.Sprintf("%s 击败了 %s！", evt.KillerName, evt.BossName),
		})
	})

	// 排行榜更新事件订阅
	bus.Subscribe(eventbus.TopicPlayerLogin, func(e eventbus.Event) {
		evt := e.(*eventbus.PlayerLoginEvent)
		ctx := context.Background()
		// 更新等级排行榜
		rankSvc.UpdateRanking(ctx, evt.PlayerID, 0, int64(evt.Level))
		// 更新战力排行榜（根据等级估算战力）
		rankSvc.UpdateRanking(ctx, evt.PlayerID, 1, int64(evt.Level)*100)
	})
	bus.Subscribe(eventbus.TopicLevelUp, func(e eventbus.Event) {
		evt := e.(*eventbus.LevelUpEvent)
		ctx := context.Background()
		rankSvc.UpdateRanking(ctx, evt.PlayerID, 0, int64(evt.NewLevel))
		rankSvc.UpdateRanking(ctx, evt.PlayerID, 1, int64(evt.NewLevel)*100)
	})
	bus.Subscribe(eventbus.TopicPvpKill, func(e eventbus.Event) {
		evt := e.(*eventbus.PvpKillEvent)
		ctx := context.Background()
		// 获取击杀者当前荣誉值更新排行榜
		if p := gm.GetOnlinePlayer(evt.KillerID); p != nil {
			p.Mu().RLock()
			honor := int64(p.Honor)
			p.Mu().RUnlock()
			rankSvc.UpdateRanking(ctx, evt.KillerID, 2, honor)
		}
	})

	// JWT 管理器：由登录/创建角色协议消息完成鉴权
	jwtMgr := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// Handler（消息处理层）
	h := handler.New(
		gm, // World 接口
		playerSvc, dungeonSvc, combatSvc, bossSvc, equipSvc,
		pvpSvc, petSvc, shopSvc, tradeSvc, skillSvc, rankSvc,
		teamSvc, chatSvc,
		bus,
		jwtMgr,
	)
	h.Register(router)
	logger.Info("消息路由注册完成")

	// 断线回调
	gw.Hub().SetCloseHandler(func(conn *gateway.Conn) {
		// 离线时自动从队伍中移除
		teamSvc.Cleanup(conn.PlayerID)
		gm.OnLogout(conn.PlayerID)
	})

	// 定时任务
	sched := scheduler.New()
	scheduler.RegisterMonsterRefreshTask(sched, gm)
	// 资源不自动刷新（产品文档3.4：资源被采集后不会自动刷新）
	// scheduler.RegisterResourceRefreshTask(sched, gm)
	scheduler.RegisterBossRefreshTask(sched, gm)
	scheduler.RegisterPetExploreTask(sched, gm, petRepo, playerRepo, invRepo)
	scheduler.RegisterAutoSaveTask(sched, gm)
	scheduler.RegisterAutoBattleTask(sched, gm, combatSvc, bus)
	scheduler.RegisterMonsterAITask(sched, gm)

	// 启动
	go func() {
		if err := gw.Run(); err != nil {
			logger.Error("网关运行出错", "err", err)
		}
	}()
	sched.Start()
	logger.Info("hero-quest 服务器已启动", "addr", addr)

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("hero-quest 服务器正在关闭...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := gw.Shutdown(ctx); err != nil {
		logger.Error("关闭网关失败", "err", err)
	} else {
		logger.Info("网关已关闭")
	}
	bus.Close()
	logger.Info("事件总线已关闭")
	sched.Stop()
	enforcer.Close()
	if err := rdb.Close(); err != nil {
		logger.Error("关闭 Redis 连接失败", "err", err)
	} else {
		logger.Info("Redis 连接已关闭")
	}
	if err := mdb.Close(); err != nil {
		logger.Error("关闭 MongoDB 连接失败", "err", err)
	} else {
		logger.Info("MongoDB 连接已关闭")
	}
	if err := db.Close(); err != nil {
		logger.Error("关闭数据库失败", "err", err)
	} else {
		logger.Info("数据库连接已关闭")
	}

	if ctx.Err() != nil {
		logger.Warn("优雅关闭超时（5秒），强制退出")
	} else {
		logger.Info("hero-quest 服务器已优雅关闭")
	}
}
