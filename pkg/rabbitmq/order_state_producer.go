package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// OrderStateProducer 訂單狀態更新生產者
type OrderStateProducer struct {
	conn    *Connection
	channel *amqp.Channel
}

// NewOrderStateProducer 建立訂單狀態更新生產者
func NewOrderStateProducer(conn *Connection) (*OrderStateProducer, error) {
	// 為 Producer 創建獨立的 Channel
	channel, err := conn.GetNewChannel()
	if err != nil {
		return nil, fmt.Errorf("建立 Channel 失敗: %w", err)
	}

	// 宣告交換器（與後端保持一致）
	err = channel.ExchangeDeclare(
		"order.state.update", // exchange name
		"direct",             // type
		true,                 // durable
		false,                // auto-deleted
		false,                // internal
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("宣告交換器失敗: %w", err)
	}

	return &OrderStateProducer{
		conn:    conn,
		channel: channel,
	}, nil
}

// OrderStatusChangedMessage 訂單狀態變更訊息
type OrderStatusChangedMessage struct {
	EventType  string `json:"eventType"`
	OrderID    string `json:"orderId"`    // 使用 RecordCode 作為 orderId
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
	Timestamp  string `json:"timestamp"`
}

// PublishOrderStatusChanged 發布訂單狀態變更事件
func (p *OrderStateProducer) PublishOrderStatusChanged(message OrderStatusChangedMessage) error {
	// 確保隊列存在並綁定
	_, err := p.channel.QueueDeclare(
		"order_state_changed_queue", // queue name
		true,                         // durable
		false,                        // delete when unused
		false,                        // exclusive
		false,                        // no-wait
		nil,                          // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告隊列失敗: %w", err)
	}

	// 綁定交換器與隊列
	err = p.channel.QueueBind(
		"order_state_changed_queue",    // queue name
		"order.state.status.changed",   // routing key
		"order.state.update",           // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("綁定隊列失敗: %w", err)
	}

	// 序列化訊息
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化訊息失敗: %w", err)
	}

	// 發布訊息
	err = p.channel.Publish(
		"order.state.update",           // exchange
		"order.state.status.changed",   // routing key
		false,                          // mandatory
		false,                          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         messageJson,
			DeliveryMode: amqp.Persistent, // 持久化訊息
		},
	)
	if err != nil {
		return fmt.Errorf("發布訊息失敗: %w", err)
	}

	log.Printf("已發布訂單狀態變更事件: orderId=%s, fromStatus=%s, toStatus=%s",
		message.OrderID, message.FromStatus, message.ToStatus)

	return nil
}

// Close 關閉生產者
func (p *OrderStateProducer) Close() error {
	if p.channel != nil {
		return p.channel.Close()
	}
	return nil
}

