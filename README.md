# Hero Quest - RPG 游戏服务器

基于 Go 语言实现的多人在线 RPG 游戏服务端，采用 WebSocket 长连接二进制协议通信。

## 技术栈

- **语言**: Go 1.25
- **网络**: WebSocket (`coder/websocket`)
- **数据库**: MySQL 8.0 (GORM)
- **缓存**: Redis 7 (go-redis)
- **认证**: JWT
- **权限**: Casbin RBAC
- **配置**: Viper (YAML + 环境变量)

## 架构概览

```
Client
  |  WebSocket (Binary Protocol)
  v
Gateway (codec/router/hub/middleware)
  |  解码 -> 路由 -> 鉴权 -> 限流
  v
Handler (参数校验 + 组装)
  |
  v
Service (业务逻辑)
  |
  v
Repo (数据访问接口)
  |
  +---> MySQL  (持久化)
  +---> Redis  (缓存/排行榜)
```

## 快速启动

```bash
# 克隆项目
git clone <repo-url> && cd rpg-go1

# 一键启动（MySQL + Redis + 游戏服务器）
docker-compose up -d

# 查看日志
docker-compose logs -f game-server
```

服务默认监听 `ws://localhost:8080`。

## 项目结构

```
rpg-go1/
├── cmd/                        # 程序入口
│   └── main.go                 # 启动、依赖注入、服务注册
├── configs/
│   └── config.yaml             # 配置文件
├── internal/
│   ├── gateway/                # 网关层（连接管理、编解码、路由、中间件）
│   │   ├── codec.go            # 二进制协议编解码
│   │   ├── conn.go             # WebSocket 连接封装
│   │   ├── gateway.go          # 网关主逻辑
│   │   ├── hub.go              # 消息中心（广播/私聊）
│   │   ├── middleware.go       # 中间件（限流/JWT/权限）
│   │   └── router.go           # 消息路由
│   ├── handler/                # Handler 层（参数校验 + 调用 Service）
│   │   ├── handler.go          # Handler 分发入口
│   │   ├── auth_handler.go     # 登录/创建角色
│   │   ├── combat_handler.go   # 攻击/技能/采集
│   │   ├── dungeon_handler.go  # 进入/离开地下城
│   │   ├── equip_handler.go    # 强化/附魔/穿戴/锻造
│   │   ├── pet_handler.go      # 宠物召唤/升级/探险/合成
│   │   ├── pvp_handler.go      # PvP/红名/悬赏/复仇
│   │   ├── rank_handler.go     # 排行榜查询
│   │   ├── shop_handler.go     # 商店购买
│   │   ├── skill_handler.go    # 技能升级/重置
│   │   ├── trade_handler.go    # 交易行上架/购买/取消
│   │   └── attr_handler.go     # 属性点分配
│   ├── service/                # Service 层（业务逻辑）
│   │   ├── game_manager.go     # 全局游戏状态管理
│   │   ├── world.go            # World 接口适配
│   │   ├── iface/world.go      # World 接口定义
│   │   ├── player/             # 玩家服务（登录/登出/属性/经验）
│   │   ├── combat/             # 战斗服务（攻击/技能/采集）
│   │   ├── boss/               # Boss 刷新与管理
│   │   ├── dungeon/            # 地下城（进入/离开/传送）
│   │   ├── equip/              # 装备（强化/附魔/穿戴/锻造）
│   │   ├── pet/                # 宠物（召唤/升级/进阶/探险/合成）
│   │   ├── pvp/                # PvP（攻击/红名/悬赏/复仇）
│   │   ├── rank/               # 排行榜（查询/更新）
│   │   ├── shop/               # 商店（购买）
│   │   ├── skill/              # 技能（升级/重置）
│   │   └── trade/              # 交易行（上架/购买/取消）
│   ├── model/                  # 数据模型 + 静态配置表
│   ├── repo/                   # 数据访问层（接口 + 实现）
│   ├── protocol/               # 消息 ID 定义
│   ├── eventbus/               # 事件总线
│   ├── scheduler/              # 定时任务（怪物刷新/宠物探险）
│   ├── cache/                  # Redis 缓存封装
│   ├── database/               # MySQL 连接封装
│   └── rbac/                   # Casbin 权限管理
├── pkg/                        # 公共工具包
│   ├── auth/jwt.go             # JWT 令牌生成与校验
│   ├── config/config.go        # 配置加载（YAML + 环境变量 HQ_*）
│   ├── errors/errors.go        # 统一错误码定义
│   ├── logger/logger.go        # 日志封装
│   └── utils/utils.go          # 通用工具函数
├── Dockerfile                  # 多阶段构建
├── docker-compose.yml          # 容器编排
├── go.mod
└── go.sum
```

## 通信协议

二进制 WebSocket 帧，格式如下：

```
+----------+----------+-----------+
| 2B 长度  | 2B 消息ID | JSON Body |
| (大端)   | (大端)    |           |
+----------+----------+-----------+
```

- **长度字段**（2字节，大端序）：消息ID字节数(2) + Body 字节数
- **消息ID**（2字节，大端序）：参见 `internal/protocol/msg_id.go`
- **Body**：JSON 编码的请求/响应结构体

消息 ID 按模块分段：

| 模块 | 范围 |
|------|------|
| 登录 | 1001-1099 |
| 地下城 | 1101-1199 |
| 战斗 | 1201-1299 |
| 装备 | 1301-1399 |
| PvP | 1401-1499 |
| 宠物 | 1601-1699 |
| 交易行 | 1701-1799 |
| 商店 | 1801-1899 |
| 技能 | 1901-1999 |
| 属性 | 2001-2099 |
| 排行榜 | 2101-2199 |
| 系统 | 9001-9099 |

## 配置说明

配置文件路径：`configs/config.yaml`，支持环境变量覆盖（前缀 `HQ_`）。

主要配置项：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.port` | 监听端口 | 8080 |
| `server.heartbeat_sec` | 心跳间隔(秒) | 30 |
| `server.max_conn_per_ip` | 单IP最大连接数 | 5 |
| `server.max_request_per_sec` | 每秒最大请求数 | 30 |
| `database.host` | MySQL 地址 | mysql |
| `redis.host` | Redis 地址 | redis |
| `jwt.secret` | JWT 密钥 | - |
| `jwt.expire_hours` | Token 有效期(小时) | 24 |
| `game.max_level` | 最大等级 | 60 |
| `game.max_dungeon_layer` | 地下城最大层数 | 30 |
| `game.red_name_threshold` | 红名杀戮值阈值 | 5 |
| `game.invincible_sec` | PvP 死亡无敌时间(秒) | 30 |

## 核心特性

- **战斗系统**: 基于属性的伤害计算（力量/等级/敏捷 -> 攻击力/闪避率/暴击率），含防御减伤、随机波动
- **装备系统**: 强化（+7 以上概率失败）、附魔（随机属性）、锻造合成、8 槽位穿戴
- **宠物系统**: 召唤/收回、升级/进阶、探险（定时收益）、同品质合成
- **PvP 系统**: 玩家对战、红名惩罚、悬赏追杀、复仇、30秒无敌保护
- **交易系统**: 玩家交易行（上架/购买/取消）
- **排行榜**: 等级/战力/荣誉排行，基于 Redis 有序集合
- **地下城**: 30层副本（翠绿森林/腐蚀沼泽/烈焰火山），每10层 Boss 战
