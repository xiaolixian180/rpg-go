package repo

import (
	"context"
	"fmt"
	"time"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
)

// ==================== 交易行数据访问实现 ====================

// tradeRepo 是 TradeRepo 接口的具体实现，使用 gorm.DB 操作 MySQL。
type tradeRepo struct {
	db *gorm.DB
}

// NewTradeRepo 创建 TradeRepo 实例。
func NewTradeRepo(db *database.DB) TradeRepo {
	return &tradeRepo{db: db.DB}
}

// ListOrders 按分类分页查询在售订单。
// category 参数对应装备品质等级，用于筛选指定品质的装备。
// 利用 idx_status 索引加速查询，按创建时间倒序返回（最新的在前）。
func (r *tradeRepo) ListOrders(ctx context.Context, category int32, page, pageSize int32) ([]*model.TradeOrder, error) {
	var rows []model.TradeOrderORM
	offset := (page - 1) * pageSize

	err := r.db.WithContext(ctx).
		Where("status = ? AND quality = ?", model.TradeOrderOnSale, category).
		Order("created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list orders category=%d page=%d: %w", category, page, err)
	}

	orders := make([]*model.TradeOrder, 0, len(rows))
	for _, o := range rows {
		orders = append(orders, &model.TradeOrder{
			ID:              o.ID,
			SellerID:        o.SellerID,
			EquipID:         o.EquipID,
			Quality:         o.Quality,
			StrengthenLevel: o.StrengthenLevel,
			Price:           o.Price,
			Status:          o.Status,
			CreatedAt:       timeFromUnix(o.CreatedAt),
		})
	}
	return orders, nil
}

// CountOrders 统计指定品质分类的在售订单总数，用于分页计算。
func (r *tradeRepo) CountOrders(ctx context.Context, category int32) (int32, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.TradeOrderORM{}).
		Where("status = ? AND quality = ?", model.TradeOrderOnSale, category).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count orders category=%d: %w", category, err)
	}
	return int32(count), nil
}

// GetOrder 根据订单 ID 查询订单详情。
// 若未找到返回 nil 和 nil error。
func (r *tradeRepo) GetOrder(ctx context.Context, orderID uint64) (*model.TradeOrder, error) {
	var o model.TradeOrderORM
	err := r.db.WithContext(ctx).First(&o, orderID).Error
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query order id=%d: %w", orderID, err)
	}
	return &model.TradeOrder{
		ID:              o.ID,
		SellerID:        o.SellerID,
		EquipID:         o.EquipID,
		Quality:         o.Quality,
		StrengthenLevel: o.StrengthenLevel,
		Price:           o.Price,
		Status:          o.Status,
		CreatedAt:       timeFromUnix(o.CreatedAt),
	}, nil
}

// CreateOrder 创建交易订单，回写自增主键。
// 订单初始状态为"在售"。
func (r *tradeRepo) CreateOrder(ctx context.Context, order *model.TradeOrder) (uint64, error) {
	o := model.TradeOrderORM{
		SellerID:        order.SellerID,
		EquipID:         order.EquipID,
		Quality:         order.Quality,
		StrengthenLevel: order.StrengthenLevel,
		Price:           order.Price,
		Status:          model.TradeOrderOnSale,
	}
	err := r.db.WithContext(ctx).Create(&o).Error
	if err != nil {
		return 0, fmt.Errorf("create order: %w", err)
	}
	return o.ID, nil
}

// BuyOrder 购买订单，将状态改为"已售出"。
// 使用 CAS（Compare And Swap）模式：只更新状态为"在售"的订单，
// 避免并发购买导致重复售出。返回是否成功更新。
func (r *tradeRepo) BuyOrder(ctx context.Context, orderID uint64, buyerID uint64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.TradeOrderORM{}).
		Where("id = ? AND status = ?", orderID, model.TradeOrderOnSale).
		Update("status", model.TradeOrderSold)
	if result.Error != nil {
		return false, fmt.Errorf("buy order %d: %w", orderID, result.Error)
	}
	// 影响行数为 0 表示订单已被他人购买或不存在
	return result.RowsAffected > 0, nil
}

// CancelOrder 取消订单，将状态改为"已下架"。
// 仅允许卖家取消自己的在售订单，通过 seller_id 和 status 双重条件保证安全。
// 返回是否成功取消。
func (r *tradeRepo) CancelOrder(ctx context.Context, orderID uint64, sellerID uint64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.TradeOrderORM{}).
		Where("id = ? AND seller_id = ? AND status = ?", orderID, sellerID, model.TradeOrderOnSale).
		Update("status", model.TradeOrderCancelled)
	if result.Error != nil {
		return false, fmt.Errorf("cancel order %d: %w", orderID, result.Error)
	}
	// 影响行数为 0 表示订单不属于该卖家或已不在售
	return result.RowsAffected > 0, nil
}

// timeFromUnix 将 Unix 时间戳（秒）转换为 time.Time。
// GORM 使用 autoCreateTime 存储为 Unix 时间戳，需手动转换为运行时模型的 time.Time。
func timeFromUnix(unix int64) time.Time {
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}
