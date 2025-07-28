package database

import (
	"context"
	"database/sql"
	"log"
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

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// 尝试连接
			if r.tryConnect() {
				r.mu.Lock()
				r.isConnected = true
				r.reconnecting = false
				r.mu.Unlock()
				log.Printf("Database reconnection successful")
				return
			}

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
				log.Printf("Retrying database connection in %v...", currentDelay)
			}
		}
	}
}

// tryConnect 尝试连接
func (r *Reconnector) tryConnect() bool {
	dsn := buildDSN(r.config)

	newDB, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("Failed to open database during reconnection: %v", err)
		return false
	}

	// 设置连接池参数
	newDB.SetMaxOpenConns(10)
	newDB.SetMaxIdleConns(5)
	newDB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := newDB.Ping(); err != nil {
		log.Printf("Failed to ping database during reconnection: %v", err)
		if closeErr := newDB.Close(); closeErr != nil {
			log.Printf("Failed to close new database connection: %v", closeErr)
		}
		return false
	}

	// 替换全局数据库连接
	mu.Lock()
	if db != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close old database connection: %v", closeErr)
		}
	}
	db = newDB
	mu.Unlock()

	return true
}

// OnConnectionLost 连接丢失时的回调
func (r *Reconnector) OnConnectionLost() {
	r.mu.Lock()
	r.isConnected = false
	r.mu.Unlock()

	log.Printf("Database connection lost, starting reconnection...")
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
