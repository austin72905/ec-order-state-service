package repository

import (
	"context"
	"ec-order-state-service/internal/domain"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Queries 查詢介面（對應 sqlc 生成的 Queries）
type Queries interface {
	GetOrderByID(ctx context.Context, id string) (Order, error)
	GetOrderByIDForUpdate(ctx context.Context, id string) (Order, error)
	CreateOrder(ctx context.Context, arg CreateOrderParams) (Order, error)
	UpdateOrderStatus(ctx context.Context, arg UpdateOrderStatusParams) (Order, error)
	GetOrderSteps(ctx context.Context, orderID string) ([]OrderStep, error)
	AddOrderStep(ctx context.Context, arg AddOrderStepParams) (OrderStep, error)
	GetOrdersByStatus(ctx context.Context, status string) ([]Order, error)
	GetOrdersByStatusWithSteps(ctx context.Context, status string, limit, offset int) ([]*domain.Order, error)
	BatchUpdateOrderStatus(ctx context.Context, orderIDs []string, newStatus, fromStatus string) error
	BatchAddOrderSteps(ctx context.Context, steps []*domain.OrderStep) error
}

// Order 資料庫模型（對應 sqlc 生成的 Order）
type Order struct {
	ID        string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrderStep 資料庫模型（對應 sqlc 生成的 OrderStep）
type OrderStep struct {
	ID         int64
	OrderID    string
	FromStatus string
	ToStatus   string
	CreatedAt  time.Time
}

// CreateOrderParams 建立訂單參數
type CreateOrderParams struct {
	ID        string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpdateOrderStatusParams 更新訂單狀態參數
type UpdateOrderStatusParams struct {
	ID     string
	Status string
}

// AddOrderStepParams 添加訂單步驟參數
type AddOrderStepParams struct {
	OrderID    string
	FromStatus string
	ToStatus   string
	CreatedAt  time.Time
}

// PgOrderRepository PostgreSQL 實作的訂單倉儲
type PgOrderRepository struct {
	pool         *pgxpool.Pool
	queries      Queries
	queryTimeout time.Duration // 查詢操作超時時間
	writeTimeout time.Duration // 寫入操作超時時間
}

// NewPgOrderRepository 創建 PostgreSQL 倉儲（使用預設超時）
// 注意：需要先運行 sqlc generate 生成 db 包
// 目前使用直接 SQL 查詢作為臨時實作
func NewPgOrderRepository(pool *pgxpool.Pool) *PgOrderRepository {
	return NewPgOrderRepositoryWithConfig(pool, 5*time.Second, 10*time.Second)
}

// NewPgOrderRepositoryWithConfig 創建 PostgreSQL 倉儲（使用自訂超時配置）
func NewPgOrderRepositoryWithConfig(pool *pgxpool.Pool, queryTimeout, writeTimeout time.Duration) *PgOrderRepository {
	// TODO: 運行 sqlc generate 後，可以改用以下代碼：
	// import "ec-order-state-service/internal/db"
	// queries := db.New(pool)
	// return &PgOrderRepository{
	// 	pool:    pool,
	// 	queries: queries,
	// }

	// 目前使用直接 SQL 查詢
	return &PgOrderRepository{
		pool:         pool,
		queries:      &directQueries{pool: pool},
		queryTimeout: queryTimeout,
		writeTimeout: writeTimeout,
	}
}

// GetByID 根據 ID 獲取訂單
func (r *PgOrderRepository) GetByID(orderID string) (*domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.queryTimeout)
	defer cancel()

	dbOrder, err := r.queries.GetOrderByID(ctx, orderID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOrderNotFound{OrderID: orderID}
		}
		return nil, fmt.Errorf("查詢訂單失敗: %w", err)
	}

	// 獲取訂單步驟
	steps, err := r.queries.GetOrderSteps(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單步驟失敗: %w", err)
	}

	return toDomainOrder(dbOrder, steps), nil
}

// Save 保存訂單（使用事務保護）
func (r *PgOrderRepository) Save(order *domain.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.writeTimeout)
	defer cancel()

	// 使用事務包裹所有操作，確保原子性
	const maxRetries = 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 重試前等待一小段時間（指數退避）
			waitTime := time.Duration(attempt) * 50 * time.Millisecond
			time.Sleep(waitTime)
			// 重新創建 context（因為之前的可能已超時）
			ctx, cancel = context.WithTimeout(context.Background(), r.writeTimeout)
			defer cancel()
		}

		err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{
			IsoLevel: pgx.Serializable, // 使用最高隔離級別
		}, func(tx pgx.Tx) error {
			// 在事務中使用帶鎖的查詢
			txQueries := &directQueries{pool: r.pool, tx: tx}

			// 使用 SELECT FOR UPDATE 檢查訂單是否已存在（悲觀鎖）
			existing, err := txQueries.GetOrderByIDForUpdate(ctx, order.ID)
			if err != nil && err != pgx.ErrNoRows {
				return fmt.Errorf("查詢訂單失敗: %w", err)
			}

			if existing.ID != "" {
				// 更新訂單狀態
				_, err = txQueries.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
					ID:     order.ID,
					Status: string(order.Status),
				})
				if err != nil {
					return fmt.Errorf("更新訂單失敗: %w", err)
				}
			} else {
				// 創建新訂單
				_, err = txQueries.CreateOrder(ctx, CreateOrderParams{
					ID:        order.ID,
					Status:    string(order.Status),
					CreatedAt: order.CreatedAt,
					UpdatedAt: order.UpdatedAt,
				})
				if err != nil {
					return fmt.Errorf("創建訂單失敗: %w", err)
				}
			}

			// 保存訂單步驟（只保存新的步驟）
			existingSteps, err := txQueries.GetOrderSteps(ctx, order.ID)
			if err != nil {
				return fmt.Errorf("查詢訂單步驟失敗: %w", err)
			}

			existingStepCount := len(existingSteps)
			if len(order.OrderSteps) > existingStepCount {
				// 添加新的步驟
				for i := existingStepCount; i < len(order.OrderSteps); i++ {
					step := order.OrderSteps[i]
					_, err = txQueries.AddOrderStep(ctx, AddOrderStepParams{
						OrderID:    step.OrderID,
						FromStatus: string(step.FromStatus),
						ToStatus:   string(step.ToStatus),
						CreatedAt:  step.CreatedAt,
					})
					if err != nil {
						return fmt.Errorf("添加訂單步驟失敗: %w", err)
					}
				}
			}

			return nil
		})

		if err == nil {
			return nil
		}

		lastErr = err
		// 檢查是否為死鎖錯誤，如果是則重試
		if isDeadlockError(err) {
			continue
		}
		// 其他錯誤直接返回
		return err
	}

	return fmt.Errorf("保存訂單失敗（已重試 %d 次）: %w", maxRetries, lastErr)
}

// isDeadlockError 檢查是否為死鎖錯誤
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// PostgreSQL 死鎖錯誤碼為 40001
	return strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "40001")
}

// AddOrderStep 添加訂單步驟
func (r *PgOrderRepository) AddOrderStep(step *domain.OrderStep) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.writeTimeout)
	defer cancel()

	_, err := r.queries.AddOrderStep(ctx, AddOrderStepParams{
		OrderID:    step.OrderID,
		FromStatus: string(step.FromStatus),
		ToStatus:   string(step.ToStatus),
		CreatedAt:  step.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("添加訂單步驟失敗: %w", err)
	}

	return nil
}

// GetOrderSteps 獲取訂單步驟列表
func (r *PgOrderRepository) GetOrderSteps(orderID string) ([]domain.OrderStep, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.queryTimeout)
	defer cancel()

	dbSteps, err := r.queries.GetOrderSteps(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("查詢訂單步驟失敗: %w", err)
	}

	steps := make([]domain.OrderStep, len(dbSteps))
	for i, dbStep := range dbSteps {
		steps[i] = toDomainOrderStep(dbStep)
	}

	return steps, nil
}

// GetOrdersByStatus 根據狀態獲取訂單列表（支持分頁）
func (r *PgOrderRepository) GetOrdersByStatus(status domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.queryTimeout)
	defer cancel()

	// 使用批量查詢方法避免 N+1 問題
	if limit <= 0 {
		limit = 100 // 預設限制
	}
	if offset < 0 {
		offset = 0
	}
	return r.queries.GetOrdersByStatusWithSteps(ctx, string(status), limit, offset)
}

// BatchUpdateOrderStatus 批量更新訂單狀態
func (r *PgOrderRepository) BatchUpdateOrderStatus(orderIDs []string, newStatus domain.OrderStatus, fromStatus domain.OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.writeTimeout)
	defer cancel()

	return r.queries.BatchUpdateOrderStatus(ctx, orderIDs, string(newStatus), string(fromStatus))
}

// BatchAddOrderSteps 批量添加訂單步驟
func (r *PgOrderRepository) BatchAddOrderSteps(steps []*domain.OrderStep) error {
	if len(steps) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.writeTimeout)
	defer cancel()

	return r.queries.BatchAddOrderSteps(ctx, steps)
}

// toDomainOrder 將資料庫模型轉換為領域模型
func toDomainOrder(dbOrder Order, dbSteps []OrderStep) *domain.Order {
	order := &domain.Order{
		ID:        dbOrder.ID,
		Status:    domain.OrderStatus(dbOrder.Status),
		CreatedAt: dbOrder.CreatedAt,
		UpdatedAt: dbOrder.UpdatedAt,
		OrderSteps: make([]domain.OrderStep, len(dbSteps)),
	}

	for i, dbStep := range dbSteps {
		order.OrderSteps[i] = toDomainOrderStep(dbStep)
	}

	return order
}

// toDomainOrderStep 將資料庫模型轉換為領域模型
func toDomainOrderStep(dbStep OrderStep) domain.OrderStep {
	return domain.OrderStep{
		ID:         dbStep.ID,
		OrderID:    dbStep.OrderID,
		FromStatus: domain.OrderStatus(dbStep.FromStatus),
		ToStatus:   domain.OrderStatus(dbStep.ToStatus),
		CreatedAt:  dbStep.CreatedAt,
	}
}

