// Package player 实现游戏业务逻辑层。
// 每个 service 通过接口依赖 repo，不直接依赖 gateway.Conn，
// 实现业务逻辑与网络传输的解耦，便于单元测试和复用。
package player

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
	// Logout 玩家登出（按ID从缓存/DB加载后保存），优先使用 LogoutWithPlayer
	Logout(ctx context.Context, playerID uint64) *errors.GameError
	// LogoutWithPlayer 直接使用内存中的玩家对象进行持久化和缓存清理
	LogoutWithPlayer(ctx context.Context, p *model.Player) *errors.GameError
	// GetPlayer 根据玩家ID获取玩家数据（先查缓存，未命中再查数据库）
	GetPlayer(ctx context.Context, playerID uint64) (*model.Player, *errors.GameError)
	// AssignAttr 分配属性点，将未分配的属性点加到指定属性上（直接操作内存中的玩家对象）
	AssignAttr(ctx context.Context, player *model.Player, attr string, val int32) *errors.GameError
	// Create 创建新角色，指定角色名和职业类型
	Create(ctx context.Context, playerID uint64, name string, class int32) (*model.Player, *errors.GameError)
	// AddExp 增加经验值，经验达到阈值自动升级
	AddExp(ctx context.Context, player *model.Player, exp int64) (*AddExpResult, *errors.GameError)
}

// ==================== 经验增加结果结构体 ====================

// AddExpResult 经验增加结果
type AddExpResult struct {
	ExpGained   int64 // 本次获得的经验
	OldLevel    int32 // 升级前等级
	NewLevel    int32 // 升级后等级
	LevelUped   bool  // 是否发生了升级
	ExpOverflow int64 // 溢出的经验（超过满级的部分）
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
		logger.TError(ctx, "查询玩家数据失败", "player_id", playerID, "err", err)
		return nil, errors.ErrInternal
	}
	if p == nil {
		return nil, errors.ErrNotLogin
	}

	// 加载地下城通关进度
	maxLayer, _ := s.playerRepo.GetMaxLayer(ctx, playerID)

	// 根据等级和属性计算生命值上限，登录时恢复满血
	p.Mu().Lock()
	p.MaxLayer = maxLayer
	p.MaxHp = p.CalcMaxHp()
	p.Hp = p.MaxHp
	p.Online = true
	p.Mu().Unlock()

	// 将玩家数据序列化后缓存到Redis，供其他服务快速查询
	s.refreshCache(ctx, playerID, p)

	logger.TInfo(ctx, "玩家登录成功", "player_id", playerID, "name", p.Name)
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

	return s.LogoutWithPlayer(ctx, p)
}

// LogoutWithPlayer 直接使用内存中的玩家对象进行登出：
//  1. 标记为离线
//  2. 持久化玩家数据到数据库
//  3. 清除Redis缓存
func (s *playerService) LogoutWithPlayer(ctx context.Context, p *model.Player) *errors.GameError {
	// 标记为离线
	p.Mu().Lock()
	p.Online = false
	playerID := p.ID
	p.Mu().Unlock()

	// 持久化到数据库
	if err := s.playerRepo.SavePlayer(ctx, p); err != nil {
		logger.TError(ctx, "保存玩家数据失败", "player_id", playerID, "err", err)
		return errors.ErrInternal
	}

	// 清除Redis缓存
	if err := s.cacheRepo.DelPlayerCache(ctx, playerID); err != nil {
		logger.TError(ctx, "清除玩家缓存失败", "player_id", playerID, "err", err)
	}

	logger.TInfo(ctx, "玩家登出成功", "player_id", playerID)
	return nil
}

// GetPlayer 根据玩家ID获取玩家数据，优先从Redis缓存读取，未命中则查数据库
// 注意：从缓存反序列化返回的是快照副本，不应用于写操作。
// 写操作应使用 GameManager 中的内存在线玩家对象。
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
	if p == nil {
		return nil, errors.ErrNotLogin
	}
	return p, nil
}

// AssignAttr 分配属性点业务逻辑（修复：直接操作传入的内存玩家对象，避免缓存快照导致脏写）：
//  1. 校验属性名是否合法（str/agi/int/con）
//  2. 校验可分配属性点是否充足
//  3. 扣减属性点，增加对应属性值
//  4. 重新计算生命值上限
//  5. 保存到数据库并刷新缓存
func (s *playerService) AssignAttr(ctx context.Context, player *model.Player, attr string, val int32) *errors.GameError {
	if player == nil {
		return errors.ErrNotLogin
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

	// 校验可分配属性点是否充足 + 扣减属性点 + 增加属性值 + 重算HP（同一把锁内完成）
	player.Mu().Lock()
	if player.AttrPoints < val || val <= 0 {
		player.Mu().Unlock()
		return errors.ErrAttrPointsNotEnough
	}
	player.AttrPoints -= val
	switch attr {
	case "str":
		player.Str += val
	case "agi":
		player.Agi += val
	case "int":
		player.Int += val
	case "con":
		player.Con += val
	}
	player.MaxHp = player.CalcMaxHp()
	playerID := player.ID
	player.Mu().Unlock()

	// 保存到数据库
	if err := s.playerRepo.SavePlayer(ctx, player); err != nil {
		logger.TError(ctx, "保存玩家属性分配结果失败", "player_id", playerID, "err", err)
		return errors.ErrInternal
	}

	// 刷新Redis缓存
	s.refreshCache(ctx, playerID, player)

	return nil
}

// AddExp 增加经验值，经验达到阈值自动升级。
// 使用 ExpTable 查表判定是否升级，支持连续升级。
// 调用方传入内存中的实时玩家对象，保证数据一致性。
func (s *playerService) AddExp(ctx context.Context, player *model.Player, exp int64) (*AddExpResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}
	if exp <= 0 {
		return nil, errors.ErrParamInvalid
	}

	player.Mu().Lock()
	oldLevel := player.Level
	player.Exp += exp
	playerID := player.ID

	// 循环升级：经验达到 ExpTable[level] 就升级，直到无法继续
	levelUped := false
	for {
		// 已达满级（ExpTable 下标最大为 len-1，等级不能超过 len-1）
		if int(player.Level) >= len(model.ExpTable) {
			break
		}
		required := model.ExpTable[player.Level]
		if required <= 0 {
			break
		}
		if player.Exp < required {
			break
		}
		player.Exp -= required
		player.Level++
		player.AttrPoints += 3 // 每次升级获得3点属性点
		levelUped = true
	}

	// 升级后重算生命值上限
	if levelUped {
		player.MaxHp = player.CalcMaxHp()
	}
	player.Mu().Unlock()

	// 持久化到数据库
	if saveErr := s.playerRepo.SavePlayer(ctx, player); saveErr != nil {
		logger.TError(ctx, "保存玩家升级结果失败", "player_id", playerID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	// 刷新缓存
	s.refreshCache(ctx, playerID, player)

	result := &AddExpResult{
		ExpGained: exp,
		OldLevel:  oldLevel,
		NewLevel:  player.Level,
		LevelUped: levelUped,
	}

	if levelUped {
		logger.TInfo(ctx, "玩家升级", "player_id", playerID, "old_level", oldLevel, "new_level", player.Level)
	}

	return result, nil
}

// playerCacheDTO only contains persistent fields for cache serialization,
// excluding runtime fields like X, Y, Layer, Online, InvincibleUntil, mu.
type playerCacheDTO struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	Class      int32  `json:"class"`
	Level      int32  `json:"level"`
	Exp        int64  `json:"exp"`
	Gold       int64  `json:"gold"`
	Honor      int32  `json:"honor"`
	KillValue  int32  `json:"kill_value"`
	Str        int32  `json:"str"`
	Agi        int32  `json:"agi"`
	Int        int32  `json:"int"`
	Con        int32  `json:"con"`
	Def        int32  `json:"def"`
	AttrPoints int32  `json:"attr_points"`
	MaxLayer   int32  `json:"max_layer"`
}

// toCacheDTO converts a Player to its cache DTO (persistent fields only)
func toCacheDTO(p *model.Player) *playerCacheDTO {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return &playerCacheDTO{
		ID:         p.ID,
		Name:       p.Name,
		Class:      p.Class,
		Level:      p.Level,
		Exp:        p.Exp,
		Gold:       p.Gold,
		Honor:      p.Honor,
		KillValue:  p.KillValue,
		Str:        p.Str,
		Agi:        p.Agi,
		Int:        p.Int,
		Con:        p.Con,
		Def:        p.Def,
		AttrPoints: p.AttrPoints,
		MaxLayer:   p.MaxLayer,
	}
}

// refreshCache 刷新玩家Redis缓存
func (s *playerService) refreshCache(ctx context.Context, playerID uint64, p *model.Player) {
	dto := toCacheDTO(p)
	data, err := json.Marshal(dto)
	if err != nil {
		logger.TError(ctx, "序列化玩家数据失败", "player_id", playerID, "err", err)
		return
	}
	if err := s.cacheRepo.SetPlayerCache(ctx, playerID, data); err != nil {
		logger.TError(ctx, "刷新玩家缓存失败", "player_id", playerID, "err", err)
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
		logger.TError(ctx, "创建玩家失败", "player_id", playerID, "name", name, "err", err)
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

	logger.TInfo(ctx, "创建角色成功", "player_id", newID, "name", name, "class", class)
	return p, nil
}
