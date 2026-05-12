// Package rbac - 策略初始化
// 定义游戏默认权限策略，包括普通玩家、红名玩家、GM、管理员等角色的权限规则
package rbac

import "hero-quest/pkg/logger"

// InitDefaultPolicies 初始化游戏默认权限策略
// 包括：
//   - 普通玩家权限：进入已解锁层、购买商品、上架交易、PvP攻击等
//   - 红名玩家限制：禁止进入安全区、禁止使用交易行
//   - GM权限：封号、踢人、发奖励、调整属性
//   - 管理员权限：管理GM、修改配置、查看日志
//
// 使用 AddPolicy 添加策略，该方法不会重复添加已存在的策略
func InitDefaultPolicies(e *Enforcer) error {
	// ==================== 普通玩家权限 ====================
	// player 角色可以进入地下城各层、购买商品、上架交易、PvP攻击
	playerPolicies := [][]string{
		// 地下城：进入、离开、传送
		{"player", "dungeon:*", "enter"},
		{"player", "dungeon:*", "leave"},
		{"player", "dungeon:*", "teleport"},

		// 商店：查看列表、购买商品
		{"player", "shop:*", "list"},
		{"player", "shop:*", "buy"},

		// 交易行：查看列表、上架、购买、取消
		{"player", "trade:*", "list"},
		{"player", "trade:*", "publish"},
		{"player", "trade:*", "buy"},
		{"player", "trade:*", "cancel"},

		// PvP：攻击、悬赏、复仇
		{"player", "pvp:zone", "attack"},
		{"player", "pvp:zone", "bounty"},
		{"player", "pvp:zone", "revenge"},

		// 装备：强化、附魔、穿戴、卸下、锻造
		{"player", "equip:*", "strengthen"},
		{"player", "equip:*", "enchant"},
		{"player", "equip:*", "wear"},
		{"player", "equip:*", "unload"},
		{"player", "equip:*", "forge"},

		// 技能：升级、重置
		{"player", "skill:*", "levelup"},
		{"player", "skill:*", "reset"},

		// 宠物：召唤、收回、升级、进阶、探险、合成
		{"player", "pet:*", "summon"},
		{"player", "pet:*", "recall"},
		{"player", "pet:*", "levelup"},
		{"player", "pet:*", "evolve"},
		{"player", "pet:*", "explore"},
		{"player", "pet:*", "compose"},

		// 属性：分配属性点
		{"player", "attr:*", "assign"},

		// 排行榜：查看
		{"player", "rank:*", "view"},
	}

	for _, p := range playerPolicies {
		if err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return err
		}
	}

	// ==================== 红名玩家权限 ====================
	// redname 角色在普通玩家权限基础上有限制：
	//   - 禁止进入安全区（dungeon:safe 不允许 enter）
	//   - 禁止使用交易行（trade 不允许 publish 和 buy）
	// 红名玩家仍可进行 PvP 攻击、悬赏等操作
	rednamePolicies := [][]string{
		// 地下城：可以进入非安全区层、离开、传送
		{"redname", "dungeon:*", "leave"},
		{"redname", "dungeon:*", "teleport"},

		// 商店：查看和购买（但购买受限）
		{"redname", "shop:*", "list"},

		// PvP：可以攻击、被悬赏
		{"redname", "pvp:zone", "attack"},

		// 装备：保留装备操作
		{"redname", "equip:*", "strengthen"},
		{"redname", "equip:*", "enchant"},
		{"redname", "equip:*", "wear"},
		{"redname", "equip:*", "unload"},

		// 技能和宠物：保留基本操作
		{"redname", "skill:*", "levelup"},
		{"redname", "pet:*", "summon"},
		{"redname", "pet:*", "recall"},

		// 排行榜：可以查看
		{"redname", "rank:*", "view"},
	}

	for _, p := range rednamePolicies {
		if err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return err
		}
	}

	// ==================== GM 权限 ====================
	// gm 角色拥有管理玩家的权限：封号、踢人、发奖励、调整属性
	gmPolicies := [][]string{
		// 继承普通玩家所有权限（通过角色继承实现）
		// 封号：封禁指定玩家账号
		{"gm", "player:*", "ban"},
		// 踢人：将玩家踢下线
		{"gm", "player:*", "kick"},
		// 发奖励：给玩家发放物品/金币/经验
		{"gm", "player:*", "reward"},
		// 调整属性：修改玩家属性值
		{"gm", "player:*", "adjust_attr"},
		// 查看玩家信息
		{"gm", "player:*", "inspect"},
		// 公告发布
		{"gm", "broadcast:*", "send"},
	}

	for _, p := range gmPolicies {
		if err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return err
		}
	}

	// ==================== 管理员权限 ====================
	// admin 角色拥有最高权限：管理GM、修改配置、查看日志
	adminPolicies := [][]string{
		// 继承 GM 所有权限（通过角色继承实现）
		// 管理GM：任命/撤销GM
		{"admin", "gm:*", "appoint"},
		{"admin", "gm:*", "revoke"},
		// 修改配置：修改游戏参数
		{"admin", "config:*", "update"},
		{"admin", "config:*", "view"},
		// 查看日志：查看操作日志和系统日志
		{"admin", "log:*", "view"},
		{"admin", "log:*", "export"},
		// 服务器管理：重启、停服
		{"admin", "server:*", "restart"},
		{"admin", "server:*", "shutdown"},
	}

	for _, p := range adminPolicies {
		if err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return err
		}
	}

	// ==================== 角色继承关系 ====================
	// gm 继承 player 的所有权限
	if err := e.AddRoleForPlayer("gm", "player"); err != nil {
		return err
	}
	// admin 继承 gm 的所有权限（进而也继承 player 的权限）
	if err := e.AddRoleForPlayer("admin", "gm"); err != nil {
		return err
	}

	logger.Info("默认权限策略初始化完成")
	return nil
}
