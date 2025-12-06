-- 添加 updated_at 索引以優化調度器查詢
CREATE INDEX IF NOT EXISTS idx_orders_updated_at ON orders(updated_at);

-- 添加複合索引以優化狀態+更新時間查詢（調度器常用）
CREATE INDEX IF NOT EXISTS idx_orders_status_updated_at ON orders(status, updated_at);

COMMENT ON INDEX idx_orders_updated_at IS '訂單更新時間索引，用於調度器按更新時間排序查詢';
COMMENT ON INDEX idx_orders_status_updated_at IS '訂單狀態和更新時間複合索引，優化調度器的狀態查詢和排序';

