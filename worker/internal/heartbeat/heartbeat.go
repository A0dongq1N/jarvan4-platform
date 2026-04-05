// Package heartbeat Worker 定时向 Master 上报节点负载（CPU/内存使用率）
package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Reporter 心跳上报器
type Reporter struct {
	masterHTTPAddr string // Master HTTP 地址，e.g. "127.0.0.1:8090"
	workerID       string
	concurrent     *int32 // 指向 engine 中的并发计数，由外部提供
}

// New 创建心跳上报器
// masterHTTPAddr: Master HTTP 服务地址（:8090），workerID: Nacos instanceId
func New(masterHTTPAddr, workerID string) *Reporter {
	return &Reporter{
		masterHTTPAddr: masterHTTPAddr,
		workerID:       workerID,
	}
}

// Start 每 10 秒上报一次心跳，直到 ctx 取消
func (r *Reporter) Start(ctx context.Context, getConcurrent func() int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 立即上报一次
	r.report(ctx, getConcurrent())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.report(ctx, getConcurrent())
		}
	}
}

type heartbeatReq struct {
	CPUUsage   float64 `json:"cpuUsage"`
	MemUsage   float64 `json:"memUsage"`
	Concurrent int     `json:"concurrent"`
}

func (r *Reporter) report(ctx context.Context, concurrent int) {
	cpuUsage := cpuUsagePercent()
	memUsage := memUsagePercent()

	body, _ := json.Marshal(heartbeatReq{
		CPUUsage:   cpuUsage,
		MemUsage:   memUsage,
		Concurrent: concurrent,
	})

	url := fmt.Sprintf("http://%s/api/internal/workers/%s/heartbeat",
		r.masterHTTPAddr, url.PathEscape(r.workerID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[Heartbeat] report failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[Heartbeat] report status: %d\n", resp.StatusCode)
	}
}

// cpuUsagePercent 通过两次读取 /proc/stat 计算 100ms 内的 CPU 使用率（0~100）
func cpuUsagePercent() float64 {
	s1 := readCPUStat()
	time.Sleep(100 * time.Millisecond)
	s2 := readCPUStat()

	total := float64((s2.total - s1.total))
	idle := float64((s2.idle - s1.idle))
	if total <= 0 {
		return 0
	}
	return (1 - idle/total) * 100
}

type cpuStat struct {
	total uint64
	idle  uint64
}

func readCPUStat() cpuStat {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStat{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			break
		}
		var vals [10]uint64
		for i := 1; i < len(fields) && i <= 10; i++ {
			vals[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
		}
		// user(0) nice(1) system(2) idle(3) iowait(4) irq(5) softirq(6) steal(7)
		idle := vals[3] + vals[4]
		total := vals[0] + vals[1] + vals[2] + vals[3] + vals[4] + vals[5] + vals[6] + vals[7]
		return cpuStat{total: total, idle: idle}
	}
	// 非 Linux 环境降级：用 runtime 近似
	return cpuStat{total: uint64(runtime.NumGoroutine()), idle: 0}
}

// memUsagePercent 读取 /proc/meminfo 计算内存使用率（0~100）
func memUsagePercent() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var memTotal, memAvailable float64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(fields[1], 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = val
		case "MemAvailable:":
			memAvailable = val
		}
	}
	if memTotal <= 0 {
		return 0
	}
	return (1 - memAvailable/memTotal) * 100
}
