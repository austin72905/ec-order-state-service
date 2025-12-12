# 訂單狀態管理微服務（Order State Service）

使用 Go 與事件驅動架構實作的訂單狀態微服務，涵蓋狀態機驗證、事件處理、資料庫交易保護、MQ 整合與 BDD 驗證。

## 架構與目錄

```
ec-order-state-service/
├── cmd/server/main.go          # 入口：載入設定、跑 migration、啟動 HTTP/MQ/排程
├── internal/
│   ├── domain/                 # 領域模型與狀態機（Order、OrderStatus、事件）
│   ├── service/                # 核心業務（狀態轉換、冪等、事件發布）
│   ├── repository/             # 資料存取（Pg + in-memory、批次/鎖/重試）
│   ├── handler/                # MQ 事件 handler
│   ├── scheduler/              # 狀態自動推進排程（批次 + worker pool）
│   └── config/                 # 環境變數設定
├── pkg/
│   ├── rabbitmq/               # 連線管理、Producer/Consumer、DLQ、重試
│   └── migrate/                # 資料庫 migration 執行
├── migrations/                 # SQL migration（golang-migrate）
├── sql/queries/                # sqlc 查詢定義
├── features/                   # BDD 規格（godog）
└── README_*.md                 # 開發輔助文件（migration、sqlc）
```

## 技術棧

- 語言/版本：Go 1.25.4
- Web/API：Gin
- 資料庫：PostgreSQL + pgx/pgxpool、sqlc（預計生成型查詢介面）
- Migration：golang-migrate（啟動時自動跑，亦支援 CLI）
- 訊息：RabbitMQ（amqp091-go，含 DLX、重試、背壓、worker pool）
- 測試：godog（BDD）、testify
- 其他：環境變數配置、可調式連線池與超時設定

## 主要功能

- 訂單狀態機：`Created → WaitingForShipment → InTransit → WaitPickup → Completed`，含非法轉換防護與步驟追蹤。
- 事件驅動：處理 PaymentCompleted、ShipmentStarted、PackageArrivedAtPickupStore、PackagePickedUp 事件並發布狀態變更。
- API：`PUT /orders/:orderId/status` 進行狀態更新。
- BDD 規格：`features/order_status_lifecycle.feature` 覆蓋核心流程。
- 排程：定期批次推進狀態（可調 worker/batch/interval）。

## 優化與實作亮點

- 冪等 + 悲觀鎖：支付完成事件使用 `SELECT ... FOR UPDATE NOWAIT`、可重試交易與重複鍵/死鎖判斷，確保重入安全與順序一致性。  
- 原子保存：事務內先鎖後查，存在即更新、無則建立；補齊缺漏的步驟紀錄，避免狀態/步驟不一致。  
- 批次與去 N+1：`GetOrdersByStatusWithSteps` 以 JOIN 拿齊訂單與步驟並分頁，`BatchUpdateOrderStatus/BatchAddOrderSteps` 降低 DB 往返。  
- 連線池/超時調優：啟動時配置 pgxpool Min/Max、MaxLifetime、IdleTime、HealthCheck 及查詢/寫入超時，防止資源耗盡。  
- RabbitMQ 穩定性：重用 channel、雙重檢查初始化；DLX + 應用層 DLQ；Prefetch 控制公平分發；worker pool + 背壓；重試計數與超時後自動 DLQ。  
- 排程併發控制：semaphore 限制並發寫入，批次處理並插入步驟，並在批次間 sleep 降低壓力。  
- 開發易用性：in-memory repository/mocks 方便測試；啟動自動跑 migration；配置具多鍵 fallback（`DB_CONNECTION_STRING` / `DATABASE_URL`）。  
- BDD 驅動：godog feature 與 step definitions 驗證狀態機與事件行為。

## 快速開始

### 前置
- PostgreSQL（預設 `postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable`）
- RabbitMQ（預設 `amqp://guest:guest@localhost:5672/`）

### 啟動
```bash
go mod download
go run cmd/server/main.go
```

### 測試
```bash
go test ./...
godog features/                    # 或 go test ./features -v
```

### Migration / SQLC
- 啟動時自動執行 migration；或使用 `go build -o bin/migrate ./cmd/migrate`、`./bin/migrate -command=up`。  
- 需更新 sqlc 代碼時：`sqlc generate`（設定見 `sqlc.yaml`，產物位於 `internal/db`）。

## 環境變數（常用）
- `PORT`（預設 8080）
- `DATABASE_URL` / `DB_CONNECTION_STRING`
- `RABBITMQ_URL`
- `DB_MAX_CONNS` / `DB_MIN_CONNS` / `DB_MAX_CONN_LIFETIME` / `DB_MAX_CONN_IDLE_TIME` / `DB_HEALTH_CHECK_PERIOD`
- `DB_QUERY_TIMEOUT` / `DB_WRITE_TIMEOUT`
- `RABBITMQ_PREFETCH_COUNT` / `RABBITMQ_WORKER_COUNT`
- `SCHEDULER_WORKER_COUNT` / `SCHEDULER_BATCH_SIZE` / `SCHEDULER_INTERVAL`



