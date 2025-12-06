-- 建立訂單步驟表（狀態變更歷史）
CREATE TABLE IF NOT EXISTS order_steps (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    from_status VARCHAR(50) NOT NULL,
    to_status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_order_steps_order_id FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

COMMENT ON TABLE order_steps IS '訂單步驟表（狀態變更歷史）';
COMMENT ON COLUMN order_steps.id IS '步驟 ID';
COMMENT ON COLUMN order_steps.order_id IS '訂單 ID（外鍵）';
COMMENT ON COLUMN order_steps.from_status IS '來源狀態';
COMMENT ON COLUMN order_steps.to_status IS '目標狀態';
COMMENT ON COLUMN order_steps.created_at IS '建立時間';

