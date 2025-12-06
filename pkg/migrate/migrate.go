package migrate

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations 執行資料庫 migration
func RunMigrations(databaseURL string, migrationPath string) error {
	// 建立資料庫連接
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("連接資料庫失敗: %w", err)
	}
	defer db.Close()

	// 建立 postgres driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("建立 migration driver 失敗: %w", err)
	}

	// 建立 migrate 實例
	m, err := migrate.NewWithDatabaseInstance(
		migrationPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("建立 migrate 實例失敗: %w", err)
	}

	// 執行 migration
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("資料庫已是最新版本，無需 migration")
			return nil
		}
		return fmt.Errorf("執行 migration 失敗: %w", err)
	}

	log.Println("資料庫 migration 執行成功")
	return nil
}

