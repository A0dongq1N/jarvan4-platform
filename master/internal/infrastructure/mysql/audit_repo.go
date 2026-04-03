package mysql

import (
	"context"
	"strconv"
	"time"

	domainAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/mysql/model"
	"gorm.io/gorm"
)

// AuditLogRepo 实现 domain/audit.AuditLogRepo
type AuditLogRepo struct{ db *gorm.DB }

func NewAuditLogRepo(db *gorm.DB) *AuditLogRepo { return &AuditLogRepo{db: db} }

func toAuditLogModel(l *domainAudit.AuditLog) *model.AuditLogModel {
	return &model.AuditLogModel{
		UserID:       l.UserID,
		Username:     l.Username,
		Action:       string(l.Action),
		ResourceType: string(l.ResourceType),
		ResourceID:   l.ResourceID,
		ResourceName: l.ResourceName,
		Detail:       l.Detail,
		IP:           l.IP,
		CreatedAt:    l.CreatedAt,
	}
}

func toAuditLogDomain(m *model.AuditLogModel) *domainAudit.AuditLog {
	return &domainAudit.AuditLog{
		ID:           strconv.FormatUint(m.ID, 10),
		UserID:       m.UserID,
		Username:     m.Username,
		Action:       domainAudit.AuditAction(m.Action),
		ResourceType: domainAudit.ResourceType(m.ResourceType),
		ResourceID:   m.ResourceID,
		ResourceName: m.ResourceName,
		Detail:       m.Detail,
		IP:           m.IP,
		CreatedAt:    m.CreatedAt,
	}
}

// Save 审计日志只插入（append-only）。
func (r *AuditLogRepo) Save(ctx context.Context, l *domainAudit.AuditLog) error {
	m := toAuditLogModel(l)
	return r.db.WithContext(ctx).Create(m).Error
}

// List 分页查询审计日志，按 created_at DESC；过滤条件为空则不添加。
func (r *AuditLogRepo) List(ctx context.Context, filter domainAudit.ListFilter, page, pageSize int) ([]*domainAudit.AuditLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.AuditLogModel{})

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		q = q.Where("username LIKE ? OR resource_name LIKE ? OR detail LIKE ?", like, like, like)
	}
	if filter.UserID != "" {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		q = q.Where("action = ?", string(filter.Action))
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", string(filter.ResourceType))
	}
	if filter.StartTime > 0 {
		q = q.Where("created_at >= ?", time.Unix(filter.StartTime, 0))
	}
	if filter.EndTime > 0 {
		q = q.Where("created_at <= ?", time.Unix(filter.EndTime, 0))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	var ms []model.AuditLogModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ms).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*domainAudit.AuditLog, 0, len(ms))
	for i := range ms {
		result = append(result, toAuditLogDomain(&ms[i]))
	}
	return result, total, nil
}
