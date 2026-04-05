package worker

import (
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
	"github.com/Aodongq1n/jarvan4-platform/shared/constant"
	"time"
)

// WorkerNode Worker 节点实体（部分充血）
type WorkerNode struct {
	id             string // 业务 ID（Nacos instanceId）
	addr           string // host:port
	cpuCores       int
	memTotalGB     float64
	maxConcurrency int
	status         constant.WorkerStatus

	// 实时负载（心跳更新）
	cpuUsage     float64
	memUsage     float64
	concurrent   int
	runningRunID string

	registeredAt time.Time
	updatedAt    time.Time
}

// NewWorkerNode 工厂函数
func NewWorkerNode(id, addr string, cpuCores int, memTotalGB float64, maxConcurrency int) *WorkerNode {
	now := time.Now()
	return &WorkerNode{
		id:             id,
		addr:           addr,
		cpuCores:       cpuCores,
		memTotalGB:     memTotalGB,
		maxConcurrency: maxConcurrency,
		status:         constant.WorkerStatusOnline,
		registeredAt:   now,
		updatedAt:      now,
	}
}

// Reconstruct 从持久化重建
func Reconstruct(id, addr string, cpuCores int, memTotalGB float64, maxConcurrency int,
	status constant.WorkerStatus, cpuUsage, memUsage float64, concurrent int,
	runningRunID string, registeredAt, updatedAt time.Time) *WorkerNode {
	return &WorkerNode{
		id: id, addr: addr, cpuCores: cpuCores, memTotalGB: memTotalGB,
		maxConcurrency: maxConcurrency, status: status,
		cpuUsage: cpuUsage, memUsage: memUsage, concurrent: concurrent,
		runningRunID: runningRunID, registeredAt: registeredAt, updatedAt: updatedAt,
	}
}

// UpdateLoad 心跳时更新负载信息
func (w *WorkerNode) UpdateLoad(cpuUsage, memUsage float64, concurrent int, runningRunID string) {
	w.cpuUsage = cpuUsage
	w.memUsage = memUsage
	w.concurrent = concurrent
	w.runningRunID = runningRunID
	w.updatedAt = time.Now()
}

// Offline 下线节点
func (w *WorkerNode) Offline() error {
	if w.status == constant.WorkerStatusOffline {
		return domain.ErrInvalidStateTransition
	}
	w.status = constant.WorkerStatusOffline
	w.updatedAt = time.Now()
	return nil
}

// LoadScore 负载评分（越低越好），用于调度器排序选取
// 公式：cpuUsage×0.6 + memUsage×0.4
func (w *WorkerNode) LoadScore() float64 {
	return w.cpuUsage*0.6 + w.memUsage*0.4
}

// IsAvailable 是否可接新任务：online 且无正在执行的 run
func (w *WorkerNode) IsAvailable() bool {
	return w.status == constant.WorkerStatusOnline && w.runningRunID == ""
}

// Rejoin Worker 重新上线，更新地址、状态及节点规格
func (w *WorkerNode) Rejoin(addr string, cpuCores int, memTotalGB float64, maxConcurrency int) {
	w.addr = addr
	w.cpuCores = cpuCores
	w.memTotalGB = memTotalGB
	w.maxConcurrency = maxConcurrency
	w.status = constant.WorkerStatusOnline
	w.updatedAt = time.Now()
}

// ── 只读访问器 ────────────────────────────────────────────────────────────

func (w *WorkerNode) ID() string                    { return w.id }
func (w *WorkerNode) Addr() string                  { return w.addr }
func (w *WorkerNode) CPUCores() int                 { return w.cpuCores }
func (w *WorkerNode) MemTotalGB() float64           { return w.memTotalGB }
func (w *WorkerNode) MaxConcurrency() int           { return w.maxConcurrency }
func (w *WorkerNode) Status() constant.WorkerStatus { return w.status }
func (w *WorkerNode) CPUUsage() float64             { return w.cpuUsage }
func (w *WorkerNode) MemUsage() float64             { return w.memUsage }
func (w *WorkerNode) Concurrent() int               { return w.concurrent }
func (w *WorkerNode) RunningRunID() string          { return w.runningRunID }
func (w *WorkerNode) RegisteredAt() time.Time       { return w.registeredAt }
func (w *WorkerNode) UpdatedAt() time.Time          { return w.updatedAt }
