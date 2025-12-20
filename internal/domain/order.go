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
}

// UpdateStatus 更新訂單狀態（包含狀態機驗證）
func (o *Order) UpdateStatus(newStatus OrderStatus) error {
	if !CanTransitionTo(o.Status, newStatus) {
		return ErrInvalidStatusTransition{
			FromStatus: o.Status,
			ToStatus:   newStatus,
		}
	}

	o.Status = newStatus
	o.UpdatedAt = time.Now()

	return nil
}

// ErrInvalidStatusTransition 無效的狀態轉換錯誤
type ErrInvalidStatusTransition struct {
	FromStatus OrderStatus
	ToStatus   OrderStatus
}

func (e ErrInvalidStatusTransition) Error() string {
	return "invalid status transition: cannot transition from " + string(e.FromStatus) + " to " + string(e.ToStatus)
}
