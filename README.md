# 訂單狀態管理微服務 (Order State Service)

這是一個使用 Go 語言開發的訂單狀態管理微服務，採用 BDD（行為驅動開發）方式開發。

## 專案結構

```
ec-order-state-service/
├── cmd/
│   └── server/
│       └── main.go              # 應用程式入口
├── internal/
│   ├── domain/                  # 領域模型
│   │   ├── order.go
│   │   ├── order_status.go
│   │   └── events.go
│   ├── service/                 # 業務邏輯
│   │   └── order_state_service.go
│   ├── repository/              # 資料存取
│   │   └── order_repository.go
│   ├── handler/                 # HTTP 處理器
│   └── config/                  # 配置
├── pkg/
│   └── rabbitmq/                # RabbitMQ 整合
├── features/                    # BDD 功能文件
│   ├── order_status_lifecycle.feature
│   ├── step_definitions/
│   │   └── order_steps.go
│   └── order_state_test.go
├── go.mod
└── README.md
```

## 功能特性

- ✅ 訂單狀態機管理
- ✅ 狀態轉換驗證
- ✅ 自動記錄 OrderStep
- ✅ 事件驅動架構
- ✅ BDD 測試覆蓋

## 訂單狀態流程

```
Created → WaitingForPayment → WaitingForShipment → InTransit → WaitPickup → Completed
```

## 運行 BDD 測試

```bash
# 使用 go test
go test ./features -v

# 使用 godog CLI
godog features/order_status_lifecycle.feature

# 運行所有 feature 文件
godog features/
```

## 運行服務

```bash
go run cmd/server/main.go
```

## 環境變數

- `PORT`: 服務端口（預設: 8080）
- `DATABASE_URL`: PostgreSQL 連接字串
- `RABBITMQ_URL`: RabbitMQ 連接字串

## 開發

### 安裝依賴

```bash
go mod download
```

### 運行測試

```bash
go test ./...
```

### 格式化代碼

```bash
go fmt ./...
```

## License

MIT

