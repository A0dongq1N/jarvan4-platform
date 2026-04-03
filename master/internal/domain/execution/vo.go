package execution

import "github.com/Aodongq1n/jarvan4-platform/shared/constant"

// WorkerSnapshot 压测启动时选定 Worker 的不可变快照
type WorkerSnapshot struct {
	WorkerID string
	Addr     string // host:port
	CPUCores int
	MemGB    float64
}

// ScriptSnapshot 压测启动时脚本版本的不可变快照
type ScriptSnapshot struct {
	ScriptID    string
	ScriptName  string
	CommitHash  string
	ArtifactURL string
	Weight      int
}

// InitStep 初始化阶段的单步状态（通过 UpdateInitStep 受控更新）
type InitStep struct {
	Key    string
	Label  string
	Status constant.InitStepStatus
	Detail string
	Items  []string // 结构化列表（worker 节点、脚本版本等）
}
