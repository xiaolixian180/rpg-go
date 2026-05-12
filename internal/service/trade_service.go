// Package service - 交易行服务
// 提供查询、上架、购买、取消等交易行相关业务逻辑
package service

import (
	"context"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 交易行服务接口 ====================

// TradeService 交易行服务接口，定义查询、上架、购买、取消等操作
type TradeService interface {
	// List 查询交易行商品列表（按分类分页）
	List(ctx context.Context, category int32, page int32) (*TradeListResult, *errors.GameError)
	// Publish 上架商品到交易行
	Publish(ctx context.Context, playerID uint64, slot int32, price int64) (*TradePublishResult, *errors.GameError)
	// Buy 贜买交易行商品
	Buy(ctx context.Context, buyer *model.Player, orderID uint64) (*TradeBuyResult, *errors.GameError)
	// Cancel 取消上架（卖家下架自己的商品）
	Cancel(ctx context.Context, playerID uint64, orderID uint64) *errors.GameError
}

// ==================== 交易行结果结构体 ====================

// TradeListResult 交易行列表查询结果
type TradeListResult struct {
	Items []*model.TradeOrder // 商品列表
	Total int32               // 总数量
}

// TradePublishResult 上架结果
type TradePublishResult struct {
	OrderID uint64 // 订单ID
}

// TradeBuyResult 购买结果
type TradeBuyResult struct {
	OrderID uint64 // 订单ID
}

// ==================== 交易行服务实现 ====================

// tradeService 交易行服务实现
type tradeService struct {
	tradeRepo repo.TradeRepo // 交易行数据访问接口
	equipRepo repo.EquipRepo // 装备数据访问接口（上架时查询装备信息）
}

// NewTradeService 创建交易行服务实例
func NewTradeService(tradeRepo repo.TradeRepo, equipRepo repo.EquipRepo) TradeService {
	return &tradeService{
		tradeRepo: tradeRepo,
		equipRepo: equipRepo,
	}
}

// List 查询交易行商品列表业务逻辑：
//  1. 计算分页参数
//  2. 查询指定分类的在售订单
//  3. 统计总数量
func (s *tradeService) List(ctx context.Context, category int32, page int32) (*TradeListResult, *errors.GameError) {
	// 计算分页参数
	pageSize := int32(20) // 每页20条
	if page <= 0 {
		page = 1
	}

	// 查询在售订单
	orders, err := s.tradeRepo.ListOrders(ctx, category, page, pageSize)
	if err != nil {
		logger.Error("查询交易行列表失败", "category", category, "page", page, "err", err)
		return nil, errors.ErrInternal
	}

	// 统计总数量
	total, countErr := s.tradeRepo.CountOrders(ctx, category)
	if countErr != nil {
		logger.Error("统计交易行订单数量失败", "category", category, "err", countErr)
		total = 0 // 统计失败时不影响列表返回
	}

	result := &TradeListResult{
		Items: orders,
		Total: total,
	}

	return result, nil
}

// Publish 上架商品到交易行业务逻辑：
//  1. 校验槽位范围
//  2. 查询指定槽位的装备信息
//  3. 校验装备是否存在
//  4. 创建交易订单
func (s *tradeService) Publish(ctx context.Context, playerID uint64, slot int32, price int64) (*TradePublishResult, *errors.GameError) {
	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return nil, errors.ErrSlotInvalid
	}

	// 查询指定槽位的装备信息
	equip, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return nil, errors.ErrSlotEmpty
	}

	// 创建交易订单
	order := &model.TradeOrder{
		SellerID:        playerID,
		EquipID:         equip.EquipID,
		Quality:         0, // 简化：品质从装备模板查询，此处暂用0
		StrengthenLevel: equip.StrengthenLevel,
		Price:           price,
		Status:          model.TradeOrderOnSale,
	}

	orderID, createErr := s.tradeRepo.CreateOrder(ctx, order)
	if createErr != nil {
		logger.Error("创建交易订单失败", "player_id", playerID, "slot", slot, "err", createErr)
		return nil, errors.ErrInternal
	}

	result := &TradePublishResult{
		OrderID: orderID,
	}

	logger.Info("上架交易行商品", "player_id", playerID, "slot", slot, "price", price, "order_id", orderID)
	return result, nil
}

// Buy 贜买交易行商品业务逻辑：
//  1. 查询订单信息
//  2. 校验订单是否存在
//  3. 校验订单状态是否为在售
//  4. 校验买家不能购买自己的商品
//  5. 校验买家金币是否充足
//  6. 扣减买家金币
//  7. 将订单状态改为已售出
func (s *tradeService) Buy(ctx context.Context, buyer *model.Player, orderID uint64) (*TradeBuyResult, *errors.GameError) {
	// 查询订单信息
	order, err := s.tradeRepo.GetOrder(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.ErrTradeNotFound
	}

	// 校验订单状态是否为在售
	if order.Status != model.TradeOrderOnSale {
		if order.Status == model.TradeOrderSold {
			return nil, errors.ErrTradeSold
		}
		return nil, errors.ErrTradeCancelled
	}

	// 校验买家不能购买自己的商品
	if order.SellerID == buyer.ID {
		return nil, errors.ErrTradeSelfBuy
	}

	// 校验买家金币是否充足
	if buyer.Gold < order.Price {
		return nil, errors.ErrGoldNotEnough
	}

	// 扣减买家金币
	buyer.Gold -= order.Price

	// 将订单状态改为已售出
	success, buyErr := s.tradeRepo.BuyOrder(ctx, orderID, buyer.ID)
	if buyErr != nil || !success {
		logger.Error("购买交易行商品失败", "order_id", orderID, "buyer_id", buyer.ID, "err", buyErr)
		// 购买失败时退还金币
		buyer.Gold += order.Price
		return nil, errors.ErrTradeSold
	}

	result := &TradeBuyResult{
		OrderID: orderID,
	}

	logger.Info("购买交易行商品", "buyer_id", buyer.ID, "order_id", orderID, "price", order.Price)
	return result, nil
}

// Cancel 取消上架业务逻辑：
//  1. 查询订单信息
//  2. 校验订单是否存在
//  3. 校验取消者是否为订单卖家
//  4. 将订单状态改为已下架
func (s *tradeService) Cancel(ctx context.Context, playerID uint64, orderID uint64) *errors.GameError {
	// 查询订单信息
	order, err := s.tradeRepo.GetOrder(ctx, orderID)
	if err != nil || order == nil {
		return errors.ErrTradeNotFound
	}

	// 校验取消者是否为订单卖家
	if order.SellerID != playerID {
		return errors.ErrParamInvalid
	}

	// 校验订单状态
	if order.Status != model.TradeOrderOnSale {
		return errors.ErrTradeCancelled
	}

	// 将订单状态改为已下架
	success, cancelErr := s.tradeRepo.CancelOrder(ctx, orderID, playerID)
	if cancelErr != nil || !success {
		logger.Error("取消交易订单失败", "order_id", orderID, "player_id", playerID, "err", cancelErr)
		return errors.ErrInternal
	}

	logger.Info("取消交易行上架", "player_id", playerID, "order_id", orderID)
	return nil
}