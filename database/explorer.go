package database

import (
	"fmt"
	"log"
	"strings"
)

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Name string `json:"name"`
}

// TableInfo 表信息
type TableInfo struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Rows    int64  `json:"rows"`
	Size    string `json:"size"`
}

// GetDatabases 获取数据库列表
func GetDatabases() ([]DatabaseInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// 检查连接状态
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("check connection: %v", err)
	}

	var databases []DatabaseInfo

	query := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA"

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query databases: %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Failed to close rows: %v", closeErr)
		}
	}()

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			continue
		}

		// 过滤系统数据库
		if !isSystemDatabase(dbName) {
			databases = append(databases, DatabaseInfo{Name: dbName})
		}
	}

	return databases, nil
}

// GetTables 获取指定数据库的表列表
func GetTables(databaseName string) ([]TableInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// 检查连接状态
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("check connection: %v", err)
	}

	var tables []TableInfo

	query := `
		SELECT 
			t.TABLE_NAME,
			COALESCE(t.TABLE_COMMENT, '') as comment,
			COALESCE(t.TABLE_ROWS, 0) as "rows",
			COALESCE(CONCAT(ROUND(((t.DATA_LENGTH + t.INDEX_LENGTH) / 1024 / 1024), 2), ' MB'), '0 MB') as size
		FROM information_schema.TABLES t
		WHERE t.TABLE_SCHEMA = ?
		AND t.TABLE_TYPE = 'BASE TABLE'
		ORDER BY t.TABLE_NAME`

	rows, err := db.Query(query, databaseName)
	if err != nil {
		return nil, fmt.Errorf("query tables: %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Failed to close rows: %v", closeErr)
		}
	}()

	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name, &table.Comment, &table.Rows, &table.Size); err != nil {
			continue
		}
		tables = append(tables, table)
	}

	return tables, nil
}

// isSystemDatabase 判断是否为系统数据库
func isSystemDatabase(dbName string) bool {
	systemDBs := []string{"information_schema", "mysql", "performance_schema", "sys"}
	for _, sysDB := range systemDBs {
		if strings.EqualFold(dbName, sysDB) {
			return true
		}
	}
	return false
}
