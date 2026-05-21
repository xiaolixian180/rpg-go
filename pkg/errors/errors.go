// Package errors 定义游戏业务统一错误码。
// 所有业务错误使用 GameError 携带错误码和描述，handler 层将错误码写入协议响应的 Code 字段。
package errors

import "fmt"

// GameError 游戏业务错误，携带错误码和描述信息
type GameError struct {
	Code uint32 // 错误码，0=成功，非0=各类业务错误
	Desc string // 错误描述，用于日志和调试
}

// Error 实现 error 接口
func (e *GameError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Desc)
}

// New 创建一个业务错误
func New(code uint32, desc string) *GameError {
	return &GameError{Code: code, Desc: desc}
}

// ==================== 通用错误码 0~99 ====================

var (
	ErrSuccess      = New(0, "成功")      // 操作成功
	ErrInternal     = New(1, "服务器内部错误") // 未预期的服务端错误
	ErrParamInvalid = New(2, "参数无效")    // 请求参数校验失败
	ErrNotLogin     = New(3, "未登录")     // 玩家未认证或token过期
	ErrFreqLimit    = New(4, "操作过于频繁")  // 触发限流
)

// ==================== 登录模块 100~199 ====================

var (
	ErrTokenInvalid  = New(100, "Token无效") // 登录token校验失败
	ErrAccountBanned = New(101, "账号被封禁")   // 账号处于封禁状态
	ErrNameDuplicate = New(102, "角色名已存在")  // 创建角色时名字重复
	ErrClassInvalid  = New(103, "职业类型无效")  // 创建角色时职业参数错误
)

// ==================== 地下城模块 200~299 ====================

var (
	ErrLayerInvalid     = New(200, "层数不合法")  // 请求的层数超出范围
	ErrLayerLocked      = New(201, "层数未解锁")  // 玩家未通关前置层
	ErrNotInDungeon     = New(202, "不在地下城中") // 离开地下城时玩家不在地下城
	ErrAlreadyInDungeon = New(203, "已在地下城中") // 重复进入
)

// ==================== 战斗模块 300~399 ====================

var (
	ErrTargetNotFound = New(300, "目标不存在")  // 攻击目标ID无效
	ErrTargetDead     = New(301, "目标已死亡")  // 攻击已死亡的目标
	ErrSkillNotFound  = New(302, "技能不存在")  // 使用的技能ID无效
	ErrSkillCD        = New(303, "技能冷却中")  // 技能还在CD
	ErrSelfDead       = New(304, "角色已死亡")  // 死亡状态下无法操作
	ErrResourceGone   = New(305, "资源已被采集") // 采集已被他人采集的资源
	ErrTooFar         = New(306, "距离太远")   // 目标超出交互范围
)

// ==================== 装备模块 400~499 ====================

var (
	ErrSlotEmpty      = New(400, "槽位为空")    // 操作的装备槽位没有装备
	ErrSlotInvalid    = New(401, "槽位无效")    // 槽位编号超出范围
	ErrEquipNotFound  = New(402, "装备不存在")   // 装备ID无效
	ErrEquipBound     = New(403, "装备已绑定")   // 绑定装备无法上架交易行
	ErrForgeNotFound  = New(404, "锻造图纸不存在") // 锻造配方ID无效
	ErrMaterialLack   = New(405, "材料不足")    // 锻造/附魔所需材料不够
	ErrStrengthenFail = New(406, "强化失败")    // +7以上强化概率失败
)

// ==================== PvP模块 500~599 ====================

var (
	ErrNotEnemy         = New(500, "不是仇人")     // 复仇目标不在仇人列表中
	ErrTargetInvincible = New(501, "目标处于无敌状态") // 攻击无敌保护期的玩家
	ErrSelfRedName      = New(502, "自己已是红名")   // 红名玩家无法悬赏他人
)

// ==================== 宠物模块 600~699 ====================

var (
	ErrPetNotFound       = New(600, "宠物不存在")    // 宠物ID无效
	ErrPetAlreadyOut     = New(601, "宠物已在出战")   // 重复召唤同一宠物
	ErrPetLevelLow       = New(602, "宠物等级不足")   // 进阶需要等级≥10
	ErrPetExploring      = New(603, "宠物正在探险中")  // 探险中的宠物无法出战
	ErrPetComposeNum     = New(604, "合成素材数量不足") // 合成需要至少3只同品质宠物
	ErrPetComposeQuality = New(605, "合成素材品质不同") // 合成需要相同品质的宠物
)

// ==================== 交易行模块 700~799 ====================

var (
	ErrTradeNotFound  = New(700, "订单不存在")     // 订单ID无效
	ErrTradeSold      = New(701, "商品已售出")     // 购买时商品已被他人买走
	ErrTradeSelfBuy   = New(702, "不能购买自己的商品") // 卖家不能买自己的商品
	ErrTradeCancelled = New(703, "订单已取消")     // 购买已取消的订单
)

// ==================== 商店模块 800~899 ====================

var (
	ErrShopNotFound = New(800, "商品不存在") // 商品ID无效
	ErrShopStockOut = New(801, "库存不足")  // 商品已售罄
	ErrShopLevelLow = New(802, "等级不足")  // 玩家等级不满足购买条件
)

// ==================== 通用资源不足 900~999 ====================

var (
	ErrGoldNotEnough        = New(900, "金币不足")  // 金币余额不够
	ErrHonorNotEnough       = New(901, "荣誉值不足") // 荣誉值不够
	ErrAttrPointsNotEnough  = New(902, "属性点不足") // 可分配属性点不够
	ErrSkillPointsNotEnough = New(903, "技能点不足") // 可分配技能点不够
	ErrSkillMaxLevel        = New(904, "技能已满级") // 技能已达最大等级
)

// IsSuccess 判断错误码是否为成功
func IsSuccess(code uint32) bool {
	return code == 0
}
