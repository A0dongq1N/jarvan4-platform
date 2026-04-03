package report

import (
	"context"
	"time"

	domainExec "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/execution"
	domainReport "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/report"
)

// FullReport 聚合了报告、时序数据、接口级指标及关联的执行记录
type FullReport struct {
	Report     *domainReport.Report
	Points     []*domainReport.MetricPoint
	APIMetrics []*domainReport.APIMetrics
	// 来自 task_run 的字段
	TaskName        string
	StartTime       *time.Time
	EndTime         *time.Time
	ScriptSnapshots []domainExec.ScriptSnapshot
	WorkerSnapshots []domainExec.WorkerSnapshot
}

// ReportUseCase 报告查询入站端口
type ReportUseCase interface {
	GetReport(ctx context.Context, id string) (*domainReport.Report, error)
	GetFullReport(ctx context.Context, id string) (*FullReport, error)
	ListReports(ctx context.Context, projectID string, page, pageSize int) ([]*domainReport.Report, int64, error)
	GetTimeSeries(ctx context.Context, runID string, startTs, endTs int64) ([]*domainReport.MetricPoint, error)
	GetAPIMetrics(ctx context.Context, reportID string) ([]*domainReport.APIMetrics, error)
	DeleteReport(ctx context.Context, id, operatedBy string) error
}

var _ ReportUseCase = (*Service)(nil)
