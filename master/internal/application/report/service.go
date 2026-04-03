// Package report 报告用例服务（应用层）
package report

import (
	"context"
	"fmt"

	domainExec "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/execution"
	domainReport "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/report"
	domainTask "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/task"
)

// Service 报告用例服务
type Service struct {
	reportRepo      domainReport.ReportRepo
	metricPointRepo domainReport.MetricPointRepo
	apiMetricsRepo  domainReport.APIMetricsRepo
	runRepo         domainExec.TaskRunRepo
	taskRepo        domainTask.TaskRepo
}

// NewService 构造函数
func NewService(
	reportRepo domainReport.ReportRepo,
	metricPointRepo domainReport.MetricPointRepo,
	apiMetricsRepo domainReport.APIMetricsRepo,
	runRepo domainExec.TaskRunRepo,
	taskRepo domainTask.TaskRepo,
) *Service {
	return &Service{
		reportRepo:      reportRepo,
		metricPointRepo: metricPointRepo,
		apiMetricsRepo:  apiMetricsRepo,
		runRepo:         runRepo,
		taskRepo:        taskRepo,
	}
}

func (s *Service) GetReport(ctx context.Context, id string) (*domainReport.Report, error) {
	return s.reportRepo.FindByID(ctx, id)
}

// GetFullReport 聚合报告详情：report + 时序 + 接口级指标 + run 快照
func (s *Service) GetFullReport(ctx context.Context, id string) (*FullReport, error) {
	report, err := s.reportRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find report: %w", err)
	}

	// 时序数据（全量，无时间范围限制）
	points, err := s.metricPointRepo.FindTimeSeriesByRunID(ctx, report.RunID, 0, 0)
	if err != nil {
		points = nil // 查询失败不阻塞，返回空
	}

	// 接口级指标
	apiMetrics, err := s.apiMetricsRepo.ListByReportID(ctx, id)
	if err != nil {
		apiMetrics = nil
	}

	full := &FullReport{
		Report:     report,
		Points:     points,
		APIMetrics: apiMetrics,
	}

	// 从 task_run 获取时间、快照信息
	run, err := s.runRepo.FindByID(ctx, report.RunID)
	if err == nil {
		full.StartTime = run.StartTime()
		full.EndTime = run.EndTime()
		full.ScriptSnapshots = run.ScriptSnapshots()
		full.WorkerSnapshots = run.WorkerSnapshots()
	}

	// 从 task 获取任务名
	if task, err := s.taskRepo.FindByID(ctx, report.TaskID); err == nil {
		full.TaskName = task.Name()
	}

	return full, nil
}

func (s *Service) ListReports(ctx context.Context, projectID string, page, pageSize int) ([]*domainReport.Report, int64, error) {
	return s.reportRepo.ListByProjectID(ctx, projectID, page, pageSize)
}

func (s *Service) GetTimeSeries(ctx context.Context, runID string, startTs, endTs int64) ([]*domainReport.MetricPoint, error) {
	return s.metricPointRepo.FindTimeSeriesByRunID(ctx, runID, startTs, endTs)
}

func (s *Service) GetAPIMetrics(ctx context.Context, reportID string) ([]*domainReport.APIMetrics, error) {
	return s.apiMetricsRepo.ListByReportID(ctx, reportID)
}

func (s *Service) DeleteReport(ctx context.Context, id, operatedBy string) error {
	if _, err := s.reportRepo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("find report: %w", err)
	}
	if err := s.reportRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete report: %w", err)
	}
	return nil
}
