package repository

import (
	"context"
	"database/sql"
	"ec-order-state-service/internal/domain"
	"fmt"
	"time"

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

// GetOrderSteps 獲取訂單步驟列表
func (q *directQueries) GetOrderSteps(ctx context.Context, orderID string) ([]OrderStep, error) {
	executor := q.getQueryExecutor(ctx)
	rows, err := executor.Query(ctx, `
		SELECT id, order_id, from_status, to_status, created_at
		FROM order_steps
		WHERE order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單步驟失敗: %w", err)
	}
	defer rows.Close()

	var steps []OrderStep
	for rows.Next() {
		var step OrderStep
		if err := rows.Scan(&step.ID, &step.OrderID, &step.FromStatus, &step.ToStatus, &step.CreatedAt); err != nil {
			return nil, fmt.Errorf("掃描訂單步驟失敗: %w", err)
		}
		steps = append(steps, step)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("讀取訂單步驟失敗: %w", err)
	}

	return steps, nil
}

// AddOrderStep 添加訂單步驟
func (q *directQueries) AddOrderStep(ctx context.Context, arg AddOrderStepParams) (OrderStep, error) {
	var step OrderStep
	executor := q.getQueryExecutor(ctx)
	err := executor.QueryRow(ctx, `
		INSERT INTO order_steps (order_id, from_status, to_status, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, order_id, from_status, to_status, created_at
	`, arg.OrderID, arg.FromStatus, arg.ToStatus, arg.CreatedAt).Scan(
		&step.ID, &step.OrderID, &step.FromStatus, &step.ToStatus, &step.CreatedAt,
	)
	if err != nil {
		return OrderStep{}, fmt.Errorf("添加訂單步驟失敗: %w", err)
	}
	return step, nil
}

// GetOrdersByStatus 根據狀態獲取訂單列表（不包含步驟，用於向後兼容）
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

// GetOrdersByStatusWithSteps 根據狀態獲取訂單列表（包含步驟，避免 N+1 查詢）
// 支持分頁查詢
func (q *directQueries) GetOrdersByStatusWithSteps(ctx context.Context, status string, limit, offset int) ([]*domain.Order, error) {
	// 使用 LEFT JOIN 一次性獲取訂單和步驟
	executor := q.getQueryExecutor(ctx)
	rows, err := executor.Query(ctx, `
		SELECT 
			o.id, o.status, o.created_at, o.updated_at,
			os.id as step_id, os.order_id, os.from_status, os.to_status, os.created_at as step_created_at
		FROM orders o
		LEFT JOIN order_steps os ON o.id = os.order_id
		WHERE o.status = $1
		ORDER BY o.updated_at ASC, os.created_at ASC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單失敗: %w", err)
	}
	defer rows.Close()

	// 使用 map 來組織訂單和步驟
	orderMap := make(map[string]*domain.Order)
	
	for rows.Next() {
		var orderID, orderStatus string
		var orderCreatedAt, orderUpdatedAt time.Time
		var stepID sql.NullInt64
		var stepOrderID sql.NullString
		var stepFromStatus, stepToStatus sql.NullString
		var stepCreatedAt sql.NullTime

		err := rows.Scan(
			&orderID, &orderStatus, &orderCreatedAt, &orderUpdatedAt,
			&stepID, &stepOrderID, &stepFromStatus, &stepToStatus, &stepCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("掃描訂單失敗: %w", err)
		}

		// 如果訂單不存在於 map 中，創建新訂單
		if _, exists := orderMap[orderID]; !exists {
			orderMap[orderID] = &domain.Order{
				ID:        orderID,
				Status:    domain.OrderStatus(orderStatus),
				CreatedAt: orderCreatedAt,
				UpdatedAt: orderUpdatedAt,
				OrderSteps: []domain.OrderStep{},
			}
		}

		// 如果有步驟數據，添加到訂單的步驟列表中
		if stepID.Valid {
			orderMap[orderID].OrderSteps = append(orderMap[orderID].OrderSteps, domain.OrderStep{
				ID:         stepID.Int64,
				OrderID:    stepOrderID.String,
				FromStatus: domain.OrderStatus(stepFromStatus.String),
				ToStatus:   domain.OrderStatus(stepToStatus.String),
				CreatedAt:  stepCreatedAt.Time,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("讀取訂單失敗: %w", err)
	}

	// 將 map 轉換為 slice
	orders := make([]*domain.Order, 0, len(orderMap))
	for _, order := range orderMap {
		orders = append(orders, order)
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

// BatchAddOrderSteps 批量添加訂單步驟
func (q *directQueries) BatchAddOrderSteps(ctx context.Context, steps []*domain.OrderStep) error {
	if len(steps) == 0 {
		return nil
	}

	// 使用 pgx Batch 進行批量插入
	batch := &pgx.Batch{}
	for _, step := range steps {
		batch.Queue(`
			INSERT INTO order_steps (order_id, from_status, to_status, created_at)
			VALUES ($1, $2, $3, $4)
		`, step.OrderID, string(step.FromStatus), string(step.ToStatus), step.CreatedAt)
	}
	
	// 執行批量操作
	if q.tx != nil {
		results := q.tx.SendBatch(ctx, batch)
		defer results.Close()
		
		for i := 0; i < len(steps); i++ {
			_, err := results.Exec()
			if err != nil {
				return fmt.Errorf("批量插入訂單步驟失敗 (第 %d 個): %w", i+1, err)
			}
		}
	} else {
		// 如果沒有事務，需要開啟一個事務來執行批量操作
		return pgx.BeginTxFunc(ctx, q.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			results := tx.SendBatch(ctx, batch)
			defer results.Close()
			
			for i := 0; i < len(steps); i++ {
				_, err := results.Exec()
				if err != nil {
					return fmt.Errorf("批量插入訂單步驟失敗 (第 %d 個): %w", i+1, err)
				}
			}
			return nil
		})
	}
	
	return nil
}

