// Package handler Worker tRPC 服务 handler（接收 Master 的 StartTask / StopTask 指令）
package handler

import (
	"context"
	"fmt"
	"sync"

	pbworker "github.com/Aodongq1n/jarvan4-platform/pb/worker"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/engine"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/loader"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/reporter"
)

// 编译时接口检查
var _ pbworker.WorkerServiceService = (*WorkerHandler)(nil)

// WorkerHandler tRPC handler 实现
type WorkerHandler struct {
	loader   *loader.ScriptLoader
	reporter *reporter.MetricsReporter

	mu      sync.Mutex
	runners map[string]*engine.Runner // runID → Runner
}

// New 创建 WorkerHandler
func New(l *loader.ScriptLoader, r *reporter.MetricsReporter) *WorkerHandler {
	return &WorkerHandler{
		loader:   l,
		reporter: r,
		runners:  make(map[string]*engine.Runner),
	}
}

// StartTask 接收 Master 下发的压测任务
// 1. 下载并加载脚本 .so
// 2. 创建 Runner，异步执行（立即返回）
func (h *WorkerHandler) StartTask(ctx context.Context, req *pbworker.StartTaskRequest) (*pbworker.Reply, error) {
	runID := req.GetRunId()
	fmt.Printf("[WorkerHandler] StartTask runID=%s taskID=%s scripts=%d\n",
		runID, req.GetTaskId(), len(req.GetScripts()))

	// 检查是否已在运行
	h.mu.Lock()
	if _, exists := h.runners[runID]; exists {
		h.mu.Unlock()
		return &pbworker.Reply{Code: 1, Message: "run already exists"}, nil
	}
	h.mu.Unlock()

	// 取第一个脚本加载（多脚本权重选择暂取权重最大的）
	scripts := req.GetScripts()
	if len(scripts) == 0 {
		return &pbworker.Reply{Code: 1, Message: "no scripts in request"}, nil
	}
	scriptRef := pickScript(scripts)

	// 下载并加载 .so（同步，通常 < 1s，因为有本地缓存）
	entry, err := h.loader.Load(ctx, scriptRef.GetArtifactUrl())
	if err != nil {
		errMsg := fmt.Sprintf("load script %s: %v", scriptRef.GetArtifactUrl(), err)
		fmt.Printf("[WorkerHandler] %s\n", errMsg)
		return &pbworker.Reply{Code: 1, Message: errMsg}, nil
	}

	// 创建 Runner 并异步执行
	r := engine.NewRunner(req, entry, h.reporter)
	h.mu.Lock()
	h.runners[runID] = r
	h.mu.Unlock()

	go func() {
		r.Run(context.Background())
		// 压测结束后从 map 中清除
		h.mu.Lock()
		delete(h.runners, runID)
		h.mu.Unlock()
		fmt.Printf("[WorkerHandler] runner %s finished\n", runID)
	}()

	return &pbworker.Reply{Code: 0, Message: "ok"}, nil
}

// StopTask 通知 Worker 停止指定任务
func (h *WorkerHandler) StopTask(ctx context.Context, req *pbworker.StopTaskRequest) (*pbworker.Reply, error) {
	runID := req.GetRunId()
	fmt.Printf("[WorkerHandler] StopTask runID=%s\n", runID)

	h.mu.Lock()
	r, ok := h.runners[runID]
	h.mu.Unlock()

	if !ok {
		return &pbworker.Reply{Code: 1, Message: "run not found"}, nil
	}

	r.Stop()
	return &pbworker.Reply{Code: 0, Message: "ok"}, nil
}

// pickScript 按权重选取脚本（取权重最大的，多脚本混合后续扩展）
func pickScript(scripts []*pbworker.ScriptRef) *pbworker.ScriptRef {
	best := scripts[0]
	for _, s := range scripts[1:] {
		if s.GetWeight() > best.GetWeight() {
			best = s
		}
	}
	return best
}
