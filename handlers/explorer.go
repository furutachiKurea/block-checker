package handlers

import (
	"net/http"
	"strings"

	"github.com/furutachiKurea/block-checker/database"
	"github.com/furutachiKurea/block-checker/templates"

	"github.com/labstack/echo/v4"
)

// DatabasesHandler 数据库列表处理器
func DatabasesHandler(c echo.Context) error {
	databases, err := database.GetDatabases()
	if err != nil {
		// 检查是否是连接问题
		if strings.Contains(err.Error(), "connection failed") {
			reconnector := database.GetReconnector()
			if reconnector.IsReconnecting() {
				data := templates.ErrorData{
					Title:   "数据库重连中",
					Message: "正在尝试重新连接数据库，请稍后再试",
				}
				html, _ := templates.RenderError(data)
				return c.HTML(http.StatusServiceUnavailable, html)
			}

			data := templates.ErrorData{
				Title:   "数据库未连接",
				Message: "请检查数据库连接配置或确保数据库服务正在运行",
			}
			html, _ := templates.RenderError(data)
			return c.HTML(http.StatusServiceUnavailable, html)
		}

		data := templates.ErrorData{
			Title:   "获取数据库列表失败",
			Message: err.Error(),
		}
		html, _ := templates.RenderError(data)
		return c.HTML(http.StatusInternalServerError, html)
	}

	// 转换数据库信息
	var dbInfos []templates.DatabaseInfo
	for _, db := range databases {
		// 获取数据库中的表数量
		tables, err := database.GetTables(db.Name)
		tableCount := 0
		if err == nil {
			tableCount = len(tables)
		}

		dbInfos = append(dbInfos, templates.DatabaseInfo{
			Name:       db.Name,
			TableCount: tableCount,
		})
	}

	data := templates.DatabasesData{
		Databases: dbInfos,
	}

	html, err := templates.RenderDatabases(data)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "模板渲染错误")
	}

	return c.HTML(http.StatusOK, html)
}

// TablesHandler 表列表处理器
func TablesHandler(c echo.Context) error {
	databaseName := c.Param("database")
	if databaseName == "" {
		data := templates.ErrorData{
			Title:   "参数错误",
			Message: "数据库名称不能为空",
		}
		html, _ := templates.RenderError(data)
		return c.HTML(http.StatusBadRequest, html)
	}

	tables, err := database.GetTables(databaseName)
	if err != nil {
		// 检查是否是连接问题
		if strings.Contains(err.Error(), "connection failed") {
			reconnector := database.GetReconnector()
			if reconnector.IsReconnecting() {
				data := templates.ErrorData{
					Title:   "数据库重连中",
					Message: "正在尝试重新连接数据库，请稍后再试",
				}
				html, _ := templates.RenderError(data)
				return c.HTML(http.StatusServiceUnavailable, html)
			}

			data := templates.ErrorData{
				Title:   "数据库未连接",
				Message: "请检查数据库连接配置或确保数据库服务正在运行",
			}
			html, _ := templates.RenderError(data)
			return c.HTML(http.StatusServiceUnavailable, html)
		}

		data := templates.ErrorData{
			Title:   "获取表列表失败",
			Message: err.Error(),
		}
		html, _ := templates.RenderError(data)
		return c.HTML(http.StatusInternalServerError, html)
	}

	// 转换表信息
	var tableInfos []templates.TableInfo
	for _, table := range tables {
		tableInfos = append(tableInfos, templates.TableInfo{
			Name:    table.Name,
			Comment: table.Comment,
			Rows:    table.Rows,
			Size:    table.Size,
		})
	}

	data := templates.TablesData{
		DatabaseName: databaseName,
		Tables:       tableInfos,
	}

	html, err := templates.RenderTables(data)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "模板渲染错误")
	}

	return c.HTML(http.StatusOK, html)
}

// TableDetailHandler 表结构详情处理器
func TableDetailHandler(c echo.Context) error {
	databaseName := c.Param("database")
	tableName := c.Param("table")
	if databaseName == "" || tableName == "" {
		data := templates.ErrorData{
			Title:   "参数错误",
			Message: "数据库名和表名不能为空",
		}
		html, _ := templates.RenderError(data)
		return c.HTML(http.StatusBadRequest, html)
	}

	detail, err := database.GetTableDetail(databaseName, tableName)
	if err != nil {
		data := templates.ErrorData{
			Title:   "获取表结构失败",
			Message: err.Error(),
		}
		html, _ := templates.RenderError(data)
		return c.HTML(http.StatusInternalServerError, html)
	}

	data := templates.TableDetailData{
		DatabaseName: databaseName,
		TableName:    tableName,
		Detail:       detail,
	}
	html, err := templates.RenderTableDetail(data)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "模板渲染错误")
	}
	return c.HTML(http.StatusOK, html)
}

// APIDatabasesHandler API 数据库列表处理器
func APIDatabasesHandler(c echo.Context) error {
	databases, err := database.GetDatabases()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"databases": databases,
	})
}

// APITablesHandler API 表列表处理器
func APITablesHandler(c echo.Context) error {
	databaseName := c.Param("database")
	if databaseName == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "数据库名称不能为空",
		})
	}

	tables, err := database.GetTables(databaseName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"database": databaseName,
		"tables":   tables,
	})
}
