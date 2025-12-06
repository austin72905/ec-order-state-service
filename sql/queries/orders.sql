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

-- name: UpdateOrderStatus :one
-- 注意：updated_at 由資料庫 trigger 自動更新，無需手動設置
UPDATE orders
SET 
    status = $2
WHERE id = $1
RETURNING *;

-- name: GetOrderSteps :many
SELECT * FROM order_steps
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: AddOrderStep :one
INSERT INTO order_steps (
    order_id,
    from_status,
    to_status,
    created_at
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

