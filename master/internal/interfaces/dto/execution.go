package dto

// ── 执行 ──────────────────────────────────────────────────────────────────

type StartExecutionReq struct {
	TaskID string `json:"taskId"`
}

// ExecutionStateResp 对应前端 ExecutionState 结构
type ExecutionStateResp struct {
	ID              string               `json:"id"`
	TaskID          string               `json:"taskId"`
	TaskName        string               `json:"taskName"`
	Status          string               `json:"status"`   // pending/running/success/stopped/failed/circuit_broken
	StartTime       *string              `json:"startTime,omitempty"`
	EndTime         *string              `json:"endTime,omitempty"`
	ElapsedSeconds  int64                `json:"elapsedSeconds"`
	ScenarioMode    string               `json:"scenarioMode,omitempty"` // step/rps
	TargetRps       int                  `json:"targetRps,omitempty"`
	ScriptSnapshots []ScriptSnapshotResp `json:"scriptSnapshots,omitempty"`
	InitSteps       []InitStepResp       `json:"initSteps,omitempty"`
	ReportID        string               `json:"reportId,omitempty"`
	WarningMsg      string               `json:"warningMsg,omitempty"`
	ErrorMsg        string               `json:"errorMsg,omitempty"`
}

type ScriptSnapshotResp struct {
	ScriptID    string `json:"scriptId"`
	ScriptName  string `json:"scriptName"`
	CommitHash  string `json:"commitHash"`
	ArtifactURL string `json:"artifactUrl"`
	Weight      int    `json:"weight"`
}

type InitStepResp struct {
	Key    string   `json:"key"`
	Label  string   `json:"label"`
	Status string   `json:"status"` // waiting/running/done/error
	Detail string   `json:"detail,omitempty"`
	Items  []string `json:"items,omitempty"`
}

// ExecutionRecordResp 对应前端 ExecutionRecord（历史列表条目）
type ExecutionRecordResp struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"taskId"`
	Status          string  `json:"status"`
	TriggerType     int     `json:"triggerType"`     // 1=手动 2=定时
	TriggeredByName string  `json:"triggeredByName"`
	StartTime       *string `json:"startTime,omitempty"`
	EndTime         *string `json:"endTime,omitempty"`
	DurationSec     *int64  `json:"durationSec,omitempty"`
	ErrorMsg        string  `json:"errorMsg,omitempty"`
	ReportID        string  `json:"reportId,omitempty"`
}

// MetricsSummaryResp 对应前端 MetricsSummary
type MetricsSummaryResp struct {
	RPS             float64 `json:"rps"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	P99ResponseTime float64 `json:"p99ResponseTime"`
	ErrorRate       float64 `json:"errorRate"`
	TotalRequests   int64   `json:"totalRequests"`
	SuccessRequests int64   `json:"successRequests"`
	FailedRequests  int64   `json:"failedRequests"`
	Concurrent      int     `json:"concurrent"`
}
