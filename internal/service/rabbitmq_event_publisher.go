package service

import (
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/pkg/rabbitmq"
	"fmt"
	"time"
)

// RabbitMQEventPublisher RabbitMQ 事件發布器
type RabbitMQEventPublisher struct {
	producer *rabbitmq.OrderStateProducer
}

// NewRabbitMQEventPublisher 創建 RabbitMQ 事件發布器
func NewRabbitMQEventPublisher(producer *rabbitmq.OrderStateProducer) *RabbitMQEventPublisher {
	return &RabbitMQEventPublisher{
		producer: producer,
	}
}

// PublishOrderStatusChanged 發布訂單狀態變更事件（統一使用數字 enum）
func (p *RabbitMQEventPublisher) PublishOrderStatusChanged(event domain.OrderStatusChangedEvent) error {
	message := rabbitmq.OrderStatusChangedMessage{
		EventType:  event.EventType(),
		OrderID:    event.OrderID(),
		FromStatus: event.FromStatus.ToInt(), // 轉換為數字 enum
		ToStatus:   event.ToStatus.ToInt(),   // 轉換為數字 enum
		Timestamp:  event.Timestamp().Format(time.RFC3339),
	}

	if err := p.producer.PublishOrderStatusChanged(message); err != nil {
		return fmt.Errorf("發布訂單狀態變更事件失敗: %w", err)
	}

	return nil
}

