package handlers

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/furutachiKurea/block-checker/database"
	"github.com/labstack/echo/v4"
)

// LogLevel 日志级别映射
var logLevelMap = map[string]database.LogLevel{
	"debug": database.LogLevelDebug,
	"info":  database.LogLevelInfo,
	"warn":  database.LogLevelWarn,
	"error": database.LogLevelError,
	"fatal": database.LogLevelFatal,
}

// LogEntry 前端日志条目结构
type LogEntry struct {
	Level          string                        `json:"level"`
	Message        string                        `json:"message"`
	Timestamp      string                        `json:"timestamp"`
	Details        string                        `json:"details,omitempty"`
	Count          int                           `json:"count,omitempty"`
	ConnectionInfo *database.ConnectionInfo      `json:"connection_info,omitempty"`
}

// LogsPageHandler 日志页面处理器
func LogsPageHandler(c echo.Context) error {
	// 读取日志页面模板
	htmlContent, err := ioutil.ReadFile("templates/logs.html")
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "日志页面加载失败")
	}
	
	return c.HTML(http.StatusOK, string(htmlContent))
}

// LogSummaryResponse 日志摘要响应
type LogSummaryResponse struct {
	TotalEntries int            `json:"total_entries"`
	LevelCounts  map[string]int `json:"level_counts"`
	LastEntry    interface{}    `json:"last_entry"`
}

// ErrorSummaryResponse 错误摘要响应
type ErrorSummaryResponse struct {
	Type          string         `json:"type"`
	Code          string         `json:"code"`
	Count         int            `json:"count"`
	FirstSeen     string         `json:"first_seen"`
	LastSeen      string         `json:"last_seen"`
	FrequencyData map[string]int `json:"frequency_data"`
	Examples      []string       `json:"examples"`
	Resolved      bool           `json:"resolved"`
}

// GetLogsHandler 获取日志处理器
func GetLogsHandler(c echo.Context) error {
	logger := database.GetDatabaseLogger()
	
	// 获取查询参数
	limitStr := c.QueryParam("limit")
	limit := 50 // 默认限制
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	// 获取日志级别筛选参数
	levelFilter := c.QueryParam("level")
	var filterLevel *database.LogLevel
	if levelFilter != "" {
		if level, exists := logLevelMap[levelFilter]; exists {
			filterLevel = &level
		}
	}
	
	// 获取最近的日志条目
	entries := logger.GetRecentEntries(limit)
	
	// 按级别筛选
	var filteredEntries []database.LogEntry
	if filterLevel != nil {
		for _, entry := range entries {
			if entry.Level == *filterLevel {
				filteredEntries = append(filteredEntries, entry)
			}
		}
	} else {
		filteredEntries = entries
	}
	
	// 转换为前端格式
	var logEntries []LogEntry
	for _, entry := range filteredEntries {
		logEntries = append(logEntries, LogEntry{
			Level:          getLevelString(entry.Level),
			Message:        entry.Message,
			Timestamp:      entry.Timestamp.Format("2006-01-02 15:04:05"),
			Details:        entry.Details,
			Count:          entry.Count,
			ConnectionInfo: entry.ConnectionInfo,
		})
	}
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"logs":  logEntries,
		"count": len(logEntries),
	})
}

// GetLogSummaryHandler 获取日志摘要处理器
func GetLogSummaryHandler(c echo.Context) error {
	logger := database.GetDatabaseLogger()
	summary := logger.GetSummary()
	
	response := LogSummaryResponse{
		TotalEntries: summary["total_entries"].(int),
		LevelCounts:  summary["level_counts"].(map[string]int),
		LastEntry:    summary["last_entry"],
	}
	
	return c.JSON(http.StatusOK, response)
}

// SetLogLevelHandler 设置日志级别处理器
func SetLogLevelHandler(c echo.Context) error {
	levelStr := c.QueryParam("level")
	if levelStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing level parameter",
		})
	}
	
	level, exists := logLevelMap[levelStr]
	if !exists {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid log level",
		})
	}
	
	logger := database.GetDatabaseLogger()
	logger.SetLogLevel(level)
	
	return c.JSON(http.StatusOK, map[string]string{
		"message": "log level updated",
		"level":   levelStr,
	})
}

// ClearLogsHandler 清空日志处理器
func ClearLogsHandler(c echo.Context) error {
	logger := database.GetDatabaseLogger()
	logger.Clear()
	
	return c.JSON(http.StatusOK, map[string]string{
		"message": "logs cleared",
	})
}

// GetErrorSummariesHandler 获取错误摘要处理器
func GetErrorSummariesHandler(c echo.Context) error {
	analyzer := database.GetErrorAnalyzer()
	summaries := analyzer.GetErrorSummaries()
	
	var response []ErrorSummaryResponse
	for _, summary := range summaries {
		response = append(response, ErrorSummaryResponse{
			Type:          string(summary.Type),
			Code:          summary.Code,
			Count:         summary.Count,
			FirstSeen:     summary.FirstSeen.Format("2006-01-02 15:04:05"),
			LastSeen:      summary.LastSeen.Format("2006-01-02 15:04:05"),
			FrequencyData: summary.FrequencyData,
			Examples:      summary.Examples,
			Resolved:      summary.Resolved,
		})
	}
	
	return c.JSON(http.StatusOK, response)
}

// GetTopErrorsHandler 获取最频繁错误处理器
func GetTopErrorsHandler(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	limit := 10 // 默认前10
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	analyzer := database.GetErrorAnalyzer()
	topErrors := analyzer.GetTopErrors(limit)
	
	var response []ErrorSummaryResponse
	for _, summary := range topErrors {
		response = append(response, ErrorSummaryResponse{
			Type:          string(summary.Type),
			Code:          summary.Code,
			Count:         summary.Count,
			FirstSeen:     summary.FirstSeen.Format("2006-01-02 15:04:05"),
			LastSeen:      summary.LastSeen.Format("2006-01-02 15:04:05"),
			FrequencyData: summary.FrequencyData,
			Examples:      summary.Examples,
			Resolved:      summary.Resolved,
		})
	}
	
	return c.JSON(http.StatusOK, response)
}

// GetErrorTrendsHandler 获取错误趋势处理器
func GetErrorTrendsHandler(c echo.Context) error {
	analyzer := database.GetErrorAnalyzer()
	trends := analyzer.GetErrorTrends()
	
	return c.JSON(http.StatusOK, trends)
}

// MarkErrorResolvedHandler 标记错误已解决处理器
func MarkErrorResolvedHandler(c echo.Context) error {
	errorType := c.QueryParam("type")
	code := c.QueryParam("code")
	
	if errorType == "" || code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing type or code parameter",
		})
	}
	
	analyzer := database.GetErrorAnalyzer()
	analyzer.MarkErrorResolved(database.ErrorType(errorType), code)
	
	return c.JSON(http.StatusOK, map[string]string{
		"message": "error marked as resolved",
		"type":    errorType,
		"code":    code,
	})
}

// ClearOldErrorsHandler 清理旧错误处理器
func ClearOldErrorsHandler(c echo.Context) error {
	hoursStr := c.QueryParam("hours")
	hours := 24 // 默认24小时
	if hoursStr != "" {
		if parsedHours, err := strconv.Atoi(hoursStr); err == nil && parsedHours > 0 {
			hours = parsedHours
		}
	}
	
	analyzer := database.GetErrorAnalyzer()
	cleared := analyzer.ClearOldErrors(time.Duration(hours) * time.Hour)
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "old errors cleared",
		"cleared": cleared,
		"hours":   hours,
	})
}

// getLevelString 获取日志级别字符串
func getLevelString(level database.LogLevel) string {
	switch level {
	case database.LogLevelDebug:
		return "debug"
	case database.LogLevelInfo:
		return "info"
	case database.LogLevelWarn:
		return "warn"
	case database.LogLevelError:
		return "error"
	case database.LogLevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}