package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection RabbitMQ 連接管理
type Connection struct {
	conn *amqp.Connection
	url  string
}

// NewConnection 建立新的 RabbitMQ 連接
func NewConnection(url string) (*Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("連接 RabbitMQ 失敗: %w", err)
	}

	return &Connection{
		conn: conn,
		url:  url,
	}, nil
}

// GetNewChannel 獲取新的 Channel（每次調用返回新的 Channel，避免共享）
func (c *Connection) GetNewChannel() (*amqp.Channel, error) {
	// 檢查連接是否已關閉
	if c.conn.IsClosed() {
		return nil, fmt.Errorf("連接已關閉")
	}

	channel, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("建立 Channel 失敗: %w", err)
	}
	return channel, nil
}

// Close 關閉連接
func (c *Connection) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Reconnect 重新連接
func (c *Connection) Reconnect() error {
	// 關閉舊連接（如果存在且未關閉）
	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.Close(); err != nil {
			// 記錄錯誤但不阻止重連
			fmt.Printf("關閉舊連接時發生錯誤: %v\n", err)
		}
	}

	// 重新連接
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("重新連接 RabbitMQ 失敗: %w", err)
	}

	c.conn = conn
	return nil
}

