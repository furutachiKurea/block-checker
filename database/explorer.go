// Package database 提供数据库元数据访问
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

// TableField 字段信息
type TableField struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	IsNullable bool    `json:"is_nullable"`
	IsPrimary  bool    `json:"is_primary"`
	Default    *string `json:"default"`
	Extra      string  `json:"extra"`
	Comment    string  `json:"comment"`
}

// TableIndex 索引信息
type TableIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// TableConstraint 约束信息
type TableConstraint struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Columns          []string `json:"columns"`
	ReferencedTable  *string  `json:"referenced_table,omitempty"`
	ReferencedColumn *string  `json:"referenced_column,omitempty"`
}

// TableDetail 表结构详情
type TableDetail struct {
	Fields      []TableField      `json:"fields"`
	Indexes     []TableIndex      `json:"indexes"`
	Constraints []TableConstraint `json:"constraints"`
}

// GetDatabases 获取数据库列表
func GetDatabases() ([]DatabaseInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
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

// GetTableDetail 获取表结构详细信息
func GetTableDetail(databaseName, tableName string) (*TableDetail, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("check connection: %v", err)
	}

	// 字段信息
	fieldQuery := `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA, COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`
	fieldRows, err := db.Query(fieldQuery, databaseName, tableName)
	if err != nil {
		return nil, fmt.Errorf("query fields: %v", err)
	}
	defer fieldRows.Close()

	var fields []TableField
	for fieldRows.Next() {
		var f TableField
		var isNullable, columnKey string
		if err := fieldRows.Scan(&f.Name, &f.Type, &isNullable, &columnKey, &f.Default, &f.Extra, &f.Comment); err != nil {
			continue
		}
		f.IsNullable = isNullable == "YES"
		f.IsPrimary = columnKey == "PRI"
		fields = append(fields, f)
	}

	// 索引信息
	indexQuery := `
		SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX), NON_UNIQUE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		GROUP BY INDEX_NAME, NON_UNIQUE
	`
	indexRows, err := db.Query(indexQuery, databaseName, tableName)
	if err != nil {
		return nil, fmt.Errorf("query indexes: %v", err)
	}
	defer indexRows.Close()

	var indexes []TableIndex
	for indexRows.Next() {
		var idx TableIndex
		var columns string
		var nonUnique int
		if err := indexRows.Scan(&idx.Name, &columns, &nonUnique); err != nil {
			continue
		}
		idx.Columns = strings.Split(columns, ",")
		idx.Unique = nonUnique == 0
		indexes = append(indexes, idx)
	}

	// 约束信息
	constraintQuery := `
		SELECT CONSTRAINT_NAME, CONSTRAINT_TYPE
		FROM information_schema.TABLE_CONSTRAINTS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`
	constraintRows, err := db.Query(constraintQuery, databaseName, tableName)
	if err != nil {
		return nil, fmt.Errorf("query constraints: %v", err)
	}
	defer constraintRows.Close()

	var constraints []TableConstraint
	for constraintRows.Next() {
		var c TableConstraint
		if err := constraintRows.Scan(&c.Name, &c.Type); err != nil {
			continue
		}
		// 获取约束涉及的字段
		colQuery := `
			SELECT COLUMN_NAME
			FROM information_schema.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = ?
			ORDER BY ORDINAL_POSITION
		`
		colRows, err := db.Query(colQuery, databaseName, tableName, c.Name)
		if err == nil {
			var cols []string
			for colRows.Next() {
				var col string
				if err := colRows.Scan(&col); err == nil {
					cols = append(cols, col)
				}
			}
			colRows.Close()
			c.Columns = cols
		}
		// 外键约束补充引用表和字段
		if c.Type == "FOREIGN KEY" {
			refQuery := `
				SELECT REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
				FROM information_schema.KEY_COLUMN_USAGE
				WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = ? LIMIT 1
			`
			refRow := db.QueryRow(refQuery, databaseName, tableName, c.Name)
			var refTable, refCol *string
			_ = refRow.Scan(&refTable, &refCol)
			c.ReferencedTable = refTable
			c.ReferencedColumn = refCol
		}
		constraints = append(constraints, c)
	}

	return &TableDetail{
		Fields:      fields,
		Indexes:     indexes,
		Constraints: constraints,
	}, nil
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