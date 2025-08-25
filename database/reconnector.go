package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/furutachiKurea/block-checker/config"
)

// Reconnector 重连器
type Reconnector struct {
	mu           sync.RWMutex
	isConnected  bool
	reconnecting bool
	ctx          context.Context
	cancel       context.CancelFunc
	config       *config.DBConfig
	retryCount   int
	lastError    error
	errorHistory []string
}

var (
	reconnector *Reconnector
	once        sync.Once
)

// GetReconnector 获取重连器实例
func GetReconnector() *Reconnector {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		reconnector = &Reconnector{
			ctx:    ctx,
			cancel: cancel,
			config: config.GetDBConfig(),
		}
	})
	return reconnector
}

// IsConnected 检查是否已连接
func (r *Reconnector) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isConnected
}

// IsReconnecting 检查是否正在重连
func (r *Reconnector) IsReconnecting() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reconnecting
}

// GetRetryCount 获取重试次数
func (r *Reconnector) GetRetryCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retryCount
}

// GetLastError 获取最后一次错误
func (r *Reconnector) GetLastError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastError
}

// GetErrorHistory 获取错误历史
func (r *Reconnector) GetErrorHistory() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// 返回副本避免并发问题
	history := make([]string, len(r.errorHistory))
	copy(history, r.errorHistory)
	return history
}

// StartReconnection 开始重连
func (r *Reconnector) StartReconnection() {
	r.mu.Lock()
	if r.reconnecting {
		r.mu.Unlock()
		return
	}
	r.reconnecting = true
	r.mu.Unlock()

	go r.reconnectionLoop()
}

// StopReconnection 停止重连
func (r *Reconnector) StopReconnection() {
	r.mu.Lock()
	r.reconnecting = false
	r.mu.Unlock()
	r.cancel()
}

// reconnectionLoop 重连循环
func (r *Reconnector) reconnectionLoop() {
	initialDelay := 1 * time.Second
	maxDelay := 30 * time.Second
	currentDelay := initialDelay
	
	// 创建重连专用日志记录器
	reconnLogger := NewReconnectionLogger()
	reconnLogger.StartReconnection()

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// 尝试连接
			if r.tryConnect() {
				r.mu.Lock()
				successRetryCount := r.retryCount
				r.isConnected = true
				r.reconnecting = false
				r.retryCount = 0 // 重置重试计数
				r.lastError = nil
				r.mu.Unlock()
				
				// 记录成功日志
				reconnLogger.LogSuccess(successRetryCount)
				return
			}

			r.mu.Lock()
			r.retryCount++
			retryCount := r.retryCount
			lastError := r.lastError
			r.mu.Unlock()

			// 使用新的日志记录器
			reconnLogger.LogRetry(retryCount, currentDelay, lastError)

			// 等待后重试
			select {
			case <-r.ctx.Done():
				return
			case <-time.After(currentDelay):
				// 指数退避，但不超过最大延迟
				currentDelay *= 2
				if currentDelay > maxDelay {
					currentDelay = maxDelay
				}
			}
		}
	}
}

// tryConnect 尝试连接
func (r *Reconnector) tryConnect() bool {
	dsn := buildDSN(r.config)

	newDB, err := sql.Open("mysql", dsn)
	if err != nil {
		r.mu.Lock()
		r.lastError = err
		r.addErrorToHistory(fmt.Sprintf("打开数据库连接失败: %v", err))
		r.mu.Unlock()
		return false
	}

	// 设置连接池参数
	newDB.SetMaxOpenConns(10)
	newDB.SetMaxIdleConns(5)
	newDB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := newDB.Ping(); err != nil {
		r.mu.Lock()
		r.lastError = err
		r.addErrorToHistory(fmt.Sprintf("数据库连接测试失败: %v", err))
		r.mu.Unlock()
		
		if closeErr := newDB.Close(); closeErr != nil {
			logger := GetDatabaseLogger()
			logger.Error("关闭新数据库连接失败", closeErr.Error())
		}
		return false
	}

	// 替换全局数据库连接
	mu.Lock()
	if db != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger := GetDatabaseLogger()
			logger.Error("关闭旧数据库连接失败", closeErr.Error())
		}
	}
	db = newDB
	mu.Unlock()

	return true
}

// addErrorToHistory 添加错误到历史记录
func (r *Reconnector) addErrorToHistory(errorMsg string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] %s", timestamp, errorMsg)
	
	// 只保留最近的10条错误记录
	if len(r.errorHistory) >= 10 {
		r.errorHistory = r.errorHistory[1:]
	}
	r.errorHistory = append(r.errorHistory, entry)
}

// OnConnectionLost 连接丢失时的回调
func (r *Reconnector) OnConnectionLost() {
	r.mu.Lock()
	r.isConnected = false
	r.mu.Unlock()

	logger := GetDatabaseLogger()
	logger.Warn("❌ 数据库连接丢失，启动重连程序...")
	r.StartReconnection()
}

// CheckConnection 检查连接状态
func (r *Reconnector) CheckConnection() bool {
	if db == nil {
		return false
	}

	if err := db.Ping(); err != nil {
		r.OnConnectionLost()
		return false
	}

	r.mu.Lock()
	r.isConnected = true
	r.mu.Unlock()
	return true
}
