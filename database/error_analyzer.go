package database

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrorPattern 错误模式
type ErrorPattern struct {
	Keywords    []string  `json:"keywords"`
	Type        ErrorType `json:"type"`
	Code        string    `json:"code"`
	Cause       string    `json:"cause"`
	Suggestion  string    `json:"suggestion"`
	Severity    int       `json:"severity"` // 1-5, 5最严重
}

// ErrorSummary 错误摘要
type ErrorSummary struct {
	Type          ErrorType         `json:"type"`
	Code          string            `json:"code"`
	Count         int               `json:"count"`
	FirstSeen     time.Time         `json:"first_seen"`
	LastSeen      time.Time         `json:"last_seen"`
	FrequencyData map[string]int    `json:"frequency_data"` // 按小时统计
	Examples      []string          `json:"examples"`
	Resolved      bool              `json:"resolved"`
}

// ErrorAnalyzer 错误分析器
type ErrorAnalyzer struct {
	mu             sync.RWMutex
	patterns       []ErrorPattern
	summaries      map[string]*ErrorSummary // key: type_code
	maxExamples    int
	logger         *DatabaseLogger
}

var (
	errorAnalyzer *ErrorAnalyzer
	analyzerOnce  sync.Once
)

// GetErrorAnalyzer 获取错误分析器实例
func GetErrorAnalyzer() *ErrorAnalyzer {
	analyzerOnce.Do(func() {
		errorAnalyzer = &ErrorAnalyzer{
			patterns:    initializeErrorPatterns(),
			summaries:   make(map[string]*ErrorSummary),
			maxExamples: 10,
			logger:      GetDatabaseLogger(),
		}
	})
	return errorAnalyzer
}

// initializeErrorPatterns 初始化错误模式
func initializeErrorPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			Keywords:   []string{"connection refused", "连接被拒绝"},
			Type:       ErrorTypeNetwork,
			Code:       "NET_001",
			Cause:      "数据库服务器拒绝连接",
			Suggestion: "检查数据库服务是否运行，端口是否正确，防火墙设置",
			Severity:   4,
		},
		{
			Keywords:   []string{"no route to host", "网络不可达"},
			Type:       ErrorTypeNetwork,
			Code:       "NET_002",
			Cause:      "网络路由问题",
			Suggestion: "检查网络连接，DNS解析，服务器IP地址",
			Severity:   4,
		},
		{
			Keywords:   []string{"timeout", "超时", "context deadline exceeded"},
			Type:       ErrorTypeTimeout,
			Code:       "TIME_001",
			Cause:      "连接或查询超时",
			Suggestion: "检查网络延迟，增加超时时间，优化查询性能",
			Severity:   3,
		},
		{
			Keywords:   []string{"access denied", "认证失败", "authentication failed"},
			Type:       ErrorTypeAuth,
			Code:       "AUTH_001",
			Cause:      "身份验证失败",
			Suggestion: "检查用户名密码，用户权限，主机访问权限",
			Severity:   5,
		},
		{
			Keywords:   []string{"unknown database", "database doesn't exist", "数据库不存在"},
			Type:       ErrorTypeConfig,
			Code:       "CFG_001",
			Cause:      "指定的数据库不存在",
			Suggestion: "确认数据库名称正确，检查数据库是否已创建",
			Severity:   4,
		},
		{
			Keywords:   []string{"too many connections", "连接数过多"},
			Type:       ErrorTypeNetwork,
			Code:       "NET_003",
			Cause:      "数据库连接数超过限制",
			Suggestion: "优化连接池配置，增加最大连接数，检查连接泄露",
			Severity:   3,
		},
		{
			Keywords:   []string{"disk full", "磁盘空间不足", "no space left"},
			Type:       ErrorTypeConfig,
			Code:       "CFG_003",
			Cause:      "磁盘空间不足",
			Suggestion: "清理磁盘空间，增加存储容量，配置日志轮转",
			Severity:   5,
		},
		{
			Keywords:   []string{"lock wait timeout", "锁等待超时"},
			Type:       ErrorTypeSQL,
			Code:       "SQL_002",
			Cause:      "事务锁等待超时",
			Suggestion: "优化事务逻辑，减少锁持有时间，检查死锁",
			Severity:   3,
		},
	}
}

// AnalyzeError 分析错误并更新统计
func (ea *ErrorAnalyzer) AnalyzeError(err error, retryCount int) *ErrorDetails {
	if err == nil {
		return nil
	}

	errorMsg := err.Error()
	now := time.Now()
	
	// 匹配错误模式
	pattern := ea.matchErrorPattern(errorMsg)
	
	details := &ErrorDetails{
		Type:       pattern.Type,
		Code:       pattern.Code,
		Message:    errorMsg,
		Cause:      pattern.Cause,
		Suggestion: ea.enhanceSuggestion(pattern, retryCount),
		Timestamp:  now.Format("2006-01-02 15:04:05"),
		RetryCount: retryCount,
	}

	// 更新错误统计
	ea.updateErrorSummary(details, errorMsg, now)
	
	// 记录到日志
	ea.logErrorAnalysis(details, pattern.Severity)

	return details
}

// matchErrorPattern 匹配错误模式
func (ea *ErrorAnalyzer) matchErrorPattern(errorMsg string) ErrorPattern {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	errorMsgLower := strings.ToLower(errorMsg)
	
	for _, pattern := range ea.patterns {
		for _, keyword := range pattern.Keywords {
			if strings.Contains(errorMsgLower, strings.ToLower(keyword)) {
				return pattern
			}
		}
	}
	
	// 默认未知错误模式
	return ErrorPattern{
		Type:       ErrorTypeUnknown,
		Code:       "UNK_001",
		Cause:      "未识别的错误类型",
		Suggestion: "请联系系统管理员并提供完整错误信息",
		Severity:   2,
	}
}

// enhanceSuggestion 增强建议
func (ea *ErrorAnalyzer) enhanceSuggestion(pattern ErrorPattern, retryCount int) string {
	suggestion := pattern.Suggestion
	
	if retryCount > 0 {
		switch pattern.Type {
		case ErrorTypeNetwork:
			if retryCount > 10 {
				suggestion += fmt.Sprintf(" | 已重试%d次，建议检查网络稳定性", retryCount)
			}
		case ErrorTypeTimeout:
			if retryCount > 5 {
				suggestion += fmt.Sprintf(" | 连续%d次超时，建议增加超时配置", retryCount)
			}
		case ErrorTypeAuth:
			suggestion += " | 认证错误通常不会通过重试解决，请立即检查配置"
		}
	}
	
	return suggestion
}

// updateErrorSummary 更新错误摘要
func (ea *ErrorAnalyzer) updateErrorSummary(details *ErrorDetails, errorMsg string, timestamp time.Time) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	key := fmt.Sprintf("%s_%s", details.Type, details.Code)
	
	summary, exists := ea.summaries[key]
	if !exists {
		summary = &ErrorSummary{
			Type:          details.Type,
			Code:          details.Code,
			Count:         0,
			FirstSeen:     timestamp,
			FrequencyData: make(map[string]int),
			Examples:      make([]string, 0),
		}
		ea.summaries[key] = summary
	}
	
	summary.Count++
	summary.LastSeen = timestamp
	
	// 按小时统计频率
	hourKey := timestamp.Format("2006-01-02-15")
	summary.FrequencyData[hourKey]++
	
	// 添加错误示例（避免重复）
	if len(summary.Examples) < ea.maxExamples {
		found := false
		for _, example := range summary.Examples {
			if example == errorMsg {
				found = true
				break
			}
		}
		if !found {
			summary.Examples = append(summary.Examples, errorMsg)
		}
	}
}

// logErrorAnalysis 记录错误分析日志
func (ea *ErrorAnalyzer) logErrorAnalysis(details *ErrorDetails, severity int) {
	message := fmt.Sprintf("错误分析: [%s] %s", details.Code, details.Type)
	logDetails := fmt.Sprintf("原因: %s | 建议: %s", details.Cause, details.Suggestion)
	
	switch severity {
	case 5:
		ea.logger.Error(message, logDetails)
	case 4:
		ea.logger.Warn(message, logDetails)
	case 3:
		ea.logger.Info(message, logDetails)
	default:
		ea.logger.Debug(message, logDetails)
	}
}

// GetErrorSummaries 获取错误摘要
func (ea *ErrorAnalyzer) GetErrorSummaries() map[string]*ErrorSummary {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	// 返回副本
	summaries := make(map[string]*ErrorSummary)
	for k, v := range ea.summaries {
		// 深拷贝
		summary := &ErrorSummary{
			Type:          v.Type,
			Code:          v.Code,
			Count:         v.Count,
			FirstSeen:     v.FirstSeen,
			LastSeen:      v.LastSeen,
			FrequencyData: make(map[string]int),
			Examples:      make([]string, len(v.Examples)),
			Resolved:      v.Resolved,
		}
		
		for fk, fv := range v.FrequencyData {
			summary.FrequencyData[fk] = fv
		}
		copy(summary.Examples, v.Examples)
		
		summaries[k] = summary
	}
	
	return summaries
}

// GetTopErrors 获取最频繁的错误
func (ea *ErrorAnalyzer) GetTopErrors(limit int) []*ErrorSummary {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	// 转换为切片并排序
	var summaries []*ErrorSummary
	for _, summary := range ea.summaries {
		summaries = append(summaries, summary)
	}
	
	// 简单的冒泡排序（按计数降序）
	for i := 0; i < len(summaries)-1; i++ {
		for j := 0; j < len(summaries)-i-1; j++ {
			if summaries[j].Count < summaries[j+1].Count {
				summaries[j], summaries[j+1] = summaries[j+1], summaries[j]
			}
		}
	}
	
	// 限制返回数量
	if limit > 0 && limit < len(summaries) {
		summaries = summaries[:limit]
	}
	
	return summaries
}

// MarkErrorResolved 标记错误已解决
func (ea *ErrorAnalyzer) MarkErrorResolved(errorType ErrorType, code string) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	key := fmt.Sprintf("%s_%s", errorType, code)
	if summary, exists := ea.summaries[key]; exists {
		summary.Resolved = true
		ea.logger.Info(fmt.Sprintf("错误已标记为解决: [%s] %s", code, errorType))
	}
}

// ClearOldErrors 清理旧错误记录
func (ea *ErrorAnalyzer) ClearOldErrors(olderThan time.Duration) int {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	cutoff := time.Now().Add(-olderThan)
	cleared := 0
	
	for key, summary := range ea.summaries {
		if summary.LastSeen.Before(cutoff) {
			delete(ea.summaries, key)
			cleared++
		}
	}
	
	if cleared > 0 {
		ea.logger.Info(fmt.Sprintf("清理了 %d 条旧错误记录", cleared))
	}
	
	return cleared
}

// GetErrorTrends 获取错误趋势分析
func (ea *ErrorAnalyzer) GetErrorTrends() map[string]interface{} {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	trends := map[string]interface{}{
		"total_errors": 0,
		"error_types":  make(map[string]int),
		"hourly_data":  make(map[string]int),
		"resolved_count": 0,
	}
	
	totalErrors := 0
	resolvedCount := 0
	errorTypes := make(map[string]int)
	hourlyData := make(map[string]int)
	
	for _, summary := range ea.summaries {
		totalErrors += summary.Count
		errorTypes[string(summary.Type)] += summary.Count
		
		if summary.Resolved {
			resolvedCount++
		}
		
		for hour, count := range summary.FrequencyData {
			hourlyData[hour] += count
		}
	}
	
	trends["total_errors"] = totalErrors
	trends["error_types"] = errorTypes
	trends["hourly_data"] = hourlyData
	trends["resolved_count"] = resolvedCount
	
	return trends
}