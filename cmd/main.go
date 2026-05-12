// main 包是 hero-quest 游戏服务器的入口程序
// 负责初始化配置、日志、数据库、缓存、网关等基础组件，
// 通过依赖注入组装所有 repo → service → handler 层，
// 启动游戏服务并监听退出信号以实现优雅关闭
package main

import (
	"context"
	"flag"      // 命令行参数解析
	"fmt"       // 格式化输出
	"os"        // 操作系统相关功能（退出码、信号等）
	"os/signal" // 系统信号监听
	"syscall"   // 系统调用常量（SIGINT、SIGTERM 等）
	"time"      // 时间相关功能（超时控制等）

	"hero-quest/internal/cache"    // Redis 缓存客户端
	"hero-quest/internal/database" // 数据库客户端
	"hero-quest/internal/gateway"  // 网络网关（WebSocket 路由、连接管理等）
	"hero-quest/internal/handler"  // 消息处理器管理器
	"hero-quest/internal/model"    // 游戏核心数据模型
	"hero-quest/internal/rbac"     // Casbin 权限管理
	"hero-quest/internal/repo"     // 数据访问层接口及实现
	"hero-quest/internal/scheduler" // 定时任务调度器
	"hero-quest/internal/service"  // 业务逻辑层（游戏管理器等）
	"hero-quest/pkg/config"        // 配置文件加载与校验
	"hero-quest/pkg/logger"        // 全局日志工具
)

// main 是程序主入口，按照以下顺序完成服务器启动：
//  1. 解析命令行参数，获取配置文件路径
//  2. 加载配置文件并校验合法性
//  3. 初始化日志系统（从配置读取日志级别）
//  4. 连接数据库并初始化表结构
//  5. 连接 Redis 缓存
//  6. 创建所有 repo 实例（注入数据库/Redis依赖）
//  7. 创建所有 service 实例（注入 repo 依赖）
//  8. 创建 Gateway + Router
//  9. 创建 GameManager（注入所有 service + hub + 配置参数）
// 10. 创建 Handler 管理器（注入所有 service + GameManager）
// 11. 注册所有消息路由（调用 handler.Register(router)）
// 12. 设置鉴权回调（临时实现：非空 token 返回 playerID=1）
// 13. 设置断线回调（调用 gm.OnLogout）
// 14. 创建 Scheduler，注册定时任务（怪物刷新、宠物探险结算）
// 15. 异步启动网关
// 16. 监听退出信号，优雅关闭（5秒超时）
func main() {
	// ========== 第1步：解析命令行参数 ==========
	// 定义 -c 命令行参数，用于指定配置文件路径，默认值为 configs/config.yaml
	cfgPath := flag.String("c", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	// ========== 第2步：加载配置文件并校验 ==========
	// 根据命令行指定的路径读取并解析 YAML 配置文件
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 校验配置项合法性，确保必填项非空、数值参数在合理范围内
	if err := cfg.Validate(); err != nil {
		fmt.Printf("配置校验失败: %v\n", err)
		os.Exit(1)
	}

	// ========== 第3步：初始化日志系统 ==========
	// 从配置中读取日志级别，初始化全局日志实例
	logLevel := "info" // 默认日志级别
	if cfg.Server.Host != "" {
		// 后续可从配置文件中增加日志级别字段，当前使用默认值
	}
	logger.Init(logLevel)
	logger.Info("hero-quest 服务器启动中...")

	// ========== 第4步：连接数据库 ==========
	// 使用配置文件中的数据库参数创建数据库连接实例
	db, err := database.New(database.Config{
		Host:     cfg.DB.Host,     // 数据库主机地址
		Port:     cfg.DB.Port,     // 数据库端口号
		User:     cfg.DB.User,     // 数据库用户名
		Password: cfg.DB.Password, // 数据库密码
		DBName:   cfg.DB.DBName,   // 数据库名称
		MaxIdle:  cfg.DB.MaxIdle,  // 最大空闲连接数
		MaxOpen:  cfg.DB.MaxOpen,  // 最大打开连接数
	})
	if err != nil {
		logger.Error("连接数据库失败", "err", err)
		os.Exit(1)
	}

	// 初始化数据库表结构（CREATE TABLE IF NOT EXISTS，幂等操作）
	if err := db.InitSchema(); err != nil {
		logger.Error("初始化数据库表结构失败", "err", err)
		os.Exit(1)
	}

	// ========== 第4.5步：创建 Casbin 权限执行器 ==========
	// 使用与业务库相同的数据库配置创建 Casbin 执行器
	// Casbin 通过 GORM adapter 将策略数据持久化到 casbin_rule 表
	enforcer, err := rbac.New(rbac.Config{
		Host:     cfg.DB.Host,     // 数据库主机地址
		Port:     cfg.DB.Port,     // 数据库端口号
		User:     cfg.DB.User,     // 数据库用户名
		Password: cfg.DB.Password, // 数据库密码
		DBName:   cfg.DB.DBName,   // 数据库名称
	})
	if err != nil {
		logger.Error("初始化权限系统失败", "err", err)
		os.Exit(1)
	}

	// 初始化默认权限策略（玩家、红名、GM、管理员等角色及其权限）
	// AddPolicy 是幂等操作，已存在的策略不会重复添加
	if err := rbac.InitDefaultPolicies(enforcer); err != nil {
		logger.Error("初始化默认权限策略失败", "err", err)
		os.Exit(1)
	}

	// ========== 第5步：连接 Redis 缓存 ==========
	// 使用配置文件中的 Redis 参数创建 Redis 客户端实例
	rdb, err := cache.New(cache.Config{
		Host:     cfg.Redis.Host,     // Redis 主机地址
		Port:     cfg.Redis.Port,     // Redis 端口号
		Password: cfg.Redis.Password, // Redis 密码
		DB:       cfg.Redis.DB,       // Redis 数据库编号
	})
	if err != nil {
		logger.Error("连接 Redis 失败", "err", err)
		os.Exit(1)
	}

	// ========== 第6步：创建所有 repo 实例 ==========
	// repo 是数据访问层，封装数据库/Redis 操作，service 通过接口依赖 repo
	playerRepo := repo.NewPlayerRepo(db)  // 玩家数据访问
	equipRepo := repo.NewEquipRepo(db)    // 装备数据访问
	petRepo := repo.NewPetRepo(db)        // 宠物数据访问
	tradeRepo := repo.NewTradeRepo(db)    // 交易行数据访问
	shopRepo := repo.NewShopRepo(db)      // 商店数据访问
	skillRepo := repo.NewSkillRepo(db)    // 技能数据访问
	cacheRepo := repo.NewCacheRepo(rdb)   // 缓存数据访问（玩家缓存、Boss冷却、排行榜）

	// ========== 第7步：创建所有 service 实例 ==========
	// service 是业务逻辑层，通过接口依赖 repo，不直接依赖数据库/缓存
	playerSvc := service.NewPlayerService(playerRepo, cacheRepo)  // 玩家服务（登录/登出/属性分配）
	dungeonSvc := service.NewDungeonService(enforcer)              // 地下城服务（进入/离开/传送，注入权限执行器）
	combatSvc := service.NewCombatService()                       // 战斗服务（攻击/技能/采集）
	bossSvc := service.NewBossService(cacheRepo, playerRepo)      // Boss服务（刷新/死亡/掉落）
	equipSvc := service.NewEquipService(equipRepo)                // 装备服务（强化/附魔/穿戴/锻造）
	pvpSvc := service.NewPvpService(enforcer)                     // PvP服务（攻击/悬赏/复仇，注入权限执行器）
	petSvc := service.NewPetService(petRepo)                      // 宠物服务（召唤/升级/探险/合成）
	shopSvc := service.NewShopService(shopRepo)                   // 商店服务（列表/购买）
	tradeSvc := service.NewTradeService(tradeRepo, equipRepo)    // 交易行服务（上架/购买/取消）
	skillSvc := service.NewSkillService(skillRepo)                // 技能服务（升级/重置）
	rankSvc := service.NewRankService(cacheRepo)                  // 排行榜服务（查询/更新）

	// ========== 第8步：创建 Gateway + Router ==========
	// 创建 WebSocket 消息路由器，用于分发不同类型的消息到对应处理器
	router := gateway.NewRouter()

	// 根据配置拼接网关监听地址（格式：host:port）
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// 创建网关实例，绑定监听地址和路由器
	gw := gateway.New(addr, router)

	// 设置限流参数：每个连接每秒最大消息数
	if cfg.Server.MaxRequestPerSec > 0 {
		gw.SetRateLimit(cfg.Server.MaxRequestPerSec)
	}

	// ========== 第9步：创建 GameManager ==========
	// GameManager 是核心协调层，组合所有 service，持有在线玩家和地下城的内存状态
	// 传入所有 service 实例 + hub + 游戏规则配置参数
	gm := service.NewGameManager(
		playerSvc, dungeonSvc, combatSvc, bossSvc, equipSvc,
		pvpSvc, petSvc, shopSvc, tradeSvc, skillSvc, rankSvc,
		gw.Hub(),                              // WebSocket连接中心
		int32(cfg.Game.MaxDungeonLayer),        // 副本最大层数
		cfg.Game.PvpGoldPenalty,                // PvP金币掠夺比例
		cfg.Game.PvpHonorGain,                  // PvP胜利荣誉奖励
		cfg.Game.RedNameThreshold,              // 红名阈值（恶意PK判定）
		cfg.Game.SkillResetCost,                // 技能重置金币消耗
	)

	// ========== 第10步：创建 Handler 管理器 ==========
	// Handler 管理器组合所有子模块的 handler，提供统一的 Register 方法
	// 传入所有 service 实例 + PvP配置参数 + 资源/仇人查找回调
	h := handler.New(
		playerSvc, dungeonSvc, combatSvc, equipSvc,
		pvpSvc, petSvc, shopSvc, tradeSvc, skillSvc, rankSvc,
		cfg.Game.PvpGoldPenalty,                // PvP金币掠夺比例（handler层用于响应）
		cfg.Game.PvpHonorGain,                  // PvP荣誉奖励（handler层用于响应）
		cfg.Game.RedNameThreshold,              // 红名阈值（handler层用于红名判定）
		makeResourceLookup(gm),                 // 资源查找回调（从GameManager内存状态中查找资源）
		makeEnemiesLookup(gm),                  // 仇人列表查找回调（从GameManager内存状态中查找仇人）
	)

	// ========== 第11步：注册所有消息路由 ==========
	// 将 Handler 管理器中的所有消息ID与处理函数注册到网关路由器
	// 调用此方法后，网关收到对应消息时会自动路由到相应的 handler 方法
	h.Register(router)
	logger.Info("消息路由注册完成")

	// ========== 第12步：设置鉴权回调 ==========
	// 当客户端建立 WebSocket 连接时，通过此回调验证 token 的合法性
	// 返回 (玩家ID, 是否合法)，合法则允许连接，非法则拒绝
	gw.Hub().SetAuthHandler(func(token string) (uint64, bool) {
		// TODO: 接入真实鉴权系统（当前为临时实现）
		// 如果 token 为空字符串，拒绝连接
		if token == "" {
			return 0, false
		}
		// 临时实现：任何非空 token 都视为合法，返回玩家 ID 为 1
		return 1, true
	})

	// ========== 第13步：设置断线回调 ==========
	// 当客户端 WebSocket 连接断开时触发
	// 调用游戏管理器的登出逻辑，清理玩家在线状态、保存数据等
	gw.Hub().SetCloseHandler(func(conn *gateway.Conn) {
		gm.OnLogout(conn.PlayerID)
	})

	// ========== 第14步：创建 Scheduler，注册定时任务 ==========
	// 创建定时任务调度器，管理所有周期性执行的后台任务
	sched := scheduler.New()

	// 注册怪物刷新定时任务：每30秒检查并刷新地下城中的怪物
	scheduler.RegisterMonsterRefreshTask(sched)

	// 注册宠物探险结算定时任务：每60秒检查已到期的探险并发放奖励
	scheduler.RegisterPetExploreTask(sched)

	// ========== 第15步：异步启动网关和调度器 ==========
	// 在新的 goroutine 中启动网关，开始监听并接受客户端连接
	// 使用 goroutine 是为了不阻塞主线程，主线程需继续监听退出信号
	go func() {
		if err := gw.Run(); err != nil {
			logger.Error("网关运行出错", "err", err)
			os.Exit(1)
		}
	}()

	// 启动定时任务调度器
	sched.Start()

	// 网关已成功启动，输出监听地址
	logger.Info("hero-quest 服务器已启动", "addr", addr)

	// ========== 第16步：监听退出信号，优雅关闭 ==========
	// 创建带缓冲的信号通道，用于接收系统信号
	quit := make(chan os.Signal, 1)
	// 注册监听 SIGINT（Ctrl+C）和 SIGTERM（kill 命令）信号
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞等待退出信号，收到信号后程序继续执行关闭流程
	<-quit

	// 收到退出信号，开始优雅关闭
	logger.Info("hero-quest 服务器正在关闭...")

	// 创建5秒超时的上下文，所有关闭操作必须在5秒内完成
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 优雅关闭顺序：停止网关 → 停止调度器 → 关闭 Redis → 关闭数据库
	// 按依赖关系逆序关闭，先停掉对外服务，再释放内部资源

	// 停止网关服务，在超时时间内等待所有连接处理完成
	if err := gw.Shutdown(ctx); err != nil {
		logger.Error("关闭网关失败", "err", err)
	} else {
		logger.Info("网关已关闭")
	}

	// 停止定时任务调度器（等待所有任务协程退出）
	sched.Stop()

	// 关闭 Redis 连接，释放缓存资源
	if err := rdb.Close(); err != nil {
		logger.Error("关闭 Redis 连接失败", "err", err)
	} else {
		logger.Info("Redis 连接已关闭")
	}

	// 关闭数据库连接，释放数据库资源
	if err := db.Close(); err != nil {
		logger.Error("关闭数据库连接失败", "err", err)
	} else {
		logger.Info("数据库连接已关闭")
	}

	// 检查超时上下文是否已过期
	if ctx.Err() != nil {
		logger.Warn("优雅关闭超时（5秒），强制退出")
	} else {
		logger.Info("hero-quest 服务器已优雅关闭")
	}
}

// ==================== 辅助函数 ====================

// makeResourceLookup 创建资源查找回调函数。
// 根据 resourceID 在 GameManager 的内存状态中查找对应的资源对象。
// 首先通过玩家ID找到玩家所在层，再在该层的资源表中查找资源。
// 简化实现：遍历所有地下城层的资源表查找匹配ID的资源。
func makeResourceLookup(gm *service.GameManager) func(uint64) *model.Resource {
	return func(resourceID uint64) *model.Resource {
		// 遍历所有地下城层，在资源表中查找指定ID的资源
		for layer := int32(1); layer <= int32(30); layer++ {
			dungeon := gm.GetDungeon(layer)
			if dungeon == nil {
				continue
			}
			// 在该层的资源映射表中查找
			if resource, ok := dungeon.Resources[resourceID]; ok {
				return resource
			}
		}
		// 未找到指定资源，返回 nil
		return nil
	}
}

// makeEnemiesLookup 创建仇人列表查找回调函数。
// 根据 playerID 返回该玩家的仇人ID列表。
// 当前为简化实现：返回空列表，后续可从 GameManager 的仇人映射表中查找。
func makeEnemiesLookup(gm *service.GameManager) func(uint64) []uint64 {
	return func(playerID uint64) []uint64 {
		// TODO: 实现仇人列表查找逻辑
		// 当前返回空列表，后续从 GameManager 的内存状态中查找
		return nil
	}
}