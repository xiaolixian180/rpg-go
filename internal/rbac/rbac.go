// Package rbac 封装 Casbin 权限管理器，
// 提供基于 RBAC 模型的权限校验、策略管理和角色分配功能。
// 策略数据通过 GORM adapter 持久化到 MySQL 数据库。
package rbac

import (
	"fmt"

	"hero-quest/pkg/logger"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Enforcer Casbin 权限执行器，封装策略加载和权限校验
type Enforcer struct {
	enforcer *casbin.Enforcer
}

// casbinModel Casbin RBAC 模型定义（嵌入代码中，无需外部配置文件）
// 模型说明：
//   - r: 请求定义，包含 sub(主体)、obj(对象)、act(操作) 三个要素
//   - p: 策略定义，同样包含 sub、obj、act
//   - g: 角色定义，支持角色继承（玩家 -> 角色）
//   - e: 策略效果，只要有一条策略匹配就允许
//   - m: 匹配器，先通过 g() 判断主体是否属于策略中的角色，再精确匹配对象和操作
const casbinModelText = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

// Config Casbin 初始化配置，包含数据库连接参数
type Config struct {
	Host     string // 数据库主机地址
	Port     int    // 数据库端口号
	User     string // 数据库用户名
	Password string // 数据库密码
	DBName   string // 数据库名称
}

// New 创建 Casbin 执行器，使用 GORM adapter 从数据库加载策略
// 流程：
//  1. 根据配置创建 GORM 数据库连接（Casbin 策略表专用）
//  2. 创建 GORM adapter，自动迁移 casbin_rule 表
//  3. 从内嵌模型文本创建 Casbin 模型实例
//  4. 创建 Enforcer 并加载已有策略
func New(cfg Config) (*Enforcer, error) {
	// 拼接 GORM 的 MySQL DSN
	// charset=utf8mb4 支持完整 Unicode，parseTime=true 自动解析时间类型
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	// 创建 GORM 数据库连接（Casbin 专用，与业务库共用同一 MySQL 实例）
	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开 GORM 数据库连接失败: %w", err)
	}

	// 创建 GORM adapter，指定使用该数据库连接
	// adapter 会自动创建 casbin_rule 表来存储策略数据
	adapter, err := gormadapter.NewAdapterByDB(gormDB)
	if err != nil {
		return nil, fmt.Errorf("创建 GORM adapter 失败: %w", err)
	}

	// 从内嵌的模型文本创建 Casbin 模型
	m, err := model.NewModelFromString(casbinModelText)
	if err != nil {
		return nil, fmt.Errorf("创建 Casbin 模型失败: %w", err)
	}

	// 创建 Casbin Enforcer，绑定模型和适配器
	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("创建 Casbin Enforcer 失败: %w", err)
	}

	// 从数据库加载已有策略到内存
	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("加载 Casbin 策略失败: %w", err)
	}

	logger.Info("Casbin 权限执行器初始化成功")
	return &Enforcer{enforcer: e}, nil
}

// Check 检查玩家是否有指定权限
// sub: 玩家角色（如 "player", "gm", "admin"）
// obj: 资源对象（如 "dungeon:10", "trade:publish", "pvp:attack"）
// act: 操作（如 "enter", "buy", "attack"）
func (e *Enforcer) Check(sub, obj, act string) bool {
	ok, err := e.enforcer.Enforce(sub, obj, act)
	if err != nil {
		logger.Error("Casbin 权限校验异常", "sub", sub, "obj", obj, "act", act, "err", err)
		return false
	}
	return ok
}

// AddPolicy 添加策略规则
// 如果策略已存在则不会重复添加
func (e *Enforcer) AddPolicy(sub, obj, act string) error {
	// AddPolicy 不会重复添加，返回 false 表示已存在，不视为错误
	_, err := e.enforcer.AddPolicy(sub, obj, act)
	if err != nil {
		return fmt.Errorf("添加策略失败 [%s, %s, %s]: %w", sub, obj, act, err)
	}
	return nil
}

// RemovePolicy 移除策略规则
func (e *Enforcer) RemovePolicy(sub, obj, act string) error {
	_, err := e.enforcer.RemovePolicy(sub, obj, act)
	if err != nil {
		return fmt.Errorf("移除策略失败 [%s, %s, %s]: %w", sub, obj, act, err)
	}
	return nil
}

// AddRoleForPlayer 为玩家分配角色
// 例如：AddRoleForPlayer("player:123", "player") 表示玩家123拥有普通玩家角色
func (e *Enforcer) AddRoleForPlayer(playerID string, role string) error {
	_, err := e.enforcer.AddGroupingPolicy(playerID, role)
	if err != nil {
		return fmt.Errorf("为玩家 %s 分配角色 %s 失败: %w", playerID, role, err)
	}
	return nil
}

// HasRole 检查玩家是否拥有指定角色
// 通过 Casbin 的角色继承关系判断
func (e *Enforcer) HasRole(playerID string, role string) bool {
	ok, err := e.enforcer.HasRoleForUser(playerID, role)
	if err != nil {
		logger.Error("检查玩家角色失败", "playerID", playerID, "role", role, "err", err)
		return false
	}
	return ok
}
