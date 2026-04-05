// Package reporter 每秒向 Master 上报压测实时指标（tRPC 调用 masterinternal 服务）
package reporter

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	"trpc.group/trpc-go/trpc-go/client"
)

// MetricsReporter 向 Master 上报实时指标
type MetricsReporter struct {
	masterAddr string // Master tRPC 内部服务地址，e.g. "127.0.0.1:8095"
}

// New 创建 MetricsReporter
func New(masterAddr string) *MetricsReporter {
	return &MetricsReporter{masterAddr: masterAddr}
}

// Collector 线程安全的指标收集器（由 engine 写入，reporter 读取）
type Collector struct {
	runID    string
	workerID string

	totalReqs  atomic.Int64
	failReqs   atomic.Int64
	totalLatMs atomic.Int64  // 累计延迟，用于计算平均 RT
	concurrent atomic.Int32

	// 滑动窗口（最近 1s 的数据），用于计算瞬时 QPS
	windowReqs  atomic.Int64
	windowStart atomic.Int64 // Unix 纳秒
}

// NewCollector 创建指标收集器
func NewCollector(runID, workerID string) *Collector {
	c := &Collector{runID: runID, workerID: workerID}
	c.windowStart.Store(time.Now().UnixNano())
	return c
}

// RecordResult 供 engine 直接调用（不经过 SDK HTTP client）
// latencyMs: 本次请求耗时(ms)，failed: 是否失败
func (c *Collector) RecordResult(latencyMs int64, failed bool) {
	c.totalReqs.Add(1)
	c.windowReqs.Add(1)
	c.totalLatMs.Add(latencyMs)
	if failed {
		c.failReqs.Add(1)
	}
}

// SetConcurrent 更新当前并发数
func (c *Collector) SetConcurrent(n int32) {
	c.concurrent.Store(n)
}

// Snapshot 生成当前 1s 指标快照并重置窗口计数器
func (c *Collector) Snapshot() *pbinternal.MetricsPayload {
	now := time.Now().UnixNano()
	windowNs := now - c.windowStart.Swap(now)
	windowReqs := c.windowReqs.Swap(0)

	total := c.totalReqs.Load()
	fail := c.failReqs.Load()

	var qps float64
	if windowNs > 0 {
		qps = float64(windowReqs) / (float64(windowNs) / float64(time.Second))
	}

	var avgRT float64
	if total > 0 {
		avgRT = float64(c.totalLatMs.Load()) / float64(total)
	}

	return &pbinternal.MetricsPayload{
		RunId:      c.runID,
		WorkerId:   c.workerID,
		Timestamp:  time.Now().Unix(),
		TotalReqs:  total,
		FailReqs:   fail,
		Qps:        qps,
		AvgRtMs:    avgRT,
		Concurrent: c.concurrent.Load(),
	}
}

// Record 实现 spec.MetricsRecorder 接口（由 SDK HTTP client 调用）
// label: 请求标签（URL 或接口名），duration: 实际耗时，err: 非 nil 表示失败
func (c *Collector) Record(label string, duration time.Duration, err error) {
	latencyMs := duration.Milliseconds()
	c.totalReqs.Add(1)
	c.windowReqs.Add(1)
	c.totalLatMs.Add(latencyMs)
	if err != nil {
		c.failReqs.Add(1)
	}
}

// Skip 撤销最近一次计数（用于跳过不计入统计的请求）
func (c *Collector) Skip() {
	c.totalReqs.Add(-1)
	c.windowReqs.Add(-1)
}

// StartReporting 每秒向 Master 上报一次指标，直到 ctx 取消
// 应在独立 goroutine 中运行
func (r *MetricsReporter) StartReporting(ctx context.Context, collector *Collector) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := collector.Snapshot()
			if err := r.report(ctx, payload); err != nil {
				// 上报失败不影响压测主流程，仅打印日志
				fmt.Printf("[WARN] report metrics failed: %v\n", err)
			}
		}
	}
}

// report 调用 Master tRPC 接口上报指标
func (r *MetricsReporter) report(ctx context.Context, payload *pbinternal.MetricsPayload) error {
	proxy := pbinternal.NewMasterInternalClientProxy(
		client.WithTarget(fmt.Sprintf("ip://%s", r.masterAddr)),
	)
	rsp, err := proxy.AggregateMetrics(ctx, payload)
	if err != nil {
		return fmt.Errorf("AggregateMetrics rpc: %w", err)
	}
	if rsp.GetCode() != 0 {
		return fmt.Errorf("AggregateMetrics: code=%d msg=%s", rsp.GetCode(), rsp.GetMessage())
	}
	return nil
}

// ReportAPIMetrics 压测结束后上报接口级指标（一次性，非定时）
func (r *MetricsReporter) ReportAPIMetrics(ctx context.Context, payload *pbinternal.APIMetricsPayload) error {
	proxy := pbinternal.NewMasterInternalClientProxy(
		client.WithTarget(fmt.Sprintf("ip://%s", r.masterAddr)),
	)
	rsp, err := proxy.ReceiveAPIMetrics(ctx, payload)
	if err != nil {
		return fmt.Errorf("ReceiveAPIMetrics rpc: %w", err)
	}
	if rsp.GetCode() != 0 {
		return fmt.Errorf("ReceiveAPIMetrics: code=%d msg=%s", rsp.GetCode(), rsp.GetMessage())
	}
	return nil
}
