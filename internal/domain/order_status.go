package domain

// OrderStatus 訂單狀態枚舉
type OrderStatus string

const (
	StatusCreated            OrderStatus = "Created"
	StatusWaitingForPayment  OrderStatus = "WaitingForPayment"
	StatusWaitingForShipment OrderStatus = "WaitingForShipment"
	StatusInTransit          OrderStatus = "InTransit"
	StatusWaitPickup         OrderStatus = "WaitPickup"
	StatusCompleted          OrderStatus = "Completed"
	StatusCanceled           OrderStatus = "Canceled"
	StatusRefund             OrderStatus = "Refund"
)

// String 返回狀態的字串表示
func (s OrderStatus) String() string {
	return string(s)
}

// ToInt 將狀態轉換為數字 enum 值（與 .NET 的 OrderStatus enum 保持一致）
func (s OrderStatus) ToInt() int {
	switch s {
	case StatusCreated:
		return 0
	case StatusWaitingForShipment:
		return 2
	case StatusInTransit:
		return 7
	case StatusWaitPickup:
		return 3
	case StatusCompleted:
		return 4
	case StatusCanceled:
		return 5
	case StatusRefund:
		return 6
	default:
		return -1 // 無效狀態
	}
}

// CanTransitionTo 檢查是否可以從當前狀態轉換到目標狀態
func CanTransitionTo(from, to OrderStatus) bool {
	allowedTransitions := map[OrderStatus][]OrderStatus{
		StatusCreated:            {StatusWaitingForShipment, StatusCanceled}, // 支付完成後直接到等待出貨
		StatusWaitingForShipment: {StatusInTransit},
		StatusInTransit:          {StatusWaitPickup},
		StatusWaitPickup:         {StatusCompleted},
		StatusCompleted:          {}, // 已完成狀態不能轉換到其他狀態
		StatusCanceled:           {}, // 已取消狀態不能轉換到其他狀態
		StatusRefund:             {}, // 退款狀態不能轉換到其他狀態
	}

	allowed, exists := allowedTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}

	return false
}

