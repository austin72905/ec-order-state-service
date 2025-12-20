package repository

import (
	"ec-order-state-service/internal/domain"
	"sync"
)

// OrderRepository 訂單倉儲介面
type OrderRepository interface {
	GetByID(orderID string) (*domain.Order, error)
	Save(order *domain.Order) error
	GetOrdersByStatus(status domain.OrderStatus, limit, offset int) ([]*domain.Order, error)
	// BatchUpdateOrderStatus 批量更新訂單狀態
	BatchUpdateOrderStatus(orderIDs []string, newStatus domain.OrderStatus, fromStatus domain.OrderStatus) error
}

// InMemoryOrderRepository 記憶體實作的訂單倉儲（用於測試）
type InMemoryOrderRepository struct {
	orders map[string]*domain.Order
	mu     sync.RWMutex
}

// NewInMemoryOrderRepository 創建記憶體倉儲
func NewInMemoryOrderRepository() *InMemoryOrderRepository {
	return &InMemoryOrderRepository{
		orders: make(map[string]*domain.Order),
	}
}

// GetByID 根據 ID 獲取訂單
func (r *InMemoryOrderRepository) GetByID(orderID string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, exists := r.orders[orderID]
	if !exists {
		return nil, ErrOrderNotFound{OrderID: orderID}
	}

	// 複製訂單以避免外部修改
	orderCopy := *order
	return &orderCopy, nil
}

// Save 保存訂單
func (r *InMemoryOrderRepository) Save(order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 深拷貝訂單
	orderCopy := *order
	r.orders[order.ID] = &orderCopy

	return nil
}

// GetOrdersByStatus 根據狀態獲取訂單列表（支持分頁）
func (r *InMemoryOrderRepository) GetOrdersByStatus(status domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orders []*domain.Order
	for _, order := range r.orders {
		if order.Status == status {
			// 複製訂單以避免外部修改
			orderCopy := *order
			orders = append(orders, &orderCopy)
		}
	}

	// 實作分頁邏輯
	if limit <= 0 {
		limit = len(orders) // 如果 limit <= 0，返回所有結果
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(orders) {
		return []*domain.Order{}, nil
	}
	end := offset + limit
	if end > len(orders) {
		end = len(orders)
	}

	return orders[offset:end], nil
}

// BatchUpdateOrderStatus 批量更新訂單狀態（記憶體實作）
func (r *InMemoryOrderRepository) BatchUpdateOrderStatus(orderIDs []string, newStatus domain.OrderStatus, fromStatus domain.OrderStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, orderID := range orderIDs {
		order, exists := r.orders[orderID]
		if exists && order.Status == fromStatus {
			order.Status = newStatus
		}
	}

	return nil
}

// ErrOrderNotFound 訂單未找到錯誤
type ErrOrderNotFound struct {
	OrderID string
}

func (e ErrOrderNotFound) Error() string {
	return "order not found: " + e.OrderID
}

