package scheduler

import (
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/internal/service"
	"log"
	"math/rand"
	"time"
)

// OrderStateScheduler 訂單狀態自動更新調度器
type OrderStateScheduler struct {
	orderStateService *service.OrderStateService
	interval          time.Duration // 檢查間隔
	stopChan          chan struct{}
	rand              *rand.Rand    // 隨機數生成器
	workerCount       int           // Worker Pool 大小
	batchSize         int           // 每次處理的訂單數量
	semaphore         chan struct{} // 信號量控制並發更新數量
}

// NewOrderStateScheduler 創建訂單狀態調度器（使用預設配置）
func NewOrderStateScheduler(orderStateService *service.OrderStateService) *OrderStateScheduler {
	return NewOrderStateSchedulerWithConfig(orderStateService, 5, 100, 10*time.Second)
}

// NewOrderStateSchedulerWithConfig 創建訂單狀態調度器（使用自訂配置）
func NewOrderStateSchedulerWithConfig(orderStateService *service.OrderStateService, workerCount, batchSize int, interval time.Duration) *OrderStateScheduler {
	// 使用當前時間作為隨機數種子
	source := rand.NewSource(time.Now().UnixNano())

	// 創建信號量，限制同時進行的資料庫操作（最多 workerCount 個）
	semaphore := make(chan struct{}, workerCount)

	return &OrderStateScheduler{
		orderStateService: orderStateService,
		interval:          interval,
		stopChan:          make(chan struct{}),
		rand:              rand.New(source),
		workerCount:       workerCount,
		batchSize:         batchSize,
		semaphore:         semaphore,
	}
}

// Start 啟動調度器
func (s *OrderStateScheduler) Start() {
	log.Printf("[定時器] 訂單狀態自動更新調度器已啟動，檢查間隔: %v", s.interval)
	ticker := time.NewTicker(s.interval) // 定時器 每 10 秒觸發一次，NewOrderStateSchedulerWithConfig 創建時有設置interval為10秒
	defer ticker.Stop()

	// 立即執行一次
	log.Printf("[定時器] 開始首次檢查...")
	s.processOrders()

	for {
		select {
		case <-ticker.C:
			s.processOrders()
		case <-s.stopChan: // Stop方法會關閉stopChan，這裡會收到關閉信號，停止定時器
			log.Println("[定時器] 訂單狀態自動更新調度器已停止")
			return
		}
	}
}

// Stop 停止調度器
func (s *OrderStateScheduler) Stop() {
	close(s.stopChan)
}

// processOrders 處理訂單狀態更新
// 每30秒從資料庫查詢已支付以後的訂單（WaitingForShipment 及之後的狀態），並更新到下一個狀態
func (s *OrderStateScheduler) processOrders() {
	log.Printf("[定時器] 開始檢查資料庫中的訂單狀態...")

	// 處理待出貨 -> 運送中（已支付後的第一個狀態）
	s.processStatusTransition(domain.StatusWaitingForShipment, domain.StatusInTransit)

	// 處理運送中 -> 待取貨
	s.processStatusTransition(domain.StatusInTransit, domain.StatusWaitPickup)

	// 處理待取貨 -> 已完成
	s.processStatusTransition(domain.StatusWaitPickup, domain.StatusCompleted)

	log.Printf("[定時器] 訂單狀態檢查完成")
}

// processStatusTransition 處理狀態轉換（使用分頁和批量更新）
// 從資料庫查詢指定狀態的訂單，使用隨機方式決定是否更新到下一個狀態
func (s *OrderStateScheduler) processStatusTransition(fromStatus, toStatus domain.OrderStatus) {
	offset := 0
	totalUpdated := 0
	totalErrors := 0

	// 使用分頁查詢，每次處理 batchSize 筆訂單
	for {
		// 從資料庫獲取該狀態的訂單（分頁查詢）
		orders, err := s.orderStateService.GetOrdersByStatus(fromStatus, s.batchSize, offset)
		if err != nil {
			log.Printf("[定時器] 從資料庫獲取 %s 狀態的訂單失敗: %v", fromStatus, err)
			return
		}

		if len(orders) == 0 {
			// 沒有更多訂單了
			break
		}

		log.Printf("[定時器] 從資料庫查詢到 %d 個 %s 狀態的訂單 (offset=%d)", len(orders), fromStatus, offset)

		// 使用批量更新處理訂單
		updatedCount, errorCount := s.processBatch(orders, fromStatus, toStatus)
		totalUpdated += updatedCount
		totalErrors += errorCount

		// 如果查詢到的訂單數量少於 batchSize，說明已經處理完所有訂單
		if len(orders) < s.batchSize {
			break
		}

		// 增加 offset 準備下一批
		offset += s.batchSize

		// 批次處理間隔，避免資料庫壓力過大
		time.Sleep(100 * time.Millisecond)
	}

	if totalUpdated > 0 {
		log.Printf("[定時器] 本次檢查共更新了 %d 個訂單的狀態 (%s -> %s)", totalUpdated, fromStatus, toStatus)
	}
	if totalErrors > 0 {
		log.Printf("[定時器] 本次檢查有 %d 個訂單更新失敗 (%s -> %s)", totalErrors, fromStatus, toStatus)
	}
}

// processBatch 處理一批訂單（使用批量更新）
func (s *OrderStateScheduler) processBatch(orders []*domain.Order, fromStatus, toStatus domain.OrderStatus) (updatedCount, errorCount int) {
	// 收集需要更新的訂單 ID（50% 機率）
	orderIDsToUpdate := make([]string, 0, len(orders)/2)

	for _, order := range orders {
		// 每筆訂單每次都用隨機的方式決定是否更新
		// 生成 0-99 的隨機數，如果小於 50（50% 機率）則更新
		randomValue := s.rand.Intn(100)

		if randomValue < 50 {
			orderIDsToUpdate = append(orderIDsToUpdate, order.ID)
		}
	}

	if len(orderIDsToUpdate) == 0 {
		log.Printf("[定時器] 本批次沒有訂單需要更新 (%s -> %s)", fromStatus, toStatus)
		return 0, 0
	}

	log.Printf("[定時器] 準備批量更新 %d 個訂單: %s -> %s", len(orderIDsToUpdate), fromStatus, toStatus)

	// 批量更新訂單狀態
	if err := s.orderStateService.BatchUpdateOrderStatus(orderIDsToUpdate, fromStatus, toStatus); err != nil {
		log.Printf("[定時器] 批量更新訂單狀態失敗: %v", err)
		return 0, len(orderIDsToUpdate)
	}

	log.Printf("[定時器] 成功批量更新 %d 個訂單: %s -> %s", len(orderIDsToUpdate), fromStatus, toStatus)
	return len(orderIDsToUpdate), 0
}
