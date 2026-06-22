// Package config 提供游戏服务器的配置加载与管理功能。
// 通过 YAML 文件定义服务器、数据库、Redis 和游戏逻辑等各项配置参数，
// 并在启动时将配置解析为结构化的 Config 对象供全局使用。
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// DefaultGameConfig 返回游戏数值的默认值，配置文件中未指定的字段使用此默认值
func DefaultGameConfig() GameConfig {
	return GameConfig{
		TeleportCost:      100,
		SkillMultiplier:   1.5,
		InvincibleSec:     30,
		PetLevelUpCost:    100,
		BountyGoldPerKill: 500,
		BountyHonorGain:   10,
		BossDropPurple:    0.1,
		BossDropOrange:    0.02,
		PageSize:          20,
	}
}

// Config 是顶层配置结构体，包含服务器运行所需的全部子配置。
// 每个字段对应 YAML 配置文件中的一个顶级区块。
type Config struct {
	Server ServerConfig `yaml:"server" mapstructure:"server"`
	DB     DBConfig     `yaml:"database" mapstructure:"database"`
	Redis  RedisConfig  `yaml:"redis" mapstructure:"redis"`
	JWT    JWTConfig    `yaml:"jwt" mapstructure:"jwt"`
	Game   GameConfig   `yaml:"game" mapstructure:"game"`
}

// ServerConfig 定义服务器的网络参数和连接控制策略，
// 包括监听地址、超时时间、限流与心跳等关键运维参数。
type ServerConfig struct {
	Host             string `yaml:"host" mapstructure:"host"`
	Port             int    `yaml:"port" mapstructure:"port"`
	ReadTimeout      int    `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout     int    `yaml:"write_timeout" mapstructure:"write_timeout"`
	MaxConnPerIP     int    `yaml:"max_conn_per_ip" mapstructure:"max_conn_per_ip"`
	HeartbeatSec     int    `yaml:"heartbeat_sec" mapstructure:"heartbeat_sec"`
	ReconnectSec     int    `yaml:"reconnect_sec" mapstructure:"reconnect_sec"`
	MaxRequestPerSec int    `yaml:"max_request_per_sec" mapstructure:"max_request_per_sec"`
}

// DBConfig 定义关系型数据库（MySQL/PostgreSQL 等）的连接参数和连接池配置。
// 合理的连接池参数对高并发游戏服务器的数据库性能至关重要。
type DBConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`     // 数据库主机地址
	Port     int    `yaml:"port" mapstructure:"port"`     // 数据库端口号
	User     string `yaml:"user" mapstructure:"user"`     // 数据库登录用户名
	Password string `yaml:"password" mapstructure:"password"` // 数据库登录密码
	DBName   string `yaml:"dbname" mapstructure:"dbname"`   // 数据库名称，即要连接的具体数据库实例
	MaxIdle  int    `yaml:"max_idle" mapstructure:"max_idle"` // 连接池最大空闲连接数，空闲连接过多浪费资源，过少则频繁建连
	MaxOpen  int    `yaml:"max_open" mapstructure:"max_open"` // 连接池最大打开连接数，限制数据库总连接数以保护数据库
}

// RedisConfig 定义 Redis 缓存的连接参数。
// Redis 在游戏中常用于会话管理、排行榜、实时数据缓存等场景。
type RedisConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`     // Redis 服务器地址
	Port     int    `yaml:"port" mapstructure:"port"`     // Redis 服务器端口号
	Password string `yaml:"password" mapstructure:"password"` // Redis 认证密码，为空表示无密码
	DB       int    `yaml:"db" mapstructure:"db"`       // Redis 数据库编号（0-15），用于隔离不同环境或业务的数据
}

// JWTConfig 定义 JWT 认证相关的配置参数。
// secret 用于 HMAC 签名，expire_hours 控制令牌有效期。
type JWTConfig struct {
	Secret      string `yaml:"secret" mapstructure:"secret"`       // JWT 签名密钥
	ExpireHours int    `yaml:"expire_hours" mapstructure:"expire_hours"` // 令牌有效时长（小时）
}

// GameConfig 定义游戏核心逻辑相关的数值参数。
// 这些参数直接影响游戏平衡性，修改时需谨慎评估对玩法的影响。
type GameConfig struct {
	MaxLevel         int     `yaml:"max_level" mapstructure:"max_level"`          // 玩家等级上限，达到此等级后无法继续升级
	MaxDungeonLayer  int     `yaml:"max_dungeon_layer" mapstructure:"max_dungeon_layer"`  // 地牢最大层数，限制玩家可探索的深度
	PvpGoldPenalty   float64 `yaml:"pvp_gold_penalty" mapstructure:"pvp_gold_penalty"`   // PVP 失败金币惩罚比例（如 0.1 表示扣除 10%）
	PvpHonorGain     int32   `yaml:"pvp_honor_gain" mapstructure:"pvp_honor_gain"`     // PVP 胜利获得荣誉值，用于荣誉系统排名和奖励
	RedNameThreshold int32   `yaml:"red_name_threshold" mapstructure:"red_name_threshold"` // 红名阈值（恶意击杀次数），超过此值玩家变为红名状态
	SkillResetCost   int64   `yaml:"skill_reset_cost" mapstructure:"skill_reset_cost"`   // 技能重置金币消耗

	// 以下为新增的游戏数值配置，统一收归到配置文件，支持热更新
	TeleportCost      int64   `yaml:"teleport_cost" mapstructure:"teleport_cost"`        // 层间传送金币消耗
	SkillMultiplier   float64 `yaml:"skill_multiplier" mapstructure:"skill_multiplier"`     // 技能伤害倍率（相对普攻）
	InvincibleSec     int     `yaml:"invincible_sec" mapstructure:"invincible_sec"`       // PvP 死亡后无敌保护时间（秒）
	PetLevelUpCost    int64   `yaml:"pet_levelup_cost" mapstructure:"pet_levelup_cost"`     // 宠物升级金币消耗
	BountyGoldPerKill int64   `yaml:"bounty_gold_per_kill" mapstructure:"bounty_gold_per_kill"` // 悬赏金币 = 目标杀戮值 * 此值
	BountyHonorGain   int32   `yaml:"bounty_honor_gain" mapstructure:"bounty_honor_gain"`    // 悬赏击杀获得荣誉值
	BossDropPurple    float64 `yaml:"boss_drop_purple" mapstructure:"boss_drop_purple"`     // Boss掉落紫色品质概率
	BossDropOrange    float64 `yaml:"boss_drop_orange" mapstructure:"boss_drop_orange"`     // Boss掉落橙色品质概率
	PageSize          int32   `yaml:"page_size" mapstructure:"page_size"`            // 交易行每页条数
}

// Validate 校验配置项的合法性，确保必填项非空、数值参数在合理范围内。
// 在程序启动时调用，若校验失败则返回错误，阻止服务以无效配置启动。
func (c *Config) Validate() error {
	// 校验服务器配置
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1~65535 之间，当前值: %d", c.Server.Port)
	}
	if c.Server.HeartbeatSec <= 0 {
		return fmt.Errorf("server.heartbeat_sec 必须大于 0，当前值: %d", c.Server.HeartbeatSec)
	}

	// 校验数据库配置
	if c.DB.Host == "" {
		return fmt.Errorf("database.host 不能为空")
	}
	if c.DB.DBName == "" {
		return fmt.Errorf("database.dbname 不能为空")
	}
	if c.DB.MaxOpen <= 0 {
		return fmt.Errorf("database.max_open 必须大于 0，当前值: %d", c.DB.MaxOpen)
	}

	// 校验 Redis 配置
	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host 不能为空")
	}

	// 校验 JWT 配置
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空")
	}
	if c.JWT.ExpireHours <= 0 {
		return fmt.Errorf("jwt.expire_hours 必须大于 0，当前值: %d", c.JWT.ExpireHours)
	}

	// 校验游戏配置
	if c.Game.MaxDungeonLayer <= 0 {
		return fmt.Errorf("game.max_dungeon_layer 必须大于 0，当前值: %d", c.Game.MaxDungeonLayer)
	}
	if c.Game.PvpGoldPenalty < 0 || c.Game.PvpGoldPenalty > 1 {
		return fmt.Errorf("game.pvp_gold_penalty 必须在 0~1 之间，当前值: %f", c.Game.PvpGoldPenalty)
	}
	if c.Game.RedNameThreshold <= 0 {
		return fmt.Errorf("game.red_name_threshold 必须大于 0，当前值: %d", c.Game.RedNameThreshold)
	}
	if c.Game.SkillResetCost < 0 {
		return fmt.Errorf("game.skill_reset_cost 不能为负数，当前值: %d", c.Game.SkillResetCost)
	}

	return nil
}

// Load 从指定路径读取 YAML 配置文件并解析为 Config 结构体。
// 使用 Viper 支持环境变量覆盖和未来热更新能力。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	// 设置游戏数值默认值
	defaults := DefaultGameConfig()
	v.SetDefault("game.teleport_cost", defaults.TeleportCost)
	v.SetDefault("game.skill_multiplier", defaults.SkillMultiplier)
	v.SetDefault("game.invincible_sec", defaults.InvincibleSec)
	v.SetDefault("game.pet_levelup_cost", defaults.PetLevelUpCost)
	v.SetDefault("game.bounty_gold_per_kill", defaults.BountyGoldPerKill)
	v.SetDefault("game.bounty_honor_gain", defaults.BountyHonorGain)
	v.SetDefault("game.boss_drop_purple", defaults.BossDropPurple)
	v.SetDefault("game.boss_drop_orange", defaults.BossDropOrange)
	v.SetDefault("game.page_size", defaults.PageSize)

	// 设置 JWT 默认值
	v.SetDefault("jwt.expire_hours", 24)

	// 支持环境变量覆盖，前缀 HQ_，如 HQ_SERVER_PORT=9090
	v.SetEnvPrefix("HQ")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
