// Package raid - 战局（撤离模式）服务
// 提供进入/离开战局、搜索战利品、撤离、战局PvP等撤离模式核心业务逻辑。
// 设计参考逃离塔科夫：玩家进入限时战局，搜集战利品后前往撤离点安全撤出，
// 死亡或超时则丢失所有战局内物品。
package raid

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== PvP结果结构体 ====================

// RaidPvpResult 战局PvP攻击结果
type RaidPvpResult struct {
	TargetID uint64 // 目标玩家ID
	Damage   int64  // 造成的伤害
	CurrHp   int64  // 目标当前HP
	IsDead   bool   // 目标是否死亡
}

// ==================== 战局服务接口 ====================

// RaidService 战局服务接口，定义战局相关业务操作
type RaidService interface {
	// StartRaid 进入战局，根据模板创建战局实例
	StartRaid(ctx context.Context, player *model.Player, templateID int32) (*model.RaidMap, *errors.GameError)
	// LeaveRaid 主动离开战局（丢弃所有战利品）
	LeaveRaid(ctx context.Context, player *model.Player) *errors.GameError
	// OpenLootContainer 打开战利品容器，生成随机物品
	OpenLootContainer(ctx context.Context, player *model.Player, containerID uint64) ([]model.RaidLootItem, *errors.GameError)
	// PickUpLoot 从最近打开的容器中拾取物品到战局背包
	PickUpLoot(ctx context.Context, player *model.Player, itemIndex int32) *errors.GameError
	// DiscardLoot 丢弃战局背包中的物品
	DiscardLoot(ctx context.Context, player *model.Player, itemIndex int32) *errors.GameError
	// AttemptExtraction 开始撤离（需在撤离点范围内），返回撤离倒计时秒数
	AttemptExtraction(ctx context.Context, player *model.Player, pointID int32) (int32, *errors.GameError)
	// CancelExtraction 取消撤离
	CancelExtraction(ctx context.Context, player *model.Player) *errors.GameError
	// TransferLoot 撤离成功后将战利品转入永久仓库
	TransferLoot(ctx context.Context, player *model.Player)
	// DiscardRaidLoot 死亡/超时/放弃时丢弃战利品
	DiscardRaidLoot(player *model.Player)
	// GetRaidMap 获取玩家当前所在战局
	GetRaidMap(playerID uint64) *model.RaidMap
	// GetAllRaids 获取所有活跃战局
	GetAllRaids() []*model.RaidMap
	// HandleRaidDeath 处理战局内玩家死亡
	HandleRaidDeath(player *model.Player)
	// HandleRaidTimeout 处理战局超时
	HandleRaidTimeout(player *model.Player)
	// ListAvailableMaps 列出可用的战局地图模板
	ListAvailableMaps(ctx context.Context, player *model.Player) ([]*model.RaidMapTemplate, *errors.GameError)
	// GetStash 获取玩家的战局仓库（成功撤离保存的物品）
	GetStash(player *model.Player) []*model.RaidLootItem
	// RaidPvpAttack 战局内PvP攻击
	RaidPvpAttack(ctx context.Context, attacker *model.Player, targetID uint64) (*RaidPvpResult, *errors.GameError)
	// TickRaidTimers 定时调用：处理撤离倒计时和战局超时
	TickRaidTimers(world iface.World)
	// TickRaidMonsterAI 定时调用：处理战局内怪物AI
	TickRaidMonsterAI(world iface.World)
}

// ==================== 战局服务实现 ====================

// raidService 战局服务实现
type raidService struct {
	raids sync.Map // key: playerID → *model.RaidMap（玩家→所在战局的映射）
}

// NewRaidService 创建战局服务实例
func NewRaidService() RaidService {
	return &raidService{}
}

// StartRaid 进入战局业务逻辑：
//  1. 校验玩家未在战局中
//  2. 校验模板存在
//  3. 调用 model.CreateRaidMap 创建战局
//  4. 存储玩家→战局映射
//  5. 返回战局实例
func (s *raidService) StartRaid(ctx context.Context, player *model.Player, templateID int32) (*model.RaidMap, *errors.GameError) {
	// 校验玩家未在战局中
	player.Mu().RLock()
	inRaid := player.RaidState != nil
	player.Mu().RUnlock()
	if inRaid {
		return nil, errors.ErrRaidAlreadyIn
	}

	// 校验模板存在
	if model.GetRaidMapTemplate(templateID) == nil {
		return nil, errors.ErrRaidTemplateNotFound
	}

	// 创建战局（内部会设置 player.RaidState）
	raid := model.CreateRaidMap(templateID, player.ID, player)
	if raid == nil {
		return nil, errors.ErrRaidTemplateNotFound
	}

	// 存储映射
	s.raids.Store(player.ID, raid)

	logger.TInfo(ctx, "玩家进入战局", "player_id", player.ID,
		"template_id", templateID, "map_name", raid.Name, "raid_id", raid.ID)
	return raid, nil
}

// LeaveRaid 主动离开战局（丢弃战利品）：
//  1. 校验玩家在战局中
//  2. 丢弃战利品
//  3. 从战局中移除玩家
//  4. 清除战局状态
func (s *raidService) LeaveRaid(ctx context.Context, player *model.Player) *errors.GameError {
	player.Mu().RLock()
	rs := player.RaidState
	player.Mu().RUnlock()

	if rs == nil {
		return errors.ErrRaidNotIn
	}

	// 丢弃战利品
	s.DiscardRaidLoot(player)

	// 从战局中移除玩家
	raid := s.GetRaidMap(player.ID)
	if raid != nil {
		raid.Mu().Lock()
		delete(raid.Players, player.ID)
		raid.Mu().Unlock()
	}

	// 清除映射和状态
	s.raids.Delete(player.ID)
	player.Mu().Lock()
	player.RaidState = nil
	player.Mu().Unlock()

	logger.TInfo(ctx, "玩家离开战局", "player_id", player.ID)
	return nil
}

// OpenLootContainer 打开战利品容器：
//  1. 校验玩家在战局中
//  2. 查找容器
//  3. 锁定容器，检查未打开
//  4. 根据战利品表生成随机物品
//  5. 标记已打开，返回物品列表
func (s *raidService) OpenLootContainer(ctx context.Context, player *model.Player, containerID uint64) ([]model.RaidLootItem, *errors.GameError) {
	raid := s.GetRaidMap(player.ID)
	if raid == nil {
		return nil, errors.ErrRaidNotIn
	}

	// 查找容器
	raid.Mu().RLock()
	container, ok := raid.LootContainers[containerID]
	raid.Mu().RUnlock()
	if !ok {
		return nil, errors.ErrRaidContainerNotFound
	}

	// 锁定容器
	container.Mu().Lock()
	defer container.Mu().Unlock()

	// 检查未打开
	if container.Opened {
		return nil, errors.ErrRaidContainerOpened
	}

	// 获取战局模板的品质范围
	player.Mu().RLock()
	rs := player.RaidState
	player.Mu().RUnlock()
	if rs == nil {
		return nil, errors.ErrRaidNotIn
	}

	tmpl := model.GetRaidMapTemplate(raid.TemplateID)
	minTier := int32(1)
	maxTier := int32(3)
	if tmpl != nil {
		minTier = tmpl.MinLootTier
		maxTier = tmpl.MaxLootTier
	}

	// 生成战利品
	items := model.GenerateRaidLoot(container.LootTable, minTier, maxTier)
	container.Items = items
	container.Opened = true

	// 记录最近打开的容器ID，供 PickUpLoot 使用
	player.Mu().Lock()
	if player.RaidState != nil {
		player.RaidState.LastOpenedContainerID = containerID
	}
	player.Mu().Unlock()

	logger.TInfo(ctx, "打开战利品容器", "player_id", player.ID,
		"container_id", containerID, "items_count", len(items))
	return items, nil
}

// PickUpLoot 从最近打开的容器中拾取指定物品到战局背包：
//  1. 校验玩家在战局中
//  2. 从 RaidState 获取最近打开的容器ID
//  3. 查找容器，确认已打开
//  4. 校验物品索引有效
//  5. 将物品从容器移到战局背包
func (s *raidService) PickUpLoot(ctx context.Context, player *model.Player, itemIndex int32) *errors.GameError {
	raid := s.GetRaidMap(player.ID)
	if raid == nil {
		return errors.ErrRaidNotIn
	}

	// 获取最近打开的容器ID
	player.Mu().RLock()
	rs := player.RaidState
	if rs == nil {
		player.Mu().RUnlock()
		return errors.ErrRaidNotIn
	}
	containerID := rs.LastOpenedContainerID
	player.Mu().RUnlock()

	if containerID == 0 {
		return errors.ErrRaidContainerNotFound
	}

	// 查找容器
	raid.Mu().RLock()
	container, ok := raid.LootContainers[containerID]
	raid.Mu().RUnlock()
	if !ok {
		return errors.ErrRaidContainerNotFound
	}

	// 锁定容器，取出物品
	container.Mu().Lock()
	if !container.Opened {
		container.Mu().Unlock()
		return errors.ErrRaidContainerNotFound
	}
	if itemIndex < 0 || int(itemIndex) >= len(container.Items) {
		container.Mu().Unlock()
		return errors.ErrRaidLootIndexInvalid
	}
	item := container.Items[itemIndex]
	// 从容器中移除
	container.Items = append(container.Items[:itemIndex], container.Items[itemIndex+1:]...)
	container.Mu().Unlock()

	// 加入战局背包
	player.Mu().Lock()
	if player.RaidState != nil {
		player.RaidState.RaidInventory = append(player.RaidState.RaidInventory, &item)
	}
	player.Mu().Unlock()

	logger.TInfo(ctx, "拾取战利品", "player_id", player.ID,
		"item_id", item.ItemID, "name", item.Name, "count", item.Count)
	return nil
}

// DiscardLoot 丢弃战局背包中指定索引的物品
func (s *raidService) DiscardLoot(ctx context.Context, player *model.Player, itemIndex int32) *errors.GameError {
	player.Mu().Lock()
	defer player.Mu().Unlock()

	rs := player.RaidState
	if rs == nil {
		return errors.ErrRaidNotIn
	}
	if itemIndex < 0 || int(itemIndex) >= len(rs.RaidInventory) {
		return errors.ErrRaidLootIndexInvalid
	}

	// 移除物品
	rs.RaidInventory = append(rs.RaidInventory[:itemIndex], rs.RaidInventory[itemIndex+1:]...)
	return nil
}

// AttemptExtraction 开始撤离：
//  1. 校验玩家在战局中
//  2. 查找撤离点
//  3. 距离检查（玩家坐标与撤离点的距离）
//  4. 设置撤离状态
//  5. 返回撤离倒计时秒数
func (s *raidService) AttemptExtraction(ctx context.Context, player *model.Player, pointID int32) (int32, *errors.GameError) {
	raid := s.GetRaidMap(player.ID)
	if raid == nil {
		return 0, errors.ErrRaidNotIn
	}

	// 查找撤离点
	point := raid.FindExtractionPoint(pointID)
	if point == nil {
		return 0, errors.ErrRaidNotNearExtraction
	}

	// 距离检查
	player.Mu().RLock()
	px, py := player.X, player.Y
	player.Mu().RUnlock()

	dist := math.Sqrt(math.Pow(px-point.X, 2) + math.Pow(py-point.Y, 2))
	if dist > point.Radius {
		return 0, errors.ErrRaidNotNearExtraction
	}

	// 设置撤离状态
	player.Mu().Lock()
	if player.RaidState != nil {
		player.RaidState.Extracting = true
		player.RaidState.ExtractTimer = point.ExtractDuration
		player.RaidState.ExtractPointID = pointID
	}
	player.Mu().Unlock()

	logger.TInfo(ctx, "开始撤离", "player_id", player.ID,
		"point_id", pointID, "duration", point.ExtractDuration)
	return point.ExtractDuration, nil
}

// CancelExtraction 取消撤离
func (s *raidService) CancelExtraction(ctx context.Context, player *model.Player) *errors.GameError {
	player.Mu().Lock()
	defer player.Mu().Unlock()

	rs := player.RaidState
	if rs == nil {
		return errors.ErrRaidNotIn
	}
	if !rs.Extracting {
		return errors.ErrRaidNotExtracting
	}

	rs.Extracting = false
	rs.ExtractTimer = 0
	rs.ExtractPointID = 0

	logger.TInfo(ctx, "取消撤离", "player_id", player.ID)
	return nil
}

// TransferLoot 撤离成功后将战利品转入永久仓库
func (s *raidService) TransferLoot(ctx context.Context, player *model.Player) {
	player.Mu().Lock()
	defer player.Mu().Unlock()

	rs := player.RaidState
	if rs == nil {
		return
	}

	// 将战局背包物品转移到永久仓库
	for _, item := range rs.RaidInventory {
		if item == nil {
			continue
		}
		player.RaidStash = append(player.RaidStash, item)
		// 同时加入背包物品（按物品ID累加数量）
		if player.Items == nil {
			player.Items = make(map[uint32]int32)
		}
		player.Items[uint32(item.ItemID)] += item.Count
	}

	logger.TInfo(ctx, "战利品转移完成", "player_id", player.ID,
		"items_count", len(rs.RaidInventory))

	// 清空战局背包
	rs.RaidInventory = nil
}

// DiscardRaidLoot 死亡/超时/放弃时丢弃所有战利品
func (s *raidService) DiscardRaidLoot(player *model.Player) {
	player.Mu().Lock()
	defer player.Mu().Unlock()

	rs := player.RaidState
	if rs == nil {
		return
	}

	discarded := len(rs.RaidInventory)
	rs.RaidInventory = nil
	rs.Extracting = false
	rs.ExtractTimer = 0

	if discarded > 0 {
		logger.Info("战利品已丢弃", "player_id", player.ID, "discarded", discarded)
	}
}

// GetRaidMap 获取玩家当前所在战局
func (s *raidService) GetRaidMap(playerID uint64) *model.RaidMap {
	if v, ok := s.raids.Load(playerID); ok {
		return v.(*model.RaidMap)
	}
	return nil
}

// GetAllRaids 获取所有活跃战局（去重）
func (s *raidService) GetAllRaids() []*model.RaidMap {
	seen := make(map[uint64]bool)
	var raids []*model.RaidMap
	s.raids.Range(func(key, value any) bool {
		r := value.(*model.RaidMap)
		if !seen[r.ID] {
			seen[r.ID] = true
			raids = append(raids, r)
		}
		return true
	})
	return raids
}

// HandleRaidDeath 处理战局内玩家死亡：
//  1. 丢弃战利品
//  2. 从战局中移除
//  3. 清除战局状态
//  4. 复活玩家
func (s *raidService) HandleRaidDeath(player *model.Player) {
	// 丢弃战利品
	s.DiscardRaidLoot(player)

	// 从战局中移除
	raid := s.GetRaidMap(player.ID)
	if raid != nil {
		raid.Mu().Lock()
		delete(raid.Players, player.ID)
		raid.Mu().Unlock()
	}

	// 清除映射和状态，复活玩家
	s.raids.Delete(player.ID)
	player.Mu().Lock()
	player.RaidState = nil
	player.Hp = player.MaxHp // 复活满血
	player.X = 0
	player.Y = 0
	player.Mu().Unlock()

	logger.Info("战局内玩家死亡", "player_id", player.ID)
}

// HandleRaidTimeout 处理战局超时（与死亡处理相同，丢弃战利品）
func (s *raidService) HandleRaidTimeout(player *model.Player) {
	s.HandleRaidDeath(player)
	logger.Info("战局超时，玩家强制退出", "player_id", player.ID)
}

// ListAvailableMaps 列出可用的战局地图模板
func (s *raidService) ListAvailableMaps(ctx context.Context, player *model.Player) ([]*model.RaidMapTemplate, *errors.GameError) {
	templates := make([]*model.RaidMapTemplate, 0, len(model.RaidMapTemplates))
	for _, t := range model.RaidMapTemplates {
		templates = append(templates, t)
	}
	return templates, nil
}

// GetStash 获取玩家的战局仓库
func (s *raidService) GetStash(player *model.Player) []*model.RaidLootItem {
	player.Mu().RLock()
	defer player.Mu().RUnlock()
	return player.RaidStash
}

// RaidPvpAttack 战局内PvP攻击：
//  1. 校验双方在同一战局
//  2. 校验目标在PvP区域内
//  3. 计算伤害（攻击者攻击力 * 随机波动 - 目标防御力）
//  4. 扣减目标血量
//  5. 目标死亡时调用 HandleRaidDeath
func (s *raidService) RaidPvpAttack(ctx context.Context, attacker *model.Player, targetID uint64) (*RaidPvpResult, *errors.GameError) {
	// 校验攻击者在战局中
	attackerRaid := s.GetRaidMap(attacker.ID)
	if attackerRaid == nil {
		return nil, errors.ErrRaidNotIn
	}

	// 校验目标在同一战局
	targetRaid := s.GetRaidMap(targetID)
	if targetRaid == nil || targetRaid.ID != attackerRaid.ID {
		return nil, errors.ErrRaidNotSameMap
	}

	// 获取目标玩家引用
	attackerRaid.Mu().RLock()
	target, ok := attackerRaid.Players[targetID]
	attackerRaid.Mu().RUnlock()
	if !ok || target == nil {
		return nil, errors.ErrRaidNotIn
	}

	// 校验目标在PvP区域内
	target.Mu().RLock()
	tx, ty := target.X, target.Y
	targetHp := target.Hp
	target.Mu().RUnlock()

	if targetHp <= 0 {
		return nil, errors.ErrTargetDead
	}

	zone := attackerRaid.FindZone(tx, ty)
	if zone == nil || !zone.PvPEnabled {
		return nil, errors.ErrRaidTargetNotInPvP
	}

	// 检查攻击者是否存活
	attacker.Mu().RLock()
	attackerHp := attacker.Hp
	attacker.Mu().RUnlock()
	if attackerHp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 计算伤害：攻击者攻击力 * 随机(0.8~1.2) - 目标防御力
	attacker.Mu().RLock()
	atkPower := attacker.CalcAttack()
	attacker.Mu().RUnlock()

	target.Mu().RLock()
	defPower := target.CalcDefense()
	target.Mu().RUnlock()

	// 随机波动 0.8~1.2
	multiplier := 0.8 + rand.Float64()*0.4
	rawDamage := int64(float64(atkPower) * multiplier)
	damage := rawDamage - defPower
	if damage < 1 {
		damage = 1
	}

	// 按ID排序加锁，防止死锁
	first, second := attacker, target
	if first.ID > second.ID {
		first, second = second, first
	}
	first.Mu().Lock()
	second.Mu().Lock()

	target.Hp -= damage
	if target.Hp < 0 {
		target.Hp = 0
	}
	currHp := target.Hp
	isDead := currHp <= 0

	second.Mu().Unlock()
	first.Mu().Unlock()

	result := &RaidPvpResult{
		TargetID: targetID,
		Damage:   damage,
		CurrHp:   currHp,
		IsDead:   isDead,
	}

	// 目标死亡处理
	if isDead {
		s.HandleRaidDeath(target)
	}

	logger.TInfo(ctx, "战局PvP攻击", "attacker_id", attacker.ID,
		"target_id", targetID, "damage", damage, "is_dead", isDead)
	return result, nil
}

// ==================== 定时任务方法 ====================

// TickRaidTimers 定时调用（建议每秒一次）：
//  1. 遍历所有战局
//  2. 对正在撤离的玩家：递减倒计时，到0则完成撤离
//  3. 对战局剩余时间为0：所有玩家超时处理
func (s *raidService) TickRaidTimers(world iface.World) {
	// 收集需要超时处理的战局ID和需要撤离成功的玩家
	type extractDone struct {
		player *model.Player
	}
	var extractPlayers []extractDone
	var timeoutRaids []uint64

	// 遍历所有活跃战局
	processedRaids := make(map[uint64]bool)
	s.raids.Range(func(key, value any) bool {
		playerID := key.(uint64)
		raid := value.(*model.RaidMap)

		// 检查战局超时（每个战局只检查一次）
		if !processedRaids[raid.ID] {
			processedRaids[raid.ID] = true
			if raid.RemainingSeconds() <= 0 {
				timeoutRaids = append(timeoutRaids, raid.ID)
			}
		}

		// 检查玩家撤离倒计时
		p := world.GetOnlinePlayer(playerID)
		if p == nil {
			return true
		}
		p.Mu().RLock()
		rs := p.RaidState
		if rs == nil {
			p.Mu().RUnlock()
			return true
		}
		if rs.Extracting {
			p.Mu().RUnlock()
			p.Mu().Lock()
			rs = p.RaidState
			if rs != nil && rs.Extracting {
				rs.ExtractTimer--
				if rs.ExtractTimer <= 0 {
					rs.Extracting = false
					extractPlayers = append(extractPlayers, extractDone{player: p})
				}
			}
			p.Mu().Unlock()
		} else {
			p.Mu().RUnlock()
		}
		return true
	})

	// 处理超时战局
	for _, raidID := range timeoutRaids {
		s.raids.Range(func(key, value any) bool {
			raid := value.(*model.RaidMap)
			if raid.ID != raidID {
				return true
			}
			p := world.GetOnlinePlayer(key.(uint64))
			if p != nil {
				s.HandleRaidTimeout(p)
			}
			return true
		})
	}

	// 处理撤离成功的玩家
	for _, ep := range extractPlayers {
		ctx := context.Background()
		s.TransferLoot(ctx, ep.player)

		// 从战局中移除
		raid := s.GetRaidMap(ep.player.ID)
		if raid != nil {
			raid.Mu().Lock()
			delete(raid.Players, ep.player.ID)
			raid.Mu().Unlock()
		}
		s.raids.Delete(ep.player.ID)
		ep.player.Mu().Lock()
		ep.player.RaidState = nil
		ep.player.Mu().Unlock()

		logger.Info("玩家成功撤离", "player_id", ep.player.ID)
	}
}

// TickRaidMonsterAI 定时调用（建议每秒一次）：处理战局内怪物AI
// 复用 scheduler/monster_ai.go 的状态机逻辑，但作用于 RaidMap 而非 DungeonLayer
func (s *raidService) TickRaidMonsterAI(world iface.World) {
	now := time.Now().Unix()

	processedRaids := make(map[uint64]bool)
	s.raids.Range(func(key, value any) bool {
		raid := value.(*model.RaidMap)
		if processedRaids[raid.ID] {
			return true
		}
		processedRaids[raid.ID] = true

		// 收集战局内所有存活玩家位置
		var players []raidPlayerPos

		raid.Mu().RLock()
		for pid := range raid.Players {
			p := world.GetOnlinePlayer(pid)
			if p == nil {
				continue
			}
			p.Mu().RLock()
			if p.Hp > 0 {
				players = append(players, raidPlayerPos{id: p.ID, x: p.X, y: p.Y, hp: p.Hp})
			}
			p.Mu().RUnlock()
		}

		// 收集所有怪物
		monsters := make([]*model.Monster, 0, len(raid.Monsters))
		for _, m := range raid.Monsters {
			monsters = append(monsters, m)
		}
		raid.Mu().RUnlock()

		if len(players) == 0 {
			// 没有玩家，所有怪物回归空闲
			for _, m := range monsters {
				m.Mu().Lock()
				if m.AIState != model.MonsterAIIdle && !m.Dead {
					m.AIState = model.MonsterAIIdle
					m.TargetID = 0
					m.X = m.SpawnX
					m.Y = m.SpawnY
					m.Hp = m.MaxHp
				}
				m.Mu().Unlock()
			}
			return true
		}

		// 处理每只怪物的AI
		for _, m := range monsters {
			m.Mu().RLock()
			dead := m.Dead
			m.Mu().RUnlock()
			if dead {
				continue
			}
			s.processRaidMonsterAI(m, players, now, world, raid)
		}
		return true
	})
}

// raidPlayerPos 战局内玩家位置快照（内部使用）
type raidPlayerPos struct {
	id uint64
	x  float64
	y  float64
	hp int64
}

// processRaidMonsterAI 处理战局内单个怪物的AI状态机
func (s *raidService) processRaidMonsterAI(m *model.Monster, players []raidPlayerPos, now int64, world iface.World, raid *model.RaidMap) {
	m.Mu().Lock()
	defer m.Mu().Unlock()

	// 辅助函数：查找最近玩家
	findNearest := func() *raidPlayerPos {
		var nearest *raidPlayerPos
		minDist := 1e9
		for i := range players {
			dx := m.X - players[i].x
			dy := m.Y - players[i].y
			d := math.Sqrt(dx*dx + dy*dy)
			if d < minDist {
				minDist = d
				nearest = &players[i]
			}
		}
		return nearest
	}

	findByID := func(id uint64) *raidPlayerPos {
		for i := range players {
			if players[i].id == id {
				return &players[i]
			}
		}
		return nil
	}

	distToPos := func(px, py float64) float64 {
		dx := m.X - px
		dy := m.Y - py
		return math.Sqrt(dx*dx + dy*dy)
	}

	switch m.AIState {
	case model.MonsterAIIdle:
		nearest := findNearest()
		if nearest != nil && distToPos(nearest.x, nearest.y) <= m.AggroRange {
			m.AIState = model.MonsterAIAlert
			m.TargetID = nearest.id
		}

	case model.MonsterAIAlert:
		target := findByID(m.TargetID)
		if target == nil || distToPos(target.x, target.y) > m.AggroRange*1.5 {
			m.AIState = model.MonsterAIIdle
			m.TargetID = 0
			return
		}
		m.AIState = model.MonsterAIChase

	case model.MonsterAIChase:
		target := findByID(m.TargetID)
		if target == nil {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}
		dist := distToPos(target.x, target.y)
		if dist > m.AggroRange*2 {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}
		if dist <= m.AtkRange {
			m.AIState = model.MonsterAIAttack
			return
		}
		// 向玩家移动
		speed := raidMonsterMoveSpeed(m)
		dx := target.x - m.X
		dy := target.y - m.Y
		if dist > speed {
			ratio := speed / dist
			m.X += dx * ratio
			m.Y += dy * ratio
		} else {
			m.X = target.x
			m.Y = target.y
		}

	case model.MonsterAIAttack:
		target := findByID(m.TargetID)
		if target == nil {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}
		dist := distToPos(target.x, target.y)
		if dist > m.AtkRange*1.2 {
			m.AIState = model.MonsterAIChase
			return
		}
		if now-m.LastAtkTime < int64(m.AtkCD) {
			return
		}
		m.LastAtkTime = now
		// 攻击玩家
		raidMonsterAttackPlayer(m, target.id, world)

	case model.MonsterAIReturn:
		dx := m.SpawnX - m.X
		dy := m.SpawnY - m.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist < 1 {
			m.X = m.SpawnX
			m.Y = m.SpawnY
			m.AIState = model.MonsterAIIdle
			m.Hp = m.MaxHp
			return
		}
		speed := raidMonsterMoveSpeed(m) * 1.5
		ratio := speed / dist
		m.X += dx * ratio
		m.Y += dy * ratio
		// 途中遇到玩家重新追击
		nearest := findNearest()
		if nearest != nil && distToPos(nearest.x, nearest.y) <= m.AggroRange {
			m.AIState = model.MonsterAIChase
			m.TargetID = nearest.id
		}
	}
}

// raidMonsterMoveSpeed 根据怪物行为类型返回移动速度
func raidMonsterMoveSpeed(m *model.Monster) float64 {
	switch m.Behavior {
	case model.MonsterBehaviorMelee:
		return 3.0
	case model.MonsterBehaviorRanged:
		return 1.5
	case model.MonsterBehaviorTank:
		return 1.0
	case model.MonsterBehaviorHealer:
		return 2.0
	default:
		return 2.0
	}
}

// raidMonsterAttackPlayer 战局内怪物攻击玩家
func raidMonsterAttackPlayer(m *model.Monster, playerID uint64, world iface.World) {
	p := world.GetOnlinePlayer(playerID)
	if p == nil {
		return
	}

	p.Mu().RLock()
	playerHp := p.Hp
	p.Mu().RUnlock()
	if playerHp <= 0 {
		return
	}

	// 伤害计算：基础攻击 + 随机波动(0~15%)
	damage := m.Atk + int64(float64(m.Atk)*0.15*float64(time.Now().UnixNano()%100)/100)
	if damage < 1 {
		damage = 1
	}

	p.Mu().Lock()
	p.Hp -= damage
	if p.Hp < 0 {
		p.Hp = 0
	}
	p.Mu().Unlock()

	logger.Debug("战局怪物攻击玩家", "monster", m.Name, "player_id", playerID, "damage", damage)
}
