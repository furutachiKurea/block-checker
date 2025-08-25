package database

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogEntry 日志条目
type LogEntry struct {
	Level          LogLevel           `json:"level"`
	Message        string             `json:"message"`
	Timestamp      time.Time          `json:"timestamp"`
	Details        string             `json:"details,omitempty"`
	Count          int                `json:"count,omitempty"` // 用于记录重复日志的次数
	ConnectionInfo *ConnectionInfo    `json:"connection_info,omitempty"` // 数据库连接信息
}

// ConnectionInfo 数据库连接信息
type ConnectionInfo struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"` // 明文显示
	Database string `json:"database"`
}

// DatabaseLogger 数据库日志管理器
type DatabaseLogger struct {
	mu           sync.RWMutex
	entries      []LogEntry
	maxEntries   int
	currentLevel LogLevel
	lastEntry    *LogEntry
	suppressDuplicates bool
}

var (
	dbLogger *DatabaseLogger
	loggerOnce sync.Once
)

// GetDatabaseLogger 获取数据库日志管理器实例
func GetDatabaseLogger() *DatabaseLogger {
	loggerOnce.Do(func() {
		dbLogger = &DatabaseLogger{
			entries:           make([]LogEntry, 0),
			maxEntries:        100, // 最多保留100条日志
			currentLevel:      LogLevelInfo,
			suppressDuplicates: true,
		}
	})
	return dbLogger
}

// SetLogLevel 设置日志级别
func (dl *DatabaseLogger) SetLogLevel(level LogLevel) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.currentLevel = level
}

// SetMaxEntries 设置最大日志条目数
func (dl *DatabaseLogger) SetMaxEntries(max int) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.maxEntries = max
}

// SetSuppressDuplicates 设置是否抑制重复日志
func (dl *DatabaseLogger) SetSuppressDuplicates(suppress bool) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.suppressDuplicates = suppress
}

// addEntry 添加日志条目
func (dl *DatabaseLogger) addEntry(level LogLevel, message, details string, connInfo ...*ConnectionInfo) {
	if level < dl.currentLevel {
		return
	}

	dl.mu.Lock()
	defer dl.mu.Unlock()

	// 检查是否是重复的日志消息
	if dl.suppressDuplicates && dl.lastEntry != nil &&
	   dl.lastEntry.Message == message && dl.lastEntry.Level == level {
		dl.lastEntry.Count++
		dl.lastEntry.Timestamp = time.Now()
		return
	}

	entry := LogEntry{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Details:   details,
		Count:     1,
	}

	// 添加连接信息（如果提供）
	if len(connInfo) > 0 && connInfo[0] != nil {
		entry.ConnectionInfo = connInfo[0]
	}

	// 保持日志条目数量在限制内
	if len(dl.entries) >= dl.maxEntries {
		dl.entries = dl.entries[1:]
	}

	dl.entries = append(dl.entries, entry)
	dl.lastEntry = &entry

	// 输出到标准日志
	dl.outputToStdLog(entry)
}

// addEntryWithConnection 专门用于记录包含连接信息的日志
func (dl *DatabaseLogger) addEntryWithConnection(level LogLevel, message, details string, connInfo *ConnectionInfo) {
	dl.addEntry(level, message, details, connInfo)
}

// outputToStdLog 输出到标准日志
func (dl *DatabaseLogger) outputToStdLog(entry LogEntry) {
	levelStr := dl.getLevelString(entry.Level)
	
	if entry.Count > 1 {
		log.Printf("[%s] %s (重复 %d 次)", levelStr, entry.Message, entry.Count)
	} else {
		log.Printf("[%s] %s", levelStr, entry.Message)
	}
	
	if entry.Details != "" && entry.Level >= LogLevelWarn {
		log.Printf("   详情: %s", entry.Details)
	}
}

// getLevelString 获取日志级别字符串
func (dl *DatabaseLogger) getLevelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "调试"
	case LogLevelInfo:
		return "信息"
	case LogLevelWarn:
		return "警告"
	case LogLevelError:
		return "错误"
	case LogLevelFatal:
		return "致命"
	default:
		return "未知"
	}
}

// Debug 记录调试日志
func (dl *DatabaseLogger) Debug(message string, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntry(LogLevelDebug, message, detail)
}

// Info 记录信息日志
func (dl *DatabaseLogger) Info(message string, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntry(LogLevelInfo, message, detail)
}

// Warn 记录警告日志
func (dl *DatabaseLogger) Warn(message string, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntry(LogLevelWarn, message, detail)
}

// Error 记录错误日志
func (dl *DatabaseLogger) Error(message string, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntry(LogLevelError, message, detail)
}

// Fatal 记录致命错误日志
func (dl *DatabaseLogger) Fatal(message string, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntry(LogLevelFatal, message, detail)
	os.Exit(1)
}

// 带连接信息的日志记录方法
// DebugWithConnection 记录包含连接信息的调试日志
func (dl *DatabaseLogger) DebugWithConnection(message string, connInfo *ConnectionInfo, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntryWithConnection(LogLevelDebug, message, detail, connInfo)
}

// InfoWithConnection 记录包含连接信息的信息日志
func (dl *DatabaseLogger) InfoWithConnection(message string, connInfo *ConnectionInfo, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntryWithConnection(LogLevelInfo, message, detail, connInfo)
}

// WarnWithConnection 记录包含连接信息的警告日志
func (dl *DatabaseLogger) WarnWithConnection(message string, connInfo *ConnectionInfo, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntryWithConnection(LogLevelWarn, message, detail, connInfo)
}

// ErrorWithConnection 记录包含连接信息的错误日志
func (dl *DatabaseLogger) ErrorWithConnection(message string, connInfo *ConnectionInfo, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntryWithConnection(LogLevelError, message, detail, connInfo)
}

// FatalWithConnection 记录包含连接信息的致命错误日志
func (dl *DatabaseLogger) FatalWithConnection(message string, connInfo *ConnectionInfo, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	dl.addEntryWithConnection(LogLevelFatal, message, detail, connInfo)
	os.Exit(1)
}

// GetEntries 获取所有日志条目
func (dl *DatabaseLogger) GetEntries() []LogEntry {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	
	// 返回副本以避免并发问题
	entries := make([]LogEntry, len(dl.entries))
	copy(entries, dl.entries)
	return entries
}

// GetRecentEntries 获取最近的N条日志
func (dl *DatabaseLogger) GetRecentEntries(n int) []LogEntry {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	
	if n <= 0 || len(dl.entries) == 0 {
		return []LogEntry{}
	}
	
	start := len(dl.entries) - n
	if start < 0 {
		start = 0
	}
	
	entries := make([]LogEntry, len(dl.entries[start:]))
	copy(entries, dl.entries[start:])
	return entries
}

// Clear 清空日志
func (dl *DatabaseLogger) Clear() {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.entries = make([]LogEntry, 0)
	dl.lastEntry = nil
}

// GetSummary 获取日志摘要
func (dl *DatabaseLogger) GetSummary() map[string]interface{} {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	
	summary := map[string]interface{}{
		"total_entries": len(dl.entries),
		"level_counts":  make(map[string]int),
		"last_entry":    nil,
	}
	
	levelCounts := make(map[LogLevel]int)
	for _, entry := range dl.entries {
		levelCounts[entry.Level]++
	}
	
	// 转换为英文字符串键（与前端保持一致）
	for level, count := range levelCounts {
		var levelKey string
		switch level {
		case LogLevelDebug:
			levelKey = "debug"
		case LogLevelInfo:
			levelKey = "info"
		case LogLevelWarn:
			levelKey = "warn"
		case LogLevelError:
			levelKey = "error"
		case LogLevelFatal:
			levelKey = "fatal"
		default:
			levelKey = "unknown"
		}
		summary["level_counts"].(map[string]int)[levelKey] = count
	}
	
	if len(dl.entries) > 0 {
		lastEntry := dl.entries[len(dl.entries)-1]
		summary["last_entry"] = map[string]interface{}{
			"level":     dl.getLevelString(lastEntry.Level),
			"message":   lastEntry.Message,
			"timestamp": lastEntry.Timestamp.Format("2006-01-02 15:04:05"),
			"count":     lastEntry.Count,
		}
	}
	
	return summary
}

// ReconnectionLogger 重连专用日志记录器
type ReconnectionLogger struct {
	logger *DatabaseLogger
	startTime time.Time
	lastProgressTime time.Time
	progressInterval time.Duration
}

// NewReconnectionLogger 创建重连日志记录器
func NewReconnectionLogger() *ReconnectionLogger {
	return &ReconnectionLogger{
		logger: GetDatabaseLogger(),
		progressInterval: 30 * time.Second, // 每30秒报告一次进度
	}
}

// StartReconnection 开始重连
func (rl *ReconnectionLogger) StartReconnection() {
	rl.startTime = time.Now()
	rl.lastProgressTime = rl.startTime
	rl.logger.Info("🔄 开始数据库重连程序")
}

// LogRetry 记录重试信息
func (rl *ReconnectionLogger) LogRetry(retryCount int, nextDelay time.Duration, lastError error) {
	now := time.Now()
	
	// 只在特定条件下输出详细信息
	shouldLog := false
	message := ""
	details := ""
	
	switch {
	case retryCount == 1:
		// 第一次重试总是记录
		shouldLog = true
		message = "开始第一次重连尝试"
		
	case retryCount <= 3:
		// 前3次重试记录简要信息
		shouldLog = true
		message = fmt.Sprintf("第 %d 次重连尝试", retryCount)
		
	case retryCount%10 == 0:
		// 每10次重试记录一次详细信息
		shouldLog = true
		elapsed := now.Sub(rl.startTime)
		message = fmt.Sprintf("重连进行中 - 第 %d 次尝试", retryCount)
		details = fmt.Sprintf("已耗时: %v, 下次尝试间隔: %v", elapsed.Round(time.Second), nextDelay)
		if lastError != nil {
			details += fmt.Sprintf(", 最后错误: %v", lastError)
		}
		
	case now.Sub(rl.lastProgressTime) >= rl.progressInterval:
		// 基于时间间隔的进度报告
		shouldLog = true
		elapsed := now.Sub(rl.startTime)
		message = fmt.Sprintf("重连进度更新 - 第 %d 次尝试", retryCount)
		details = fmt.Sprintf("已耗时: %v", elapsed.Round(time.Second))
		rl.lastProgressTime = now
	}
	
	if shouldLog {
		if len(details) > 0 {
			rl.logger.Warn(message, details)
		} else {
			rl.logger.Info(message)
		}
	}
}

// LogSuccess 记录重连成功
func (rl *ReconnectionLogger) LogSuccess(totalRetries int) {
	elapsed := time.Since(rl.startTime)
	message := "✅ 数据库重连成功"
	details := fmt.Sprintf("总计重试: %d 次, 耗时: %v", totalRetries, elapsed.Round(time.Second))
	rl.logger.Info(message, details)
}

// LogFailure 记录重连失败
func (rl *ReconnectionLogger) LogFailure(totalRetries int, finalError error) {
	elapsed := time.Since(rl.startTime)
	message := "❌ 数据库重连最终失败"
	details := fmt.Sprintf("总计重试: %d 次, 耗时: %v, 最终错误: %v", 
		totalRetries, elapsed.Round(time.Second), finalError)
	rl.logger.Error(message, details)
}