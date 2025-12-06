package service

import (
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/internal/repository"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OrderStateService 訂單狀態服務
type OrderStateService struct {
	orderRepo repository.OrderRepository
	eventPublisher EventPublisher
	logger         Logger
}

// EventPublisher 事件發布介面
type EventPublisher interface {
	PublishOrderStatusChanged(event domain.OrderStatusChangedEvent) error
}

// Logger 日誌介面
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, err error, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// NewOrderStateService 創建訂單狀態服務
func NewOrderStateService() *OrderStateService {
	return &OrderStateService{
		orderRepo:     repository.NewInMemoryOrderRepository(),
		eventPublisher: NewMockEventPublisher(),
		logger:         NewMockLogger(),
	}
}

// NewOrderStateServiceWithDeps 使用依賴注入創建服務
func NewOrderStateServiceWithDeps(
	orderRepo repository.OrderRepository,
	eventPublisher EventPublisher,
	logger Logger,
) *OrderStateService {
	return &OrderStateService{
		orderRepo:     orderRepo,
		eventPublisher: eventPublisher,
		logger:         logger,
	}
}

// HandleOrderCreated 處理訂單創建事件
// 注意：此服務不會修改訂單狀態為 WaitingForPayment
// 訂單創建時狀態保持為 Created，等待支付服務完成後調用此服務
func (s *OrderStateService) HandleOrderCreated(event domain.OrderCreatedEvent) error {
	// 訂單創建事件不需要處理狀態轉換
	// 訂單狀態保持為 Created，等待支付完成
	s.logger.Info("Order created event received, order status remains Created", "orderId", event.OrderID())
	return nil
}

// HandlePaymentCompleted 處理付款完成事件（帶冪等性保證）
// 支付服務完成後調用此服務，將訂單從 Created 轉換為 WaitingForShipment
// 注意：後端已經將訂單狀態更新為 WaitingForShipment，此服務負責同步狀態
// 使用事務和悲觀鎖確保冪等性，即使同一事件被處理多次也不會造成問題
func (s *OrderStateService) HandlePaymentCompleted(event domain.PaymentCompletedEvent) error {
	// 記錄處理開始時間（用於監控）
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		s.logger.Info("HandlePaymentCompleted 處理完成", 
			"orderId", event.OrderID(), 
			"duration", duration.String())
	}()

	order, err := s.orderRepo.GetByID(event.OrderID())
	
	// 如果訂單不存在，創建它（狀態為 WaitingForShipment，因為後端已經更新了）
	if err != nil {
		// 檢查是否是訂單不存在的錯誤
		var notFoundErr repository.ErrOrderNotFound
		if !errors.As(err, &notFoundErr) {
			s.logger.Error("查詢訂單失敗", err, "orderId", event.OrderID())
			return fmt.Errorf("查詢訂單失敗: %w", err)
		}
		
		// 訂單不存在，創建新訂單（狀態為 WaitingForShipment）
		// 注意：Save 方法使用事務和悲觀鎖，確保即使並發創建也不會重複
		s.logger.Info("訂單不存在，創建新訂單", "orderId", event.OrderID())
		now := time.Now()
		newOrder := &domain.Order{
			ID:        event.OrderID(),
			Status:    domain.StatusWaitingForShipment,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// 添加狀態轉換步驟
		newOrder.AddOrderStep(domain.StatusCreated, domain.StatusWaitingForShipment)
		
		if err := s.orderRepo.Save(newOrder); err != nil {
			// 檢查是否為重複創建錯誤（可能由並發導致）
			if isDuplicateKeyError(err) {
				s.logger.Info("訂單已存在（可能由並發創建），重新查詢", "orderId", event.OrderID())
				// 重新查詢訂單
				order, err = s.orderRepo.GetByID(event.OrderID())
				if err != nil {
					s.logger.Error("重新查詢訂單失敗", err, "orderId", event.OrderID())
					return fmt.Errorf("重新查詢訂單失敗: %w", err)
				}
				// 繼續處理已存在的訂單
			} else {
				s.logger.Error("創建訂單失敗", err, "orderId", event.OrderID())
				return fmt.Errorf("創建訂單失敗: %w", err)
			}
		} else {
			s.logger.Info("訂單創建成功", "orderId", event.OrderID(), "status", newOrder.Status)
			return nil
		}
	}

	// 訂單已存在，檢查狀態（冪等性檢查）
	// 由於 Save 方法使用悲觀鎖，這裡的檢查是安全的
	if order.Status == domain.StatusWaitingForShipment {
		// 已經是目標狀態，不需要更新（冪等性保證）
		s.logger.Info("訂單已經是 WaitingForShipment 狀態，跳過更新（冪等性）", "orderId", event.OrderID())
		return nil
	}

	// 如果訂單狀態是 Created，更新為 WaitingForShipment
	if order.Status == domain.StatusCreated {
		fromStatus := order.Status
		if err := order.UpdateStatus(domain.StatusWaitingForShipment); err != nil {
			s.logger.Error("invalid status transition", err, "orderId", event.OrderID(), 
				"fromStatus", fromStatus, "toStatus", domain.StatusWaitingForShipment)
			return err
		}

		// Save 方法使用事務和悲觀鎖，確保原子性更新
		// 即使多個相同事件並發處理，也只會有一個成功更新
		if err := s.orderRepo.Save(order); err != nil {
			// 檢查是否為並發更新衝突
			if isDeadlockOrConflictError(err) {
				s.logger.Warn("並發更新衝突，重新查詢訂單狀態", "orderId", event.OrderID())
				// 重新查詢訂單狀態
				order, err = s.orderRepo.GetByID(event.OrderID())
				if err != nil {
					s.logger.Error("重新查詢訂單失敗", err, "orderId", event.OrderID())
					return fmt.Errorf("重新查詢訂單失敗: %w", err)
				}
				// 如果狀態已經是目標狀態，說明其他請求已成功處理（冪等性）
				if order.Status == domain.StatusWaitingForShipment {
					s.logger.Info("訂單狀態已由其他請求更新，跳過（冪等性）", "orderId", event.OrderID())
					return nil
				}
				// 否則返回錯誤
				return fmt.Errorf("並發更新衝突後狀態仍不正確: %w", err)
			}
			s.logger.Error("保存訂單失敗", err, "orderId", event.OrderID())
			return err
		}

		statusChangedEvent := domain.NewOrderStatusChangedEvent(order.ID, fromStatus, order.Status)
		if err := s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent); err != nil {
			// 事件發布失敗不影響主流程，只記錄日誌
			s.logger.Error("發布狀態變更事件失敗", err, "orderId", event.OrderID())
		}
		
		s.logger.Info("訂單狀態更新成功", "orderId", event.OrderID(), 
			"fromStatus", fromStatus, "toStatus", order.Status)
		return nil
	}

	// 其他狀態，記錄警告但不報錯（冪等性：已處理過的事件不會報錯）
	s.logger.Warn("訂單狀態不是 Created 或 WaitingForShipment，跳過處理", 
		"orderId", event.OrderID(), "currentStatus", order.Status)
	return nil
}

// isDuplicateKeyError 檢查是否為重複鍵錯誤
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// PostgreSQL 重複鍵錯誤碼為 23505
	return strings.Contains(errStr, "duplicate key") || 
		strings.Contains(errStr, "23505") || 
		strings.Contains(errStr, "unique constraint")
}

// isDeadlockOrConflictError 檢查是否為死鎖或衝突錯誤
func isDeadlockOrConflictError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "deadlock") || 
		strings.Contains(errStr, "40001") ||
		strings.Contains(errStr, "could not serialize")
}

// HandleShipmentStarted 處理物流開始事件
func (s *OrderStateService) HandleShipmentStarted(event domain.ShipmentStartedEvent) error {
	order, err := s.orderRepo.GetByID(event.OrderID())
	if err != nil {
		return err
	}

	fromStatus := order.Status
	if err := order.UpdateStatus(domain.StatusInTransit); err != nil {
		s.logger.Error("invalid status transition", err, "orderId", event.OrderID())
		return err
	}

	if err := s.orderRepo.Save(order); err != nil {
		return err
	}

	statusChangedEvent := domain.NewOrderStatusChangedEvent(order.ID, fromStatus, order.Status)
	s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent)

	return nil
}

// HandlePackageArrivedAtPickupStore 處理包裹到達取貨門市事件
func (s *OrderStateService) HandlePackageArrivedAtPickupStore(event domain.PackageArrivedAtPickupStoreEvent) error {
	order, err := s.orderRepo.GetByID(event.OrderID())
	if err != nil {
		return err
	}

	fromStatus := order.Status
	if err := order.UpdateStatus(domain.StatusWaitPickup); err != nil {
		s.logger.Error("invalid status transition", err, "orderId", event.OrderID())
		return err
	}

	if err := s.orderRepo.Save(order); err != nil {
		return err
	}

	statusChangedEvent := domain.NewOrderStatusChangedEvent(order.ID, fromStatus, order.Status)
	s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent)

	return nil
}

// HandlePackagePickedUp 處理包裹已取貨事件
func (s *OrderStateService) HandlePackagePickedUp(event domain.PackagePickedUpEvent) error {
	order, err := s.orderRepo.GetByID(event.OrderID())
	if err != nil {
		return err
	}

	fromStatus := order.Status
	if err := order.UpdateStatus(domain.StatusCompleted); err != nil {
		s.logger.Error("invalid status transition", err, "orderId", event.OrderID())
		return err
	}

	if err := s.orderRepo.Save(order); err != nil {
		return err
	}

	statusChangedEvent := domain.NewOrderStatusChangedEvent(order.ID, fromStatus, order.Status)
	s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent)

	return nil
}

// GetOrder 獲取訂單
func (s *OrderStateService) GetOrder(orderID string) (*domain.Order, error) {
	return s.orderRepo.GetByID(orderID)
}

// GetOrderSteps 獲取訂單步驟
func (s *OrderStateService) GetOrderSteps(orderID string) ([]domain.OrderStep, error) {
	return s.orderRepo.GetOrderSteps(orderID)
}

// CheckStatusTransition 檢查狀態轉換是否允許
func (s *OrderStateService) CheckStatusTransition(fromStatus, toStatus domain.OrderStatus) bool {
	return domain.CanTransitionTo(fromStatus, toStatus)
}

// UpdateOrderStatus 直接更新訂單狀態（用於測試和自動更新）
func (s *OrderStateService) UpdateOrderStatus(orderID string, newStatus domain.OrderStatus) error {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}

	fromStatus := order.Status
	if err := order.UpdateStatus(newStatus); err != nil {
		// 記錄錯誤日誌
		s.logger.Error("invalid status transition", err, "orderId", orderID, "fromStatus", fromStatus, "toStatus", newStatus)
		return err
	}

	if err := s.orderRepo.Save(order); err != nil {
		return err
	}

	// 發布狀態變更事件
	statusChangedEvent := domain.NewOrderStatusChangedEvent(order.ID, fromStatus, order.Status)
	s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent)

	s.logger.Info("訂單狀態已更新", "orderId", orderID, "fromStatus", fromStatus, "toStatus", newStatus)
	return nil
}

// GetOrdersByStatus 根據狀態獲取訂單列表（支持分頁）
func (s *OrderStateService) GetOrdersByStatus(status domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	return s.orderRepo.GetOrdersByStatus(status, limit, offset)
}

// BatchUpdateOrderStatus 批量更新訂單狀態（用於調度器）
func (s *OrderStateService) BatchUpdateOrderStatus(orderIDs []string, fromStatus, toStatus domain.OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}

	// 批量更新訂單狀態
	if err := s.orderRepo.BatchUpdateOrderStatus(orderIDs, toStatus, fromStatus); err != nil {
		s.logger.Error("批量更新訂單狀態失敗", err, "orderCount", len(orderIDs), "fromStatus", fromStatus, "toStatus", toStatus)
		return err
	}

	// 批量添加訂單步驟
	now := time.Now()
	steps := make([]*domain.OrderStep, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		steps = append(steps, &domain.OrderStep{
			OrderID:    orderID,
			FromStatus: fromStatus,
			ToStatus:   toStatus,
			CreatedAt:  now,
		})
	}

	if err := s.orderRepo.BatchAddOrderSteps(steps); err != nil {
		s.logger.Error("批量添加訂單步驟失敗", err, "stepCount", len(steps))
		// 步驟添加失敗不影響主流程，只記錄日誌
	}

	// 批量發布狀態變更事件（異步處理，不阻塞主流程）
	for _, orderID := range orderIDs {
		statusChangedEvent := domain.NewOrderStatusChangedEvent(orderID, fromStatus, toStatus)
		if err := s.eventPublisher.PublishOrderStatusChanged(statusChangedEvent); err != nil {
			// 事件發布失敗不影響主流程，只記錄日誌
			s.logger.Error("發布狀態變更事件失敗", err, "orderId", orderID)
		}
	}

	s.logger.Info("批量更新訂單狀態成功", "orderCount", len(orderIDs), "fromStatus", fromStatus, "toStatus", toStatus)
	return nil
}

// MockEventPublisher 模擬事件發布器（用於測試）
type MockEventPublisher struct {
	Events []domain.OrderStatusChangedEvent
}

func (m *MockEventPublisher) PublishOrderStatusChanged(event domain.OrderStatusChangedEvent) error {
	m.Events = append(m.Events, event)
	return nil
}

func (m *MockEventPublisher) GetPublishedEvents() []domain.OrderStatusChangedEvent {
	return m.Events
}

func (m *MockEventPublisher) Clear() {
	m.Events = make([]domain.OrderStatusChangedEvent, 0)
}

// NewMockEventPublisher 創建模擬事件發布器
func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{
		Events: make([]domain.OrderStatusChangedEvent, 0),
	}
}

// LogEntry 日誌條目
type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

// MockLogger 模擬日誌器（用於測試）
type MockLogger struct {
	Logs []LogEntry
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.Logs = append(m.Logs, LogEntry{
		Level:   "INFO",
		Message: msg,
		Fields:  parseFields(fields...),
	})
}

func (m *MockLogger) Error(msg string, err error, fields ...interface{}) {
	fieldsMap := parseFields(fields...)
	if err != nil {
		fieldsMap["error"] = err.Error()
	}
	m.Logs = append(m.Logs, LogEntry{
		Level:   "ERROR",
		Message: msg,
		Fields:  fieldsMap,
	})
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.Logs = append(m.Logs, LogEntry{
		Level:   "WARN",
		Message: msg,
		Fields:  parseFields(fields...),
	})
}

func (m *MockLogger) GetLogs() []LogEntry {
	return m.Logs
}

func (m *MockLogger) Clear() {
	m.Logs = make([]LogEntry, 0)
}

// NewMockLogger 創建模擬日誌器
func NewMockLogger() *MockLogger {
	return &MockLogger{
		Logs: make([]LogEntry, 0),
	}
}

func parseFields(fields ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			result[key] = fields[i+1]
		}
	}
	return result
}

