-- 建立訂單表
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(255) PRIMARY KEY,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE orders IS '訂單表';
COMMENT ON COLUMN orders.id IS '訂單 ID';
COMMENT ON COLUMN orders.status IS '訂單狀態 (Created, WaitingForPayment, WaitingForShipment, InTransit, WaitPickup, Completed, Canceled, Refund)';
COMMENT ON COLUMN orders.created_at IS '建立時間';
COMMENT ON COLUMN orders.updated_at IS '更新時間';

