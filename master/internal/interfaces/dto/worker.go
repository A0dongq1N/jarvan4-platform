package dto

// ── Worker 节点 ───────────────────────────────────────────────────────────────

// WorkerNodeResp 对应前端 WorkerNode 结构
type WorkerNodeResp struct {
	ID                 string  `json:"id"`
	WorkerID           string  `json:"workerId"`
	Hostname           string  `json:"hostname"`
	IP                 string  `json:"ip"`
	Port               int     `json:"port"`
	Status             string  `json:"status"` // "online" | "busy" | "offline"
	CPUCores           int     `json:"cpuCores"`
	MemTotalGb         float64 `json:"memTotalGb"`
	MaxConcurrency     int     `json:"maxConcurrency"`
	CPUUsage           float64 `json:"cpuUsage"`
	MemUsage           float64 `json:"memUsage"`
	CurrentConcurrency int     `json:"currentConcurrency"`
	RunningRunID       string  `json:"runningRunId,omitempty"`
	RunningTaskName    string  `json:"runningTaskName,omitempty"`
	LastHeartbeat      string  `json:"lastHeartbeat"`
}

// WorkerHeartbeatReq Worker 心跳上报请求
type WorkerHeartbeatReq struct {
	CPUUsage   float64 `json:"cpuUsage"`
	MemUsage   float64 `json:"memUsage"`
	Concurrent int     `json:"concurrent"`
}
