package config

import (
	"os"
	"strconv"
)

// Config结构体
type Config struct {
	Port          string // 端口
	DBDSN         string // 数据库连接字符串
	JWTSecret     string // JWT密钥
	AdminUsername string // 管理员用户名
	AdminPassword string // 管理员密码
	SyncCron      string // 同步 cron 表达式
}

// Load加载配置
func Load() Config {
	return Config{
		Port:          value("PORT", "8080"),
		DBDSN:         value("DB_DSN", "root:root@tcp(127.0.0.1:3306)/gitlab_analytics?charset=utf8mb4&parseTime=True&loc=Local"), // 环境变量优先，否则使用默认值
		JWTSecret:     value("JWT_SECRET", "development-secret-change-me"),                                                        // 环境变量优先，否则使用默认值
		AdminUsername: value("ADMIN_USERNAME", "admin"),                                                                           // 环境变量优先，否则使用默认值
		AdminPassword: value("ADMIN_PASSWORD", "change-me-now"),                                                                   // 环境变量优先，否则使用默认值
		SyncCron:      value("SYNC_CRON", "0 */12 * * *"),                                                                         // 数据库settings表中的配置优先，其次环境变量，否则使用默认值
	}
}

// value获取环境变量值
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// boolValue获取环境变量值并转换为bool
func boolValue(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
