package domain

import (
	"time"
)

// Order 訂單聚合根
type Order struct {
	ID        string      `json:"orderId"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	OrderSteps []OrderStep `json:"orderSteps,omitempty"`
}

// OrderStep 訂單步驟記錄
type OrderStep struct {
	ID         int64       `json:"id"`
	OrderID    string      `json:"orderId"`
	FromStatus OrderStatus `json:"fromStatus"`
	ToStatus   OrderStatus `json:"toStatus"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// UpdateStatus 更新訂單狀態（包含狀態機驗證）
func (o *Order) UpdateStatus(newStatus OrderStatus) error {
	if !CanTransitionTo(o.Status, newStatus) {
		return ErrInvalidStatusTransition{
			FromStatus: o.Status,
			ToStatus:   newStatus,
		}
	}

	fromStatus := o.Status
	o.Status = newStatus
	o.UpdatedAt = time.Now()

	// 自動添加 OrderStep 記錄
	o.AddOrderStep(fromStatus, newStatus)

	return nil
}

// AddOrderStep 添加訂單步驟記錄
func (o *Order) AddOrderStep(fromStatus, toStatus OrderStatus) {
	step := OrderStep{
		OrderID:    o.ID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		CreatedAt:  time.Now(),
	}
	o.OrderSteps = append(o.OrderSteps, step)
}

// GetOrderSteps 獲取訂單步驟列表
func (o *Order) GetOrderSteps() []OrderStep {
	return o.OrderSteps
}

// ErrInvalidStatusTransition 無效的狀態轉換錯誤
type ErrInvalidStatusTransition struct {
	FromStatus OrderStatus
	ToStatus   OrderStatus
}

func (e ErrInvalidStatusTransition) Error() string {
	return "invalid status transition: cannot transition from " + string(e.FromStatus) + " to " + string(e.ToStatus)
}

