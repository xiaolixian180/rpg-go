// Package config 提供游戏服务器的配置加载与管理功能。
// 通过 YAML 文件定义服务器、数据库、Redis 和游戏逻辑等各项配置参数，
// 并在启动时将配置解析为结构化的 Config 对象供全局使用。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 是顶层配置结构体，包含服务器运行所需的全部子配置。
// 每个字段对应 YAML 配置文件中的一个顶级区块。
type Config struct {
	Server ServerConfig `yaml:"server"` // 服务器网络与连接相关配置
	DB     DBConfig     `yaml:"database"` // 数据库连接与连接池配置
	Redis  RedisConfig  `yaml:"redis"`    // Redis 缓存连接配置
	Game   GameConfig   `yaml:"game"`     // 游戏核心逻辑与数值配置
}

// ServerConfig 定义服务器的网络参数和连接控制策略，
// 包括监听地址、超时时间、限流与心跳等关键运维参数。
type ServerConfig struct {
	Host            string `yaml:"host"`              // 服务器监听地址，如 "0.0.0.0" 或 "127.0.0.1"
	Port            int    `yaml:"port"`              // 服务器监听端口号
	ReadTimeout     int    `yaml:"read_timeout"`      // 读取请求超时时间（秒），防止慢速客户端占用连接
	WriteTimeout    int    `yaml:"write_timeout"`     // 写入响应超时时间（秒），防止响应阻塞
	MaxConnPerIP    int    `yaml:"max_conn_per_ip"`   // 单 IP 最大并发连接数，用于防止单个客户端过度占用资源
	HeartbeatSec    int    `yaml:"heartbeat_sec"`     // 客户端心跳间隔（秒），用于检测连接存活状态
	ReconnectSec    int    `yaml:"reconnect_sec"`     // 断线重连等待时间（秒），客户端掉线后允许重连的窗口期
	MaxRequestPerSec int   `yaml:"max_request_per_sec"` // 单 IP 每秒最大请求数，限流防刷
}

// DBConfig 定义关系型数据库（MySQL/PostgreSQL 等）的连接参数和连接池配置。
// 合理的连接池参数对高并发游戏服务器的数据库性能至关重要。
type DBConfig struct {
	Host     string `yaml:"host"`     // 数据库主机地址
	Port     int    `yaml:"port"`     // 数据库端口号
	User     string `yaml:"user"`     // 数据库登录用户名
	Password string `yaml:"password"` // 数据库登录密码
	DBName   string `yaml:"dbname"`   // 数据库名称，即要连接的具体数据库实例
	MaxIdle  int    `yaml:"max_idle"` // 连接池最大空闲连接数，空闲连接过多浪费资源，过少则频繁建连
	MaxOpen  int    `yaml:"max_open"` // 连接池最大打开连接数，限制数据库总连接数以保护数据库
}

// RedisConfig 定义 Redis 缓存的连接参数。
// Redis 在游戏中常用于会话管理、排行榜、实时数据缓存等场景。
type RedisConfig struct {
	Host     string `yaml:"host"`     // Redis 服务器地址
	Port     int    `yaml:"port"`     // Redis 服务器端口号
	Password string `yaml:"password"` // Redis 认证密码，为空表示无密码
	DB       int    `yaml:"db"`       // Redis 数据库编号（0-15），用于隔离不同环境或业务的数据
}

// GameConfig 定义游戏核心逻辑相关的数值参数。
// 这些参数直接影响游戏平衡性，修改时需谨慎评估对玩法的影响。
type GameConfig struct {
	MaxLevel         int     `yaml:"max_level"`          // 玩家等级上限，达到此等级后无法继续升级
	MaxDungeonLayer  int     `yaml:"max_dungeon_layer"`  // 地牢最大层数，限制玩家可探索的深度
	PvpGoldPenalty   float64 `yaml:"pvp_gold_penalty"`  // PVP 失败金币惩罚比例（如 0.1 表示扣除 10%）
	PvpHonorGain     int32   `yaml:"pvp_honor_gain"`    // PVP 胜利获得荣誉值，用于荣誉系统排名和奖励
	RedNameThreshold int32   `yaml:"red_name_threshold"` // 红名阈值（恶意击杀次数），超过此值玩家变为红名状态
	SkillResetCost   int64   `yaml:"skill_reset_cost"`   // 技能重置金币消耗，重置所有技能等级时扣除
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
// 这是获取运行时配置的唯一入口，应在程序启动时调用。
// 参数 path 为配置文件的文件系统路径（相对或绝对路径均可）。
// 返回解析后的配置指针，或读取/解析过程中遇到的错误。
func Load(path string) (*Config, error) {
	// 读取配置文件的原始字节数据
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 将 YAML 字节流反序列化到 Config 结构体中
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
