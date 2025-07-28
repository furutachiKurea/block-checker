package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/furutachiKurea/block-checker/config"

	_ "github.com/go-sql-driver/mysql"
)

var (
	db *sql.DB
	mu sync.RWMutex
)

// DBStatus 数据库状态响应
type DBStatus struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp,omitempty"`
	Error     string `json:"error,omitempty"`
}

// InitDB 初始化数据库连接
func InitDB() error {
	config := config.GetDBConfig()
	dsn := buildDSN(config)

	var err error
	mu.Lock()
	db, err = sql.Open("mysql", dsn)
	mu.Unlock()

	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return fmt.Errorf("open database: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping database: %v", err)
		// 启动重连器
		reconnector := GetReconnector()
		reconnector.StartReconnection()
		return nil
	}

	log.Printf("Database connected successfully to %s:%s", config.Host, config.Port)

	// 标记为已连接
	reconnector := GetReconnector()
	reconnector.mu.Lock()
	reconnector.isConnected = true
	reconnector.mu.Unlock()

	return nil
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

// CloseDB 关闭数据库连接
func CloseDB() {
	// 停止重连器
	reconnector := GetReconnector()
	reconnector.StopReconnection()

	// 关闭数据库连接
	mu.Lock()
	if db != nil {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
		db = nil
	}
	mu.Unlock()
}

// CheckStatus 检查数据库状态
func CheckStatus() *DBStatus {
	reconnector := GetReconnector()

	if db == nil {
		return &DBStatus{
			Status: "Not Connected",
			Error:  "Database not initialized",
		}
	}

	// 先测试连接
	if err := db.Ping(); err != nil {
		// 触发重连
		reconnector.OnConnectionLost()

		if reconnector.IsReconnecting() {
			return &DBStatus{
				Status: "Reconnecting",
				Error:  "Attempting to reconnect to database...",
			}
		}

		return &DBStatus{
			Status: "Not Connected",
			Error:  fmt.Sprintf("Database connection failed: %v", err),
		}
	}

	// 执行简单查询获取当前时间
	var currentTime string
	err := db.QueryRow("SELECT NOW()").Scan(&currentTime)
	if err != nil {
		return &DBStatus{
			Status: "Failed",
			Error:  fmt.Sprintf("Query failed: %v", err),
		}
	}

	return &DBStatus{
		Status:    "OK",
		Timestamp: currentTime,
	}
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *config.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		config.User, config.Pass, config.Host, config.Port, config.Name)
}
