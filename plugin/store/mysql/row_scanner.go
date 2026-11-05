package sqldb

// rowScanner 统一单行和多行游标的扫描边界，由 SQL 驱动保留查询/解码失败。
type rowScanner interface {
	Scan(...interface{}) error
}
