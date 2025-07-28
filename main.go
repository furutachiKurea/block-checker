package main

import (
	"log"

	"github.com/furutachiKurea/block-checker/database"
	"github.com/furutachiKurea/block-checker/handlers"

	"github.com/labstack/echo/v4"
)

func main() {
	// 初始化数据库连接
	if err := database.InitDB(); err != nil {
		log.Printf("Failed to initialize database: %v", err)
		// 不退出应用，继续运行
	}
	defer database.CloseDB()

	// 创建 Echo 实例
	e := echo.New()

	// 注册路由
	e.GET("/", handlers.HomeHandler)
	e.GET("/healthz", handlers.HealthHandler)

	// 数据库浏览路由
	e.GET("/databases", handlers.DatabasesHandler)
	e.GET("/databases/:database/tables", handlers.TablesHandler)

	// API 路由
	e.GET("/api/databases", handlers.APIDatabasesHandler)
	e.GET("/api/databases/:database/tables", handlers.APITablesHandler)

	// 启动服务器
	serverAddr := "0.0.0.0:8080"
	log.Printf("Starting server on %s", serverAddr)
	if err := e.Start(serverAddr); err != nil {
		log.Printf("Server error: %v", err)
	}
}
