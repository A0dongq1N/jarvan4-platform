// Package execution 执行用例编排（应用层）
package execution

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainExec "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/execution"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/script"
	domainTask "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/task"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/worker"
	domainReport "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/report"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	pbworker "github.com/Aodongq1n/jarvan4-platform/pb/worker"
	"github.com/Aodongq1n/jarvan4-platform/shared/constant"
)

// Service 执行用例服务
type Service struct {
	runRepo    domainExec.TaskRunRepo
	taskRepo   domainTask.TaskRepo
	scriptRepo script.ScriptRepo
	workerRepo worker.WorkerRepo
	reportRepo domainReport.ReportRepo
	metricPointRepo domainReport.MetricPointRepo
	apiMetricsRepo  domainReport.APIMetricsRepo
	scheduler  *domainExec.Scheduler
	rpcClient  WorkerClient
	cache      ExecutionCache
}

// NewService 构造函数（依赖注入）
func NewService(
	runRepo domainExec.TaskRunRepo,
	taskRepo domainTask.TaskRepo,
	scriptRepo script.ScriptRepo,
	workerRepo worker.WorkerRepo,
	reportRepo domainReport.ReportRepo,
	metricPointRepo domainReport.MetricPointRepo,
	apiMetricsRepo domainReport.APIMetricsRepo,
	rpcClient WorkerClient,
	cache ExecutionCache,
) *Service {
	return &Service{
		runRepo:         runRepo,
		taskRepo:        taskRepo,
		scriptRepo:      scriptRepo,
		workerRepo:      workerRepo,
		reportRepo:      reportRepo,
		metricPointRepo: metricPointRepo,
		apiMetricsRepo:  apiMetricsRepo,
		scheduler:       &domainExec.Scheduler{},
		rpcClient:       rpcClient,
		cache:           cache,
	}
}

// StartExecution 启动压测：快照版本 → 调度 Worker → 下发 SubTask
func (s *Service) StartExecution(ctx context.Context, taskID, triggeredBy string) (*domainExec.TaskRun, error) {
	// 1. 查询任务及场景配置
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}

	scene := task.SceneConfig()

	// 2. 为每个脚本读取最新 commitHash，构建 ScriptSnapshot
	taskScripts := task.Scripts()
	scriptSnapshots := make([]domainExec.ScriptSnapshot, 0, len(taskScripts))
	for _, ts := range taskScripts {
		sc, err := s.scriptRepo.FindByID(ctx, ts.ScriptID)
		if err != nil {
			return nil, fmt.Errorf("find script %s: %w", ts.ScriptID, err)
		}
		scriptSnapshots = append(scriptSnapshots, domainExec.ScriptSnapshot{
			ScriptID:    sc.ID,
			ScriptName:  sc.Name,
			CommitHash:  sc.CommitHash,
			ArtifactURL: sc.ArtifactURL,
			Weight:      ts.Weight,
		})
	}

	// 3. 估算所需 Worker 数量
	maxConcurrent := estimateMaxConcurrent(scene)
	need := s.scheduler.EstimateWorkerCount(int8(scene.Mode), maxConcurrent)

	// 4. 查询可用 Worker，调度器选取
	allWorkers, err := s.workerRepo.FindAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("find available workers: %w", err)
	}
	selected, degraded := s.scheduler.SelectWorkers(allWorkers, need)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no available workers")
	}

	warningMsg := ""
	if degraded {
		warningMsg = fmt.Sprintf("降级模式：预期 %d 个 Worker，实际分配 %d 个", need, len(selected))
	}

	// 5. 构建 WorkerSnapshot 列表
	workerSnapshots := make([]domainExec.WorkerSnapshot, 0, len(selected))
	for _, w := range selected {
		workerSnapshots = append(workerSnapshots, domainExec.WorkerSnapshot{
			WorkerID: w.ID(),
			Addr:     w.Addr(),
			CPUCores: w.CPUCores(),
			MemGB:    w.MemTotalGB(),
		})
	}

	// 6. NewTaskRun → run.Start
	runID := uuid.NewString()
	run := domainExec.NewTaskRun(runID, taskID, triggeredBy, constant.TriggerTypeManual)
	if err := run.Start(workerSnapshots, scriptSnapshots, warningMsg); err != nil {
		return nil, fmt.Errorf("start run: %w", err)
	}

	// 7. 保存 TaskRun
	if err := s.runRepo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("save task run: %w", err)
	}

	// 8. 并发下发给各 Worker（每个 Worker 构建独立请求）
	var failedWorkers []string
	for _, w := range selected {
		req := buildStartTaskRequest(runID, taskID, w.ID(), scene, scriptSnapshots)
		if err := s.rpcClient.StartTask(ctx, w.Addr(), req); err != nil {
			failedWorkers = append(failedWorkers, w.ID())
		}
	}

	// 若有 Worker 下发失败，回滚 run 状态
	if len(failedWorkers) == len(selected) {
		_ = run.Fail(fmt.Sprintf("all workers failed to start: %v", failedWorkers))
		_ = s.runRepo.Save(ctx, run)
		// 通知已成功下发的 Worker 停止（无失败则跳过）
		return nil, fmt.Errorf("failed to start on any worker")
	}


	// 9. 更新 Redis 状态缓存
	_ = s.cache.SetStatus(ctx, runID, int8(constant.TaskRunStatusRunning))

	return run, nil
}

// StopExecution 停止压测
func (s *Service) StopExecution(ctx context.Context, runID, operatedBy string) error {
	run, err := s.runRepo.FindByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("find run: %w", err)
	}
	if err := run.Stop(); err != nil {
		return fmt.Errorf("stop run: %w", err)
	}
	if err := s.runRepo.Save(ctx, run); err != nil {
		return fmt.Errorf("save run: %w", err)
	}

	// 通知所有 Worker 停止
	for _, ws := range run.WorkerSnapshots() {
		_ = s.rpcClient.StopTask(ctx, ws.Addr, runID)
	}

	_ = s.cache.SetStatus(ctx, runID, int8(constant.TaskRunStatusStopped))
	return nil
}

// ListRuns 分页查询任务的执行历史
func (s *Service) ListRuns(ctx context.Context, taskID string, page, pageSize int) ([]*domainExec.TaskRun, int64, error) {
	return s.runRepo.ListByTaskID(ctx, taskID, page, pageSize)
}

// AggregateMetrics 接收 Worker 上报的全局实时指标，写入 Redis + 持久化时序
func (s *Service) AggregateMetrics(ctx context.Context, payload *pbinternal.MetricsPayload) error {
	runID := payload.GetRunId()

	// 写入 Redis（最新指标，供前端轮询）
	if err := s.cache.SaveMetrics(ctx, runID, payload); err != nil {
		return fmt.Errorf("save metrics to cache: %w", err)
	}

	// 持久化时序数据
	point := &domainReport.MetricPoint{
		RunID:      runID,
		Timestamp:  payload.GetTimestamp(),
		QPS:        payload.GetQps(),
		AvgRT:      payload.GetAvgRtMs(),
		P95RT:      payload.GetP95RtMs(),
		P99RT:      payload.GetP99RtMs(),
		FailRate:   failRate(payload.GetTotalReqs(), payload.GetFailReqs()),
		Concurrent: int(payload.GetConcurrent()),
	}
	if err := s.metricPointRepo.BatchSave(ctx, []*domainReport.MetricPoint{point}); err != nil {
		// 持久化失败不阻塞实时上报
		return nil
	}

	return nil
}

// ReceiveAPIMetrics 接收 Worker 压测结束后的接口级指标，触发报告生成
func (s *Service) ReceiveAPIMetrics(ctx context.Context, payload *pbinternal.APIMetricsPayload) error {
	runID := payload.GetRunId()

	// 转换并批量存储接口级指标
	metrics := make([]*domainReport.APIMetrics, 0, len(payload.GetApis()))
	for _, item := range payload.GetApis() {
		metrics = append(metrics, &domainReport.APIMetrics{
			Label:     item.GetLabel(),
			RunID:     runID,
			TotalReqs: item.GetTotalReqs(),
			FailReqs:  item.GetFailReqs(),
			AvgRT:     item.GetAvgRtMs(),
			P50RT:     item.GetP50RtMs(),
			P90RT:     item.GetP90RtMs(),
			P95RT:     item.GetP95RtMs(),
			P99RT:     item.GetP99RtMs(),
			MaxRT:     item.GetMaxRtMs(),
			MinRT:     item.GetMinRtMs(),
		})
	}
	if len(metrics) > 0 {
		if err := s.apiMetricsRepo.BatchSave(ctx, metrics); err != nil {
			return fmt.Errorf("save api metrics: %w", err)
		}
	}

	// 标记 run 完成并生成报告
	run, err := s.runRepo.FindByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("find run: %w", err)
	}
	_ = run.Complete()
	_ = s.runRepo.Save(ctx, run)
	_ = s.cache.SetStatus(ctx, runID, int8(constant.TaskRunStatusSuccess))

	return s.GenerateReport(ctx, runID)
}

// GenerateReport 压测结束后汇总生成报告
func (s *Service) GenerateReport(ctx context.Context, runID string) error {
	run, err := s.runRepo.FindByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("find run: %w", err)
	}

	// 查询全量时序数据
	points, err := s.metricPointRepo.FindTimeSeriesByRunID(ctx, runID, 0, 0)
	if err != nil {
		return fmt.Errorf("find time series: %w", err)
	}

	summary := aggregateSummary(points)

	// 计算压测时长
	if run.StartTime() != nil && run.EndTime() != nil {
		summary.Duration = int(run.EndTime().Sub(*run.StartTime()).Seconds())
	}

	// 查询接口级指标（需要先通过 run 关联 report）
	// 暂时用 runID 作为 report 中转 key 查询
	reportName := fmt.Sprintf("压测报告 - %s", runID[:8])

	report := domainReport.NewReport(runID, run.TaskID(), "", reportName, summary)
	report.ID = uuid.NewString()

	// 回填 api metrics 的 report ID
	apiMetrics, err := s.apiMetricsRepo.ListByReportID(ctx, runID)
	if err == nil && len(apiMetrics) > 0 {
		for _, m := range apiMetrics {
			m.ReportID = report.ID
		}
		_ = s.apiMetricsRepo.BatchSave(ctx, apiMetrics)
	}

	if err := s.reportRepo.Save(ctx, report); err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	return nil
}

// GetRunStatus 查询执行状态（含实时指标）
func (s *Service) GetRunStatus(ctx context.Context, runID string) (*domainExec.TaskRun, *pbinternal.MetricsPayload, error) {
	run, err := s.runRepo.FindByID(ctx, runID)
	if err != nil {
		return nil, nil, fmt.Errorf("find run: %w", err)
	}

	// 从 Redis 获取最新实时指标（执行中时才有）
	var metrics *pbinternal.MetricsPayload
	if run.Status() == constant.TaskRunStatusRunning {
		metrics, _ = s.cache.GetLatestMetrics(ctx, runID)
		// Redis 无数据则返回 nil，不报错
	}

	return run, metrics, nil
}

// ── 内部辅助函数 ──────────────────────────────────────────────────────────

// estimateMaxConcurrent 从场景配置提取峰值并发/RPS
func estimateMaxConcurrent(scene domainTask.SceneConfig) int {
	if scene.Mode == domainTask.SceneModeVUStep {
		max := 0
		for _, step := range scene.Steps {
			if step.Concurrent > max {
				max = step.Concurrent
			}
		}
		return max
	}
	// RPS 模式
	if scene.RPSSubMode == domainTask.RPSSubModeFixed {
		return scene.TargetRPS
	}
	max := 0
	for _, step := range scene.RPSSteps {
		if step.RPS > max {
			max = step.RPS
		}
	}
	return max
}

// buildStartTaskRequest 将领域场景配置转换为 pb 下发请求
func buildStartTaskRequest(runID, taskID, workerID string, scene domainTask.SceneConfig, scripts []domainExec.ScriptSnapshot) *pbworker.StartTaskRequest {
	req := &pbworker.StartTaskRequest{
		RunId:     runID,
		TaskId:    taskID,
		WorkerId:  workerID,
		Mode:      int32(scene.Mode),
		TimeoutMs: int32(scene.TimeoutMs),
		Duration:  int32(scene.Duration),
		Envs:      scene.EnvVars,
	}

	// VU 阶梯
	for _, step := range scene.Steps {
		req.Steps = append(req.Steps, &pbworker.VUStep{
			Concurrent: int32(step.Concurrent),
			RampTime:   int32(step.RampTime),
			Duration:   int32(step.Duration),
		})
	}

	// RPS
	req.RpsMode = int32(scene.RPSSubMode)
	req.TargetRps = int32(scene.TargetRPS)
	req.RpsRampTime = int32(scene.RPSRampTime)
	for _, step := range scene.RPSSteps {
		req.RpsSteps = append(req.RpsSteps, &pbworker.RPSStep{
			Rps:      int32(step.RPS),
			Duration: int32(step.Duration),
			RampTime: int32(step.RampTime),
		})
	}

	// 熔断配置
	cb := scene.CircuitBreaker
	req.CircuitBreaker = &pbworker.CircuitBreakerConfig{
		Enabled:     cb.Enabled,
		ErrorRate:   cb.GlobalErrorRateThreshold,
		WindowSec:   int32(cb.GlobalWindowSeconds),
		MinRequests: int32(cb.GlobalMinRequests),
	}

	// 脚本列表
	for _, sc := range scripts {
		req.Scripts = append(req.Scripts, &pbworker.ScriptRef{
			ScriptId:    sc.ScriptID,
			ScriptName:  sc.ScriptName,
			CommitHash:  sc.CommitHash,
			ArtifactUrl: sc.ArtifactURL,
			Weight:      int32(sc.Weight),
		})
	}

	return req
}

// failRate 计算失败率
func failRate(total, fail int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(fail) / float64(total)
}

// aggregateSummary 从时序点聚合报告摘要
func aggregateSummary(points []*domainReport.MetricPoint) domainReport.ReportSummary {
	if len(points) == 0 {
		return domainReport.ReportSummary{}
	}

	var totalQPS, totalRT, totalP95, totalP99 float64
	var maxRT float64
	var totalReqs, failReqs int64

	for _, p := range points {
		totalQPS += p.QPS
		totalRT += p.AvgRT
		totalP95 += p.P95RT
		totalP99 += p.P99RT
		if p.AvgRT > maxRT {
			maxRT = p.AvgRT
		}
	}

	n := float64(len(points))
	summary := domainReport.ReportSummary{
		TotalReqs:   totalReqs,
		FailReqs:    failReqs,
		AvgQPS:      totalQPS / n,
		AvgRT:       totalRT / n,
		P95RT:       totalP95 / n,
		P99RT:       totalP99 / n,
		MaxRT:       maxRT,
	}
	if totalReqs > 0 {
		summary.SuccessRate = float64(totalReqs-failReqs) / float64(totalReqs)
	}
	return summary
}
