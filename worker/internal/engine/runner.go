// Package engine 压测执行引擎（VU 阶梯 + 定速 RPS 两种模式）
// 核心逻辑从 cmd/runner 提炼，并增加指标收集和 Master 上报能力
package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	pbworker "github.com/Aodongq1n/jarvan4-platform/pb/worker"
	sdkhttp "github.com/Aodongq1n/jarvan4-platform/sdk/http"
	"github.com/Aodongq1n/jarvan4-platform/sdk/spec"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/reporter"
)

const (
	modeVUStep int32 = 1 // VU 阶梯模式
	modeRPS    int32 = 2 // 定速 RPS 模式

	rpsModeFixed int32 = 1
	rpsModeStep  int32 = 2
)

// Runner 单次压测执行器
type Runner struct {
	req       *pbworker.StartTaskRequest
	script    spec.ScriptEntry
	reporter  *reporter.MetricsReporter
	collector *reporter.Collector
	cancel    context.CancelFunc
}

// NewRunner 创建 Runner
func NewRunner(req *pbworker.StartTaskRequest, script spec.ScriptEntry, rep *reporter.MetricsReporter) *Runner {
	collector := reporter.NewCollector(req.GetRunId(), req.GetWorkerId())
	return &Runner{
		req:      req,
		script:   script,
		reporter: rep,
		collector: collector,
	}
}

// Run 执行压测（阻塞直到完成或被 Stop 取消）
// 应在独立 goroutine 中调用
func (r *Runner) Run(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.cancel = cancel
	defer cancel()

	runID := r.req.GetRunId()
	workerID := r.req.GetWorkerId()
	fmt.Printf("[Runner] start runID=%s workerID=%s mode=%d\n", runID, workerID, r.req.GetMode())

	// 1. 启动实时指标上报（每秒向 Master 上报）
	go r.reporter.StartReporting(ctx, r.collector)

	// 2. 执行 Setup（全局一次）
	envMap := r.req.GetEnvs()
	if envMap == nil {
		envMap = make(map[string]string)
	}
	setupCtx := r.newRunContext(ctx, 0, 0, envMap, nil)
	setupData, err := r.script.Setup(setupCtx)
	if err != nil {
		fmt.Printf("[Runner] Setup failed: %v\n", err)
		r.notifyFailed(ctx, fmt.Sprintf("Setup failed: %v", err))
		return
	}

	// 3. 按模式执行压测
	switch r.req.GetMode() {
	case modeVUStep:
		r.runVUStep(ctx, envMap, setupData)
	case modeRPS:
		r.runRPS(ctx, envMap, setupData)
	default:
		fmt.Printf("[Runner] unknown mode %d, fallback to VU step\n", r.req.GetMode())
		r.runVUStep(ctx, envMap, setupData)
	}

	// 4. 执行 Teardown
	teardownCtx := r.newRunContext(ctx, 0, 0, envMap, setupData)
	if err := r.script.Teardown(teardownCtx, setupData); err != nil {
		fmt.Printf("[Runner] Teardown failed: %v\n", err)
	}

	// 5. 上报最终接口级指标（触发 Master 生成报告）
	r.notifyCompleted(ctx)
	fmt.Printf("[Runner] done runID=%s\n", runID)
}

// Stop 优雅停止压测（取消 context，VU goroutine 会在当前迭代结束后退出）
func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

// ── VU 阶梯模式 ──────────────────────────────────────────────────────────────

func (r *Runner) runVUStep(ctx context.Context, envMap map[string]string, setupData interface{}) {
	steps := r.req.GetSteps()
	if len(steps) == 0 {
		return
	}

	for _, step := range steps {
		select {
		case <-ctx.Done():
			return
		default:
		}

		target := int(step.GetConcurrent())
		rampSec := int(step.GetRampTime())
		durationSec := int(step.GetDuration())

		// 每个 step 有独立的 ctx，duration 到期后 cancel 让 VU 退出
		stepCtx, stepCancel := context.WithTimeout(ctx, time.Duration(durationSec)*time.Second)

		var wg sync.WaitGroup
		var vuCounter int32

		// 爬坡：每隔 interval 启动一个新 VU
		if rampSec > 0 && target > 0 {
			interval := time.Duration(rampSec) * time.Second / time.Duration(target)
			for i := 0; i < target; i++ {
				select {
				case <-stepCtx.Done():
					break
				case <-time.After(interval):
				}
				if stepCtx.Err() != nil {
					break
				}
				vuID := int(atomic.AddInt32(&vuCounter, 1))
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					r.runVU(stepCtx, id, envMap, setupData)
				}(vuID)
			}
		} else {
			// 瞬变：同时启动所有 VU
			for i := 0; i < target; i++ {
				vuID := int(atomic.AddInt32(&vuCounter, 1))
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					r.runVU(stepCtx, id, envMap, setupData)
				}(vuID)
			}
		}

		r.collector.SetConcurrent(int32(target))

		// 等 step duration 到期（stepCtx 超时会自动触发）
		<-stepCtx.Done()
		stepCancel()

		// 等所有 VU goroutine 退出（当前迭代结束后自然退出）
		wg.Wait()
		r.collector.SetConcurrent(0)
	}
}

// runVU 单个 VU 持续执行 Default 直到 ctx 取消
func (r *Runner) runVU(ctx context.Context, vuID int, envMap map[string]string, setupData interface{}) {
	var iteration int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		runCtx := r.newRunContext(ctx, vuID, iteration, envMap, setupData)

		var failed bool
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					failed = true
					fmt.Printf("[VU%d] panic: %v\n", vuID, rec)
				}
			}()
			if err := r.script.Default(runCtx); err != nil {
				failed = true
			}
		}()

		latencyMs := time.Since(start).Milliseconds()
		r.collector.RecordResult(latencyMs, failed)
		iteration++
	}
}

// ── 定速 RPS 模式 ──────────────────────────────────────────────────────────────

func (r *Runner) runRPS(ctx context.Context, envMap map[string]string, setupData interface{}) {
	switch r.req.GetRpsMode() {
	case rpsModeStep:
		r.runRPSStep(ctx, envMap, setupData)
	default: // fixed
		targetRPS := int(r.req.GetTargetRps())
		durationSec := int(r.req.GetDuration())
		r.runRPSFixed(ctx, targetRPS, durationSec, envMap, setupData)
	}
}

// runRPSFixed 固定 RPS 模式
func (r *Runner) runRPSFixed(ctx context.Context, targetRPS, durationSec int, envMap map[string]string, setupData interface{}) {
	if targetRPS <= 0 {
		return
	}
	deadline := time.Now().Add(time.Duration(durationSec) * time.Second)
	ticker := time.NewTicker(time.Second / time.Duration(targetRPS))
	defer ticker.Stop()

	var vuCounter int32
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case t := <-ticker.C:
			if t.After(deadline) {
				wg.Wait()
				return
			}
			vuID := int(atomic.AddInt32(&vuCounter, 1))
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				start := time.Now()
				runCtx := r.newRunContext(ctx, id, 0, envMap, setupData)
				var failed bool
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							failed = true
						}
					}()
					if err := r.script.Default(runCtx); err != nil {
						failed = true
					}
				}()
				r.collector.RecordResult(time.Since(start).Milliseconds(), failed)
			}(vuID)
		}
	}
}

// runRPSStep RPS 阶梯模式
func (r *Runner) runRPSStep(ctx context.Context, envMap map[string]string, setupData interface{}) {
	for _, step := range r.req.GetRpsSteps() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		rps := int(step.GetRps())
		durationSec := int(step.GetDuration())
		r.runRPSFixed(ctx, rps, durationSec, envMap, setupData)
	}
}

// ── RunContext 构建 ───────────────────────────────────────────────────────────

func (r *Runner) newRunContext(ctx context.Context, vuID int, iteration int64, envMap map[string]string, setupData interface{}) *spec.RunContext {
	timeoutMs := r.req.GetTimeoutMs()
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return &spec.RunContext{
		Context:   ctx,
		VUId:      vuID,
		WorkerID:  r.req.GetWorkerId(),
		Iteration: iteration,
		SetupData: setupData,
		Vars:      &varStore{env: envMap, data: make(map[string]interface{})},
		HTTP:      buildHTTPClient(ctx, r.collector, time.Duration(timeoutMs)*time.Millisecond),
		Check:     &checker{},
		Log:       &logger{vuID: vuID},
		Sleep:     &sleeper{},
		Recorder:  r.collector,
	}
}

// ── 通知 Master 结果 ──────────────────────────────────────────────────────────

func (r *Runner) notifyCompleted(ctx context.Context) {
	// 构建空的 APIMetricsPayload，触发 Master 生成报告
	// 实际接口级指标需要 SDK 支持 label 维度采集，当前先上报汇总数据
	payload := &pbinternal.APIMetricsPayload{
		RunId: r.req.GetRunId(),
	}
	if err := r.reporter.ReportAPIMetrics(context.Background(), payload); err != nil {
		fmt.Printf("[Runner] ReportAPIMetrics failed: %v\n", err)
	}
}

func (r *Runner) notifyFailed(ctx context.Context, reason string) {
	fmt.Printf("[Runner] task failed: %s\n", reason)
	// 可扩展：通过单独 RPC 通知 Master 任务失败
}

// ── 辅助实现 ─────────────────────────────────────────────────────────────────

// varStore VU 级变量存储
type varStore struct {
	env  map[string]string
	data map[string]interface{}
	mu   sync.RWMutex
}

func (v *varStore) Set(key string, value interface{}) {
	v.mu.Lock()
	v.data[key] = value
	v.mu.Unlock()
}
func (v *varStore) Get(key string) interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.data[key]
}
func (v *varStore) GetString(key string) string {
	val := v.Get(key)
	if val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}
func (v *varStore) GetInt(key string) int {
	val := v.Get(key)
	if val == nil {
		return 0
	}
	i, _ := val.(int)
	return i
}
func (v *varStore) Delete(key string) {
	v.mu.Lock()
	delete(v.data, key)
	v.mu.Unlock()
}
func (v *varStore) Env(key string) string {
	return v.env[key]
}

// checker 断言器
type checker struct{}

func (c *checker) That(res *spec.HTTPResponse) *spec.Assertion {
	return spec.NewAssertion(res)
}

// logger VU 日志
type logger struct{ vuID int }

func (l *logger) Debug(format string, args ...interface{}) {
	fmt.Printf("[VU%d][DEBUG] %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *logger) Info(format string, args ...interface{}) {
	fmt.Printf("[VU%d][INFO]  %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *logger) Warn(format string, args ...interface{}) {
	fmt.Printf("[VU%d][WARN]  %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *logger) Error(format string, args ...interface{}) {
	fmt.Printf("[VU%d][ERROR] %s\n", l.vuID, fmt.Sprintf(format, args...))
}

// sleeper
type sleeper struct{}

func (s *sleeper) Sleep(d time.Duration) { time.Sleep(d) }

// buildHTTPClient 构建 SDK HTTP 客户端（挂载指标 Recorder）
func buildHTTPClient(ctx context.Context, recorder spec.MetricsRecorder, timeout time.Duration) spec.HTTPClient {
	runCtx := &spec.RunContext{
		Context:  ctx,
		Recorder: recorder,
	}
	return sdkhttp.New(runCtx, timeout)
}
