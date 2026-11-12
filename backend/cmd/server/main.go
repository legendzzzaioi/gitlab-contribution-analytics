package main

import (
	"log"

	"gitlab-analytics/internal/config"
	"gitlab-analytics/internal/database"
	"gitlab-analytics/internal/httpapi"
	"gitlab-analytics/internal/model"
	"gitlab-analytics/internal/service"

	"github.com/robfig/cron/v3"
)

func main() {
	// 加载配置
	cfg := config.Load()
	// 初始化数据库
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}

	// 获取数据库里保存的同步计划任务 cron，没有再用环境变量
	cronSpec := cfg.SyncCron
	var setting model.Setting
	if err := db.First(&setting, 1).Error; err == nil && setting.SyncCron != "" {
		if _, err := cron.ParseStandard(setting.SyncCron); err == nil {
			cronSpec = setting.SyncCron
		} else {
			log.Printf("invalid sync_cron in database (%q), falling back to %q", setting.SyncCron, cfg.SyncCron)
		}
	}

	// 初始化同步服务
	syncer := service.NewSyncService(db)
	// 初始化调度器
	scheduler, err := service.StartScheduler(cronSpec, syncer)
	if err != nil {
		log.Fatalf("invalid SYNC_CRON: %v", err)
	}
	defer scheduler.Stop()
	server := httpapi.New(db, cfg, syncer)
	server.SetScheduler(scheduler)

	// 启动服务器
	log.Printf("GitLab contribution analytics API listening on :%s", cfg.Port)
	if err := server.Router().Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
