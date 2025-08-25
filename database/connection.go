package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/furutachiKurea/block-checker/config"

	_ "github.com/go-sql-driver/mysql"
)

var (
	db *sql.DB
	mu sync.RWMutex
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	ErrorTypeNetwork       ErrorType = "network"
	ErrorTypeAuth         ErrorType = "authentication"
	ErrorTypeConfig       ErrorType = "configuration"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeSQL          ErrorType = "sql"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// ErrorDetails 详细错误信息
type ErrorDetails struct {
	Type        ErrorType `json:"type"`
	Code        string    `json:"code,omitempty"`
	Message     string    `json:"message"`
	Cause       string    `json:"cause,omitempty"`
	Suggestion  string    `json:"suggestion,omitempty"`
	Timestamp   string    `json:"timestamp"`
	RetryCount  int       `json:"retry_count,omitempty"`
}

// DBStatus 数据库状态响应
type DBStatus struct {
	Status       string        `json:"status"`
	Timestamp    string        `json:"timestamp,omitempty"`
	Error        string        `json:"error,omitempty"`
	ErrorDetails *ErrorDetails `json:"error_details,omitempty"`
}

// InitDB 初始化数据库连接
func InitDB() error {
	config := config.GetDBConfig()
	dsn := buildDSN(config)

	// 创建连接信息对象
	connInfo := &ConnectionInfo{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.User,
		Password: config.Pass, // 明文显示
		Database: config.Name,
	}

	var err error
	mu.Lock()
	db, err = sql.Open("mysql", dsn)
	mu.Unlock()

	if err != nil {
		logger := GetDatabaseLogger()
		logger.ErrorWithConnection("数据库连接打开失败", connInfo, err.Error())
		return fmt.Errorf("open database: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := db.Ping(); err != nil {
		logger := GetDatabaseLogger()
		
		// 分析错误并记录
		errorDetails := analyzeError(err, 0)
		logger.ErrorWithConnection("❌ 数据库连接测试失败", connInfo,
			fmt.Sprintf("错误类型: %s, 错误代码: %s, 问题原因: %s, 解决建议: %s",
				errorDetails.Type, errorDetails.Code, errorDetails.Cause, errorDetails.Suggestion))
		
		// 启动重连器
		reconnector := GetReconnector()
		reconnector.StartReconnection()
		return nil
	}

	logger := GetDatabaseLogger()
	logger.InfoWithConnection(fmt.Sprintf("✅ 数据库连接成功: %s:%s", config.Host, config.Port), connInfo)

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

	// 获取当前连接配置用于日志记录
	config := config.GetDBConfig()
	connInfo := &ConnectionInfo{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.User,
		Password: config.Pass,
		Database: config.Name,
	}

	// 关闭数据库连接
	mu.Lock()
	if db != nil {
		if err := db.Close(); err != nil {
			logger := GetDatabaseLogger()
			logger.ErrorWithConnection("关闭数据库连接失败", connInfo, err.Error())
		} else {
			logger := GetDatabaseLogger()
			logger.InfoWithConnection("数据库连接已关闭", connInfo)
		}
		db = nil
	}
	mu.Unlock()
}

// CheckStatus 检查数据库状态
func CheckStatus() *DBStatus {
	reconnector := GetReconnector()

	if db == nil {
		errorDetails := &ErrorDetails{
			Type:       ErrorTypeConfig,
			Code:       "CFG_002",
			Message:    "Database not initialized",
			Cause:      "数据库连接未初始化",
			Suggestion: "请检查数据库配置和启动过程",
			Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
		}
		return &DBStatus{
			Status:       "Not Connected",
			Error:        "Database not initialized",
			ErrorDetails: errorDetails,
		}
	}

	// 先测试连接
	if err := db.Ping(); err != nil {
		// 获取重连次数
		retryCount := reconnector.GetRetryCount()
		errorDetails := analyzeError(err, retryCount)
		
		// 触发重连
		reconnector.OnConnectionLost()

		if reconnector.IsReconnecting() {
			errorDetails.Message = "正在尝试重新连接数据库..."
			if retryCount > 0 {
				errorDetails.Message = fmt.Sprintf("正在尝试重新连接数据库... (第 %d 次重试)", retryCount)
			}
			
			return &DBStatus{
				Status:       "Reconnecting",
				Error:        errorDetails.Message,
				ErrorDetails: errorDetails,
			}
		}

		return &DBStatus{
			Status:       "Not Connected",
			Error:        fmt.Sprintf("Database connection failed: %v", err),
			ErrorDetails: errorDetails,
		}
	}

	// 执行简单查询获取当前时间
	var currentTime string
	err := db.QueryRow("SELECT NOW()").Scan(&currentTime)
	if err != nil {
		errorDetails := analyzeError(err, 0)
		return &DBStatus{
			Status:       "Failed",
			Error:        fmt.Sprintf("Query failed: %v", err),
			ErrorDetails: errorDetails,
		}
	}

	return &DBStatus{
		Status:    "OK",
		Timestamp: currentTime,
	}
}

// analyzeError 分析错误类型和详情
func analyzeError(err error, retryCount int) *ErrorDetails {
	if err == nil {
		return nil
	}

	// 使用新的错误分析器
	analyzer := GetErrorAnalyzer()
	return analyzer.AnalyzeError(err, retryCount)
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *config.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		config.User, config.Pass, config.Host, config.Port, config.Name)
}
