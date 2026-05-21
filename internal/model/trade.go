package model

import "time"

// TradeOrder 交易行订单，玩家上架出售的装备
type TradeOrder struct {
	ID              uint64    // 订单唯一标识
	SellerID        uint64    // 卖家玩家ID
	EquipID         int32     // 装备模板ID
	Quality         int32     // 装备品质等级
	StrengthenLevel int32     // 装备强化等级
	Price           int64     // 挂单价格（金币）
	Status          int32     // 订单状态：0=在售 1=已售出 2=已下架
	CreatedAt       time.Time // 订单创建时间
}

// 交易订单状态常量
const (
	TradeOrderOnSale    = 0 // 在售
	TradeOrderSold      = 1 // 已售出
	TradeOrderCancelled = 2 // 已下架
)

// TradeOrderORM 交易订单表持久化模型，对应 trade_order 表。
type TradeOrderORM struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	SellerID        uint64 `gorm:"not null;index:idx_seller_id" json:"seller_id"`
	EquipID         int32  `gorm:"not null" json:"equip_id"`
	Quality         int32  `gorm:"not null" json:"quality"`
	StrengthenLevel int32  `gorm:"not null" json:"strengthen_level"`
	Price           int64  `gorm:"not null" json:"price"`
	Status          int32  `gorm:"not null;default:0;index:idx_status" json:"status"`
	CreatedAt       int64  `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定 TradeOrderORM 对应的数据库表名
func (TradeOrderORM) TableName() string { return "trade_order" }
