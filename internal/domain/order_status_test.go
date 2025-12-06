package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     OrderStatus
		to       OrderStatus
		expected bool
	}{
		// 合法的轉換
		{
			name:     "Created 可以轉換為 WaitingForShipment",
			from:     StatusCreated,
			to:       StatusWaitingForShipment,
			expected: true,
		},
		{
			name:     "Created 可以轉換為 Canceled",
			from:     StatusCreated,
			to:       StatusCanceled,
			expected: true,
		},
		{
			name:     "WaitingForShipment 可以轉換為 InTransit",
			from:     StatusWaitingForShipment,
			to:       StatusInTransit,
			expected: true,
		},
		{
			name:     "InTransit 可以轉換為 WaitPickup",
			from:     StatusInTransit,
			to:       StatusWaitPickup,
			expected: true,
		},
		{
			name:     "WaitPickup 可以轉換為 Completed",
			from:     StatusWaitPickup,
			to:       StatusCompleted,
			expected: true,
		},
		// 不合法的轉換
		{
			name:     "Created 不能轉換為 WaitingForPayment",
			from:     StatusCreated,
			to:       StatusWaitingForPayment,
			expected: false,
		},
		{
			name:     "Completed 不能轉換為 WaitPickup",
			from:     StatusCompleted,
			to:       StatusWaitPickup,
			expected: false,
		},
		{
			name:     "Completed 不能轉換為 WaitingForShipment",
			from:     StatusCompleted,
			to:       StatusWaitingForShipment,
			expected: false,
		},
		{
			name:     "InTransit 不能直接轉換為 Completed",
			from:     StatusInTransit,
			to:       StatusCompleted,
			expected: false,
		},
		{
			name:     "WaitingForShipment 不能直接轉換為 Completed",
			from:     StatusWaitingForShipment,
			to:       StatusCompleted,
			expected: false,
		},
		{
			name:     "WaitingForShipment 不能轉換為 WaitPickup",
			from:     StatusWaitingForShipment,
			to:       StatusWaitPickup,
			expected: false,
		},
		{
			name:     "Canceled 不能轉換到任何狀態",
			from:     StatusCanceled,
			to:       StatusCompleted,
			expected: false,
		},
		{
			name:     "Refund 不能轉換到任何狀態",
			from:     StatusRefund,
			to:       StatusCompleted,
			expected: false,
		},
		// 相同狀態（不應該允許）
		{
			name:     "相同狀態不應該允許轉換",
			from:     StatusCompleted,
			to:       StatusCompleted,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransitionTo(tt.from, tt.to)
			assert.Equal(t, tt.expected, result, "狀態轉換 %s -> %s 應該為 %v", tt.from, tt.to, tt.expected)
		})
	}
}

func TestOrderStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   OrderStatus
		expected string
	}{
		{
			name:     "Created 狀態字串",
			status:   StatusCreated,
			expected: "Created",
		},
		{
			name:     "WaitingForShipment 狀態字串",
			status:   StatusWaitingForShipment,
			expected: "WaitingForShipment",
		},
		{
			name:     "InTransit 狀態字串",
			status:   StatusInTransit,
			expected: "InTransit",
		},
		{
			name:     "WaitPickup 狀態字串",
			status:   StatusWaitPickup,
			expected: "WaitPickup",
		},
		{
			name:     "Completed 狀態字串",
			status:   StatusCompleted,
			expected: "Completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

