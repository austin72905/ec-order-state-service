package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// directQueries 直接 SQL 查詢實作（臨時方案，待 sqlc generate 後替換）
type directQueries struct {
	pool *pgxpool.Pool
	tx   pgx.Tx // 事務（如果有的話）
}

// getQueryExecutor 獲取查詢執行器（優先使用事務，否則使用連接池）
func (q *directQueries) getQueryExecutor(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
} {
	if q.tx != nil {
		return q.tx
	}
	return q.pool
}

// GetOrderByID 根據 ID 獲取訂單
func (q *directQueries) GetOrderByID(ctx context.Context, id string) (Order, error) {
	var order Order
	executor := q.getQueryExecutor(ctx)
	err := executor.QueryRow(ctx, `
		SELECT id, status, created_at, updated_at
		FROM orders
		WHERE id = $1
	`, id).Scan(&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// GetOrderByIDForUpdate 根據 ID 獲取訂單（使用悲觀鎖 SELECT FOR UPDATE）
func (q *directQueries) GetOrderByIDForUpdate(ctx context.Context, id string) (Order, error) {
	var order Order
	executor := q.getQueryExecutor(ctx)
	// 使用 SELECT FOR UPDATE NOWAIT 避免長時間等待鎖
	err := executor.QueryRow(ctx, `
		SELECT id, status, created_at, updated_at
		FROM orders
		WHERE id = $1
		FOR UPDATE NOWAIT
	`, id).Scan(&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// CreateOrder 創建訂單
func (q *directQueries) CreateOrder(ctx context.Context, arg CreateOrderParams) (Order, error) {
	var order Order
	executor := q.getQueryExecutor(ctx)
	err := executor.QueryRow(ctx, `
		INSERT INTO orders (id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at, updated_at
	`, arg.ID, arg.Status, arg.CreatedAt, arg.UpdatedAt).Scan(
		&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return Order{}, fmt.Errorf("創建訂單失敗: %w", err)
	}
	return order, nil
}

// UpdateOrderStatus 更新訂單狀態
// 注意：updated_at 由資料庫 trigger 自動更新，無需手動設置
func (q *directQueries) UpdateOrderStatus(ctx context.Context, arg UpdateOrderStatusParams) (Order, error) {
	var order Order
	executor := q.getQueryExecutor(ctx)
	err := executor.QueryRow(ctx, `
		UPDATE orders
		SET status = $2
		WHERE id = $1
		RETURNING id, status, created_at, updated_at
	`, arg.ID, arg.Status).Scan(
		&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Order{}, fmt.Errorf("訂單不存在: %s", arg.ID)
		}
		return Order{}, fmt.Errorf("更新訂單失敗: %w", err)
	}
	return order, nil
}

// GetOrdersByStatus 根據狀態獲取訂單列表
func (q *directQueries) GetOrdersByStatus(ctx context.Context, status string) ([]Order, error) {
	executor := q.getQueryExecutor(ctx)
	rows, err := executor.Query(ctx, `
		SELECT id, status, created_at, updated_at
		FROM orders
		WHERE status = $1
		ORDER BY updated_at ASC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單失敗: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("掃描訂單失敗: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("讀取訂單失敗: %w", err)
	}

	return orders, nil
}

// GetOrdersByStatusWithPagination 根據狀態獲取訂單列表（支持分頁）
func (q *directQueries) GetOrdersByStatusWithPagination(ctx context.Context, status string, limit, offset int) ([]Order, error) {
	executor := q.getQueryExecutor(ctx)
	rows, err := executor.Query(ctx, `
		SELECT id, status, created_at, updated_at
		FROM orders
		WHERE status = $1
		ORDER BY updated_at ASC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單失敗: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("掃描訂單失敗: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("讀取訂單失敗: %w", err)
	}

	return orders, nil
}

// BatchUpdateOrderStatus 批量更新訂單狀態
func (q *directQueries) BatchUpdateOrderStatus(ctx context.Context, orderIDs []string, newStatus, fromStatus string) error {
	if len(orderIDs) == 0 {
		return nil
	}

	executor := q.getQueryExecutor(ctx)
	
	// 使用 PostgreSQL 的 ANY 操作符進行批量更新
	// 只更新狀態為 fromStatus 的訂單，確保冪等性
	_, err := executor.Exec(ctx, `
		UPDATE orders
		SET status = $1
		WHERE id = ANY($2) AND status = $3
	`, newStatus, orderIDs, fromStatus)
	
	if err != nil {
		return fmt.Errorf("批量更新訂單狀態失敗: %w", err)
	}
	
	return nil
}

