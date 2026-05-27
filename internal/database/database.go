// database 包封装了 MySQL 数据库的连接与初始化逻辑，
// 基于 GORM 提供结构化的 ORM 操作能力，供上层业务使用。
package database

import (
	"fmt"
	"time"

	"hero-quest/internal/model"
	"hero-quest/pkg/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 是对 gorm.DB 的薄封装，继承 GORM 全部能力，
// 同时可在其上扩展项目特有的数据库方法。
type DB struct {
	*gorm.DB
}

// Config 包含数据库连接所需的所有配置项，
// 通常从配置文件或环境变量中读取后传入 New 函数。
type Config struct {
	Host     string // 数据库主机地址，如 "127.0.0.1"
	Port     int    // 数据库端口号，如 3306
	User     string // 连接用户名
	Password string // 连接密码
	DBName   string // 要使用的数据库名称
	MaxIdle  int    // 最大空闲连接数，空闲连接过多会占用资源，过少会导致突发流量时连接建立延迟
	MaxOpen  int    // 最大打开连接数，限制与 MySQL 的并发连接上限，防止打满 MySQL 连接池
}

// New 根据配置创建并初始化数据库连接。
// 关键逻辑：
//  1. 拼接 DSN（数据源名称），指定 charset=utf8mb4 以支持完整 Unicode（包括 emoji），
//     parseTime=true 让 MySQL 的 DATETIME 自动扫描为 Go 的 time.Time，
//     loc=Local 使时间解析使用本地时区。
//  2. 调用 gorm.Open 建立连接（使用 mysql 驱动）。
//  3. 获取底层 *sql.DB 设置连接池参数：空闲数、最大打开数、连接最大生命周期（1小时后回收，防止 MySQL 主动断开导致错误）。
//  4. Ping 验证连接确实可用。
func New(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	// 获取底层 *sql.DB 设置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Info("database connected", "host", cfg.Host, "db", cfg.DBName)
	return &DB{db}, nil
}

// InitSchema 初始化数据库表结构，使用 GORM 的 AutoMigrate 自动迁移。
// 若表已存在则不会重建，可安全重复调用。
// 共迁移 4 张表（技能、背包、宠物已迁移到 MongoDB）：
//   - player: 玩家主表，存储角色基础属性（职业、等级、经验、金币、荣誉、击杀值、
//     力量/敏捷/智力/体质、可用属性点等）
//   - player_equip: 玩家装备表，按槽位存储装备信息，含强化等级（附魔属性存 MongoDB）
//   - dungeon_progress: 副本进度表，记录每位玩家的最大通关层数
//   - trade_order: 交易订单表，记录玩家上架出售的装备、品质、价格及订单状态
func (db *DB) InitSchema() error {
	err := db.AutoMigrate(
		&model.PlayerORM{},
		&model.PlayerEquipORM{},
		&model.DungeonProgressORM{},
		&model.TradeOrderORM{},
	)
	if err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	logger.Info("database schema initialized")
	return nil
}

// Close 关闭数据库连接，释放资源。
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}
