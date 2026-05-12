// Package service 实现游戏业务逻辑层。
// 每个 service 通过接口依赖 repo，不直接依赖 gateway.Conn，
// 实现业务逻辑与网络传输的解耦，便于单元测试和复用。
package service

import (
	"context"
	"encoding/json"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 玩家服务接口 ====================

// PlayerService 玩家服务接口，定义玩家登录、登出、查询、属性分配、角色创建等业务操作
type PlayerService interface {
	// Login 玩家登录，加载玩家数据并缓存到Redis
	Login(ctx context.Context, playerID uint64) (*model.Player, *errors.GameError)
	// Logout 玩家登出，保存数据并清理内存状态
	Logout(ctx context.Context, playerID uint64) *errors.GameError
	// GetPlayer 根据玩家ID获取玩家数据（先查缓存，未命中再查数据库）
	GetPlayer(ctx context.Context, playerID uint64) (*model.Player, *errors.GameError)
	// AssignAttr 分配属性点，将未分配的属性点加到指定属性上
	AssignAttr(ctx context.Context, playerID uint64, attr string, val int32) *errors.GameError
	// Create 创建新角色，指定角色名和职业类型
	Create(ctx context.Context, playerID uint64, name string, class int32) (*model.Player, *errors.GameError)
}

// ==================== 玩家服务实现 ====================

// playerService 玩家服务实现，依赖 PlayerRepo 和 CacheRepo
type playerService struct {
	playerRepo repo.PlayerRepo // 玩家数据访问接口
	cacheRepo  repo.CacheRepo  // 缓存访问接口
}

// NewPlayerService 创建玩家服务实例
func NewPlayerService(playerRepo repo.PlayerRepo, cacheRepo repo.CacheRepo) PlayerService {
	return &playerService{
		playerRepo: playerRepo,
		cacheRepo:  cacheRepo,
	}
}

// Login 玩家登录业务逻辑：
//  1. 从数据库加载玩家基础属性
//  2. 查询地下城通关进度（最高通关层数）
//  3. 计算生命值上限并恢复满血
//  4. 标记在线状态
//  5. 将玩家数据序列化后缓存到Redis
func (s *playerService) Login(ctx context.Context, playerID uint64) (*model.Player, *errors.GameError) {
	// 从数据库加载玩家基础数据
	p, err := s.playerRepo.GetPlayerByID(ctx, playerID)
	if err != nil {
		logger.Error("查询玩家数据失败", "player_id", playerID, "err", err)
		return nil, errors.ErrInternal
	}

	// 加载地下城通关进度
	maxLayer, _ := s.playerRepo.GetMaxLayer(ctx, playerID)
	p.MaxLayer = maxLayer

	// 根据等级和属性计算生命值上限，登录时恢复满血
	p.MaxHp = p.CalcMaxHp()
	p.Hp = p.MaxHp
	p.Online = true

	// 将玩家数据序列化后缓存到Redis，供其他服务快速查询
	data, err := json.Marshal(p)
	if err != nil {
		logger.Error("序列化玩家数据失败", "player_id", playerID, "err", err)
	} else {
		if err := s.cacheRepo.SetPlayerCache(ctx, playerID, data); err != nil {
			logger.Error("缓存玩家数据到Redis失败", "player_id", playerID, "err", err)
		}
	}

	logger.Info("玩家登录成功", "player_id", playerID, "name", p.Name)
	return p, nil
}

// Logout 玩家登出业务逻辑：
//  1. 从缓存或数据库获取玩家数据
//  2. 标记为离线
//  3. 持久化玩家数据到数据库
//  4. 清除Redis缓存
func (s *playerService) Logout(ctx context.Context, playerID uint64) *errors.GameError {
	// 获取玩家数据
	p, gameErr := s.GetPlayer(ctx, playerID)
	if gameErr != nil {
		return gameErr
	}

	// 标记为离线
	p.Online = false

	// 持久化到数据库
	if err := s.playerRepo.SavePlayer(ctx, p); err != nil {
		logger.Error("保存玩家数据失败", "player_id", playerID, "err", err)
		return errors.ErrInternal
	}

	// 清除Redis缓存
	if err := s.cacheRepo.DelPlayerCache(ctx, playerID); err != nil {
		logger.Error("清除玩家缓存失败", "player_id", playerID, "err", err)
	}

	logger.Info("玩家登出成功", "player_id", playerID)
	return nil
}

// GetPlayer 根据玩家ID获取玩家数据，优先从Redis缓存读取，未命中则查数据库
func (s *playerService) GetPlayer(ctx context.Context, playerID uint64) (*model.Player, *errors.GameError) {
	// 优先从Redis缓存读取
	data, err := s.cacheRepo.GetPlayerCache(ctx, playerID)
	if err == nil && data != nil {
		var p model.Player
		if jsonErr := json.Unmarshal(data, &p); jsonErr == nil {
			return &p, nil
		}
	}

	// 缓存未命中，从数据库加载
	p, err := s.playerRepo.GetPlayerByID(ctx, playerID)
	if err != nil {
		return nil, errors.ErrNotLogin
	}
	return p, nil
}

// AssignAttr 分配属性点业务逻辑：
//  1. 校验属性名是否合法（str/agi/int/con）
//  2. 校验可分配属性点是否充足
//  3. 扣减属性点，增加对应属性值
//  4. 重新计算生命值上限
//  5. 保存到数据库并刷新缓存
func (s *playerService) AssignAttr(ctx context.Context, playerID uint64, attr string, val int32) *errors.GameError {
	// 获取玩家数据
	p, gameErr := s.GetPlayer(ctx, playerID)
	if gameErr != nil {
		return gameErr
	}

	// 校验属性名合法性
	var validAttr bool
	switch attr {
	case "str", "agi", "int", "con":
		validAttr = true
	default:
		validAttr = false
	}
	if !validAttr {
		return errors.ErrParamInvalid
	}

	// 校验可分配属性点是否充足
	if p.AttrPoints < val || val <= 0 {
		return errors.ErrAttrPointsNotEnough
	}

	// 扣减属性点，增加对应属性值
	p.AttrPoints -= val
	switch attr {
	case "str":
		p.Str += val
	case "agi":
		p.Agi += val
	case "int":
		p.Int += val
	case "con":
		p.Con += val
	}

	// 属性变更后重新计算生命值上限
	p.MaxHp = p.CalcMaxHp()

	// 保存到数据库
	if err := s.playerRepo.SavePlayer(ctx, p); err != nil {
		logger.Error("保存玩家属性分配结果失败", "player_id", playerID, "err", err)
		return errors.ErrInternal
	}

	// 刷新Redis缓存
	s.refreshCache(ctx, playerID, p)

	return nil
}

// refreshCache 刷新玩家Redis缓存
func (s *playerService) refreshCache(ctx context.Context, playerID uint64, p *model.Player) {
	data, err := json.Marshal(p)
	if err != nil {
		logger.Error("序列化玩家数据失败", "player_id", playerID, "err", err)
		return
	}
	if err := s.cacheRepo.SetPlayerCache(ctx, playerID, data); err != nil {
		logger.Error("刷新玩家缓存失败", "player_id", playerID, "err", err)
	}
}

// Create 创建新角色业务逻辑：
//  1. 校验职业类型是否合法
//  2. 创建玩家记录到数据库
//  3. 计算初始生命值上限
//  4. 缓存到Redis
func (s *playerService) Create(ctx context.Context, playerID uint64, name string, class int32) (*model.Player, *errors.GameError) {
	// 校验职业类型是否合法
	if _, ok := model.ClassName[class]; !ok {
		return nil, errors.ErrClassInvalid
	}

	// 创建玩家记录到数据库
	newID, err := s.playerRepo.CreatePlayer(ctx, playerID, name, class)
	if err != nil {
		logger.Error("创建玩家失败", "player_id", playerID, "name", name, "err", err)
		return nil, errors.ErrNameDuplicate
	}

	// 构建玩家对象
	p := &model.Player{
		ID:         newID,
		Name:       name,
		Class:      class,
		Level:      1,
		AttrPoints: 5,
		Online:     true,
	}
	p.MaxHp = p.CalcMaxHp()
	p.Hp = p.MaxHp

	// 缓存到Redis
	s.refreshCache(ctx, newID, p)

	logger.Info("创建角色成功", "player_id", newID, "name", name, "class", class)
	return p, nil
}
