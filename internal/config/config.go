package config

import (
	"os"
	"strconv"
	"time"
)

// Config 配置結構
type Config struct {
	// 服務端口
	Port string
	// 資料庫配置
	DatabaseURL string
	// RabbitMQ 配置
	RabbitMQURL string
	// 資料庫連接池配置
	DBMaxConns          int
	DBMinConns          int
	DBMaxConnLifetime   time.Duration // 連接最大生命週期
	DBMaxConnIdleTime   time.Duration // 連接最大空閒時間
	DBHealthCheckPeriod time.Duration // 健康檢查週期
	// 查詢超時配置
	DBQueryTimeout time.Duration // 查詢操作超時時間
	DBWriteTimeout time.Duration // 寫入操作超時時間
	// RabbitMQ Consumer 配置
	RabbitMQPrefetchCount int // Prefetch count
	RabbitMQWorkerCount    int // Worker 數量
	// 調度器配置
	SchedulerWorkerCount int           // 調度器 Worker 數量
	SchedulerBatchSize   int           // 每次處理的訂單數量
	SchedulerInterval    time.Duration // 調度器檢查間隔
}

// LoadConfig 載入配置（從環境變數）
func LoadConfig() *Config {
	cfg := &Config{
		Port: getEnv("PORT", "8080"),
		// 資料庫配置（使用與主程式相同的 PostgreSQL 實體，但不同的資料庫）
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable"),
		// RabbitMQ 配置
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		// 資料庫連接池配置
		DBMaxConns:          getEnvAsInt("DB_MAX_CONNS", 20),
		DBMinConns:          getEnvAsInt("DB_MIN_CONNS", 5),
		DBMaxConnLifetime:   getEnvAsDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		DBMaxConnIdleTime:   getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
		DBHealthCheckPeriod: getEnvAsDuration("DB_HEALTH_CHECK_PERIOD", 1*time.Minute),
		// 查詢超時配置
		DBQueryTimeout: getEnvAsDuration("DB_QUERY_TIMEOUT", 5*time.Second),
		DBWriteTimeout: getEnvAsDuration("DB_WRITE_TIMEOUT", 10*time.Second),
		// RabbitMQ Consumer 配置
		RabbitMQPrefetchCount: getEnvAsInt("RABBITMQ_PREFETCH_COUNT", 10),
		RabbitMQWorkerCount:    getEnvAsInt("RABBITMQ_WORKER_COUNT", 10),
		// 調度器配置
		SchedulerWorkerCount: getEnvAsInt("SCHEDULER_WORKER_COUNT", 5),
		SchedulerBatchSize:   getEnvAsInt("SCHEDULER_BATCH_SIZE", 100),
		SchedulerInterval:    getEnvAsDuration("SCHEDULER_INTERVAL", 30*time.Second),
	}

	return cfg
}

// GetPort 獲取服務端口
func (c *Config) GetPort() string {
	return c.Port
}

// GetDatabaseURL 獲取資料庫連接字串
func (c *Config) GetDatabaseURL() string {
	return c.DatabaseURL
}

// GetRabbitMQURL 獲取 RabbitMQ 連接字串
func (c *Config) GetRabbitMQURL() string {
	return c.RabbitMQURL
}

// getEnv 獲取環境變數，如果不存在則返回預設值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 獲取環境變數並轉換為整數，如果不存在則返回預設值
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration 獲取環境變數並轉換為時間間隔，如果不存在則返回預設值
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

