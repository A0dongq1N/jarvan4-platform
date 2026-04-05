// Worker 服务入口
// 职责：Nacos 注册 + tRPC 服务（:9090）+ 压测引擎
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	trpc "trpc.group/trpc-go/trpc-go"

	pbworker "github.com/Aodongq1n/jarvan4-platform/pb/worker"
	"github.com/Aodongq1n/jarvan4-platform/shared/cos"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/handler"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/heartbeat"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/loader"
	workerNacos "github.com/Aodongq1n/jarvan4-platform/worker/internal/nacos"
	"github.com/Aodongq1n/jarvan4-platform/worker/internal/reporter"
)

func main() {
	// ── 1. 读取配置（环境变量）──────────────────────────────────────────────
	// Worker IP：生产环境通过 Downward API 注入 POD_IP，本地开发用 127.0.0.1
	workerIP   := getEnv("POD_IP", "127.0.0.1")
	workerAddr := workerIP + ":9090"
	masterAddr := getEnv("MASTER_ADDR", "127.0.0.1:8095")

	fmt.Printf("[Worker] starting workerAddr=%s masterAddr=%s\n", workerAddr, masterAddr)

	// ── 2. 初始化 COS 客户端（脚本下载）────────────────────────────────────
	cosClient, err := cos.NewClient(cos.Config{
		SecretID:  getEnv("COS_SECRET_ID", ""),
		SecretKey: getEnv("COS_SECRET_KEY", ""),
		Bucket:    getEnv("COS_BUCKET", ""),
		Region:    getEnv("COS_REGION", "ap-guangzhou"),
	})
	if err != nil {
		// COS 不可用时不退出，本地测试可以绕过
		fmt.Printf("[WARN] cos init failed: %v\n", err)
		cosClient = nil
	}

	// ── 3. 组装依赖 ────────────────────────────────────────────────────────
	scriptLoader := loader.New(cosClient, getEnv("SCRIPT_CACHE_DIR", "/tmp/worker-scripts"))
	metricsReporter := reporter.New(masterAddr)

	// ── 4. 初始化 trpc-go（读取 trpc_go.yaml 端口配置）────────────────────
	// 如果指定了 MASTER_ADDR，覆盖 yaml 中 master internal 服务地址
	if masterAddr != "127.0.0.1:8095" {
		overrideMasterAddr(masterAddr)
	}
	s := trpc.NewServer()

	// ── 5. 注册 tRPC Worker 服务 ────────────────────────────────────────────
	workerHandler := handler.New(scriptLoader, metricsReporter)
	pbworker.RegisterWorkerServiceService(
		s.Service("trpc.worker.stress.service"),
		workerHandler,
	)

	// ── 6. 注册到 Nacos（失败不退出，降级为直连模式）──────────────────────
	var workerID string
	if err := workerNacos.Init(); err != nil {
		fmt.Printf("[WARN] nacos init failed: %v\n", err)
	} else {
		if err := workerNacos.Register(workerAddr); err != nil {
			fmt.Printf("[WARN] nacos register failed: %v\n", err)
		} else {
			workerID = workerNacos.InstanceID(workerAddr)
			fmt.Printf("[Worker] registered to nacos addr=%s id=%s\n", workerAddr, workerID)
			defer workerNacos.Deregister(workerAddr)
		}
	}

	// ── 6.5. 启动心跳上报（每 10 秒向 Master 上报 CPU/内存使用率）───────────
	masterHTTPAddr := getEnv("MASTER_HTTP_ADDR", "127.0.0.1:8090")
	if workerID != "" {
		hb := heartbeat.New(masterHTTPAddr, workerID)
		go hb.Start(context.Background(), func() int {
			return workerHandler.RunningCount()
		})
	}

	// ── 7. 启动服务 ────────────────────────────────────────────────────────
	fmt.Printf("[Worker] serving on :9090\n")
	if err := s.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "[Worker] serve error: %v\n", err)
		os.Exit(1)
	}
}

// overrideMasterAddr 在 trpc client 配置里覆盖 Master 内部服务地址
func overrideMasterAddr(addr string) {
	// trpc-go 通过 trpc_go.yaml 读取 client target，
	// 这里直接修改环境变量让 yaml 中地址生效（目前 yaml 已指向正确地址）
	// 如需动态覆盖，可用 trpcclient.RegisterClientConfig（同 master 的做法）
	_ = addr // yaml 中 target 已配置，环境变量传递到启动参数即可
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// 确保 MASTER_ADDR 里不带 http:// 前缀
func cleanAddr(addr string) string {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	return addr
}

var _ = cleanAddr // 避免 unused 报错
