// cmd/runner：本地极简压测运行器，用于开发阶段验证脚本和 SDK。
// 不依赖 Master/Worker/Nacos，直接加载 .so 文件在本地执行压测，
// 将指标打印到终端。
//
// 用法：
//
//	go run ./cmd/runner \
//	  -so ./dist/http_demo.so \
//	  -vu 10 \
//	  -duration 30s \
//	  -env BASE_URL=https://httpbin.org
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"plugin"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	sdkhttp "github.com/Aodongq1n/jarvan4-platform/sdk/http"
	"github.com/Aodongq1n/jarvan4-platform/sdk/spec"
)

func main() {
	soPath := flag.String("so", "", "压测脚本 .so 文件路径（必填）")
	vuCount := flag.Int("vu", 5, "并发 VU 数量")
	duration := flag.Duration("duration", 30*time.Second, "压测持续时长")
	envFlags := flag.String("env", "", "环境变量，格式：KEY=VAL,KEY2=VAL2")
	flag.Parse()

	if *soPath == "" {
		fmt.Fprintln(os.Stderr, "错误：-so 参数必填，请指定 .so 文件路径")
		flag.Usage()
		os.Exit(1)
	}

	// 解析环境变量
	envMap := parseEnv(*envFlags)

	// 加载 .so
	fmt.Printf("▶ 加载脚本：%s\n", *soPath)
	p, err := plugin.Open(*soPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 .so 失败：%v\n", err)
		os.Exit(1)
	}
	sym, err := p.Lookup("Script")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lookup('Script') 失败：%v\n", err)
		os.Exit(1)
	}
	// 脚本导出的是 spec.ScriptEntry 接口值（var Script = &XxxScript{}）
	entry, ok := sym.(spec.ScriptEntry)
	if !ok {
		// 兼容导出为指针的情况（*spec.ScriptEntry）
		entryPtr, ok2 := sym.(*spec.ScriptEntry)
		if !ok2 {
			fmt.Fprintf(os.Stderr, "Script 符号类型不匹配，请确认脚本实现了 spec.ScriptEntry 接口\n")
			os.Exit(1)
		}
		run(*entryPtr, *vuCount, *duration, envMap)
		return
	}
	run(entry, *vuCount, *duration, envMap)
}

func run(script spec.ScriptEntry, vuCount int, duration time.Duration, envMap map[string]string) {
	// 全局指标计数器
	var (
		totalReqs   int64
		totalFails  int64
		totalLatMs  int64
	)

	recorder := &localRecorder{
		totalReqs:  &totalReqs,
		totalFails: &totalFails,
		totalLatMs: &totalLatMs,
	}

	// 构建 Setup 用的 ctx（VUId=0）
	setupCtx := newRunContext(0, envMap, recorder, nil)

	fmt.Println("▶ 执行 Setup...")
	setupData, err := script.Setup(setupCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Setup 失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Setup 完成")

	// 启动压测
	fmt.Printf("▶ 开始压测：%d VU，持续 %s\n\n", vuCount, duration)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// 监听 Ctrl+C 提前停止
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\n⚠ 收到停止信号，正在停止...")
		cancel()
	}()

	startTime := time.Now()
	var wg sync.WaitGroup

	for i := 1; i <= vuCount; i++ {
		wg.Add(1)
		go func(vuID int) {
			defer wg.Done()
			var iteration int64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				runCtx := newRunContext(vuID, envMap, recorder, setupData)
				runCtx.Iteration = iteration

				func() {
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&totalFails, 1)
							fmt.Fprintf(os.Stderr, "[VU%d] panic: %v\n", vuID, r)
						}
					}()
					if err := script.Default(runCtx); err != nil {
						atomic.AddInt64(&totalFails, 1)
					}
				}()
				iteration++
			}
		}(i)
	}

	// 每秒打印实时指标
	ticker := time.NewTicker(time.Second)
	go func() {
		var lastReqs int64
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				cur := atomic.LoadInt64(&totalReqs)
				qps := cur - lastReqs
				lastReqs = cur
				fails := atomic.LoadInt64(&totalFails)
				elapsed := time.Since(startTime).Seconds()
				var avgLat int64
				if cur > 0 {
					avgLat = atomic.LoadInt64(&totalLatMs) / cur
				}
				errRate := 0.0
				if cur > 0 {
					errRate = float64(fails) / float64(cur) * 100
				}
				fmt.Printf("[%5.0fs] QPS: %-6d | 总请求: %-8d | 失败: %-6d | 错误率: %5.2f%% | 平均RT: %dms\n",
					elapsed, qps, cur, fails, errRate, avgLat)
			}
		}
	}()

	wg.Wait()
	elapsed := time.Since(startTime)

	// 打印最终汇总
	reqs := atomic.LoadInt64(&totalReqs)
	fails := atomic.LoadInt64(&totalFails)
	var avgLat int64
	if reqs > 0 {
		avgLat = atomic.LoadInt64(&totalLatMs) / reqs
	}
	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	fmt.Printf("压测完成\n")
	fmt.Printf("  持续时长：%s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  总请求数：%d\n", reqs)
	fmt.Printf("  失败数：  %d\n", fails)
	fmt.Printf("  成功率：  %.2f%%\n", func() float64 {
		if reqs == 0 {
			return 0
		}
		return float64(reqs-fails) / float64(reqs) * 100
	}())
	fmt.Printf("  平均 QPS：%.1f\n", float64(reqs)/elapsed.Seconds())
	fmt.Printf("  平均 RT： %dms\n", avgLat)
	fmt.Printf("%s\n", strings.Repeat("─", 60))

	// Teardown
	fmt.Println("\n▶ 执行 Teardown...")
	teardownCtx := newRunContext(0, envMap, recorder, setupData)
	if err := script.Teardown(teardownCtx, setupData); err != nil {
		fmt.Fprintf(os.Stderr, "Teardown 失败：%v\n", err)
	} else {
		fmt.Println("✓ Teardown 完成")
	}
}

// ── 本地实现：RunContext 构造 ──────────────────────────────────────────────

func newRunContext(vuID int, envMap map[string]string, recorder spec.MetricsRecorder, setupData interface{}) *spec.RunContext {
	return &spec.RunContext{
		Context:   context.Background(),
		VUId:      vuID,
		WorkerID:  "local-runner",
		Vars:      &localVarStore{env: envMap, data: make(map[string]interface{})},
		SetupData: setupData,
		HTTP:      newHTTPClient(recorder),
		Check:     &localChecker{},
		Log:       &localLogger{vuID: vuID},
		Sleep:     &localSleeper{},
		Recorder:  recorder,
	}
}

// ── MetricsRecorder 实现 ───────────────────────────────────────────────────

type localRecorder struct {
	totalReqs  *int64
	totalFails *int64
	totalLatMs *int64
	skip       bool
}

func (r *localRecorder) Record(label string, duration time.Duration, err error) {
	r.skip = false
	atomic.AddInt64(r.totalReqs, 1)
	atomic.AddInt64(r.totalLatMs, duration.Milliseconds())
	if err != nil {
		atomic.AddInt64(r.totalFails, 1)
	}
}

func (r *localRecorder) Skip() {
	// 撤销最近一次 Record 的失败计数（若已计入）
	atomic.AddInt64(r.totalReqs, -1)
	atomic.AddInt64(r.totalFails, -1)
}

// ── VarStore 实现 ─────────────────────────────────────────────────────────

type localVarStore struct {
	env  map[string]string
	data map[string]interface{}
	mu   sync.RWMutex
}

func (v *localVarStore) Set(key string, value interface{}) {
	v.mu.Lock()
	v.data[key] = value
	v.mu.Unlock()
}
func (v *localVarStore) Get(key string) interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.data[key]
}
func (v *localVarStore) GetString(key string) string {
	val := v.Get(key)
	if val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}
func (v *localVarStore) GetInt(key string) int {
	val := v.Get(key)
	if val == nil {
		return 0
	}
	i, _ := val.(int)
	return i
}
func (v *localVarStore) Delete(key string) {
	v.mu.Lock()
	delete(v.data, key)
	v.mu.Unlock()
}
func (v *localVarStore) Env(key string) string {
	return v.env[key]
}

// ── Logger 实现 ───────────────────────────────────────────────────────────

type localLogger struct{ vuID int }

func (l *localLogger) Debug(format string, args ...interface{}) {
	fmt.Printf("[VU%d][DEBUG] %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *localLogger) Info(format string, args ...interface{}) {
	fmt.Printf("[VU%d][INFO]  %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *localLogger) Warn(format string, args ...interface{}) {
	fmt.Printf("[VU%d][WARN]  %s\n", l.vuID, fmt.Sprintf(format, args...))
}
func (l *localLogger) Error(format string, args ...interface{}) {
	fmt.Printf("[VU%d][ERROR] %s\n", l.vuID, fmt.Sprintf(format, args...))
}

// ── Sleeper 实现 ──────────────────────────────────────────────────────────

type localSleeper struct{}

func (s *localSleeper) Sleep(d time.Duration) { time.Sleep(d) }

// ── Checker 实现 ──────────────────────────────────────────────────────────

type localChecker struct{}

func (c *localChecker) That(res *spec.HTTPResponse) *spec.Assertion {
	return spec.NewAssertion(res)
}

// ── HTTP Client 实现 ──────────────────────────────────────────────────────

func newHTTPClient(recorder spec.MetricsRecorder) spec.HTTPClient {
	ctx := &spec.RunContext{
		Context:  context.Background(),
		Recorder: recorder,
	}
	return sdkhttp.New(ctx, 30*time.Second)
}

// ── 工具函数 ──────────────────────────────────────────────────────────────

func parseEnv(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}
