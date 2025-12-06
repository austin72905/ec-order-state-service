package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrder_UpdateStatus(t *testing.T) {
	t.Run("應該成功更新狀態並添加 OrderStep", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-001",
			Status:    StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		err := order.UpdateStatus(StatusInTransit)

		assert.NoError(t, err)
		assert.Equal(t, StatusInTransit, order.Status)
		assert.Len(t, order.OrderSteps, 1)
		assert.Equal(t, StatusWaitingForShipment, order.OrderSteps[0].FromStatus)
		assert.Equal(t, StatusInTransit, order.OrderSteps[0].ToStatus)
		assert.Equal(t, "ORDER-001", order.OrderSteps[0].OrderID)
		assert.False(t, order.UpdatedAt.IsZero())
	})

	t.Run("應該拒絕不合法的狀態轉換", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-002",
			Status:    StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		originalStatus := order.Status
		originalUpdatedAt := order.UpdatedAt

		err := order.UpdateStatus(StatusWaitPickup)

		assert.Error(t, err)
		assert.IsType(t, ErrInvalidStatusTransition{}, err)
		assert.Equal(t, originalStatus, order.Status, "狀態不應該改變")
		assert.Equal(t, originalUpdatedAt, order.UpdatedAt, "UpdatedAt 不應該改變")
		assert.Len(t, order.OrderSteps, 0, "不應該添加 OrderStep")
	})

	t.Run("應該正確處理錯誤訊息", func(t *testing.T) {
		order := &Order{
			ID:     "ORDER-003",
			Status: StatusCompleted,
		}

		err := order.UpdateStatus(StatusWaitPickup)

		assert.Error(t, err)
		errInvalid, ok := err.(ErrInvalidStatusTransition)
		assert.True(t, ok)
		assert.Equal(t, StatusCompleted, errInvalid.FromStatus)
		assert.Equal(t, StatusWaitPickup, errInvalid.ToStatus)
		assert.Contains(t, err.Error(), "invalid status transition")
		assert.Contains(t, err.Error(), "Completed")
		assert.Contains(t, err.Error(), "WaitPickup")
	})

	t.Run("應該支援完整的狀態轉換流程", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-004",
			Status:    StatusCreated,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		// Created -> WaitingForShipment
		err := order.UpdateStatus(StatusWaitingForShipment)
		assert.NoError(t, err)
		assert.Equal(t, StatusWaitingForShipment, order.Status)
		assert.Len(t, order.OrderSteps, 1)

		// WaitingForShipment -> InTransit
		err = order.UpdateStatus(StatusInTransit)
		assert.NoError(t, err)
		assert.Equal(t, StatusInTransit, order.Status)
		assert.Len(t, order.OrderSteps, 2)

		// InTransit -> WaitPickup
		err = order.UpdateStatus(StatusWaitPickup)
		assert.NoError(t, err)
		assert.Equal(t, StatusWaitPickup, order.Status)
		assert.Len(t, order.OrderSteps, 3)

		// WaitPickup -> Completed
		err = order.UpdateStatus(StatusCompleted)
		assert.NoError(t, err)
		assert.Equal(t, StatusCompleted, order.Status)
		assert.Len(t, order.OrderSteps, 4)

		// 驗證所有步驟
		assert.Equal(t, StatusCreated, order.OrderSteps[0].FromStatus)
		assert.Equal(t, StatusWaitingForShipment, order.OrderSteps[0].ToStatus)

		assert.Equal(t, StatusWaitingForShipment, order.OrderSteps[1].FromStatus)
		assert.Equal(t, StatusInTransit, order.OrderSteps[1].ToStatus)

		assert.Equal(t, StatusInTransit, order.OrderSteps[2].FromStatus)
		assert.Equal(t, StatusWaitPickup, order.OrderSteps[2].ToStatus)

		assert.Equal(t, StatusWaitPickup, order.OrderSteps[3].FromStatus)
		assert.Equal(t, StatusCompleted, order.OrderSteps[3].ToStatus)
	})
}

func TestOrder_AddOrderStep(t *testing.T) {
	t.Run("應該添加 OrderStep 記錄", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-005",
			Status:    StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		order.AddOrderStep(StatusWaitingForShipment, StatusInTransit)

		assert.Len(t, order.OrderSteps, 1)
		step := order.OrderSteps[0]
		assert.Equal(t, "ORDER-005", step.OrderID)
		assert.Equal(t, StatusWaitingForShipment, step.FromStatus)
		assert.Equal(t, StatusInTransit, step.ToStatus)
		assert.False(t, step.CreatedAt.IsZero())
	})

	t.Run("應該可以添加多個 OrderStep", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-006",
			Status:    StatusCreated,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		order.AddOrderStep(StatusCreated, StatusWaitingForShipment)
		order.AddOrderStep(StatusWaitingForShipment, StatusInTransit)
		order.AddOrderStep(StatusInTransit, StatusWaitPickup)

		assert.Len(t, order.OrderSteps, 3)
	})
}

func TestOrder_GetOrderSteps(t *testing.T) {
	t.Run("應該返回所有 OrderStep", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-007",
			Status:    StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{
				{
					OrderID:    "ORDER-007",
					FromStatus: StatusCreated,
					ToStatus:   StatusWaitingForShipment,
					CreatedAt:  time.Now(),
				},
				{
					OrderID:    "ORDER-007",
					FromStatus: StatusWaitingForShipment,
					ToStatus:   StatusInTransit,
					CreatedAt:  time.Now(),
				},
			},
		}

		steps := order.GetOrderSteps()

		assert.Len(t, steps, 2)
		assert.Equal(t, StatusCreated, steps[0].FromStatus)
		assert.Equal(t, StatusWaitingForShipment, steps[0].ToStatus)
		assert.Equal(t, StatusWaitingForShipment, steps[1].FromStatus)
		assert.Equal(t, StatusInTransit, steps[1].ToStatus)
	})

	t.Run("沒有 OrderStep 時應該返回空切片", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-008",
			Status:    StatusCreated,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			OrderSteps: []OrderStep{},
		}

		steps := order.GetOrderSteps()

		assert.NotNil(t, steps)
		assert.Len(t, steps, 0)
	})
}

func TestErrInvalidStatusTransition(t *testing.T) {
	t.Run("應該包含正確的錯誤訊息", func(t *testing.T) {
		err := ErrInvalidStatusTransition{
			FromStatus: StatusCompleted,
			ToStatus:   StatusWaitPickup,
		}

		errorMsg := err.Error()

		assert.Contains(t, errorMsg, "invalid status transition")
		assert.Contains(t, errorMsg, "Completed")
		assert.Contains(t, errorMsg, "WaitPickup")
	})
}

