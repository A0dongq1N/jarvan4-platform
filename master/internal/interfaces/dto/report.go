package dto

// ── 报告 ──────────────────────────────────────────────────────────────────

// ReportSummaryResp 列表页轻量摘要（不含时序和分位数）
type ReportSummaryResp struct {
	ID        string             `json:"id"`
	TaskID    string             `json:"taskId"`
	TaskName  string             `json:"taskName"`
	RunID     string             `json:"executionId"`
	Status    string             `json:"status"`
	StartTime string             `json:"startTime"`
	EndTime   string             `json:"endTime"`
	Duration  int                `json:"duration"`
	Summary   MetricsSummaryResp `json:"summary"`
	CreatedAt string             `json:"createdAt"`
}

// ReportResp 详情页完整报告（含时序、分位数、错误分析）
type ReportResp struct {
	ID               string              `json:"id"`
	TaskID           string              `json:"taskId"`
	TaskName         string              `json:"taskName"`
	RunID            string              `json:"executionId"`
	Status           string              `json:"status"`
	StartTime        string              `json:"startTime"`
	EndTime          string              `json:"endTime"`
	Duration         int                 `json:"duration"`
	Summary          MetricsSummaryResp  `json:"summary"`
	RpsData          []MetricPointResp   `json:"rpsData"`
	ResponseTimeData []MetricPointResp   `json:"responseTimeData"`
	ErrorRateData    []MetricPointResp   `json:"errorRateData"`
	ConcurrentData   []MetricPointResp   `json:"concurrentData"`
	Percentiles      []PercentileResp    `json:"percentiles"`
	Errors           []ErrorDataResp     `json:"errors"`
	CreatedAt        string              `json:"createdAt"`
	ScenarioMode     string              `json:"scenarioMode,omitempty"`
	TargetRps        int                 `json:"targetRps,omitempty"`
	ScriptSnapshots  []ScriptSnapshotResp `json:"scriptSnapshots,omitempty"`
	WorkerSnapshots  []WorkerSnapshotResp `json:"workerSnapshots,omitempty"`
}

type MetricPointResp struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// PercentileResp 接口级分位数指标
type PercentileResp struct {
	API      string  `json:"api"`
	Requests int64   `json:"requests"`
	Errors   int64   `json:"errors"`
	ErrorRate float64 `json:"errorRate"`
	P50      float64 `json:"p50"`
	P75      float64 `json:"p75"`
	P90      float64 `json:"p90"`
	P95      float64 `json:"p95"`
	P99      float64 `json:"p99"`
	Max      float64 `json:"max"`
	Min      float64 `json:"min"`
}

// ErrorDataResp 错误分析
type ErrorDataResp struct {
	Code      string  `json:"code"`
	Message   string  `json:"message"`
	ErrorType string  `json:"errorType"` // "business" | "system"
	Count     int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// WorkerSnapshotResp Worker 快照
type WorkerSnapshotResp struct {
	WorkerID       string `json:"workerId"`
	Hostname       string `json:"hostname"`
	IP             string `json:"ip"`
	CPUCores       int    `json:"cpuCores"`
	MaxConcurrency int    `json:"maxConcurrency"`
}
