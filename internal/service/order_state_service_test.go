package service

import (
	"ec-order-state-service/internal/domain"
	"ec-order-state-service/internal/repository"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestOrderRepository 測試用的訂單倉儲
type TestOrderRepository struct {
	orders map[string]*domain.Order
}

func NewTestOrderRepository() *TestOrderRepository {
	return &TestOrderRepository{
		orders: make(map[string]*domain.Order),
	}
}

func (r *TestOrderRepository) GetByID(orderID string) (*domain.Order, error) {
	order, exists := r.orders[orderID]
	if !exists {
		return nil, repository.ErrOrderNotFound{OrderID: orderID}
	}
	orderCopy := *order
	return &orderCopy, nil
}

func (r *TestOrderRepository) Save(order *domain.Order) error {
	orderCopy := *order
	r.orders[order.ID] = &orderCopy
	return nil
}

func (r *TestOrderRepository) GetOrdersByStatus(status domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	var orders []*domain.Order
	for _, order := range r.orders {
		if order.Status == status {
			orderCopy := *order
			orders = append(orders, &orderCopy)
		}
	}

	// 實作分頁邏輯
	if limit <= 0 {
		limit = len(orders) // 如果 limit <= 0，返回所有結果
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(orders) {
		return []*domain.Order{}, nil
	}
	end := offset + limit
	if end > len(orders) {
		end = len(orders)
	}

	return orders[offset:end], nil
}

// BatchUpdateOrderStatus 測試用批次更新（簡化版）
func (r *TestOrderRepository) BatchUpdateOrderStatus(orderIDs []string, newStatus domain.OrderStatus, fromStatus domain.OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}
	for _, id := range orderIDs {
		if order, ok := r.orders[id]; ok && order.Status == fromStatus {
			order.Status = newStatus
		}
	}
	return nil
}


func TestOrderStateService_HandlePaymentCompleted(t *testing.T) {
	t.Run("訂單不存在時應該創建新訂單", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-001"
		event := domain.NewPaymentCompletedEvent(orderID)

		// Act
		err := service.HandlePaymentCompleted(event)

		// Assert
		assert.NoError(t, err)
		order, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusWaitingForShipment, order.Status)
	})

	t.Run("訂單已存在且狀態為 Created 時應該更新為 WaitingForShipment", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-002"
		event := domain.NewPaymentCompletedEvent(orderID)

		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusCreated,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)

		// Act
		err := service.HandlePaymentCompleted(event)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusWaitingForShipment, updatedOrder.Status)
		assert.Len(t, mockPublisher.GetPublishedEvents(), 1)
		publishedEvent := mockPublisher.GetPublishedEvents()[0]
		assert.Equal(t, domain.StatusCreated, publishedEvent.FromStatus)
		assert.Equal(t, domain.StatusWaitingForShipment, publishedEvent.ToStatus)
	})

	t.Run("訂單已經是 WaitingForShipment 時應該跳過", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-003"
		event := domain.NewPaymentCompletedEvent(orderID)

		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)
		initialEventCount := len(mockPublisher.GetPublishedEvents())

		// Act
		err := service.HandlePaymentCompleted(event)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusWaitingForShipment, updatedOrder.Status)
		assert.Equal(t, initialEventCount, len(mockPublisher.GetPublishedEvents()), "不應該發布新事件")
	})
}

func TestOrderStateService_UpdateOrderStatus(t *testing.T) {
	t.Run("應該成功更新訂單狀態並發布事件", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-004"
		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)

		// Act
		err := service.UpdateOrderStatus(orderID, domain.StatusInTransit)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusInTransit, updatedOrder.Status)
		assert.Len(t, mockPublisher.GetPublishedEvents(), 1)
		event := mockPublisher.GetPublishedEvents()[0]
		assert.Equal(t, domain.StatusWaitingForShipment, event.FromStatus)
		assert.Equal(t, domain.StatusInTransit, event.ToStatus)
	})

	t.Run("不合法的狀態轉換應該返回錯誤並記錄日誌", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-005"
		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)
		mockLogger.Clear()

		// Act
		err := service.UpdateOrderStatus(orderID, domain.StatusWaitPickup)

		// Assert
		assert.Error(t, err)
		assert.IsType(t, domain.ErrInvalidStatusTransition{}, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusCompleted, updatedOrder.Status) // 狀態不應該改變
		assert.Len(t, mockPublisher.GetPublishedEvents(), 0, "不應該發布事件")
		logs := mockLogger.GetLogs()
		assert.Greater(t, len(logs), 0, "應該記錄錯誤日誌")
	})

	t.Run("訂單不存在時應該返回錯誤", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-006"

		// Act
		err := service.UpdateOrderStatus(orderID, domain.StatusInTransit)

		// Assert
		assert.Error(t, err)
		assert.IsType(t, repository.ErrOrderNotFound{}, err)
	})
}

func TestOrderStateService_CheckStatusTransition(t *testing.T) {
	t.Run("應該正確委託給 domain.CanTransitionTo", func(t *testing.T) {
		// Arrange
		service := NewOrderStateService()

		// Act & Assert
		assert.True(t, service.CheckStatusTransition(domain.StatusCreated, domain.StatusWaitingForShipment))
		assert.True(t, service.CheckStatusTransition(domain.StatusWaitingForShipment, domain.StatusInTransit))
		assert.True(t, service.CheckStatusTransition(domain.StatusInTransit, domain.StatusWaitPickup))
		assert.True(t, service.CheckStatusTransition(domain.StatusWaitPickup, domain.StatusCompleted))

		assert.False(t, service.CheckStatusTransition(domain.StatusCompleted, domain.StatusWaitPickup))
		assert.False(t, service.CheckStatusTransition(domain.StatusCreated, domain.StatusWaitingForPayment))
	})
}

func TestOrderStateService_GetOrder(t *testing.T) {
	t.Run("應該返回訂單", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-007"
		expectedOrder := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(expectedOrder)

		// Act
		order, err := service.GetOrder(orderID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, orderID, order.ID)
		assert.Equal(t, domain.StatusWaitingForShipment, order.Status)
	})

	t.Run("訂單不存在時應該返回錯誤", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-008"

		// Act
		order, err := service.GetOrder(orderID)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, order)
		assert.IsType(t, repository.ErrOrderNotFound{}, err)
	})
}

func TestOrderStateService_GetOrdersByStatus(t *testing.T) {
	t.Run("應該返回指定狀態的訂單列表", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		order1 := &domain.Order{
			ID:     "ORDER-009",
			Status: domain.StatusWaitingForShipment,
		}
		order2 := &domain.Order{
			ID:     "ORDER-010",
			Status: domain.StatusWaitingForShipment,
		}
		order3 := &domain.Order{
			ID:     "ORDER-011",
			Status: domain.StatusInTransit,
		}
		testRepo.Save(order1)
		testRepo.Save(order2)
		testRepo.Save(order3)

		// Act
		orders, err := service.GetOrdersByStatus(domain.StatusWaitingForShipment, 100, 0)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, orders, 2)
		for _, order := range orders {
			assert.Equal(t, domain.StatusWaitingForShipment, order.Status)
		}
	})
}

func TestOrderStateService_HandleShipmentStarted(t *testing.T) {
	t.Run("應該更新訂單狀態為 InTransit 並發布事件", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-011"
		trackingNumber := "TRACK-001"
		event := domain.NewShipmentStartedEvent(orderID, trackingNumber)

		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)

		// Act
		err := service.HandleShipmentStarted(event)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusInTransit, updatedOrder.Status)
		assert.Len(t, mockPublisher.GetPublishedEvents(), 1)
	})
}

func TestOrderStateService_HandlePackageArrivedAtPickupStore(t *testing.T) {
	t.Run("應該更新訂單狀態為 WaitPickup 並發布事件", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-012"
		storeID := "STORE-001"
		event := domain.NewPackageArrivedAtPickupStoreEvent(orderID, storeID)

		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusInTransit,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)

		// Act
		err := service.HandlePackageArrivedAtPickupStore(event)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusWaitPickup, updatedOrder.Status)
		assert.Len(t, mockPublisher.GetPublishedEvents(), 1)
	})
}

func TestOrderStateService_HandlePackagePickedUp(t *testing.T) {
	t.Run("應該更新訂單狀態為 Completed 並發布事件", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-013"
		pickedUpAt := time.Now()
		event := domain.NewPackagePickedUpEvent(orderID, pickedUpAt)

		order := &domain.Order{
			ID:        orderID,
			Status:    domain.StatusWaitPickup,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		testRepo.Save(order)

		// Act
		err := service.HandlePackagePickedUp(event)

		// Assert
		assert.NoError(t, err)
		updatedOrder, err := testRepo.GetByID(orderID)
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusCompleted, updatedOrder.Status)
		assert.Len(t, mockPublisher.GetPublishedEvents(), 1)
	})
}

func TestOrderStateService_HandleOrderCreated(t *testing.T) {
	t.Run("應該記錄日誌但不改變訂單狀態", func(t *testing.T) {
		// Arrange
		testRepo := NewTestOrderRepository()
		mockPublisher := NewMockEventPublisher()
		mockLogger := NewMockLogger()
		service := NewOrderStateServiceWithDeps(testRepo, mockPublisher, mockLogger)

		orderID := "ORDER-014"
		event := domain.NewOrderCreatedEvent(orderID)
		mockLogger.Clear()

		// Act
		err := service.HandleOrderCreated(event)

		// Assert
		assert.NoError(t, err)
		_, err = testRepo.GetByID(orderID)
		assert.Error(t, err, "不應該創建訂單")
		assert.Len(t, mockPublisher.GetPublishedEvents(), 0, "不應該發布事件")
		logs := mockLogger.GetLogs()
		assert.Greater(t, len(logs), 0, "應該記錄日誌")
	})
}

