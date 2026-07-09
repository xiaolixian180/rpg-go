package protocol

// ==================== 登录 ====================

// C2SLogin 客户端登录请求，携带鉴权token
type C2SLogin struct {
	Token string `json:"token"` // 鉴权令牌
}

// S2CLoginResp 服务端登录响应，成功时返回玩家完整数据
type S2CLoginResp struct {
	Code   uint32     `json:"code"` // 0=成功 1=token无效 2=账号封禁
	Player PlayerData `json:"player"`
}

// C2SCreatePlayer 客户端创建角色请求
// 创建角色同样携带 token，便于新账号在未登录状态下创建首个角色。
type C2SCreatePlayer struct {
	Token string `json:"token"` // 鉴权令牌
	Name  string `json:"name"`  // 角色名称
	Class int32  `json:"class"` // 职业：0=战士 1=法师 2=射手 3=牧师 4=刺客
}

// S2CCreatePlayerResp 服务端创建角色响应
type S2CCreatePlayerResp struct {
	Code   uint32     `json:"code"` // 0=成功 1=名字重复 2=参数错误
	Player PlayerData `json:"player"`
}

// ==================== 玩家数据 ====================

// PlayerData 玩家完整数据，登录成功后下发
type PlayerData struct {
	ID            uint64           `json:"id"`          // 玩家唯一ID
	Name          string           `json:"name"`        // 角色名称
	Class         int32            `json:"class"`       // 职业
	Level         int32            `json:"level"`       // 等级（上限60）
	Exp           int64            `json:"exp"`         // 当前经验值
	Gold          int64            `json:"gold"`        // 金币
	Honor         int32            `json:"honor"`       // 荣誉值（PvP获得）
	KillValue     int32            `json:"kill_value"`  // 杀戮值（杀白名玩家增加）
	Str           int32            `json:"str"`         // 力量属性
	Agi           int32            `json:"agi"`         // 敏捷属性
	Int           int32            `json:"int"`         // 智力属性
	Con           int32            `json:"con"`         // 体质属性
	Def           int32            `json:"def"`         // 防御属性
	AttrPoints    int32            `json:"attr_points"` // 未分配属性点
	MaxLayer      int32            `json:"max_layer"`   // 最高通关层数
	Hp            int64            `json:"hp"`          // 当前生命值
	MaxHp         int64            `json:"max_hp"`      // 生命值上限
	Mp            int64            `json:"mp"`          // 当前魔法值
	MaxMp         int64            `json:"max_mp"`      // 魔法值上限
	Items         map[uint32]int32 `json:"items"`       // 背包物品（item_id -> 数量）
	EquippedItems []EquipmentData  `json:"equipment"`   // 已装备的装备列表
}

// EquipmentData 装备实例数据，随 PlayerData 下发
type EquipmentData struct {
	Slot            int32             `json:"slot"`              // 槽位（0~7）
	EquipID         int32             `json:"equip_id"`          // 装备模板ID
	Name            string            `json:"name"`              // 装备名称（来自模板）
	Quality         int32             `json:"quality"`           // 品质
	StrengthenLevel int32             `json:"strengthen_level"`  // 强化等级
	EnchantAttr     string            `json:"enchant_attr"`      // 附魔属性描述
	BaseAtk         int64             `json:"base_atk"`          // 基础攻击力（含品质系数）
	BaseDef         int64             `json:"base_def"`          // 基础防御力
	BaseHp          int64             `json:"base_hp"`           // 基础生命值加成
	RequireLevel    int32             `json:"require_level"`     // 装备需求等级
	SkillEffects    []SkillEffectData `json:"skill_effects"`     // 技能特效列表（可为空）
}

// SkillEffectData 装备技能特效数据
type SkillEffectData struct {
	SkillID    int32   `json:"skill_id"`    // 绑定技能ID（0=所有技能）
	EffectType int32   `json:"effect_type"` // 效果类型：1=技能增伤
	Value      float64 `json:"value"`       // 效果值（如0.15=15%）
	Desc       string  `json:"desc"`        // 效果描述
}

// ==================== 地下城 ====================

// C2SEnterDungeon 客户端请求进入地下城指定层
type C2SEnterDungeon struct {
	Layer int32 `json:"layer"` // 目标层数（1~30）
}

// S2CEnterDungeonResp 服务端返回地下城场景数据
type S2CEnterDungeonResp struct {
	Code      uint32         `json:"code"`      // 0=成功 1=未登录 2=层数不合法
	Layer     int32          `json:"layer"`     // 当前层数
	Zone      string         `json:"zone"`      // 区域名称（翠绿森林/腐蚀沼泽/烈焰火山）
	Monsters  []MonsterData  `json:"monsters"`  // 场景内怪物列表
	Players   []PlayerBrief  `json:"players"`   // 场景内其他玩家列表
	Resources []ResourceData `json:"resources"` // 场景内可采集资源列表
}

// C2SLeaveDungeon 客户端请求离开地下城
type C2SLeaveDungeon struct{}

// S2CLeaveDungeonResp 服务端离开地下城响应
type S2CLeaveDungeonResp struct {
	Code uint32 `json:"code"` // 0=成功
}

// S2CDungeonInfo 服务端推送地下城进度信息
type S2CDungeonInfo struct {
	CurrentLayer int32 `json:"current_layer"` // 当前所在层数
	MaxLayer     int32 `json:"max_layer"`     // 最高通关层数
}

// S2CMonsterRefresh 服务端推送怪物刷新通知
type S2CMonsterRefresh struct {
	Monsters []MonsterData `json:"monsters"` // 新刷新的怪物列表
}

// C2SLayerTeleport 客户端请求层间传送
type C2SLayerTeleport struct {
	TargetLayer int32 `json:"target_layer"` // 目标层数
}

// S2CLayerTeleportResp 服务端层间传送响应，复用地下城场景数据格式
type S2CLayerTeleportResp = S2CEnterDungeonResp

// ==================== 场景实体 ====================

// MonsterData 怪物数据
type MonsterData struct {
	ID    uint64  `json:"id"`     // 怪物唯一ID
	Name  string  `json:"name"`   // 怪物名称
	Hp    int64   `json:"hp"`     // 当前血量
	MaxHp int64   `json:"max_hp"` // 最大血量
	X     float64 `json:"x"`      // X坐标
	Y     float64 `json:"y"`      // Y坐标
	Elite bool    `json:"elite"`  // 是否为精英怪
}

// PlayerBrief 场景内其他玩家简要信息
type PlayerBrief struct {
	ID    uint64  `json:"id"`     // 玩家ID
	Name  string  `json:"name"`   // 玩家名称
	Class int32   `json:"class"`  // 职业
	Level int32   `json:"level"`  // 等级
	Hp    int64   `json:"hp"`     // 当前血量
	MaxHp int64   `json:"max_hp"` // 最大血量
	X     float64 `json:"x"`      // X坐标
	Y     float64 `json:"y"`      // Y坐标
	PetID int32   `json:"pet_id"` // 当前出战宠物ID
}

// ResourceData 可采集资源数据
type ResourceData struct {
	ID        uint64  `json:"id"`        // 资源唯一ID
	Type      int32   `json:"type"`      // 资源类型：0=矿石 1=草药 2=木材
	Name      string  `json:"name"`      // 资源名称
	X         float64 `json:"x"`         // X坐标
	Y         float64 `json:"y"`         // Y坐标
	Harvested bool    `json:"harvested"` // 是否已被采集
}

// ==================== 战斗 ====================

// C2SAttack 客户端普通攻击请求
type C2SAttack struct {
	TargetID uint64 `json:"target_id"` // 攻击目标ID（怪物/Boss/玩家）
	SkillID  int32  `json:"skill_id"`  // 使用的技能ID（0=普攻）
}

// S2CDamage 服务端伤害结算结果
type S2CDamage struct {
	TargetID  uint64 `json:"target_id"`  // 目标ID
	Damage    int64  `json:"damage"`     // 伤害值
	CurrHp    int64  `json:"curr_hp"`    // 目标当前血量
	IsDead    bool   `json:"is_dead"`    // 目标是否死亡
	ExpGain   int64  `json:"exp_gain"`   // 获得经验（击杀时）
	GoldGain  int64  `json:"gold_gain"`  // 获得金币（击杀时）
	LevelUp   bool   `json:"level_up"`   // 攻击者是否升级
	NewLevel  int32  `json:"new_level"`  // 升级后等级
	PetDamage int64  `json:"pet_damage"` // 宠物造成的伤害（0=无宠物）
	PetCrit   bool   `json:"pet_crit"`   // 宠物是否暴击
	PetDead   bool   `json:"pet_dead"`   // 宠物是否死亡
}

// C2SSkillCast 客户端技能释放请求
type C2SSkillCast struct {
	SkillID  int32   `json:"skill_id"`  // 技能ID
	TargetID uint64  `json:"target_id"` // 目标ID
	X        float64 `json:"x"`         // 释放位置X
	Y        float64 `json:"y"`         // 释放位置Y
}

// S2CSkillEffect 服务端技能效果广播
type S2CSkillEffect struct {
	CasterID uint64       `json:"caster_id"` // 施法者ID
	SkillID  int32        `json:"skill_id"`  // 技能ID
	Targets  []DamageInfo `json:"targets"`   // 受击目标列表
	X        float64      `json:"x"`         // 释放位置X
	Y        float64      `json:"y"`         // 释放位置Y
}

// DamageInfo 单个目标的伤害信息
type DamageInfo struct {
	TargetID uint64 `json:"target_id"` // 目标ID
	Damage   int64  `json:"damage"`    // 伤害值
	CurrHp   int64  `json:"curr_hp"`   // 目标当前血量
	IsDead   bool   `json:"is_dead"`   // 目标是否死亡
}

// S2CPlayerDie 服务端通知玩家死亡
type S2CPlayerDie struct {
	PlayerID   uint64 `json:"player_id"`   // 死亡玩家ID
	KillerID   uint64 `json:"killer_id"`   // 击杀者ID
	KillerName string `json:"killer_name"` // 击杀者名称
}

// S2CPlayerRevive 服务端通知玩家复活
type S2CPlayerRevive struct {
	PlayerID uint64  `json:"player_id"` // 复活玩家ID
	Hp       int64   `json:"hp"`        // 复活后血量
	X        float64 `json:"x"`         // 复活位置X
	Y        float64 `json:"y"`         // 复活位置Y
}

// C2SCollectResource 客户端采集资源请求
type C2SCollectResource struct {
	ResourceID uint64 `json:"resource_id"` // 资源ID
}

// S2CCollectResult 服务端采集结果
type S2CCollectResult struct {
	Code       uint32 `json:"code"`        // 0=成功 1=已被采集 2=距离太远
	ResourceID uint64 `json:"resource_id"` // 资源ID
	ItemID     uint64 `json:"item_id"`     // 获得的物品ID
	ItemName   string `json:"item_name"`   // 物品名称
	Count      int32  `json:"count"`       // 物品数量
}

// ==================== 自动战斗 ====================

// C2SAutoBattle 客户端开启/关闭自动战斗请求
type C2SAutoBattle struct {
	Enable bool `json:"enable"` // true=开启 false=关闭
}

// S2CAutoBattleResp 服务端自动战斗状态响应
type S2CAutoBattleResp struct {
	Code   uint32 `json:"code"`   // 错误码
	Enable bool   `json:"enable"` // 当前自动战斗状态
}

// C2SUseItem 客户端使用消耗品请求
type C2SUseItem struct {
	ItemId uint32 `json:"item_id"` // 物品ID
}

// S2CUseItemResp 服务端使用消耗品结果
type S2CUseItemResp struct {
	Code   uint32 `json:"code"`    // 错误码
	ItemId uint32 `json:"item_id"` // 物品ID
	Count  int32  `json:"count"`   // 剩余数量
	Hp     int64  `json:"hp"`      // 当前HP
	MaxHp  int64  `json:"max_hp"`  // HP上限
	Mp     int64  `json:"mp"`      // 当前MP
	MaxMp  int64  `json:"max_mp"`  // MP上限
}

// S2CInventorySync 服务端库存同步推送
type S2CInventorySync struct {
	Items map[uint32]int32 `json:"items"` // item_id -> 数量
}

// ==================== Boss ====================

// S2CBossSpawn 服务端Boss生成通知
type S2CBossSpawn struct {
	BossID uint64          `json:"boss_id"` // Boss唯一ID
	Name   string          `json:"name"`    // Boss名称
	Hp     int64           `json:"hp"`      // 当前血量
	MaxHp  int64           `json:"max_hp"`  // 最大血量
	Layer  int32           `json:"layer"`   // 所在层数
	X      float64         `json:"x"`       // X坐标
	Y      float64         `json:"y"`       // Y坐标
	Skills []BossSkillData `json:"skills"`  // Boss技能列表
}

// BossSkillData Boss技能数据
type BossSkillData struct {
	SkillID int32   `json:"skill_id"` // 技能ID
	Name    string  `json:"name"`     // 技能名称
	CD      float64 `json:"cd"`       // 冷却时间（秒）
	Range   float64 `json:"range"`    // 技能范围
}

// S2CBossDie 服务端Boss死亡通知，附带掉落物品
type S2CBossDie struct {
	BossID uint64     `json:"boss_id"` // BossID
	Drops  []DropItem `json:"drops"`   // 掉落物品列表
}

// DropItem 掉落物品数据
type DropItem struct {
	ItemID  uint64 `json:"item_id"` // 物品ID
	Name    string `json:"name"`    // 物品名称
	Quality int32  `json:"quality"` // 品质：0=白 1=绿 2=蓝 3=紫 4=橙 5=红
	Count   int32  `json:"count"`   // 物品数量
}

// ==================== 装备 ====================

// C2SEquipStrengthen 客户端装备强化请求
type C2SEquipStrengthen struct {
	Slot int32 `json:"slot"` // 装备槽位：0~7
}

// S2CEquipStrengthenResp 服务端装备强化结果
type S2CEquipStrengthenResp struct {
	Code      uint32 `json:"code"`       // 0=成功 1=金币不足 2=槽位空
	Slot      int32  `json:"slot"`       // 槽位
	NewLevel  int32  `json:"new_level"`  // 强化后等级
	CostGold  int64  `json:"cost_gold"`  // 消耗金币
	IsSuccess bool   `json:"is_success"` // 强化是否成功（+7以上有失败概率）
}

// C2SEquipEnchant 客户端装备附魔请求
type C2SEquipEnchant struct {
	Slot       int32  `json:"slot"`        // 装备槽位
	MaterialID uint64 `json:"material_id"` // 附魔材料ID
}

// S2CEquipEnchantResp 服务端装备附魔结果
type S2CEquipEnchantResp struct {
	Code     uint32 `json:"code"`      // 0=成功 1=材料不足
	Slot     int32  `json:"slot"`      // 槽位
	AttrName string `json:"attr_name"` // 附魔属性名（力量/敏捷/智力/体质）
	AttrVal  int32  `json:"attr_val"`  // 附魔属性值
}

// C2SEquipWear 客户端穿戴装备请求
type C2SEquipWear struct {
	Slot    int32  `json:"slot"`     // 目标槽位
	EquipID uint64 `json:"equip_id"` // 装备ID
}

// S2CEquipWearResp 服务端穿戴装备结果
type S2CEquipWearResp struct {
	Code uint32 `json:"code"` // 0=成功
	Slot int32  `json:"slot"` // 槽位
}

// C2SEquipUnload 客户端卸下装备请求
type C2SEquipUnload struct {
	Slot int32 `json:"slot"` // 目标槽位
}

// S2CEquipUnloadResp 服务端卸下装备结果
type S2CEquipUnloadResp struct {
	Code uint32 `json:"code"` // 0=成功
	Slot int32  `json:"slot"` // 槽位
}

// C2SForge 客户端锻造合成请求
type C2SForge struct {
	RecipeID  uint64   `json:"recipe_id"` // 锻造图纸ID
	Materials []uint64 `json:"materials"` // 消耗的材料ID列表
}

// S2CForgeResp 服务端锻造合成结果
type S2CForgeResp struct {
	Code       uint32 `json:"code"`        // 0=成功 1=材料不足 2=图纸不存在
	ResultID   uint64 `json:"result_id"`   // 产出装备ID
	ResultName string `json:"result_name"` // 产出装备名称
	Quality    int32  `json:"quality"`     // 产出品质
}

// ==================== PvP ====================

// C2SPvpAttack 客户端PvP攻击请求
type C2SPvpAttack struct {
	TargetID uint64 `json:"target_id"` // 目标玩家ID
	SkillID  int32  `json:"skill_id"`  // 使用的技能ID
}

// S2CPvpResult 服务端PvP结算结果
type S2CPvpResult struct {
	Code       uint32 `json:"code"`        // 错误码，0表示成功
	AttackerID uint64 `json:"attacker_id"` // 攻击者ID
	TargetID   uint64 `json:"target_id"`   // 目标ID
	Damage     int64  `json:"damage"`      // 伤害值
	GoldGain   int64  `json:"gold_gain"`   // 攻击者获得的金币（目标10%）
	HonorGain  int32  `json:"honor_gain"`  // 攻击者获得的荣誉值
	IsDead     bool   `json:"is_dead"`     // 目标是否死亡
}

// S2CRedNameList 服务端红名玩家列表
type S2CRedNameList struct {
	Players []RedNameInfo `json:"players"` // 红名玩家列表
}

// RedNameInfo 红名玩家信息
type RedNameInfo struct {
	PlayerID  uint64 `json:"player_id"`  // 玩家ID
	Name      string `json:"name"`       // 玩家名称
	KillValue int32  `json:"kill_value"` // 杀戮值
	Bounty    int64  `json:"bounty"`     // 赏金金额
}

// C2SBountyHunt 客户端悬赏追杀请求
type C2SBountyHunt struct {
	TargetID uint64 `json:"target_id"` // 目标红名玩家ID
}

// S2CBountyReward 服务端悬赏奖励
type S2CBountyReward struct {
	Code      uint32 `json:"code"`       // 错误码，0表示成功
	TargetID  uint64 `json:"target_id"`  // 目标ID
	GoldGain  int64  `json:"gold_gain"`  // 获得金币
	HonorGain int32  `json:"honor_gain"` // 获得荣誉
}

// C2SRevenge 客户端复仇请求（标记仇人）
type C2SRevenge struct {
	TargetID uint64 `json:"target_id"` // 仇人ID
}

// S2CRevengeResp 服务端复仇响应
type S2CRevengeResp struct {
	Code     uint32 `json:"code"`      // 0=成功 1=不是仇人 2=参数错误
	TargetID uint64 `json:"target_id"` // 仇人ID
}

// ==================== 移动 ====================

// C2SMove 客户端位置移动请求
type C2SMove struct {
	X float64 `json:"x"` // 目标X坐标
	Y float64 `json:"y"` // 目标Y坐标
}

// S2CPlayerMove 服务端广播其他玩家移动
type S2CPlayerMove struct {
	PlayerID uint64  `json:"player_id"` // 移动的玩家ID
	X        float64 `json:"x"`         // 新X坐标
	Y        float64 `json:"y"`         // 新Y坐标
}

// ==================== 宠物 ====================

// C2SPetSummon 客户端召唤宠物出战
type C2SPetSummon struct {
	PetUID uint64 `json:"pet_uid"` // 宠物唯一实例ID
}

// S2CPetSummonResp 服务端召唤宠物结果
type S2CPetSummonResp struct {
	Code uint32  `json:"code"` // 0=成功 1=宠物不存在 2=已在出战
	Pet  PetData `json:"pet"`  // 宠物数据
}

// C2SPetRecall 客户端收回宠物
type C2SPetRecall struct {
	PetUID uint64 `json:"pet_uid"` // 宠物唯一实例ID
}

// S2CPetRecallResp 服务端收回宠物结果
type S2CPetRecallResp struct {
	Code   uint32 `json:"code"`    // 0=成功 1=宠物不存在
	PetUID uint64 `json:"pet_uid"` // 宠物ID
}

// C2SPetLevelUp 客户端宠物升级请求
type C2SPetLevelUp struct {
	PetUID uint64 `json:"pet_uid"` // 宠物唯一实例ID
}

// S2CPetLevelUp 服务端宠物升级结果
type S2CPetLevelUp struct {
	Code   uint32 `json:"code"`    // 0=成功
	PetUID uint64 `json:"pet_uid"` // 宠物ID
	Level  int32  `json:"level"`   // 新等级
}

// C2SPetEvolve 客户端宠物进阶请求（需等级≥10）
type C2SPetEvolve struct {
	PetUID uint64 `json:"pet_uid"` // 宠物唯一实例ID
}

// S2CPetEvolveResp 服务端宠物进阶结果
type S2CPetEvolveResp struct {
	Code       uint32 `json:"code"`        // 0=成功 1=等级不足 2=材料不足
	PetUID     uint64 `json:"pet_uid"`     // 宠物ID
	NewPetID   int32  `json:"new_pet_id"`  // 进阶后宠物模板ID
	NewQuality int32  `json:"new_quality"` // 进阶后品质
}

// C2SPetExplore 客户端宠物探险派遣请求
type C2SPetExplore struct {
	PetUID   uint64 `json:"pet_uid"`  // 宠物ID
	Duration int32  `json:"duration"` // 探险时长（分钟，上限12小时）
}

// S2CPetExploreResp 服务端宠物探险派遣结果
type S2CPetExploreResp struct {
	Code    uint32 `json:"code"`     // 0=成功 1=已在探险 2=宠物不存在
	PetUID  uint64 `json:"pet_uid"`  // 宠物ID
	EndTime int64  `json:"end_time"` // 探险结束时间戳
}

// S2CPetExploreDone 服务端推送探险完成奖励
type S2CPetExploreDone struct {
	PetUID  uint64     `json:"pet_uid"` // 宠物ID
	Rewards []DropItem `json:"rewards"` // 探险奖励列表
}

// C2SPetCompose 客户端宠物合成请求（3只同品质合成升阶）
type C2SPetCompose struct {
	PetUIDs []uint64 `json:"pet_uids"` // 素材宠物ID列表（至少3只）
}

// S2CPetComposeResp 服务端宠物合成结果
type S2CPetComposeResp struct {
	Code     uint32 `json:"code"`      // 0=成功 1=数量不足 2=品质不同
	ResultID uint64 `json:"result_id"` // 新宠物实例ID
	PetID    int32  `json:"pet_id"`    // 新宠物模板ID
	Quality  int32  `json:"quality"`   // 新品质
}

// C2SPetEquip 客户端宠物穿戴装备请求
type C2SPetEquip struct {
	PetUID  uint64 `json:"pet_uid"`  // 宠物实例ID
	Slot    int32  `json:"slot"`     // 装备槽位（0=项圈 1=护甲 2=饰品）
	EquipID int32  `json:"equip_id"` // 宠物装备模板ID
}

// S2CPetEquipResp 服务端宠物穿戴装备结果
type S2CPetEquipResp struct {
	Code   uint32 `json:"code"`    // 0=成功
	PetUID uint64 `json:"pet_uid"` // 宠物实例ID
	Slot   int32  `json:"slot"`    // 装备槽位
}

// C2SPetUnequip 客户端宠物卸下装备请求
type C2SPetUnequip struct {
	PetUID uint64 `json:"pet_uid"` // 宠物实例ID
	Slot   int32  `json:"slot"`    // 装备槽位
}

// S2CPetUnequipResp 服务端宠物卸下装备结果
type S2CPetUnequipResp struct {
	Code   uint32 `json:"code"`    // 0=成功
	PetUID uint64 `json:"pet_uid"` // 宠物实例ID
	Slot   int32  `json:"slot"`    // 装备槽位
}

// PetData 宠物完整数据
type PetData struct {
	UID     uint64 `json:"uid"`     // 宠物唯一实例ID
	PetID   int32  `json:"pet_id"`  // 宠物模板ID
	Name    string `json:"name"`    // 宠物名称
	Level   int32  `json:"level"`   // 宠物等级
	Quality int32  `json:"quality"` // 宠物品质
	Type    int32  `json:"type"`    // 宠物类型：0=攻击 1=防御 2=辅助 3=掠夺
	Skills  string `json:"skills"`  // 宠物技能列表（JSON字符串）
}

// ==================== 交易行 ====================

// C2STradeList 客户端查询交易行列表
type C2STradeList struct {
	Category int32 `json:"category"` // 物品分类
	Page     int32 `json:"page"`     // 页码（从1开始）
}

// S2CTradeListResp 服务端交易行列表
type S2CTradeListResp struct {
	Code  uint32      `json:"code"`  // 0=成功
	Items []TradeItem `json:"items"` // 商品列表
	Total int32       `json:"total"` // 总数量
}

// TradeItem 交易行商品数据
type TradeItem struct {
	OrderID         uint64 `json:"order_id"`         // 订单ID
	SellerID        uint64 `json:"seller_id"`        // 卖家ID
	SellerName      string `json:"seller_name"`      // 卖家名称
	EquipID         int32  `json:"equip_id"`         // 装备模板ID
	Name            string `json:"name"`             // 装备名称
	Quality         int32  `json:"quality"`          // 品质
	StrengthenLevel int32  `json:"strengthen_level"` // 强化等级
	Price           int64  `json:"price"`            // 售价（金币）
}

// C2STradePublish 客户端上架商品请求
type C2STradePublish struct {
	Slot  int32 `json:"slot"`  // 装备槽位
	Price int64 `json:"price"` // 售价
}

// S2CTradePublishResp 服务端上架结果
type S2CTradePublishResp struct {
	Code    uint32 `json:"code"`     // 0=成功 1=装备已绑定
	OrderID uint64 `json:"order_id"` // 订单ID
}

// C2STradeBuy 客户端购买交易行商品
type C2STradeBuy struct {
	OrderID uint64 `json:"order_id"` // 订单ID
}

// S2CTradeBuyResp 服务端购买结果
type S2CTradeBuyResp struct {
	Code    uint32 `json:"code"`     // 0=成功 1=金币不足 2=已售出
	OrderID uint64 `json:"order_id"` // 订单ID
}

// C2STradeCancel 客户端取消上架
type C2STradeCancel struct {
	OrderID uint64 `json:"order_id"` // 订单ID
}

// S2CTradeCancelResp 服务端取消上架结果
type S2CTradeCancelResp struct {
	Code    uint32 `json:"code"`     // 0=成功 1=订单不存在 2=非本人订单
	OrderID uint64 `json:"order_id"` // 订单ID
}

// ==================== 商店 ====================

// C2SShopList 客户端查询商店列表
type C2SShopList struct {
	Type int32 `json:"type"` // 商店类型：0=普通 1=荣誉 2=公会
}

// S2CShopListResp 服务端商店列表
type S2CShopListResp struct {
	Code  uint32     `json:"code"`  // 0=成功
	Items []ShopItem `json:"items"` // 商品列表
}

// ShopItem 商店商品数据
type ShopItem struct {
	ID           uint64 `json:"id"`            // 商品ID
	Name         string `json:"name"`          // 商品名称
	Price        int64  `json:"price"`         // 价格
	CurrencyType int32  `json:"currency_type"` // 货币类型：0=金币 1=荣誉 2=公会币
	Stock        int32  `json:"stock"`         // 库存（-1=无限）
	RequireLevel int32  `json:"require_level"` // 需求等级
}

// C2SShopBuy 客户端购买商店商品
type C2SShopBuy struct {
	ItemID       uint64 `json:"item_id"`       // 商品ID
	Count        int32  `json:"count"`         // 购买数量
	CurrencyType int32  `json:"currency_type"` // 货币类型：0=金币 1=荣誉 2=公会币
}

// S2CShopBuyResp 服务端购买结果
type S2CShopBuyResp struct {
	Code   uint32 `json:"code"`    // 0=成功 1=金币不足 2=库存不足 3=等级不足
	ItemID uint64 `json:"item_id"` // 商品ID
	Count  int32  `json:"count"`   // 购买数量
}

// ==================== 技能 ====================

// C2SSkillLevelUp 客户端技能升级请求
type C2SSkillLevelUp struct {
	SkillID int32 `json:"skill_id"` // 技能ID
}

// S2CSkillLevelUpResp 服务端技能升级结果
type S2CSkillLevelUpResp struct {
	Code     uint32 `json:"code"`      // 0=成功 1=技能点不足 2=技能不存在
	SkillID  int32  `json:"skill_id"`  // 技能ID
	NewLevel int32  `json:"new_level"` // 技能新等级
}

// C2SSkillReset 客户端技能重置请求（消耗金币）
type C2SSkillReset struct{}

// S2CSkillResetResp 服务端技能重置结果
type S2CSkillResetResp struct {
	Code         uint32 `json:"code"`          // 0=成功 1=金币不足
	RefundPoints int32  `json:"refund_points"` // 返还的技能点数
}

// ==================== 属性 ====================

// C2SAttrAssign 客户端分配属性点请求
type C2SAttrAssign struct {
	Attr string `json:"attr"` // 属性名：str/agi/int/con
	Val  int32  `json:"val"`  // 分配点数
}

// S2CAttrAssignResp 服务端属性分配结果
type S2CAttrAssignResp struct {
	Code       uint32 `json:"code"`        // 0=成功 1=点数不足 2=属性名无效
	Attr       string `json:"attr"`        // 属性名
	Val        int32  `json:"val"`         // 分配点数
	AttrPoints int32  `json:"attr_points"` // 剩余属性点
}

// ==================== 排行榜 ====================

// C2SRankingList 客户端查询排行榜
type C2SRankingList struct {
	Type int32 `json:"type"` // 排行榜类型：0=等级 1=战力 2=荣誉
}

// S2CRankingListResp 服务端排行榜数据
type S2CRankingListResp struct {
	Code     uint32        `json:"code"`     // 0=成功
	Type     int32         `json:"type"`     // 排行榜类型
	Rankings []RankingItem `json:"rankings"` // 排行数据列表
}

// RankingItem 排行榜单项数据
type RankingItem struct {
	Rank     int32  `json:"rank"`      // 排名
	PlayerID uint64 `json:"player_id"` // 玩家ID
	Name     string `json:"name"`      // 玩家名称
	Value    int64  `json:"value"`     // 排行数值
}

// ==================== 系统 ====================

// S2CBroadcast 服务端全服广播（稀有掉落/首通/红名等）
type S2CBroadcast struct {
	Type    int32  `json:"type"`    // 广播类型：1=稀有掉落 2=红名 3=首通
	Content string `json:"content"` // 广播内容
}

// C2SHeartbeat 客户端心跳请求
type C2SHeartbeat struct {
	Timestamp int64 `json:"timestamp"` // 客户端时间戳
}

// S2CHeartbeat 服务端心跳响应
type S2CHeartbeat struct {
	Timestamp int64 `json:"timestamp"` // 服务端时间戳
}

// S2CKick 服务端踢下线通知
type S2CKick struct {
	Reason string `json:"reason"` // 被踢原因
}

// ==================== 组队模块 DTO ====================

// TeamMember 队员信息
type TeamMember struct {
	PlayerID uint64 `json:"player_id"` // 玩家ID
	Name     string `json:"name"`      // 玩家名称
	Class    int32  `json:"class"`     // 职业
	Level    int32  `json:"level"`     // 等级
	Hp       int64  `json:"hp"`        // 当前血量
	MaxHp    int64  `json:"max_hp"`    // 最大血量
	IsLeader bool   `json:"is_leader"` // 是否队长
	Online   bool   `json:"online"`    // 是否在线
	Layer    int32  `json:"layer"`     // 所在层（0=不在地下城）
}

// TeamInfo 队伍信息
type TeamInfo struct {
	TeamID      uint64       `json:"team_id"`      // 队伍唯一ID
	LeaderID    uint64       `json:"leader_id"`    // 队长ID
	MemberCount int32        `json:"member_count"` // 当前队员数
	Members     []TeamMember `json:"members"`      // 队员列表
}

// C2STeamCreate 创建队伍请求
type C2STeamCreate struct {
	// 无字段，由服务端自动创建以申请人为队长的队伍
}

// S2CTeamInfoResp 队伍信息响应（用于创建/查询响应）
type S2CTeamInfoResp struct {
	Code uint32   `json:"code"` // 错误码，0表示成功
	Team TeamInfo `json:"team"` // 队伍信息（无队伍时为空）
}

// C2STeamInvite 邀请玩家入队
type C2STeamInvite struct {
	TargetID uint64 `json:"target_id"` // 被邀请玩家ID
}

// S2CTeamInvitePush 被邀请通知（推送给被邀请方）
type S2CTeamInvitePush struct {
	TeamID      uint64 `json:"team_id"`      // 队伍ID
	InviterID   uint64 `json:"inviter_id"`   // 邀请者ID
	InviterName string `json:"inviter_name"` // 邀请者名称
	MemberCount int32  `json:"member_count"` // 当前队员数
}

// C2STeamInviteReply 邀请回复
type C2STeamInviteReply struct {
	TeamID uint64 `json:"team_id"` // 队伍ID
	Accept bool   `json:"accept"`  // true=接受，false=拒绝
}

// S2CTeamInviteResult 邀请结果（推送给邀请者）
type S2CTeamInviteResult struct {
	Code       uint32 `json:"code"`        // 错误码，0=对方接受，700=对方拒绝，其他=失败
	TargetID   uint64 `json:"target_id"`   // 被邀请玩家ID
	TargetName string `json:"target_name"` // 被邀请玩家名称
	Accept     bool   `json:"accept"`      // 对方是否接受
}

// C2STeamLeave 离开队伍
type C2STeamLeave struct{}

// S2CTeamLeaveResp 离开队伍结果
type S2CTeamLeaveResp struct {
	Code   uint32 `json:"code"`    // 错误码
	TeamID uint64 `json:"team_id"` // 离开的队伍ID
}

// C2STeamDismiss 解散队伍（仅队长）
type C2STeamDismiss struct{}

// S2CTeamDismissResp 解散队伍结果
type S2CTeamDismissResp struct {
	Code   uint32 `json:"code"`    // 错误码
	TeamID uint64 `json:"team_id"` // 解散的队伍ID
}

// C2STeamKick 踢出队员
type C2STeamKick struct {
	TargetID uint64 `json:"target_id"` // 被踢玩家ID
}

// S2CTeamKickResp 踢出队员结果
type S2CTeamKickResp struct {
	Code     uint32 `json:"code"`      // 错误码
	TargetID uint64 `json:"target_id"` // 被踢玩家ID
}

// C2STeamQuery 查询我的队伍
type C2STeamQuery struct{}

// S2CTeamUpdate 队伍状态变更推送（全员）
// Action: 1=成员加入 2=成员离开 3=队伍解散 4=成员被踢 5=队员状态变更(血量/位置/上下线)
type S2CTeamUpdate struct {
	Action   uint32       `json:"action"`    // 变更类型
	TeamID   uint64       `json:"team_id"`   // 队伍ID
	LeaderID uint64       `json:"leader_id"` // 当前队长ID
	Members  []TeamMember `json:"members"`   // 当前队员快照
	Reason   string       `json:"reason"`    // 变更说明（如"XX 离开了队伍"）
}

// ==================== 聊天模块 DTO ====================

// 聊天频道常量
const (
	ChatChannelWorld   int32 = 1 // 世界频道
	ChatChannelPrivate int32 = 2 // 私聊
	ChatChannelTeam    int32 = 3 // 队伍频道
)

// C2SChatSend 发送聊天消息
type C2SChatSend struct {
	Channel  int32  `json:"channel"`   // 频道：1=世界 2=私聊 3=队伍
	TargetID uint64 `json:"target_id"` // 私聊目标ID（仅 channel=2 时使用）
	Content  string `json:"content"`   // 消息内容
}

// S2CChatSendResp 发送结果
type S2CChatSendResp struct {
	Code      uint32 `json:"code"`      // 错误码，0=成功
	Channel   int32  `json:"channel"`   // 频道
	TargetID  uint64 `json:"target_id"` // 私聊目标
	Timestamp int64  `json:"timestamp"` // 服务端时间戳（毫秒）
}

// S2CChatMessage 聊天消息推送
type S2CChatMessage struct {
	Channel    int32  `json:"channel"`     // 频道
	SenderID   uint64 `json:"sender_id"`   // 发送者ID
	SenderName string `json:"sender_name"` // 发送者名称
	TargetID   uint64 `json:"target_id"`   // 私聊接收方ID（仅私聊）
	Content    string `json:"content"`     // 内容
	Timestamp  int64  `json:"timestamp"`   // 时间戳（毫秒）
}

// C2SChatHistory 查询历史消息
type C2SChatHistory struct {
	Channel int32 `json:"channel"` // 频道（仅支持世界频道历史）
	Count   int32 `json:"count"`   // 拉取条数（最多50）
}

// S2CChatHistoryResp 历史消息响应
type S2CChatHistoryResp struct {
	Code     uint32           `json:"code"`     // 错误码
	Channel  int32            `json:"channel"`  // 频道
	Messages []S2CChatMessage `json:"messages"` // 历史消息（按时间升序）
}

// ==================== 战局模块 ====================

// C2SRaidEnter 客户端请求进入战局
type C2SRaidEnter struct {
	MapID int32 `json:"map_id"` // 地图模板ID
}

// S2CRaidEnterResp 服务端返回战局地图数据
type S2CRaidEnterResp struct {
	Code             uint32              `json:"code"`              // 错误码，0=成功
	MapID            uint64              `json:"map_id"`            // 战局实例ID
	MapName          string              `json:"map_name"`          // 地图名称
	Duration         int64               `json:"duration"`          // 总时长秒
	Monsters         []MonsterData       `json:"monsters"`          // 场景内怪物列表
	Zones            []RaidZoneData      `json:"zones"`             // 区域列表
	ExtractionPoints []ExtractionPointData `json:"extraction_points"` // 撤离点列表
	LootContainers   []LootContainerData `json:"loot_containers"`   // 战利品容器列表
}

// RaidZoneData 战局区域数据
type RaidZoneData struct {
	ID         int32   `json:"id"`          // 区域ID
	Name       string  `json:"name"`        // 区域名称
	X1         float64 `json:"x1"`          // 左上角X
	Y1         float64 `json:"y1"`          // 左上角Y
	X2         float64 `json:"x2"`          // 右下角X
	Y2         float64 `json:"y2"`          // 右下角Y
	PvPEnabled bool    `json:"pvp_enabled"` // 是否允许PvP
	LootTier   int32   `json:"loot_tier"`   // 掉落品质等级
}

// ExtractionPointData 撤离点数据
type ExtractionPointData struct {
	ID              int32   `json:"id"`               // 撤离点ID
	X               float64 `json:"x"`                // X坐标
	Y               float64 `json:"y"`                // Y坐标
	Radius          float64 `json:"radius"`           // 触发半径
	ExtractDuration int32   `json:"extract_duration"` // 撤离所需秒数
}

// LootContainerData 战利品容器数据
type LootContainerData struct {
	ID     uint64  `json:"id"`     // 容器唯一ID
	X      float64 `json:"x"`      // X坐标
	Y      float64 `json:"y"`      // Y坐标
	Opened bool    `json:"opened"` // 是否已打开
}

// C2SRaidLeave 客户端请求离开战局
type C2SRaidLeave struct{}

// S2CRaidLeaveResp 服务端离开战局确认
type S2CRaidLeaveResp struct {
	Code uint32 `json:"code"` // 错误码，0=成功
}

// S2CRaidInfo 服务端推送战局状态
type S2CRaidInfo struct {
	Remaining int64 `json:"remaining"` // 剩余秒数
	ZoneID    int32 `json:"zone_id"`   // 当前所在区域ID
	PvPFlag   bool  `json:"pvp_flag"`  // 是否在PvP区域
}

// C2SRaidExtract 客户端请求开始撤离
type C2SRaidExtract struct {
	PointID int32 `json:"point_id"` // 撤离点ID
}

// S2CRaidExtractResp 服务端撤离结果
type S2CRaidExtractResp struct {
	Code  uint32 `json:"code"`  // 错误码，0=成功
	Timer int32  `json:"timer"` // 撤离倒计时秒数
}

// S2CRaidExtractProgress 服务端推送撤离倒计时
type S2CRaidExtractProgress struct {
	Timer int32 `json:"timer"` // 剩余撤离秒数
}

// C2SRaidLootOpen 客户端请求打开战利品容器
type C2SRaidLootOpen struct {
	ContainerID uint64 `json:"container_id"` // 容器ID
}

// S2CRaidLootOpenResp 服务端返回容器内容
type S2CRaidLootOpenResp struct {
	Code  uint32         `json:"code"`  // 错误码，0=成功
	Items []RaidLootData `json:"items"` // 容器内物品列表
}

// RaidLootData 战局物品数据
type RaidLootData struct {
	Index   int32  `json:"index"`   // 物品在容器中的索引
	ItemID  int32  `json:"item_id"` // 物品模板ID
	Count   int32  `json:"count"`   // 物品数量
	Quality int32  `json:"quality"` // 品质等级
	Name    string `json:"name"`    // 物品名称
}

// C2SRaidLootPickup 客户端请求拾取战利品
type C2SRaidLootPickup struct {
	ItemIndex int32 `json:"item_index"` // 物品索引
}

// S2CRaidLootPickupResp 服务端拾取结果
type S2CRaidLootPickupResp struct {
	Code uint32 `json:"code"` // 错误码，0=成功
}

// C2SRaidLootDiscard 客户端请求丢弃战利品
type C2SRaidLootDiscard struct {
	ItemIndex int32 `json:"item_index"` // 物品索引
}

// S2CRaidLootDiscardResp 服务端丢弃结果
type S2CRaidLootDiscardResp struct {
	Code uint32 `json:"code"` // 错误码，0=成功
}

// S2CRaidInventory 服务端推送战局背包同步
type S2CRaidInventory struct {
	Items []RaidLootData `json:"items"` // 当前战局背包物品
}

// S2CRaidDeath 服务端通知战局内玩家死亡
type S2CRaidDeath struct {
	PlayerID uint64 `json:"player_id"` // 死亡玩家ID
	Reason   string `json:"reason"`    // 死亡原因："killed" / "timeout" / "abandon"
}

// S2CRaidTimer 服务端推送战局剩余时间
type S2CRaidTimer struct {
	Remaining int64 `json:"remaining"` // 剩余秒数
}

// C2SRaidPvpAttack 客户端战局内PvP攻击请求
type C2SRaidPvpAttack struct {
	TargetID uint64 `json:"target_id"` // 目标玩家ID
}

// S2CRaidPvpResult 服务端战局PvP结果
type S2CRaidPvpResult struct {
	Code       uint32 `json:"code"`        // 错误码，0=成功
	AttackerID uint64 `json:"attacker_id"` // 攻击者ID
	TargetID   uint64 `json:"target_id"`   // 目标ID
	Damage     int64  `json:"damage"`      // 伤害值
	CurrHp     int64  `json:"curr_hp"`     // 目标当前血量
	IsDead     bool   `json:"is_dead"`     // 目标是否死亡
}

// C2SRaidMapList 客户端请求查询可用战局地图列表
type C2SRaidMapList struct{}

// S2CRaidMapListResp 服务端返回地图列表
type S2CRaidMapListResp struct {
	Code uint32        `json:"code"` // 错误码，0=成功
	Maps []RaidMapInfo `json:"maps"` // 可用地图列表
}

// RaidMapInfo 战局地图信息
type RaidMapInfo struct {
	TemplateID int32  `json:"template_id"` // 地图模板ID
	Name       string `json:"name"`        // 地图名称
	Duration   int64  `json:"duration"`    // 战局时长秒
	ZoneCount  int32  `json:"zone_count"`  // 区域数量
	LootTier   int32  `json:"loot_tier"`   // 掉落品质等级
}

// C2SRaidStash 客户端请求查询战局仓库
type C2SRaidStash struct{}

// S2CRaidStashResp 服务端返回仓库内容
type S2CRaidStashResp struct {
	Code  uint32         `json:"code"`  // 错误码，0=成功
	Items []RaidLootData `json:"items"` // 仓库物品列表
}
