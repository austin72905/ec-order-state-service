# SQLC 使用說明

## 生成代碼

在實作完成後，需要運行以下命令生成 SQLC 代碼：

```bash
# 安裝 sqlc（如果尚未安裝）
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# 生成代碼
sqlc generate
```

這會根據 `sql/queries/orders.sql` 和 `sqlc.yaml` 配置生成 `internal/db` 目錄下的代碼。

## 資料庫 Migration

在運行服務之前，需要先執行 migration：

```sql
-- 連接到 PostgreSQL
psql -h localhost -p 5433 -U postgres -d ec_order_state

-- 執行 migration
\i migrations/000001_create_orders_table.up.sql
\i migrations/000002_create_order_steps_table.up.sql
\i migrations/000003_create_indexes.up.sql
```

或者使用 migrate 工具：

```bash
# 安裝 migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 執行 migration
migrate -path migrations -database "postgres://postgres:postgres@localhost:5433/ec_order_state?sslmode=disable" up
```

## 更新 Repository

生成 sqlc 代碼後，需要更新 `internal/repository/pg_order_repository.go` 來使用生成的代碼：

```go
import "ec-order-state-service/internal/db"

func NewPgOrderRepository(pool *pgxpool.Pool) *PgOrderRepository {
    queries := db.New(pool)
    return &PgOrderRepository{
        pool:    pool,
        queries: queries,
    }
}
```

然後可以移除 `internal/repository/direct_queries.go` 檔案。

