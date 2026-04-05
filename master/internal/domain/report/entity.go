// Package report 压测报告聚合
package report

import "time"

// Report 压测报告实体
type Report struct {
	ID        string
	RunID     string
	TaskID    string
	ProjectID string
	Name      string
	Summary   ReportSummary
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ReportSummary 报告摘要数据
type ReportSummary struct {
	Duration    int // 实际压测时长(秒)
	TotalReqs   int64
	FailReqs    int64
	AvgQPS      float64
	AvgRT       float64
	P50RT       float64
	P90RT       float64
	P95RT       float64
	P99RT       float64
	MaxRT       float64
	SuccessRate float64
}

// MetricPoint 时序指标点（用于图表）
type MetricPoint struct {
	RunID      string
	Timestamp  int64
	TotalReqs  int64
	FailReqs   int64
	QPS        float64
	AvgRT      float64
	P95RT      float64
	P99RT      float64
	FailRate   float64
	Concurrent int
}

// APIMetrics 接口级聚合指标
type APIMetrics struct {
	ReportID  string
	RunID     string
	Label     string
	TotalReqs int64
	FailReqs  int64
	AvgRT     float64
	P50RT     float64
	P90RT     float64
	P95RT     float64
	P99RT     float64
	MaxRT     float64
	MinRT     float64
}

// NewReport 报告工厂函数
func NewReport(runID, taskID, projectID, name string, summary ReportSummary) *Report {
	now := time.Now()
	return &Report{
		RunID:     runID,
		TaskID:    taskID,
		ProjectID: projectID,
		Name:      name,
		Summary:   summary,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
