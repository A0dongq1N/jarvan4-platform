package model

import "time"

// ReportModel GORM model for `report` table
type ReportModel struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	BizID       string     `gorm:"column:biz_id;type:varchar(64);not null;uniqueIndex"`
	RunBizID    string     `gorm:"column:run_id;type:varchar(64);not null;uniqueIndex"`
	TaskBizID   string     `gorm:"column:task_id;type:varchar(64);not null;index"`
	ProjectID   string     `gorm:"column:project_id;type:varchar(64);not null"`
	Name        string     `gorm:"column:name;type:varchar(256);not null"`
	SummaryJSON string     `gorm:"column:summary_json;type:json"`
	StartTime   *time.Time `gorm:"column:start_time"`
	EndTime     *time.Time `gorm:"column:end_time"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (ReportModel) TableName() string { return "report" }

// MetricPointModel GORM model for `report_metrics` table（时序数据）
type MetricPointModel struct {
	ID         uint64  `gorm:"primaryKey;autoIncrement"`
	RunBizID   string  `gorm:"column:run_id;type:varchar(64);not null;index"`
	Timestamp  int64   `gorm:"column:ts;not null"`
	TotalReqs  int64   `gorm:"column:total_reqs"`
	FailReqs   int64   `gorm:"column:fail_reqs"`
	QPS        float64 `gorm:"column:qps"`
	AvgRT      float64 `gorm:"column:avg_rt_ms"`
	P95RT      float64 `gorm:"column:p95_rt_ms"`
	P99RT      float64 `gorm:"column:p99_rt_ms"`
	FailRate   float64 `gorm:"column:fail_rate"`
	Concurrent int     `gorm:"column:concurrent"`
}

func (MetricPointModel) TableName() string { return "report_metrics" }

// APIMetricsModel GORM model for `report_api_metrics` table
type APIMetricsModel struct {
	ID          uint64  `gorm:"primaryKey;autoIncrement"`
	ReportBizID string  `gorm:"column:report_id;type:varchar(64);not null;index"`
	RunBizID    string  `gorm:"column:run_id;type:varchar(64);not null"`
	Label       string  `gorm:"column:label;type:varchar(256);not null"`
	TotalReqs   int64   `gorm:"column:total_reqs"`
	FailReqs    int64   `gorm:"column:fail_reqs"`
	AvgRT       float64 `gorm:"column:avg_rt_ms"`
	P50RT       float64 `gorm:"column:p50_rt_ms"`
	P90RT       float64 `gorm:"column:p90_rt_ms"`
	P95RT       float64 `gorm:"column:p95_rt_ms"`
	P99RT       float64 `gorm:"column:p99_rt_ms"`
	MaxRT       float64 `gorm:"column:max_rt_ms"`
	MinRT       float64 `gorm:"column:min_rt_ms"`
}

func (APIMetricsModel) TableName() string { return "report_api_metrics" }
