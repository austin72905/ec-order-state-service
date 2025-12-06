# 資料庫 Migration 使用說明

## 安裝依賴

專案已包含 `golang-migrate/migrate/v4` 依賴，無需額外安裝。

## Migration 檔案格式

Migration 檔案使用 `golang-migrate` 的命名格式：
- `{version}_{name}.up.sql` - 升級 migration
- `{version}_{name}.down.sql` - 降級 migration

例如：
- `000001_create_orders_table.up.sql`
- `000001_create_orders_table.down.sql`

## 使用方式

### 方式 1：使用 migrate 命令工具

```bash
# 編譯 migrate 工具
go build -o bin/migrate ./cmd/migrate

# 執行 migration up（執行所有待執行的 migration）
./bin/migrate -command=up

# 執行 migration down（回退所有 migration）
./bin/migrate -command=down

# 執行指定步數的 migration
./bin/migrate -command=up -steps=1

# 查看當前版本
./bin/migrate -command=version

# 強制設定版本（用於修復 dirty 狀態）
./bin/migrate -command=force -version=1
```

### 方式 2：服務啟動時自動執行

服務啟動時會自動執行 migration（在 `cmd/server/main.go` 中）。

### 方式 3：使用 golang-migrate CLI 工具

```bash
# 安裝 golang-migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 執行 migration
migrate -path migrations -database "postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable" up

# 回退 migration
migrate -path migrations -database "postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable" down

# 查看版本
migrate -path migrations -database "postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable" version
```

## 環境變數

Migration 工具會從環境變數或配置中讀取資料庫連接字串：

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable"
```

## 建立新的 Migration

1. 在 `migrations/` 目錄下建立新的檔案：
   - `000004_{name}.up.sql` - 升級腳本
   - `000004_{name}.down.sql` - 降級腳本

2. 版本號必須是遞增的整數（000001, 000002, 000003...）

3. 檔案內容為標準 SQL，無需特殊標記

## 注意事項

1. Migration 檔案會按照版本號順序執行
2. 執行 migration 前請確保資料庫已建立（`ec_order_state`）
3. 建議在開發環境先測試 migration 腳本
4. 生產環境建議手動執行 migration，而不是自動執行

