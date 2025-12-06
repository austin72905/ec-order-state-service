package handler

import (
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/internal/service"
	"ec-order-state-service/pkg/rabbitmq"
	"fmt"
	"log"
	"time"
)

// OrderStateEventHandlerImpl 訂單狀態事件處理器實作
type OrderStateEventHandlerImpl struct {
	orderStateService *service.OrderStateService
}

// NewOrderStateEventHandler 建立訂單狀態事件處理器
func NewOrderStateEventHandler(orderStateService *service.OrderStateService) *OrderStateEventHandlerImpl {
	return &OrderStateEventHandlerImpl{
		orderStateService: orderStateService,
	}
}

// HandlePaymentCompleted 處理支付完成事件（帶結構化日誌和錯誤處理）
func (h *OrderStateEventHandlerImpl) HandlePaymentCompleted(msg rabbitmq.PaymentCompletedMessage) error {
	startTime := time.Now()
	log.Printf("[Handler] 開始處理支付完成事件: orderId=%s, timestamp=%s", msg.OrderID, msg.Timestamp)

	// 創建 PaymentCompletedEvent
	event := domain.NewPaymentCompletedEvent(msg.OrderID)

	// 使用服務處理事件
	if err := h.orderStateService.HandlePaymentCompleted(event); err != nil {
		duration := time.Since(startTime)
		log.Printf("[Handler] 處理支付完成事件失敗: orderId=%s, error=%v, duration=%v", 
			msg.OrderID, err, duration)
		return fmt.Errorf("處理支付完成事件失敗 (orderId=%s): %w", msg.OrderID, err)
	}

	duration := time.Since(startTime)
	log.Printf("[Handler] 支付完成事件處理成功: orderId=%s, duration=%v", msg.OrderID, duration)
	return nil
}

