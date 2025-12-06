package main

import (
	"database/sql"
	"ec-order-state-service/internal/config"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	var (
		command = flag.String("command", "up", "migration 命令: up, down, version, force")
		steps   = flag.Int("steps", 0, "執行步數（用於 up/down，0 表示執行所有）")
		version = flag.Int("version", 0, "版本號（用於 force 命令）")
	)
	flag.Parse()

	// 載入配置
	cfg := config.LoadConfig()
	databaseURL := cfg.GetDatabaseURL()

	// 建立資料庫連接
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("連接資料庫失敗: %v", err)
	}
	defer db.Close()

	// 建立 postgres driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("建立 migration driver 失敗: %v", err)
	}

	// 獲取 migration 檔案路徑（相對於專案根目錄）
	migrationPath := "file://migrations"
	if _, err := os.Stat("migrations"); os.IsNotExist(err) {
		// 嘗試從 cmd/migrate 目錄查找
		migrationPath = "file://../../migrations"
	}

	// 建立 migrate 實例
	m, err := migrate.NewWithDatabaseInstance(
		migrationPath,
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatalf("建立 migrate 實例失敗: %v", err)
	}

	// 執行命令
	switch *command {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
		if err != nil && err != migrate.ErrNoChange {
			log.Fatalf("執行 migration up 失敗: %v", err)
		}
		if err == migrate.ErrNoChange {
			fmt.Println("資料庫已是最新版本，無需 migration")
		} else {
			fmt.Println("Migration up 執行成功")
		}

	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
		if err != nil && err != migrate.ErrNoChange {
			log.Fatalf("執行 migration down 失敗: %v", err)
		}
		if err == migrate.ErrNoChange {
			fmt.Println("資料庫已是最舊版本，無需 migration")
		} else {
			fmt.Println("Migration down 執行成功")
		}

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("獲取版本失敗: %v", err)
		}
		fmt.Printf("當前版本: %d, Dirty: %v\n", version, dirty)

	case "force":
		if *version == 0 {
			log.Fatal("force 命令需要指定 -version 參數")
		}
		err = m.Force(*version)
		if err != nil {
			log.Fatalf("執行 force 失敗: %v", err)
		}
		fmt.Printf("強制設定版本為: %d\n", *version)

	default:
		log.Fatalf("未知的命令: %s", *command)
	}
}

