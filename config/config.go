package config

import "os"

// DBConfig 数据库连接配置
type DBConfig struct {
	Host string
	Port string
	User string
	Pass string
	Name string
}

// GetDBConfig 从环境变量读取数据库配置
func GetDBConfig() *DBConfig {
	return &DBConfig{
		Host: getEnv("DB_HOST", "localhost"),
		Port: getEnv("DB_PORT", "3306"),
		User: getEnv("DB_USER", "root"),
		Pass: getEnv("DB_PASS", ""),
		Name: getEnv("DB_NAME", "mysql"),
	}
}

// getEnv 获取环境变量，提供默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
