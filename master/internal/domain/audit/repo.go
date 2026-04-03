package audit

import "context"

// AuditLogRepo 审计日志仓储接口
type AuditLogRepo interface {
	Save(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, filter ListFilter, page, pageSize int) ([]*AuditLog, int64, error)
}

// ListFilter 审计日志查询过滤条件
type ListFilter struct {
	Keyword      string // 模糊搜索：username / resource_name / detail
	UserID       string
	Action       AuditAction
	ResourceType ResourceType
	StartTime    int64 // Unix 秒
	EndTime      int64
}
