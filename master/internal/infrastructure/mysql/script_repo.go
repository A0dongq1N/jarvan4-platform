package mysql

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
	domainScript "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/script"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/mysql/model"
	"gorm.io/gorm"
)

// ── ScriptRepo ────────────────────────────────────────────────────────────────

// ScriptRepo 实现 domain/script.ScriptRepo
type ScriptRepo struct{ db *gorm.DB }

func NewScriptRepo(db *gorm.DB) *ScriptRepo { return &ScriptRepo{db: db} }

func toScriptModel(s *domainScript.Script) *model.ScriptModel {
	return &model.ScriptModel{
		BizID:       s.ID,
		ProjectID:   s.ProjectID,
		Name:        s.Name,
		Description: s.Description,
		Lang:        s.Lang,
		CommitHash:  s.CommitHash,
		ArtifactURL: s.ArtifactURL,
		CommitMsg:   s.CommitMsg,
		Author:      s.Author,
		Status:      s.Status,
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toScriptDomain(m *model.ScriptModel) *domainScript.Script {
	return &domainScript.Script{
		ID:          m.BizID,
		ProjectID:   m.ProjectID,
		Name:        m.Name,
		Description: m.Description,
		Lang:        m.Lang,
		CommitHash:  m.CommitHash,
		ArtifactURL: m.ArtifactURL,
		CommitMsg:   m.CommitMsg,
		Author:      m.Author,
		Status:      m.Status,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// Save 若 BizID 为空则生成新 UUID；先查库，未找到则 INSERT，已存在则 UPDATE。
func (r *ScriptRepo) Save(ctx context.Context, s *domainScript.Script) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	m := toScriptModel(s)
	var existing model.ScriptModel
	err := r.db.WithContext(ctx).Where("biz_id = ?", s.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(m).Error
	}
	if err != nil {
		return err
	}
	m.ID = existing.ID
	return r.db.WithContext(ctx).Save(m).Error
}

// FindByID 按 BizID 查询。未找到返回 domain.ErrNotFound。
func (r *ScriptRepo) FindByID(ctx context.Context, id string) (*domainScript.Script, error) {
	var m model.ScriptModel
	err := r.db.WithContext(ctx).Where("biz_id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toScriptDomain(&m), nil
}

// FindByName 按名称查询脚本（不区分 status，支持下线后重新发布）。未找到返回 domain.ErrNotFound。
func (r *ScriptRepo) FindByName(ctx context.Context, projectID, name string) (*domainScript.Script, error) {
	var m model.ScriptModel
	q := r.db.WithContext(ctx).Where("name = ?", name)
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	err := q.First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toScriptDomain(&m), nil
}

// ListByProjectID 分页查询脚本列表，projectID 为空时查全量在线脚本，按 created_at DESC。
func (r *ScriptRepo) ListByProjectID(ctx context.Context, projectID string, page, pageSize int) ([]*domainScript.Script, int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&model.ScriptModel{}).Where("status = 1")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	var ms []model.ScriptModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ms).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*domainScript.Script, 0, len(ms))
	for i := range ms {
		result = append(result, toScriptDomain(&ms[i]))
	}
	return result, total, nil
}

// Delete 软删除：将 status 置为 0。
func (r *ScriptRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&model.ScriptModel{}).
		Where("biz_id = ?", id).
		Update("status", 0).Error
}

// ── ScriptVersionRepo ─────────────────────────────────────────────────────────

// ScriptVersionRepo 实现 domain/script.ScriptVersionRepo
type ScriptVersionRepo struct{ db *gorm.DB }

func NewScriptVersionRepo(db *gorm.DB) *ScriptVersionRepo { return &ScriptVersionRepo{db: db} }

func toScriptVersionModel(v *domainScript.ScriptVersion) *model.ScriptVersionModel {
	return &model.ScriptVersionModel{
		BizID:       v.ID,
		ScriptBizID: v.ScriptID,
		CommitHash:  v.CommitHash,
		ArtifactURL: v.ArtifactURL,
		CommitMsg:   v.CommitMsg,
		Author:      v.Author,
		CreatedAt:   v.CreatedAt,
	}
}

func toScriptVersionDomain(m *model.ScriptVersionModel) *domainScript.ScriptVersion {
	return &domainScript.ScriptVersion{
		ID:          m.BizID,
		ScriptID:    m.ScriptBizID,
		CommitHash:  m.CommitHash,
		ArtifactURL: m.ArtifactURL,
		CommitMsg:   m.CommitMsg,
		Author:      m.Author,
		CreatedAt:   m.CreatedAt,
	}
}

// Save 版本只增不改；BizID 为空时自动生成。
func (r *ScriptVersionRepo) Save(ctx context.Context, v *domainScript.ScriptVersion) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	m := toScriptVersionModel(v)
	return r.db.WithContext(ctx).Create(m).Error
}

// ListByScriptID 分页查询某脚本的版本历史，按 created_at DESC。
func (r *ScriptVersionRepo) ListByScriptID(ctx context.Context, scriptID string, page, pageSize int) ([]*domainScript.ScriptVersion, int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&model.ScriptVersionModel{}).Where("script_id = ?", scriptID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	var ms []model.ScriptVersionModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ms).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*domainScript.ScriptVersion, 0, len(ms))
	for i := range ms {
		result = append(result, toScriptVersionDomain(&ms[i]))
	}
	return result, total, nil
}
