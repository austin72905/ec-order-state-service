package main

import (
	"context"
	"ec-order-state-service/internal/config"
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/internal/handler"
	"ec-order-state-service/internal/repository"
	"ec-order-state-service/internal/scheduler"
	"ec-order-state-service/internal/service"
	"ec-order-state-service/pkg/migrate"
	"ec-order-state-service/pkg/rabbitmq"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 載入配置
	cfg := config.LoadConfig()

	// 執行資料庫 migration
	log.Println("正在執行資料庫 migration...")
	migrationPath := "file://migrations"
	if err := migrate.RunMigrations(cfg.GetDatabaseURL(), migrationPath); err != nil {
		log.Fatalf("資料庫 migration 失敗: %v", err)
	}

	// 初始化資料庫連接池
	log.Println("正在連接資料庫...")
	dbConfig, err := pgxpool.ParseConfig(cfg.GetDatabaseURL())
	if err != nil {
		log.Fatalf("解析資料庫配置失敗: %v", err)
	}

	// 配置連接池大小
	dbConfig.MaxConns = int32(cfg.DBMaxConns)
	dbConfig.MinConns = int32(cfg.DBMinConns)
	
	// 配置連接池進階參數
	dbConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	dbConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime
	dbConfig.HealthCheckPeriod = cfg.DBHealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("連接資料庫失敗: %v", err)
	}
	defer pool.Close()

	// 測試資料庫連接
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("資料庫連接測試失敗: %v", err)
	}
	log.Printf("資料庫連接成功 (連接池: Min=%d, Max=%d, MaxLifetime=%v, MaxIdleTime=%v, HealthCheckPeriod=%v)",
		cfg.DBMinConns, cfg.DBMaxConns, cfg.DBMaxConnLifetime, cfg.DBMaxConnIdleTime, cfg.DBHealthCheckPeriod)

	// 建立 Repository（傳入配置）
	orderRepo := repository.NewPgOrderRepositoryWithConfig(pool, cfg.DBQueryTimeout, cfg.DBWriteTimeout)

	// 初始化 RabbitMQ 連接
	log.Println("正在連接 RabbitMQ...")
	rabbitMQConn, err := rabbitmq.NewConnection(cfg.GetRabbitMQURL())
	if err != nil {
		log.Fatalf("連接 RabbitMQ 失敗: %v", err)
	}
	defer rabbitMQConn.Close()
	log.Println("RabbitMQ 連接成功")

	// 建立 RabbitMQ Producer
	orderStateProducer, err := rabbitmq.NewOrderStateProducer(rabbitMQConn)
	if err != nil {
		log.Fatalf("建立 RabbitMQ Producer 失敗: %v", err)
	}
	defer orderStateProducer.Close()

	// 建立 RabbitMQ Event Publisher
	eventPublisher := service.NewRabbitMQEventPublisher(orderStateProducer)

	// 建立服務（使用資料庫 repository 和 RabbitMQ Event Publisher）
	orderStateService := service.NewOrderStateServiceWithDeps(
		orderRepo,
		eventPublisher,
		service.NewMockLogger(),
	)

	// 建立事件處理器
	eventHandler := handler.NewOrderStateEventHandler(orderStateService)

	// 建立 RabbitMQ Consumer（傳入配置）
	consumer, err := rabbitmq.NewOrderStateConsumerWithConfig(rabbitMQConn, eventHandler, cfg.RabbitMQPrefetchCount, cfg.RabbitMQWorkerCount)
	if err != nil {
		log.Fatalf("建立 RabbitMQ Consumer 失敗: %v", err)
	}

	// 啟動 Consumer（在 goroutine 中）
	if err := consumer.Start(); err != nil {
		log.Fatalf("啟動 RabbitMQ Consumer 失敗: %v", err)
	}
	log.Println("RabbitMQ Consumer 已啟動")

	// 啟動訂單狀態自動更新調度器（傳入配置）
	orderStateScheduler := scheduler.NewOrderStateSchedulerWithConfig(orderStateService, cfg.SchedulerWorkerCount, cfg.SchedulerBatchSize, cfg.SchedulerInterval)
	go orderStateScheduler.Start()
	defer orderStateScheduler.Stop()
	log.Printf("訂單狀態自動更新調度器已啟動 (Workers=%d, BatchSize=%d, Interval=%v)",
		cfg.SchedulerWorkerCount, cfg.SchedulerBatchSize, cfg.SchedulerInterval)

	// 設置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 創建 Gin 路由器
	r := gin.Default()

	// 健康檢查端點
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "order-state-service",
		})
	})

	// 訂單狀態更新端點（帶結構化日誌和錯誤處理）
	r.PUT("/orders/:orderId/status", func(c *gin.Context) {
		orderId := c.Param("orderId")
		startTime := time.Now()

		var req struct {
			Status string `json:"status" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[API] 請求參數錯誤: orderId=%s, error=%v", orderId, err)
			c.JSON(400, gin.H{"error": err.Error(), "orderId": orderId})
			return
		}

		log.Printf("[API] 收到訂單狀態更新請求: orderId=%s, status=%s", orderId, req.Status)

		// 使用服務更新訂單狀態
		newStatus := domain.OrderStatus(req.Status)
		err := orderStateService.UpdateOrderStatus(orderId, newStatus)
		if err != nil {
			duration := time.Since(startTime)
			log.Printf("[API] 更新訂單狀態失敗: orderId=%s, status=%s, error=%v, duration=%v", 
				orderId, req.Status, err, duration)
			c.JSON(400, gin.H{
				"error":   err.Error(),
				"orderId": orderId,
				"status":  req.Status,
			})
			return
		}

		duration := time.Since(startTime)
		log.Printf("[API] 訂單狀態更新成功: orderId=%s, status=%s, duration=%v", 
			orderId, req.Status, duration)
		c.JSON(200, gin.H{
			"orderId": orderId,
			"status":  req.Status,
			"message": "狀態更新成功",
		})
	})

	// 設置優雅關閉
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 在 goroutine 中啟動 HTTP 服務器
	port := cfg.GetPort()
	go func() {
		log.Printf("HTTP 服務啟動在端口 %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("HTTP 服務啟動失敗: %v", err)
		}
	}()

	// 等待中斷信號
	<-sigChan
	log.Println("收到關閉信號，正在關閉服務...")

	// 停止 Consumer
	if err := consumer.Stop(); err != nil {
		log.Printf("停止 RabbitMQ Consumer 失敗: %v", err)
	}

	log.Println("服務已關閉")
}

