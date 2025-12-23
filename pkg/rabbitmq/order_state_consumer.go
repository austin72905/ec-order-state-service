package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// OrderStateConsumer 訂單狀態更新消費者
type OrderStateConsumer struct {
	conn           *Connection
	channel        *amqp.Channel
	handler        OrderStateEventHandler
	workerCount    int // Worker Pool 大小
	prefetchCount  int // Prefetch count
	workerChan     chan amqp.Delivery
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	messageTimeout time.Duration // 消息處理超時時間
}

// OrderStateEventHandler 訂單狀態事件處理器介面
type OrderStateEventHandler interface {
	HandlePaymentCompleted(event PaymentCompletedMessage) error
}

// NewOrderStateConsumer 建立訂單狀態更新消費者（使用預設配置）
func NewOrderStateConsumer(conn *Connection, handler OrderStateEventHandler) (*OrderStateConsumer, error) {
	return NewOrderStateConsumerWithConfig(conn, handler, 10, 10)
}

// NewOrderStateConsumerWithConfig 建立訂單狀態更新消費者（使用自訂配置）
func NewOrderStateConsumerWithConfig(conn *Connection, handler OrderStateEventHandler, prefetchCount, workerCount int) (*OrderStateConsumer, error) {
	// 為 Consumer 創建獨立的 Channel
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

	// 宣告 RabbitMQ 死信交換器和佇列（與主後端保持一致）
	dlx := "dead.letter.exchange"
	dlq := "order_state_queue.dlq"
	dlqRoutingKey := "order.state.dlq"

	// 宣告死信交換器
	err = channel.ExchangeDeclare(
		dlx,      // exchange name
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("宣告死信交換器失敗: %w", err)
	}

	// 宣告 RabbitMQ 死信佇列（用於 RabbitMQ 自動轉發失敗訊息）
	_, err = channel.QueueDeclare(
		dlq,   // dead letter queue name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("宣告 RabbitMQ 死信佇列失敗: %w", err)
	}

	// 綁定 RabbitMQ 死信佇列
	err = channel.QueueBind(
		dlq,           // queue name
		dlqRoutingKey, // routing key
		dlx,           // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("綁定 RabbitMQ 死信佇列失敗: %w", err)
	}

	// 宣告應用層死信隊列（用於處理重試超過限制的消息）
	_, err = channel.QueueDeclare(
		"order_state_queue_dlq", // application-level dead letter queue name
		true,                    // durable
		false,                   // delete when unused
		false,                   // exclusive
		false,                   // no-wait
		nil,                     // arguments
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("宣告應用層死信隊列失敗: %w", err)
	}

	// 綁定應用層死信隊列（使用不同的 routing key）
	err = channel.QueueBind(
		"order_state_queue_dlq",             // queue name
		"order.state.payment.completed.dlq", // routing key for DLQ
		"order.state.update",                // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("綁定應用層死信隊列失敗: %w", err)
	}

	// 宣告隊列（與後端保持一致，包含死信參數）
	_, err = channel.QueueDeclare(
		"order_state_queue", // queue name
		true,                // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		amqp.Table{ // arguments - 設定死信參數
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": dlqRoutingKey,
		},
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("宣告隊列失敗: %w", err)
	}

	// 綁定交換器與隊列（使用與後端一致的 routing key）
	err = channel.QueueBind(
		"order_state_queue",             // queue name
		"order.state.payment.completed", // routing key
		"order.state.update",            // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("綁定隊列失敗: %w", err)
	}

	// 設置公平分發（使用配置的 prefetch count）
	err = channel.Qos(
		prefetchCount, // prefetch count（可配置）
		0,             // prefetch size
		false,         // global
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("設置 Qos 失敗: %w", err)
	}

	// 計算 worker channel 緩衝區大小（至少為 worker 數量的 2 倍，但最多 100）
	bufferSize := workerCount * 2
	if bufferSize > 100 {
		bufferSize = 100
	}

	return &OrderStateConsumer{
		conn:           conn,
		channel:        channel,
		handler:        handler,
		workerCount:    workerCount,
		prefetchCount:  prefetchCount,
		workerChan:     make(chan amqp.Delivery, bufferSize), // Worker 都在忙碌時，messageReceiver 仍可繼續接收並放入緩衝區
		stopChan:       make(chan struct{}),
		messageTimeout: 30 * time.Second, // 預設 30 秒超時
	}, nil
}

// PaymentCompletedMessage 支付完成訊息（對應後端發送的消息格式）
type PaymentCompletedMessage struct {
	EventType string `json:"eventType"`
	OrderID   string `json:"orderId"` // 使用 RecordCode 作為 orderId
	Timestamp string `json:"timestamp"`
}

// Start 開始消費訊息（使用 Worker Pool 實現並發處理，支持自動重連）
func (c *OrderStateConsumer) Start() error {
	// 啟動 worker goroutines
	// 這邊用waitgroup 等待 go routine 完成
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go func(workerID int) {
			defer c.wg.Done()
			for {
				select {
				case msg, ok := <-c.workerChan:
					if !ok {
						log.Printf("Worker %d: worker channel 已關閉", workerID)
						return
					}
					c.handleMessage(msg, workerID)
				case <-c.stopChan:
					log.Printf("Worker %d: 收到停止信號", workerID)
					return
				}
			}
		}(i)
	}

	// 啟動消息接收 goroutine（支持自動重連）
	c.wg.Add(1)
	go c.messageReceiver()

	log.Printf("訂單狀態更新消費者已啟動 (Workers=%d, Prefetch=%d)", c.workerCount, c.prefetchCount)
	return nil
}

// messageReceiver 接收消息並分發到 worker channel（支持自動重連）
func (c *OrderStateConsumer) messageReceiver() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
			msgs, err := c.consumeMessages()
			if err != nil {
				log.Printf("接收消息失敗，嘗試重連: %v", err)
				// 嘗試重連
				if err := c.reconnect(); err != nil {
					log.Printf("重連失敗: %v，5 秒後重試", err)
					time.Sleep(5 * time.Second)
					continue
				}
				log.Println("重連成功，繼續接收消息")
				continue
			}

			// 接收消息並分發
			for msg := range msgs {
				select {
				case c.workerChan <- msg:
					// 成功發送到 worker channel (緩衝區有空間)
				case <-c.stopChan:
					// 收到停止信號，拒絕消息並返回
					msg.Nack(false, true) // 重新入隊
					return
				default:
					// worker channel 已滿，實現背壓：等待一小段時間 (背壓處理邏輯)  當下游處理速度跟不上上游生產速度時，向上游施加壓力，減緩或暫停生產，避免系統過載。
					/*
						檢測到緩衝區滿，記錄警告
						等待最多 5 秒，嘗試發送
						若 5 秒內有空間，發送成功
						若 5 秒後仍滿載，拒絕消息並重新入隊

					*/
					log.Printf("警告: worker channel 已滿，等待空間...")
					select {
					case c.workerChan <- msg:
						// 成功發送
					case <-time.After(5 * time.Second):
						// 5秒超時
						// 超時，拒絕消息並記錄
						log.Printf("錯誤: worker channel 長時間滿載，拒絕消息: orderId=%s",
							c.extractOrderID(msg))
						msg.Nack(false, true) // 重新入隊
					case <-c.stopChan:
						msg.Nack(false, true)
						return
					}
				}
			}

			// 消息通道關閉，可能是連接斷開，嘗試重連
			log.Println("消息通道已關閉，嘗試重連...")
			if err := c.reconnect(); err != nil {
				log.Printf("重連失敗: %v，5 秒後重試", err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// consumeMessages 消費消息
func (c *OrderStateConsumer) consumeMessages() (<-chan amqp.Delivery, error) {
	c.mu.RLock()
	channel := c.channel
	c.mu.RUnlock()

	if channel == nil {
		return nil, fmt.Errorf("channel 為 nil")
	}

	// 檢查 channel 是否已關閉
	if channel.IsClosed() {
		return nil, fmt.Errorf("channel 已關閉")
	}

	msgs, err := channel.Consume(
		"order_state_queue", // queue
		"",                  // consumer
		false,               // auto-ack
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	if err != nil {
		return nil, fmt.Errorf("註冊消費者失敗: %w", err)
	}

	return msgs, nil
}

// reconnect 重新連接並重新設置 channel
func (c *OrderStateConsumer) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 先保存舊 channel 的引用，然後設置為 nil，避免其他 goroutine 使用
	oldChannel := c.channel
	c.channel = nil

	// 關閉舊 channel（在設置為 nil 之後，避免競態條件）
	if oldChannel != nil && !oldChannel.IsClosed() {
		oldChannel.Close()
	}

	// 重新連接
	if err := c.conn.Reconnect(); err != nil {
		return fmt.Errorf("重新連接失敗: %w", err)
	}

	// 創建新 channel
	channel, err := c.conn.GetNewChannel()
	if err != nil {
		return fmt.Errorf("創建新 channel 失敗: %w", err)
	}

	// 重新設置交換器和隊列
	if err := c.setupChannel(channel); err != nil {
		channel.Close()
		return fmt.Errorf("設置 channel 失敗: %w", err)
	}

	// 設置新 channel
	c.channel = channel
	return nil
}

// setupChannel 設置 channel（宣告交換器、隊列等）
func (c *OrderStateConsumer) setupChannel(channel *amqp.Channel) error {
	// 宣告交換器
	if err := channel.ExchangeDeclare(
		"order.state.update", // exchange name
		"direct",             // type
		true,                 // durable
		false,                // auto-deleted
		false,                // internal
		false,                // no-wait
		nil,                  // arguments
	); err != nil {
		return fmt.Errorf("宣告交換器失敗: %w", err)
	}

	// 宣告 RabbitMQ 死信交換器和佇列（與主後端保持一致）
	dlx := "dead.letter.exchange"
	dlq := "order_state_queue.dlq"
	dlqRoutingKey := "order.state.dlq"

	// 宣告死信交換器
	if err := channel.ExchangeDeclare(
		dlx,      // exchange name
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	); err != nil {
		return fmt.Errorf("宣告死信交換器失敗: %w", err)
	}

	// 宣告 RabbitMQ 死信佇列（用於 RabbitMQ 自動轉發失敗訊息）
	_, err := channel.QueueDeclare(
		dlq,   // dead letter queue name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告 RabbitMQ 死信佇列失敗: %w", err)
	}

	// 綁定 RabbitMQ 死信佇列
	if err := channel.QueueBind(
		dlq,           // queue name
		dlqRoutingKey, // routing key
		dlx,           // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("綁定 RabbitMQ 死信佇列失敗: %w", err)
	}

	// 宣告應用層死信隊列（用於處理重試超過限制的消息）
	_, err = channel.QueueDeclare(
		"order_state_queue_dlq", // application-level dead letter queue name
		true,                    // durable
		false,                   // delete when unused
		false,                   // exclusive
		false,                   // no-wait
		nil,                     // arguments
	)
	if err != nil {
		return fmt.Errorf("宣告應用層死信隊列失敗: %w", err)
	}

	// 綁定應用層死信隊列
	if err := channel.QueueBind(
		"order_state_queue_dlq",
		"order.state.payment.completed.dlq",
		"order.state.update",
		false,
		nil,
	); err != nil {
		return fmt.Errorf("綁定應用層死信隊列失敗: %w", err)
	}

	// 宣告隊列（與後端保持一致，包含死信參數）
	_, err = channel.QueueDeclare(
		"order_state_queue",
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{ // arguments - 設定死信參數
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": dlqRoutingKey,
		},
	)
	if err != nil {
		return fmt.Errorf("宣告隊列失敗: %w", err)
	}

	// 綁定隊列
	if err := channel.QueueBind(
		"order_state_queue",
		"order.state.payment.completed",
		"order.state.update",
		false,
		nil,
	); err != nil {
		return fmt.Errorf("綁定隊列失敗: %w", err)
	}

	// 設置 Qos
	if err := channel.Qos(
		c.prefetchCount,
		0,
		false,
	); err != nil {
		return fmt.Errorf("設置 Qos 失敗: %w", err)
	}

	return nil
}

// extractOrderID 從消息中提取 OrderID（用於日誌）
func (c *OrderStateConsumer) extractOrderID(msg amqp.Delivery) string {
	var message PaymentCompletedMessage
	if err := json.Unmarshal(msg.Body, &message); err == nil {
		return message.OrderID
	}
	return "unknown"
}

// handleMessage 處理訊息（帶重試限制和超時機制）
func (c *OrderStateConsumer) handleMessage(msg amqp.Delivery, workerID int) {
	var message PaymentCompletedMessage
	if err := json.Unmarshal(msg.Body, &message); err != nil {
		log.Printf("[Worker %d] 解析訊息失敗: %v, 訊息內容: %s", workerID, err, string(msg.Body))
		msg.Nack(false, false) // 拒絕訊息，不重新入隊
		return
	}

	log.Printf("[Worker %d] 收到訂單狀態更新事件: eventType=%s, orderId=%s", workerID, message.EventType, message.OrderID)

	// 獲取重試次數
	retryCount := c.getRetryCount(msg)

	// 使用 context 實現超時機制
	ctx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
	defer cancel()

	// 使用 channel 來接收處理結果
	resultChan := make(chan error, 1)
	go func() {
		// 根據事件類型處理
		switch message.EventType {
		case "PaymentCompleted":
			resultChan <- c.handler.HandlePaymentCompleted(message)
		default:
			resultChan <- fmt.Errorf("未知的事件類型: %s", message.EventType)
		}
	}()

	// 等待處理結果或超時
	var err error
	select {
	case err = <-resultChan:
		// 處理完成
	case <-ctx.Done():
		// 超時
		err = fmt.Errorf("處理消息超時（超過 %v）", c.messageTimeout)
		log.Printf("[Worker %d] 處理消息超時: orderId=%s", workerID, message.OrderID)
	}

	if err != nil {
		log.Printf("[Worker %d] 處理支付完成事件失敗: %v (重試次數: %d, orderId=%s)",
			workerID, err, retryCount, message.OrderID)

		// 檢查重試次數
		if retryCount >= 3 {
			log.Printf("[Worker %d] 重試次數已達上限，將消息發送到死信隊列: orderId=%s", workerID, message.OrderID)
			if c.sendToDLQ(msg, message) {
				msg.Ack(false) // 確認消息已處理（已移到 DLQ）
			} else {
				// 發送 DLQ 失敗，重新入隊
				msg.Nack(false, true)
			}
			return
		}

		// 增加重試次數並重新發布消息（帶更新的 headers）
		newRetryCount := retryCount + 1
		log.Printf("[Worker %d] 增加重試次數並重新發布消息: orderId=%s, retryCount=%d",
			workerID, message.OrderID, newRetryCount)
		if c.republishWithRetryCount(msg, message, newRetryCount) {
			msg.Ack(false) // 確認原消息（已重新發布）
		} else {
			// 重新發布失敗，重新入隊
			msg.Nack(false, true)
		}
		return
	}

	log.Printf("[Worker %d] 支付完成事件處理成功: orderId=%s", workerID, message.OrderID)
	// 確認訊息
	msg.Ack(false)
}

// getRetryCount 從消息 headers 獲取重試次數
func (c *OrderStateConsumer) getRetryCount(msg amqp.Delivery) int {
	if msg.Headers == nil {
		return 0
	}

	if retryCount, ok := msg.Headers["x-retry-count"]; ok {
		if count, ok := retryCount.(int); ok {
			return count
		}
	}
	return 0
}

// republishWithRetryCount 重新發布消息並增加重試計數（返回是否成功）
func (c *OrderStateConsumer) republishWithRetryCount(originalMsg amqp.Delivery, message PaymentCompletedMessage, retryCount int) bool {
	c.mu.RLock()
	channel := c.channel
	c.mu.RUnlock()

	if channel == nil {
		log.Printf("重新發布消息失敗: channel 為 nil")
		return false
	}

	// 檢查 channel 是否已關閉
	if channel.IsClosed() {
		log.Printf("重新發布消息失敗: channel 已關閉")
		return false
	}

	messageJson, err := json.Marshal(message)
	if err != nil {
		log.Printf("序列化消息失敗: %v", err)
		return false
	}

	// 重新發布消息，帶更新的重試計數
	err = channel.Publish(
		"order.state.update",            // exchange
		"order.state.payment.completed", // routing key
		false,                           // mandatory
		false,                           // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         messageJson,
			DeliveryMode: amqp.Persistent,
			Headers: amqp.Table{
				"x-retry-count": retryCount,
			},
		},
	)
	if err != nil {
		log.Printf("重新發布消息失敗: %v", err)
		return false
	}
	log.Printf("消息已重新發布: orderId=%s, retryCount=%d", message.OrderID, retryCount)
	return true
}

// sendToDLQ 將消息發送到死信隊列（返回是否成功）
func (c *OrderStateConsumer) sendToDLQ(originalMsg amqp.Delivery, message PaymentCompletedMessage) bool {
	c.mu.RLock()
	channel := c.channel
	c.mu.RUnlock()

	if channel == nil {
		log.Printf("發送消息到死信隊列失敗: channel 為 nil")
		return false
	}

	// 檢查 channel 是否已關閉
	if channel.IsClosed() {
		log.Printf("發送消息到死信隊列失敗: channel 已關閉")
		return false
	}

	// 重新發布消息到死信隊列
	dlqMessage, err := json.Marshal(message)
	if err != nil {
		log.Printf("序列化死信消息失敗: %v", err)
		return false
	}

	// 使用不同的 routing key 發送到死信隊列
	err = channel.Publish(
		"order.state.update",                // exchange
		"order.state.payment.completed.dlq", // routing key for DLQ
		false,                               // mandatory
		false,                               // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         dlqMessage,
			DeliveryMode: amqp.Persistent,
			Headers: amqp.Table{
				"x-original-queue": "order_state_queue",
				"x-retry-count":    c.getRetryCount(originalMsg),
				"x-failed-reason":  "max retries exceeded",
			},
		},
	)
	if err != nil {
		log.Printf("發送消息到死信隊列失敗: %v", err)
		return false
	}
	log.Printf("消息已發送到死信隊列: orderId=%s", message.OrderID)
	return true
}

// Stop 停止消費者（優雅關閉）
func (c *OrderStateConsumer) Stop() error {
	close(c.stopChan)
	close(c.workerChan)

	// 等待所有 worker 完成
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		if err := c.channel.Cancel("", false); err != nil {
			return err
		}
		c.channel.Close()
	}

	log.Println("RabbitMQ Consumer 已停止")
	return nil
}
