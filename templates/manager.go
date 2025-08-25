package templates

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed *.html
var templateFS embed.FS

var (
	homeTemplate      *template.Template
	errorTemplate     *template.Template
	databasesTemplate *template.Template
	tablesTemplate    *template.Template
)

var (
	tableDetailTemplate *template.Template
)

// 初始化模板
func init() {
	var err error

	// 加载主页模板
	homeTemplate, err = template.ParseFS(templateFS, "home.html")
	if err != nil {
		panic("failed to parse home template: " + err.Error())
	}

	// 加载错误模板
	errorTemplate, err = template.ParseFS(templateFS, "error.html")
	if err != nil {
		panic("failed to parse error template: " + err.Error())
	}

	// 加载数据库列表模板
	databasesTemplate, err = template.ParseFS(templateFS, "databases.html")
	if err != nil {
		panic("failed to parse databases template: " + err.Error())
	}

	// 加载表列表模板
	tablesTemplate, err = template.ParseFS(templateFS, "tables.html")
	if err != nil {
		panic("failed to parse tables template: " + err.Error())
	}
	// 加载表结构详情模板
	tableDetailTemplate, err = template.ParseFS(templateFS, "table_detail.html")
	if err != nil {
		panic("failed to parse table_detail template: " + err.Error())
	}
}

// HomeData 主页数据
type HomeData struct {
	Status      string
	StatusClass string
	Timestamp   string
	Error       string
}

// ErrorData 错误页面数据
type ErrorData struct {
	Title   string
	Message string
}

// DatabasesData 数据库列表数据
type DatabasesData struct {
	Databases []DatabaseInfo
}

// TablesData 表列表数据
type TablesData struct {
	DatabaseName string
	Tables       []TableInfo
}

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Name       string
	TableCount int
}

// TableInfo 表信息
type TableInfo struct {
	Name    string
	Comment string
	Rows    int64
	Size    string
}

type TableDetailData struct {
	DatabaseName string
	TableName    string
	Detail       interface{}
}

func RenderTableDetail(data TableDetailData) (string, error) {
	var buf bytes.Buffer
	err := tableDetailTemplate.Execute(&buf, data)
	return buf.String(), err
}

// RenderHome 渲染主页
func RenderHome(data HomeData) (string, error) {
	var buf bytes.Buffer
	err := homeTemplate.Execute(&buf, data)
	return buf.String(), err
}

// RenderError 渲染错误页面
func RenderError(data ErrorData) (string, error) {
	var buf bytes.Buffer
	err := errorTemplate.Execute(&buf, data)
	return buf.String(), err
}

// RenderDatabases 渲染数据库列表页面
func RenderDatabases(data DatabasesData) (string, error) {
	var buf bytes.Buffer
	err := databasesTemplate.Execute(&buf, data)
	return buf.String(), err
}

// RenderTables 渲染表列表页面
func RenderTables(data TablesData) (string, error) {
	var buf bytes.Buffer
	err := tablesTemplate.Execute(&buf, data)
	return buf.String(), err
}
