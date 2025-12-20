package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrder_UpdateStatus(t *testing.T) {
	t.Run("應該成功更新狀態", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-001",
			Status:    StatusWaitingForShipment,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := order.UpdateStatus(StatusInTransit)

		assert.NoError(t, err)
		assert.Equal(t, StatusInTransit, order.Status)
		assert.False(t, order.UpdatedAt.IsZero())
	})

	t.Run("應該拒絕不合法的狀態轉換", func(t *testing.T) {
		order := &Order{
			ID:        "ORDER-002",
			Status:    StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		originalStatus := order.Status
		originalUpdatedAt := order.UpdatedAt

		err := order.UpdateStatus(StatusWaitPickup)

		assert.Error(t, err)
		assert.IsType(t, ErrInvalidStatusTransition{}, err)
		assert.Equal(t, originalStatus, order.Status, "狀態不應該改變")
		assert.Equal(t, originalUpdatedAt, order.UpdatedAt, "UpdatedAt 不應該改變")
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
		}

		// Created -> WaitingForShipment
		err := order.UpdateStatus(StatusWaitingForShipment)
		assert.NoError(t, err)
		assert.Equal(t, StatusWaitingForShipment, order.Status)

		// WaitingForShipment -> InTransit
		err = order.UpdateStatus(StatusInTransit)
		assert.NoError(t, err)
		assert.Equal(t, StatusInTransit, order.Status)

		// InTransit -> WaitPickup
		err = order.UpdateStatus(StatusWaitPickup)
		assert.NoError(t, err)
		assert.Equal(t, StatusWaitPickup, order.Status)

		// WaitPickup -> Completed
		err = order.UpdateStatus(StatusCompleted)
		assert.NoError(t, err)
		assert.Equal(t, StatusCompleted, order.Status)
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

