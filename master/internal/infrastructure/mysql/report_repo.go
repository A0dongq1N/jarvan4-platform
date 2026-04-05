package mysql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
	domainReport "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/report"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/mysql/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReportRepo 实现 domain/report.ReportRepo
type ReportRepo struct{ db *gorm.DB }

func NewReportRepo(db *gorm.DB) *ReportRepo { return &ReportRepo{db: db} }

func toReportModel(rp *domainReport.Report) (*model.ReportModel, error) {
	summaryJSON, err := json.Marshal(rp.Summary)
	if err != nil {
		return nil, err
	}
	return &model.ReportModel{
		BizID:       rp.ID,
		RunBizID:    rp.RunID,
		TaskBizID:   rp.TaskID,
		ProjectID:   rp.ProjectID,
		Name:        rp.Name,
		SummaryJSON: string(summaryJSON),
		StartTime:   rp.StartTime,
		EndTime:     rp.EndTime,
		CreatedAt:   rp.CreatedAt,
	}, nil
}

func toReportDomain(m *model.ReportModel) (*domainReport.Report, error) {
	var summary domainReport.ReportSummary
	if m.SummaryJSON != "" {
		if err := json.Unmarshal([]byte(m.SummaryJSON), &summary); err != nil {
			return nil, err
		}
	}
	return &domainReport.Report{
		ID:        m.BizID,
		RunID:     m.RunBizID,
		TaskID:    m.TaskBizID,
		ProjectID: m.ProjectID,
		Name:      m.Name,
		Summary:   summary,
		StartTime: m.StartTime,
		EndTime:   m.EndTime,
		CreatedAt: m.CreatedAt,
	}, nil
}

// Save 新增或更新 report
func (r *ReportRepo) Save(ctx context.Context, rp *domainReport.Report) error {
	if rp.ID == "" {
		rp.ID = uuid.NewString()
	}
	m, err := toReportModel(rp)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "biz_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "summary_json", "start_time", "end_time",
		}),
	}).Create(m).Error
}

// FindByID 按 BizID 查询
func (r *ReportRepo) FindByID(ctx context.Context, id string) (*domainReport.Report, error) {
	var m model.ReportModel
	if err := r.db.WithContext(ctx).Where("biz_id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toReportDomain(&m)
}

// FindByRunID 按 RunBizID 查询
func (r *ReportRepo) FindByRunID(ctx context.Context, runID string) (*domainReport.Report, error) {
	var m model.ReportModel
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toReportDomain(&m)
}

// ListByProjectID 分页查询，按 created_at DESC
func (r *ReportRepo) ListByProjectID(ctx context.Context, projectID string, page, pageSize int) ([]*domainReport.Report, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ReportModel{}).Where("project_id = ?", projectID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	var ms []model.ReportModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ms).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*domainReport.Report, 0, len(ms))
	for i := range ms {
		rp, err := toReportDomain(&ms[i])
		if err != nil {
			return nil, 0, err
		}
		result = append(result, rp)
	}
	return result, total, nil
}

// Delete 删除报告
func (r *ReportRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("biz_id = ?", id).Delete(&model.ReportModel{}).Error
}

// ── MetricPointRepo ──────────────────────────────────────────────────────

// MetricPointRepo 实现 domain/report.MetricPointRepo
type MetricPointRepo struct{ db *gorm.DB }

func NewMetricPointRepo(db *gorm.DB) *MetricPointRepo { return &MetricPointRepo{db: db} }

// BatchSave 批量写入时序指标点（100 条一批）
func (r *MetricPointRepo) BatchSave(ctx context.Context, points []*domainReport.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}
	ms := make([]model.MetricPointModel, 0, len(points))
	for _, p := range points {
		ms = append(ms, model.MetricPointModel{
			RunBizID:   p.RunID,
			Timestamp:  p.Timestamp,
			TotalReqs:  p.TotalReqs,
			FailReqs:   p.FailReqs,
			QPS:        p.QPS,
			AvgRT:      p.AvgRT,
			P95RT:      p.P95RT,
			P99RT:      p.P99RT,
			FailRate:   p.FailRate,
			Concurrent: p.Concurrent,
		})
	}
	return r.db.WithContext(ctx).CreateInBatches(ms, 100).Error
}

// FindTimeSeriesByRunID 按时间范围查询时序数据（升序）
func (r *MetricPointRepo) FindTimeSeriesByRunID(ctx context.Context, runID string, startTs, endTs int64) ([]*domainReport.MetricPoint, error) {
	q := r.db.WithContext(ctx).Where("run_id = ?", runID)
	if startTs > 0 {
		q = q.Where("ts >= ?", startTs)
	}
	if endTs > 0 {
		q = q.Where("ts <= ?", endTs)
	}
	var ms []model.MetricPointModel
	if err := q.Order("ts ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	result := make([]*domainReport.MetricPoint, 0, len(ms))
	for i := range ms {
		result = append(result, &domainReport.MetricPoint{
			RunID:      ms[i].RunBizID,
			Timestamp:  ms[i].Timestamp,
			TotalReqs:  ms[i].TotalReqs,
			FailReqs:   ms[i].FailReqs,
			QPS:        ms[i].QPS,
			AvgRT:      ms[i].AvgRT,
			P95RT:      ms[i].P95RT,
			P99RT:      ms[i].P99RT,
			FailRate:   ms[i].FailRate,
			Concurrent: ms[i].Concurrent,
		})
	}
	return result, nil
}

// ── APIMetricsRepo ───────────────────────────────────────────────────────

// APIMetricsRepo 实现 domain/report.APIMetricsRepo
type APIMetricsRepo struct{ db *gorm.DB }

func NewAPIMetricsRepo(db *gorm.DB) *APIMetricsRepo { return &APIMetricsRepo{db: db} }

// BatchSave 批量写入接口级指标（100 条一批）
func (r *APIMetricsRepo) BatchSave(ctx context.Context, metrics []*domainReport.APIMetrics) error {
	if len(metrics) == 0 {
		return nil
	}
	ms := make([]model.APIMetricsModel, 0, len(metrics))
	for _, a := range metrics {
		ms = append(ms, model.APIMetricsModel{
			ReportBizID: a.ReportID,
			RunBizID:    a.RunID,
			Label:       a.Label,
			TotalReqs:   a.TotalReqs,
			FailReqs:    a.FailReqs,
			AvgRT:       a.AvgRT,
			P50RT:       a.P50RT,
			P90RT:       a.P90RT,
			P95RT:       a.P95RT,
			P99RT:       a.P99RT,
			MaxRT:       a.MaxRT,
			MinRT:       a.MinRT,
		})
	}
	return r.db.WithContext(ctx).CreateInBatches(ms, 100).Error
}

// ListByReportID 查询报告下所有接口级指标
func (r *APIMetricsRepo) ListByReportID(ctx context.Context, reportID string) ([]*domainReport.APIMetrics, error) {
	var ms []model.APIMetricsModel
	if err := r.db.WithContext(ctx).Where("report_id = ?", reportID).Find(&ms).Error; err != nil {
		return nil, err
	}
	result := make([]*domainReport.APIMetrics, 0, len(ms))
	for i := range ms {
		result = append(result, &domainReport.APIMetrics{
			ReportID:  ms[i].ReportBizID,
			RunID:     ms[i].RunBizID,
			Label:     ms[i].Label,
			TotalReqs: ms[i].TotalReqs,
			FailReqs:  ms[i].FailReqs,
			AvgRT:     ms[i].AvgRT,
			P50RT:     ms[i].P50RT,
			P90RT:     ms[i].P90RT,
			P95RT:     ms[i].P95RT,
			P99RT:     ms[i].P99RT,
			MaxRT:     ms[i].MaxRT,
			MinRT:     ms[i].MinRT,
		})
	}
	return result, nil
}
