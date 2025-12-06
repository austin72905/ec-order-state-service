-- 移除 trigger
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;

-- 移除函數
DROP FUNCTION IF EXISTS update_updated_at_column();

