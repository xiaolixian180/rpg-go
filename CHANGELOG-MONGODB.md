# MongoDB 混合存储集成 — 变更记录

> 日期：2026-05-27
> 分支：main
> 范围：rpg-go 后端服务

---

## 一、背景与目标

rpg-go 原有存储方案为 MySQL + Redis。随着装备附魔属性、宠物技能、背包物品等文档型数据的增长，纯关系型存储在灵活结构和查询效率上存在局限。

本次变更引入 MongoDB 作为混合存储层，验证架构可行性，同时保留后续扩展余量。

---

## 二、存储分层策略

| 存储 | 定位 | 存什么 |
|------|------|--------|
| **MySQL** | 核心事务数据 | Player 主表、TradeOrder、DungeonProgress、Equip 主记录 |
| **MongoDB** | 文档型/灵活结构数据 | 装备附魔属性（结构化）、玩家背包、宠物数据（技能数组）、玩家技能表 |
| **Redis** | 缓存/排行榜 | 排行榜 ZSet、Boss 冷却、在线状态缓存（不变） |

---

## 三、文件变更清单

### 3.1 新建文件（6 个）

| 文件 | 说明 |
|------|------|
| `internal/database/mongo.go` | MongoDB 连接层：`MongoDB` 结构体封装 `*mongo.Client` 和 `*mongo.Database`，提供 `NewMongo(uri, database)` 连接和 `Close()` 优雅关闭 |
| `internal/model/mongo_doc.go` | MongoDB 文档模型：`EquipDoc`（含结构化 `EnchantAttr` 数组）、`SkillDoc`、`InventoryDoc`、`PetDoc`（含 `PetSkill` 数组）、Collection 名称常量 |
| `internal/repo/mongo_skill_repo.go` | 纯 MongoDB 技能 Repo：实现 `SkillRepo` 接口，upsert by (player_id, skill_id)，自动创建唯一索引 |
| `internal/repo/mongo_inventory_repo.go` | 纯 MongoDB 背包 Repo：实现 `InventoryRepo` 接口，`$inc` 原子增减，自动清理 count<=0 的文档 |
| `internal/repo/mongo_pet_repo.go` | 纯 MongoDB 宠物 Repo：实现 `PetRepo` 接口，pet_uid 唯一索引 + player_id 普通索引，含计数器自增 UID 生成 |
| `internal/repo/mongo_equip_repo.go` | 混合模式装备 Repo：MySQL 存主记录 + MongoDB 存附魔属性数组，`GetAllEquips` 批量合并，`parseEnchantAttr`/`formatEnchantAttrs` 双向转换 |

### 3.2 修改文件（8 个）

| 文件 | 变更内容 |
|------|----------|
| `go.mod` | 新增依赖 `go.mongodb.org/mongo-driver/v2 v2.6.0`，降级 `casbin/gorm-adapter/v3` 至 v3.26.0（修复接口兼容） |
| `go.sum` | 依赖锁文件自动更新 |
| `pkg/config/config.go` | 新增 `MongoConfig` 结构体（URI、Database），`Config` 新增 `Mongo` 字段；`Load()` 改用 `mapstructure` 的 `yaml` tag 解决 Viper `AutomaticEnv` 键名映射问题 |
| `configs/config.yaml` | 新增 `mongodb:` 配置段（uri、database） |
| `docker-compose.yml` | 新增 `mongodb` 服务（mongo:7 镜像、健康检查、mongo_data volume）；game-server 增加 mongodb depends_on 和 `HQ_MONGODB_URI` 环境变量；MySQL 端口改为 3307 避免宿主机冲突 |
| `cmd/main.go` | 新增 MongoDB 初始化（`database.NewMongo`）；Repo 注入改为 MongoDB 版本（`NewMongoSkillRepo`、`NewMongoInventoryRepo`、`NewMongoPetRepo`、`NewMongoEquipRepo`）；优雅关闭增加 `mdb.Close()` |
| `internal/database/database.go` | `InitSchema()` 从 AutoMigrate 移除 `PlayerSkillORM`、`PlayerInventoryORM`、`PlayerPetORM`（已迁移到 MongoDB）；更新注释 |

---

## 四、架构变更

### 4.1 依赖注入变化

```
改造前：
  db (MySQL) → PlayerRepo, EquipRepo, SkillRepo, PetRepo, InventoryRepo, TradeRepo, ShopRepo
  rdb (Redis) → CacheRepo, RankRepo

改造后：
  db (MySQL)  → PlayerRepo, TradeRepo, ShopRepo
  mdb (Mongo) → MongoSkillRepo, MongoInventoryRepo, MongoPetRepo
  db + mdb    → MongoEquipRepo（混合：MySQL 主记录 + Mongo 附魔属性）
  rdb (Redis) → CacheRepo, RankRepo（不变）
```

### 4.2 接口层无变化

所有 Repo 接口定义不变（`repo.go`），Service 层无感知。MongoDB 实现实现相同的接口，通过依赖注入替换。

### 4.3 装备附魔属性升级

```
改造前（MySQL）：
  EnchantAttr string → "力量+5,敏捷+3"

改造后（MongoDB）：
  EnchantAttrs []EnchantAttr → [{Type:"力量", Value:5}, {Type:"敏捷", Value:3}]
```

支持结构化查询，无需字符串解析。

### 4.4 宠物技能升级

```
改造前（MySQL）：
  Skills string → "1:3,2:1"

改造后（MongoDB）：
  Skills []PetSkill → [{SkillID:1, Level:3}, {SkillID:2, Level:1}]
```

---

## 五、MongoDB 索引

| Collection | 索引 | 类型 |
|------------|------|------|
| `player_skills` | (player_id, skill_id) | 唯一索引 |
| `player_inventory_mongo` | (player_id, item_id) | 唯一索引 |
| `player_pets` | (pet_uid) | 唯一索引 |
| `player_pets` | (player_id) | 普通索引 |
| `equip_enchant` | (player_id, slot) | 唯一索引 |

索引在 Repo 构造函数中自动创建，无需手动维护。

---

## 六、Docker Compose 服务

```yaml
# 新增服务
mongodb:
  image: mongo:7
  ports: ["27017:27017"]
  environment:
    MONGO_INITDB_ROOT_USERNAME: root
    MONGO_INITDB_ROOT_PASSWORD: ${HQ_MONGO_PASSWORD:-hero_quest_mongo_2026}
  volumes: [mongo_data:/data/db]
  healthcheck: mongosh --eval "db.adminCommand('ping')"
```

---

## 七、配置示例

```yaml
# configs/config.yaml 新增段
mongodb:
  uri: "mongodb://root:HQ_MONGO_PASSWORD@mongodb:27017"
  database: "hero_quest"
```

本地开发通过环境变量覆盖：
```bash
HQ_MONGODB_URI="mongodb://root:hero_quest_mongo_2026@127.0.0.1:27017"
```

---

## 八、附带修复

### 8.1 Viper 配置加载修复

**问题：** Viper `AutomaticEnv()` + `Unmarshal()` 对带下划线的 YAML 键名（如 `heartbeat_sec`）映射失败，导致字段值为 0。

**原因：** Viper 默认使用 `mapstructure` tag 解析，与 `yaml` tag 名称不一致。

**修复：** `Load()` 改用 `v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) { dc.TagName = "yaml" })`。

### 8.2 Casbin 版本兼容修复

**问题：** `casbin/v2` v2.135.0 的 `persist.Adapter` 接口与 `gorm-adapter/v3` v3.41.0 不兼容，运行时 panic。

**修复：** 降级 `gorm-adapter/v3` 至 v3.26.0。

---

## 九、测试验证

- `go build ./...` — 编译通过
- `go test ./...` — 4 个测试套件全部通过（combat、equip、player、rank）
- 服务端启动验证：
  - MySQL 连接 + 4 张表 AutoMigrate ✓
  - MongoDB 连接 + 索引创建 ✓
  - Redis 连接 ✓
  - Casbin RBAC 初始化 ✓
  - 35 个消息路由注册 ✓
  - 3 个定时任务启动 ✓
  - 优雅关闭全部连接释放 ✓

---

## 十、后期扩展余量

- **装备附魔：** `[]EnchantAttr` 文档模型天然支持新增属性类型，无需 migration
- **宠物技能：** 嵌入式数组可随时加字段（如 skill_name、cooldown），无需 ALTER TABLE
- **背包聚合：** MongoDB aggregation pipeline 可做物品统计、价值排序、稀有度分布
- **战报/BattleLog：** 天然文档型数据，可直接存 MongoDB collection + TTL 索引自动过期
- **玩家行为分析：** MongoDB 时序数据 + TTL 索引，自动清理过期数据
