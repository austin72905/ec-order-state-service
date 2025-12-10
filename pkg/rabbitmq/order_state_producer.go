package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MQ 常數定義（與主後端保持一致）
const (
	// 主 Exchange 和 Queue
	OrderStateExchange    = "order.state.update"
	OrderStateQueue       = "order_state_changed_queue"
	OrderStateRoutingKey  = "order.state.status.changed"
	
	// 死信 Exchange 和 Queue
	DeadLetterExchange    = "dead.letter.exchange"
	DeadLetterQueue       = "order_state_changed_queue.dlq"
	DeadLetterRoutingKey  = "order.state.changed.dlq"
)

// OrderStateProducer 訂單狀態更新生產者
type OrderStateProducer struct {
	conn        *Connection
	channel     *amqp.Channel
	channelLock sync.Mutex
	initialized bool
}

// NewOrderStateProducer 建立訂單狀態更新生產者
func NewOrderStateProducer(conn *Connection) (*OrderStateProducer, error) {
	return &OrderStateProducer{
		conn:        conn,
		initialized: false,
	}, nil
}

// OrderStatusChangedMessage 訂單狀態變更訊息（統一使用數字 enum）
type OrderStatusChangedMessage struct {
	EventType  string `json:"eventType"`
	OrderID    string `json:"orderId"`    // 使用 RecordCode 作為 orderId
	FromStatus int    `json:"fromStatus"` // 統一使用數字 enum
	ToStatus   int    `json:"toStatus"`   // 統一使用數字 enum
	Timestamp  string `json:"timestamp"`
}

// PublishOrderStatusChanged 發布訂單狀態變更事件
func (p *OrderStateProducer) PublishOrderStatusChanged(message OrderStatusChangedMessage) error {
	// 取得或建立可重用的 Channel，並確保 Exchange/Queue/Binding 已宣告
	channel, err := p.getOrCreateChannel()
	if err != nil {
		return fmt.Errorf("取得 Channel 失敗: %w", err)
	}

	// 序列化訊息
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化訊息失敗: %w", err)
	}

	// 發布訊息（不再每次宣告 Queue，因為已在初始化時完成）
	err = channel.Publish(
		OrderStateExchange,    // exchange
		OrderStateRoutingKey,  // routing key
		false,                 // mandatory
		false,                 // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         messageJson,
			DeliveryMode: amqp.Persistent, // 持久化訊息
		},
	)
	if err != nil {
		// 發送失敗時重置 channel，下次會重新建立
		p.channel = nil
		p.initialized = false
		return fmt.Errorf("發布訊息失敗: %w", err)
	}

	log.Printf("已發布訂單狀態變更事件: orderId=%s, fromStatus=%d, toStatus=%d",
		message.OrderID, message.FromStatus, message.ToStatus)

	return nil
}

// getOrCreateChannel 取得或建立可重用的 Channel，並確保 Exchange/Queue/Binding 已宣告
func (p *OrderStateProducer) getOrCreateChannel() (*amqp.Channel, error) {
	// 檢查 channel 是否已開啟
	if p.channel != nil && !p.channel.IsClosed() && p.initialized {
		return p.channel, nil
	}

	// 使用鎖保護，避免併發建立多個 channel
	p.channelLock.Lock()
	defer p.channelLock.Unlock()

	// 雙重檢查（Double-check locking）
	if p.channel != nil && !p.channel.IsClosed() && p.initialized {
		return p.channel, nil
	}

	// 關閉舊 channel（如果存在）
	if p.channel != nil {
		if !p.channel.IsClosed() {
			p.channel.Close()
		}
		p.channel = nil
	}

	// 建立新 channel
	channel, err := p.conn.GetNewChannel()
	if err != nil {
		return nil, fmt.Errorf("建立 Channel 失敗: %w", err)
	}

	// 初始化 Exchange/Queue/Binding
	if err := p.initializeExchangeAndQueue(channel); err != nil {
		channel.Close()
		return nil, fmt.Errorf("初始化 Exchange/Queue 失敗: %w", err)
	}

	p.channel = channel
	p.initialized = true
	return channel, nil
}

// initializeExchangeAndQueue 初始化 Exchange、Queue 和 Binding（與主後端保持一致）
func (p *OrderStateProducer) initializeExchangeAndQueue(channel *amqp.Channel) error {
	// 宣告主交換器
	err := channel.ExchangeDeclare(
		OrderStateExchange, // exchange name
		"direct",           // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告主交換器失敗: %w", err)
	}

	// 宣告死信交換器
	err = channel.ExchangeDeclare(
		DeadLetterExchange, // exchange name
		"direct",           // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告死信交換器失敗: %w", err)
	}

	// 宣告死信佇列
	_, err = channel.QueueDeclare(
		DeadLetterQueue, // queue name
		true,            // durable
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告死信佇列失敗: %w", err)
	}

	// 綁定死信佇列
	err = channel.QueueBind(
		DeadLetterQueue,      // queue name
		DeadLetterRoutingKey, // routing key
		DeadLetterExchange,   // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("綁定死信佇列失敗: %w", err)
	}

	// 宣告主佇列（帶死信參數）
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    DeadLetterExchange,
		"x-dead-letter-routing-key": DeadLetterRoutingKey,
	}
	_, err = channel.QueueDeclare(
		OrderStateQueue, // queue name
		true,            // durable
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		queueArgs,       // arguments - 設定死信參數
	)
	if err != nil {
		return fmt.Errorf("宣告主佇列失敗: %w", err)
	}

	// 綁定主佇列
	err = channel.QueueBind(
		OrderStateQueue,      // queue name
		OrderStateRoutingKey, // routing key
		OrderStateExchange,   // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("綁定主佇列失敗: %w", err)
	}

	return nil
}

// Close 關閉生產者
func (p *OrderStateProducer) Close() error {
	p.channelLock.Lock()
	defer p.channelLock.Unlock()

	if p.channel != nil {
		if !p.channel.IsClosed() {
			if err := p.channel.Close(); err != nil {
				return fmt.Errorf("關閉 Channel 失敗: %w", err)
			}
		}
		p.channel = nil
	}
	p.initialized = false
	return nil
}

