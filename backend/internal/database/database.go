package database

import (
	"fmt"
	"log"
	"time"

	"gitlab-analytics/internal/config"
	"gitlab-analytics/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open打开数据库连接
func Open(cfg config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		// Request errors are returned by handlers; avoid logging SQL with access tokens or other secrets.
		db, err = gorm.Open(mysql.Open(cfg.DBDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err == nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr == nil && sqlDB.Ping() == nil {
				break
			}
		}
		log.Printf("database not ready (attempt %d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := db.AutoMigrate(
		&model.Setting{},
		&model.Group{},
		&model.Project{},
		&model.User{},
		&model.UserAlias{},
		&model.Commit{},
		&model.SyncRun{},
		&model.Member{},
	); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}
