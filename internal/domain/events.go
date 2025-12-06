package domain

import "time"

// Event 事件介面
type Event interface {
	EventType() string
	OrderID() string
	Timestamp() time.Time
}

// OrderCreatedEvent 訂單創建事件
type OrderCreatedEvent struct {
	orderID   string    `json:"orderId"`
	timestamp time.Time `json:"timestamp"`
}

func (e OrderCreatedEvent) EventType() string { return "OrderCreated" }
func (e OrderCreatedEvent) OrderID() string     { return e.orderID }
func (e OrderCreatedEvent) Timestamp() time.Time { return e.timestamp }

// NewOrderCreatedEvent 創建訂單創建事件
func NewOrderCreatedEvent(orderID string) OrderCreatedEvent {
	return OrderCreatedEvent{
		orderID:   orderID,
		timestamp: time.Now(),
	}
}

// PaymentCompletedEvent 付款完成事件
type PaymentCompletedEvent struct {
	orderID   string    `json:"orderId"`
	timestamp time.Time `json:"timestamp"`
}

func (e PaymentCompletedEvent) EventType() string { return "PaymentCompleted" }
func (e PaymentCompletedEvent) OrderID() string   { return e.orderID }
func (e PaymentCompletedEvent) Timestamp() time.Time { return e.timestamp }

// NewPaymentCompletedEvent 創建付款完成事件
func NewPaymentCompletedEvent(orderID string) PaymentCompletedEvent {
	return PaymentCompletedEvent{
		orderID:   orderID,
		timestamp: time.Now(),
	}
}

// ShipmentStartedEvent 物流開始事件
type ShipmentStartedEvent struct {
	orderID        string    `json:"orderId"`
	TrackingNumber string    `json:"trackingNumber"`
	timestamp      time.Time `json:"timestamp"`
}

func (e ShipmentStartedEvent) EventType() string { return "ShipmentStarted" }
func (e ShipmentStartedEvent) OrderID() string   { return e.orderID }
func (e ShipmentStartedEvent) Timestamp() time.Time { return e.timestamp }

// NewShipmentStartedEvent 創建物流開始事件
func NewShipmentStartedEvent(orderID, trackingNumber string) ShipmentStartedEvent {
	return ShipmentStartedEvent{
		orderID:        orderID,
		TrackingNumber: trackingNumber,
		timestamp:      time.Now(),
	}
}

// PackageArrivedAtPickupStoreEvent 包裹到達取貨門市事件
type PackageArrivedAtPickupStoreEvent struct {
	orderID   string    `json:"orderId"`
	StoreID   string    `json:"storeId"`
	timestamp time.Time `json:"timestamp"`
}

func (e PackageArrivedAtPickupStoreEvent) EventType() string { return "PackageArrivedAtPickupStore" }
func (e PackageArrivedAtPickupStoreEvent) OrderID() string   { return e.orderID }
func (e PackageArrivedAtPickupStoreEvent) Timestamp() time.Time { return e.timestamp }

// NewPackageArrivedAtPickupStoreEvent 創建包裹到達取貨門市事件
func NewPackageArrivedAtPickupStoreEvent(orderID, storeID string) PackageArrivedAtPickupStoreEvent {
	return PackageArrivedAtPickupStoreEvent{
		orderID:   orderID,
		StoreID:   storeID,
		timestamp: time.Now(),
	}
}

// PackagePickedUpEvent 包裹已取貨事件
type PackagePickedUpEvent struct {
	orderID    string    `json:"orderId"`
	PickedUpAt time.Time `json:"pickedUpAt"`
	timestamp  time.Time `json:"timestamp"`
}

func (e PackagePickedUpEvent) EventType() string { return "PackagePickedUp" }
func (e PackagePickedUpEvent) OrderID() string   { return e.orderID }
func (e PackagePickedUpEvent) Timestamp() time.Time { return e.timestamp }

// NewPackagePickedUpEvent 創建包裹已取貨事件
func NewPackagePickedUpEvent(orderID string, pickedUpAt time.Time) PackagePickedUpEvent {
	return PackagePickedUpEvent{
		orderID:    orderID,
		PickedUpAt: pickedUpAt,
		timestamp:  time.Now(),
	}
}

// OrderStatusChangedEvent 訂單狀態變更事件（發布用）
type OrderStatusChangedEvent struct {
	orderID    string      `json:"orderId"`
	FromStatus OrderStatus `json:"fromStatus"`
	ToStatus   OrderStatus `json:"toStatus"`
	timestamp  time.Time   `json:"timestamp"`
}

func (e OrderStatusChangedEvent) EventType() string { return "OrderStatusChanged" }
func (e OrderStatusChangedEvent) OrderID() string   { return e.orderID }
func (e OrderStatusChangedEvent) Timestamp() time.Time { return e.timestamp }

// NewOrderStatusChangedEvent 創建訂單狀態變更事件
func NewOrderStatusChangedEvent(orderID string, fromStatus, toStatus OrderStatus) OrderStatusChangedEvent {
	return OrderStatusChangedEvent{
		orderID:    orderID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		timestamp:  time.Now(),
	}
}

