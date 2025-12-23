-- name: CreateOrder :one
INSERT INTO orders (
    id,
    status,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders
WHERE id = $1 LIMIT 1;

-- name: GetOrderByIDForUpdate :one
-- 使用悲觀鎖 SELECT FOR UPDATE NOWAIT 避免長時間等待鎖
SELECT * FROM orders
WHERE id = $1
FOR UPDATE NOWAIT
LIMIT 1;

-- name: UpdateOrderStatus :one
-- 注意：updated_at 由資料庫 trigger 自動更新，無需手動設置
UPDATE orders
SET 
    status = $2
WHERE id = $1
RETURNING *;

-- name: GetOrdersByStatusWithPagination :many
-- 根據狀態獲取訂單列表（支持分頁）
SELECT * FROM orders
WHERE status = $1
ORDER BY updated_at ASC
LIMIT $2 OFFSET $3;

-- name: BatchUpdateOrderStatus :exec
-- 批量更新訂單狀態（使用 PostgreSQL 的 ANY 操作符）
-- 只更新狀態為 fromStatus 的訂單，確保冪等性
UPDATE orders
SET status = $1
WHERE id = ANY($2::text[]) AND status = $3;
