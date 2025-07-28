package handlers

import (
	"net/http"

	"github.com/furutachiKurea/block-checker/database"
	"github.com/furutachiKurea/block-checker/templates"

	"github.com/labstack/echo/v4"
)

// HomeHandler 主页处理器
func HomeHandler(c echo.Context) error {
	status := database.CheckStatus()

	statusClass := "status-ok"
	if status.Status == "Not Connected" {
		statusClass = "status-not-connected"
	} else if status.Status == "Reconnecting" {
		statusClass = "status-reconnecting"
	} else if status.Status != "OK" {
		statusClass = "status-failed"
	}

	data := templates.HomeData{
		Status:      status.Status,
		StatusClass: statusClass,
		Timestamp:   status.Timestamp,
		Error:       status.Error,
	}

	html, err := templates.RenderHome(data)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "模板渲染错误")
	}

	return c.HTML(http.StatusOK, html)
}

// HealthHandler 健康检查处理器
func HealthHandler(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}
