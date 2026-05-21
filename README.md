# Hero Quest - RPG 游戏服务器

基于 Go 语言实现的多人在线 RPG 游戏服务端，采用 WebSocket 长连接二进制协议通信。

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.25 |
| 网络 | WebSocket (`coder/websocket`) |
| 数据库 | MySQL 8.0 (GORM) |
| 缓存 | Redis 7 (go-redis/v9) |
| 认证 | JWT (HMAC-SHA256) |
| 权限 | Casbin RBAC |
| 配置 | Viper (YAML + 环境变量 `HQ_*`) |
| 日志 | slog 结构化 JSON + trace_id 链路追踪 |
| 可观测性 | Loki + Promtail + Grafana |

## 架构概览

```
Client
  |  WebSocket (Binary Protocol)
  v
Gateway (codec / router / hub / middleware)
  |  解码 -> Recovery -> Trace -> AccessLog -> RateLimit -> MaxBody -> AuthGuard -> Metrics
  v
Handler (JSON 反序列化 + 参数校验 + 组装)
  |  connCtx(conn) 传播 trace_id
  v
Service (业务逻辑, 锁下预计算属性)
  |
  v
Repo (数据访问接口)
  |
  +---> MySQL  (持久化)
  +---> Redis  (缓存 / 排行榜 / 令牌桶限流)
```

## 快速启动

```bash
# 克隆项目
git clone <repo-url> && cd rpg-go1

# 配置环境变量
cp .env.example .env
# 编辑 .env 填入实际密码

# 一键启动（MySQL + Redis + Loki + Grafana + 游戏服务器）
docker-compose up -d

# 查看日志
docker-compose logs -f game-server
```

服务端口：

| 服务 | 地址 |
|------|------|
| 游戏服务器 | `ws://localhost:8080/ws` |
| MySQL | `localhost:3306` |
| Redis | `localhost:6379` |
| Grafana | `http://localhost:3000` (admin / .env 中密码) |
| Loki | `http://localhost:3100` |

## 项目结构

```
rpg-go1/
├── cmd/
│   └── main.go                    # 程序入口：依赖注入、服务注册、优雅停机
├── configs/
│   ├── config.yaml                # 主配置文件
│   ├── promtail.yml               # Promtail 日志采集配置
│   └── grafana/provisioning/      # Grafana 数据源自动配置
├── internal/
│   ├── gateway/                   # 网关层
│   │   ├── codec.go               # 二进制协议编解码 [2B len][2B msgID][JSON]
│   │   ├── conn.go                # WebSocket 连接封装（发送缓冲 + 限流器 + trace）
│   │   ├── gateway.go             # 网关主逻辑（/ws, /health, /online, /debug/vars）
│   │   ├── hub.go                 # 连接管理中心（广播 / 私聊 / 踢人重连）
│   │   ├── middleware.go          # 7 层中间件链
│   │   └── router.go              # 消息路由（msgID → handler）
│   ├── handler/                   # Handler 层（35 个消息映射）
│   │   ├── handler.go             # 分发入口 + handleReq[T] 泛型辅助
│   │   ├── auth_handler.go        # 登录 / 创建角色
│   │   ├── combat_handler.go      # 攻击 / 技能 / 采集
│   │   ├── dungeon_handler.go     # 进入 / 离开 / 传送 / 移动
│   │   ├── equip_handler.go       # 强化 / 附魔 / 穿戴 / 卸下 / 锻造
│   │   ├── pet_handler.go         # 召唤 / 收回 / 升级 / 进阶 / 探险 / 合成
│   │   ├── pvp_handler.go         # PvP 攻击 / 悬赏 / 复仇
│   │   ├── rank_handler.go        # 排行榜查询
│   │   ├── shop_handler.go        # 商店列表 / 购买
│   │   ├── skill_handler.go       # 技能升级 / 重置
│   │   ├── trade_handler.go       # 交易行上架 / 购买 / 取消 / 列表
│   │   └── attr_handler.go        # 属性点分配
│   ├── service/                   # Service 层（核心业务逻辑）
│   │   ├── game_manager.go        # 全局游戏状态（玩家在线管理 / 脏数据存档）
│   │   ├── iface/world.go         # World 接口 + GameConfig 定义
│   │   ├── player/                # 玩家：登录登出 / 属性分配 / 经验升级
│   │   ├── combat/                # 战斗：伤害计算 / 技能释放 / 采集
│   │   ├── boss/                  # Boss 刷新与击杀奖励
│   │   ├── dungeon/               # 地下城：进入 / 离开 / 层间传送
│   │   ├── equip/                 # 装备：强化 / 附魔 / 穿戴 / 锻造（含材料消耗）
│   │   ├── pet/                   # 宠物：召唤 / 升级 / 进阶 / 探险 / 合成
│   │   ├── pvp/                   # PvP：攻击 / 红名 / 悬赏 / 复仇
│   │   ├── rank/                  # 排行榜：等级 / 战力 / 荣誉
│   │   ├── shop/                  # 商店：金币商店 / 荣誉商店
│   │   ├── skill/                 # 技能：升级 / 重置
│   │   └── trade/                 # 交易行：上架 / 购买（实时打款卖家）/ 取消
│   ├── model/                     # 数据模型 + 静态配置表
│   │   ├── player.go              # 玩家模型 / ExpTable / 属性计算公式
│   │   ├── equipment.go           # 装备模型 / EquipTemplates / 品质常量
│   │   ├── skill.go               # 技能模型 / SkillDefs（含倍率 / CD）
│   │   ├── pet.go                 # 宠物模型 / PetTemplates / 探险奖励计算
│   │   ├── dungeon.go             # 地下城 / 怪物模板 / 资源模板 / 生成函数
│   │   ├── boss.go                # Boss 模型
│   │   ├── shop.go                # 商品 / StaticShopItems / StaticForgeRecipes
│   │   └── trade.go               # 交易行模型
│   ├── repo/                      # 数据访问层（接口 + GORM 实现）
│   │   ├── repo.go                # 接口定义
│   │   ├── player_repo.go         # 玩家 CRUD
│   │   ├── equip_repo.go          # 装备 CRUD
│   │   ├── pet_repo.go            # 宠物 CRUD
│   │   ├── skill_repo.go          # 技能 CRUD
│   │   ├── trade_repo.go          # 交易行 CRUD
│   │   ├── shop_repo.go           # 商店库存（并发安全 mutex）
│   │   ├── cache_repo.go          # Redis 缓存（玩家 / 排行榜）
│   │   └── inventory_repo.go      # 背包（ON CONFLICT 增减 / 事务扣除）
│   ├── protocol/                  # 消息 ID + 请求/响应结构体
│   │   ├── msg_id.go              # 53 个消息 ID（12 模块）
│   │   └── message.go             # 所有 C2S / S2C JSON 结构体
│   ├── eventbus/                  # 异步事件总线（4 worker / 1024 buffer）
│   │   ├── bus.go                 # Subscribe / Publish / Close
│   │   └── events.go              # 事件类型定义（Boss击杀 / 玩家死亡 / 登录）
│   ├── scheduler/                 # 定时任务
│   │   ├── scheduler.go           # 调度器核心（Add / Start / Stop）
│   │   ├── monster_refresh.go     # 怪物刷新（30s）
│   │   ├── pet_explore.go         # 宠物探险结算（60s）
│   │   └── auto_save.go           # 脏数据自动存档（60s）
│   ├── cache/                     # Redis 连接封装
│   ├── database/                  # MySQL 连接封装 + Schema 自动迁移
│   └── rbac/                      # Casbin 权限管理（含 Close 防泄漏）
├── pkg/                           # 公共工具包
│   ├── auth/jwt.go                # JWT 生成与校验（HMAC-SHA256）
│   ├── config/config.go           # 配置加载（YAML + 环境变量 HQ_*）
│   ├── errors/errors.go           # 统一错误码
│   ├── logger/
│   │   ├── logger.go              # slog 结构化 JSON 日志
│   │   └── trace.go               # trace_id 传播（TInfo / TError / TWarn / TDebug）
│   └── utils/utils.go             # 通用工具函数
├── Dockerfile                     # 多阶段构建（golang:1.25-alpine → alpine:3.19）
├── docker-compose.yml             # 6 服务编排（game / mysql / redis / loki / promtail / grafana）
├── .env.example                   # 环境变量模板
├── go.mod
└── go.sum
```

## 通信协议

### 帧格式

```
+----------+----------+-----------+
| 2B 长度  | 2B 消息ID | JSON Body |
| (大端)   | (大端)    |           |
+----------+----------+-----------+
```

- **长度字段**（2 字节，大端序）：消息 ID 字节数(2) + Body 字节数
- **消息 ID**（2 字节，大端序）：参见 `internal/protocol/msg_id.go`
- **Body**：JSON 编码的请求/响应结构体

### 消息 ID 总览（53 个，12 模块）

| 模块 | ID 范围 | C→S | S→C |
|------|---------|-----|-----|
| 登录 | 1001-1099 | `Login`(1001) `CreatePlayer`(1003) | `LoginResp`(1002) `CreatePlayerResp`(1004) |
| 地下城 | 1101-1199 | `EnterDungeon`(1101) `LeaveDungeon`(1103) `LayerTeleport`(1107) | `EnterDungeonResp`(1102) `LeaveDungeonResp`(1104) `DungeonInfo`(1105) `MonsterRefresh`(1106) `LayerTeleportResp`(1108) |
| 战斗 | 1201-1299 | `Attack`(1201) `SkillCast`(1205) `CollectResource`(1209) | `Damage`(1202) `BossSpawn`(1203) `BossDie`(1204) `SkillEffect`(1206) `PlayerDie`(1207) `PlayerRevive`(1208) `CollectResult`(1210) |
| 装备 | 1301-1399 | `Strengthen`(1301) `Enchant`(1303) `Wear`(1305) `Unload`(1307) `Forge`(1309) | 对应 1302/1304/1306/1308/1310 |
| PvP | 1401-1499 | `PvpAttack`(1401) `BountyHunt`(1404) `Revenge`(1406) | `PvpResult`(1402) `RedNameList`(1403) `BountyReward`(1405) `RevengeResp`(1407) |
| 移动 | 1501-1599 | `Move`(1501) | `PlayerMove`(1502) |
| 宠物 | 1601-1699 | `Summon`(1601) `Recall`(1603) `LevelUp`(1604) `Evolve`(1605) `Explore`(1607) `Compose`(1609) | 对应 1602/1611/1606/1608/1610 |
| 交易行 | 1701-1799 | `TradeList`(1701) `TradePublish`(1703) `TradeBuy`(1705) `TradeCancel`(1707) | 对应 1702/1704/1706/1708 |
| 商店 | 1801-1899 | `ShopList`(1801) `ShopBuy`(1803) | 对应 1802/1804 |
| 技能 | 1901-1999 | `SkillLevelUp`(1901) `SkillReset`(1903) | 对应 1902/1904 |
| 属性 | 2001-2099 | `AttrAssign`(2001) | 对应 2002 |
| 排行榜 | 2101-2199 | `RankingList`(2101) | 对应 2102 |
| 系统 | 9001-9099 | `Heartbeat`(9002) | `Broadcast`(9001) `Heartbeat`(9002) `Kick`(9003) |

## 中间件链

每条消息依次经过 7 层中间件处理：

```
Recovery → Trace → AccessLog → RateLimit → MaxBodySize(4KB) → AuthGuard → Metrics
```

| 中间件 | 功能 |
|--------|------|
| Recovery | 捕获 panic，防止连接崩溃 |
| Trace | 生成唯一 `trace_id`（xid），注入连接上下文 |
| AccessLog | 记录每条消息的 msgID / connID / playerID / 耗时 |
| RateLimit | 令牌桶限流（默认 30 req/s per conn） |
| MaxBodySize | 拒绝超过 4KB 的消息 |
| AuthGuard | 白名单放行登录(1001)/创角(1003)，其余需已认证 |
| Metrics | expvar 采集每类消息计数和延迟 |

## 核心特性

### 战斗系统

- 伤害公式：`攻击力 = 力量*2 + 等级*5 + 敏捷*0.5`，防御减伤，随机 ±10% 波动
- 闪避率 = `敏捷 * 0.005`（上限 30%），暴击率 = `敏捷*0.003 + 力量*0.001`（上限 50%）
- 暴击伤害 = `1.5 + 力量 * 0.01`
- 技能伤害乘数由 `SkillDefs` 配置（如旋风斩 1.8x、火球术 2.5x）
- 锁下预计算属性，防止并发战斗数据不一致

### 装备系统

- **强化**：金币消耗递增，+7 以上有失败概率
- **附魔**：随机附加属性（攻击/防御/生命/暴击等）
- **锻造**：多件材料合成高级装备（`StaticForgeRecipes` 配方表）
- **穿戴**：8 槽位（武器/头盔/铠甲/护腿/靴子/项链/戒指/护符），需满足等级要求
- 30+ 装备模板，5 级品质（白/绿/蓝/紫/橙）

### 宠物系统

- 5 种类型（攻击/防御/辅助/掠夺），11 个模板
- 召唤/收回、升级/进阶、探险（最长 12h，品质越高收益越高）
- 同品质合成随机升级品质

### PvP 系统

- 玩家对战（被击杀后 30s 无敌保护，`invincible_sec` 可配置）
- 杀戮值累积 → 红名惩罚（杀戮值阈值可配置）
- 悬赏追杀、复仇机制
- RBAC 权限控制（`player:<id>` 粒度）

### 交易系统

- 玩家交易行（上架/购买/取消/列表）
- 购买时实时打款给在线卖家（通过 World 查找）

### 地下城

- 30 层副本，3 大区域（翠绿森林 1-10 / 腐蚀沼泽 11-20 / 烈焰火山 21-30）
- 每 10 层 Boss 战，Boss 掉落紫/橙品质装备概率
- 怪物 30s 自动刷新，资源点可采集（矿石/草药/木材）

### 排行榜

- 等级 / 战力 / 荣誉三类排行，基于 Redis Sorted Set
- 分页查询

### 自动存档

- 每 60s 扫描所有在线玩家，仅持久化有变更（Dirty=true）的玩家数据

## 日志追踪

所有业务日志自动携带 `trace_id`，可通过 Grafana + Loki 按 trace_id 查询完整请求链路：

```go
// Service 层使用 traced logger
logger.TInfo(ctx, "玩家登录", "player_id", playerID)
logger.TError(ctx, "数据库写入失败", "err", err)
```

**查询示例**（Grafana Explore）：
```
{job="game-server"} |= "trace_id=xxx"
```

Promtail 自动采集 Docker 容器日志，解析 `trace_id` 和 `player_id` 为 Loki 标签。

## 定时任务

| 任务 | 间隔 | 说明 |
|------|------|------|
| 怪物刷新 | 30s | 重置已死亡怪物的 HP/Dead 状态 |
| 宠物探险结算 | 60s | 结算到期探险，发放经验/金币/掉落 |
| 自动存档 | 60s | 持久化所有 Dirty 在线玩家数据 |

## 事件总线

异步进程内事件总线（4 worker goroutine，1024 buffer channel）：

| Topic | 事件 | 当前订阅 |
|-------|------|----------|
| `boss.die` | Boss 被击杀 | 全服广播击杀消息 |
| `player.die` | 玩家死亡 | - |
| `player.login` | 玩家登录 | - |

## 配置说明

配置文件路径：`configs/config.yaml`，支持环境变量覆盖（前缀 `HQ_`）。

### 环境变量

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `HQ_DATABASE_PASSWORD` | MySQL root 密码 | `hero_quest_2026` |
| `HQ_REDIS_PASSWORD` | Redis 密码 | `hero_quest_redis_2026` |
| `HQ_JWT_SECRET` | JWT 签名密钥 | `aB3dE7fG9hJ2kL5mN8pQ1rS4tU6vW0xY` |
| `GF_ADMIN_PASSWORD` | Grafana 管理员密码 | `hq_grafana_admin_2026` |

### 游戏配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.port` | 监听端口 | 8080 |
| `server.heartbeat_sec` | 心跳间隔(秒) | 30 |
| `server.max_conn_per_ip` | 单 IP 最大连接数 | 5 |
| `server.max_request_per_sec` | 每秒最大请求数 | 30 |
| `jwt.expire_hours` | Token 有效期(小时) | 24 |
| `game.max_level` | 最大等级 | 60 |
| `game.max_dungeon_layer` | 地下城最大层数 | 30 |
| `game.red_name_threshold` | 红名杀戮值阈值 | 5 |
| `game.invincible_sec` | PvP 死亡无敌时间(秒) | 30 |
| `game.pvp_gold_penalty` | PvP 死亡金币掉落比例 | 0.1 |
| `game.pvp_honor_gain` | PvP 击杀荣誉奖励 | 10 |
| `game.skill_reset_cost` | 技能重置金币消耗 | 1000 |
| `game.teleport_cost` | 层间传送金币消耗 | 100 |
| `game.skill_multiplier` | 默认技能伤害倍率 | 1.5 |
| `game.pet_levelup_cost` | 宠物升级金币消耗 | 100 |
| `game.bounty_gold_per_kill` | 悬赏击杀金币奖励 | 500 |
| `game.bounty_honor_gain` | 悬赏击杀荣誉奖励 | 10 |
| `game.boss_drop_purple` | Boss 紫装掉落概率 | 0.1 |
| `game.boss_drop_orange` | Boss 橙装掉落概率 | 0.02 |
| `game.page_size` | 分页大小 | 20 |

## 测试

```bash
# 运行全部测试
go test ./... -count=1

# 运行指定模块测试（含 verbose）
go test ./internal/service/combat/ -v -count=1
```

当前测试覆盖 4 个核心 Service 模块，共 36 个测试用例：

| 模块 | 测试数 | 覆盖点 |
|------|--------|--------|
| combat | 12 | 伤害计算 / 技能倍率 / 暴击 / 防御减伤 / 击杀奖励 / 采集 / 死亡状态 |
| player | 11 | 属性分配 / 经验升级 / 连续升级 / MaxHp 重算 / 边界校验 |
| equip | 6 | 强化金币扣除 / 金币不足 / 槽位校验 / +7 失败率 |
| rank | 6 | 排行解析 / 空排行 / 无效类型 / ID 解析降级 / 全类型 |

## 启动流程

```
main.go 启动顺序：

1. 解析命令行参数 (-c config path)
2. 加载配置 + 校验
3. 初始化日志 (slog JSON)
4. 连接 MySQL + Schema 迁移
5. 初始化 RBAC (Casbin + 默认策略)
6. 连接 Redis
7. 创建 8 个 Repo (Player/Equip/Pet/Trade/Shop/Skill/Cache/Inventory)
8. 创建核心 Service (player/equip/pet/shop/skill/rank)
9. 创建 Gateway (Router + Middleware)
10. 创建 GameManager (全局游戏状态)
11. 创建依赖 Service (boss/pvp/trade/combat/dungeon)
12. 创建 EventBus (4 worker) + 订阅 Boss 击杀广播
13. 创建 Handler + 注册 35 个消息路由
14. 设置 JWT 认证 + 登出回调
15. 注册定时任务 (怪物刷新/探险结算/自动存档)
16. 启动 Gateway + Scheduler
17. 阻塞等待 SIGINT/SIGTERM
18. 优雅停机 (5s timeout): Gateway → EventBus → Scheduler → Casbin → Redis → MySQL
```

## 并发安全

| 场景 | 策略 |
|------|------|
| Player 属性读写 | `sync.RWMutex`，读多写少 |
| 战斗伤害计算 | 锁下预计算所有属性（`readCombatAttrs`），避免持锁时间过长 |
| 多玩家同时攻击同一怪物 | `Monster.mu` 保护 `Dead` 字段，CAS 式检查防止重复发奖 |
| 商店库存扣减 | `ShopRepo` 独立 `sync.Mutex`，原子扣减 |
| 交易行卖家打款 | 通过 `World.GetOnlinePlayer` 获取内存玩家引用，直接加金币 |
| 背包物品增减 | `ON CONFLICT` 原子更新 + 事务扣除 |
| Hub 连接管理 | 专用 goroutine 串行处理 register/unregister |
| 跨层锁定顺序 | 统一按 Player → Monster → Resource → DungeonLayer 顺序加锁，防止 ABBA 死锁 |
