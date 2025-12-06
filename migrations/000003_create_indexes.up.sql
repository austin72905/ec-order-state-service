-- 建立索引以提升查詢效能
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_order_steps_order_id ON order_steps(order_id);
CREATE INDEX IF NOT EXISTS idx_order_steps_created_at ON order_steps(created_at);

COMMENT ON INDEX idx_orders_status IS '訂單狀態索引，用於查詢特定狀態的訂單';
COMMENT ON INDEX idx_orders_created_at IS '訂單建立時間索引，用於時間範圍查詢';
COMMENT ON INDEX idx_order_steps_order_id IS '訂單步驟訂單 ID 索引，用於快速查詢訂單的狀態變更歷史';
COMMENT ON INDEX idx_order_steps_created_at IS '訂單步驟建立時間索引，用於時間範圍查詢';

