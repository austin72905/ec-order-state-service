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

