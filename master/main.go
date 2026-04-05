// Master 服务入口（trpc-go 框架）
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	trpcclient "trpc.group/trpc-go/trpc-go/client"

	appAudit   "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	appAuth    "github.com/Aodongq1n/jarvan4-platform/master/internal/application/auth"
	appExec    "github.com/Aodongq1n/jarvan4-platform/master/internal/application/execution"
	appProject "github.com/Aodongq1n/jarvan4-platform/master/internal/application/project"
	appReport  "github.com/Aodongq1n/jarvan4-platform/master/internal/application/report"
	appScript  "github.com/Aodongq1n/jarvan4-platform/master/internal/application/script"
	appTask    "github.com/Aodongq1n/jarvan4-platform/master/internal/application/task"
	appWorker  "github.com/Aodongq1n/jarvan4-platform/master/internal/application/worker"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/handler"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/trpchandler"
	infraJWT    "github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/jwt"
	infraMySQL  "github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/mysql"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/nacos"
	infraRedis  "github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/redis"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/rpc"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/sms"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"

	"github.com/gorilla/mux"
	trpc "trpc.group/trpc-go/trpc-go"
	thttp "trpc.group/trpc-go/trpc-go/http"
	gormplugin "trpc.group/trpc-go/trpc-database/gorm"
)

// loadNacosConfig 从 Nacos 加载配置。
// 连接参数从环境变量读取（不含敏感信息）：
//   NACOS_ADDR      e.g. "9.134.73.4:8848"
//   NACOS_NAMESPACE e.g. "dev"
//   NACOS_DATA_ID   e.g. "master.yaml"（默认值）
//   NACOS_GROUP     e.g. "DEFAULT_GROUP"（默认值）
func loadNacosConfig() (*nacos.MasterConfig, error) {
	addr := getEnv("NACOS_ADDR", "9.134.73.4:8848")
	namespace := getEnv("NACOS_NAMESPACE", "7681a7b6-2c9a-4770-850f-b7c96bbdb7d1")
	dataID := getEnv("NACOS_DATA_ID", "master.yaml")
	group := getEnv("NACOS_GROUP", "DEFAULT_GROUP")

	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.ParseUint(portStr, 10, 64)
	if port == 0 {
		port = 8848
	}

	return nacos.LoadConfig(nacos.ConfigOptions{
		ServerAddr:  host,
		ServerPort:  port,
		NamespaceID: namespace,
		DataID:      dataID,
		Group:       group,
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// ── 1. 从 Nacos 加载配置 ──────────────────────────────────────────────
	cfg, err := loadNacosConfig()
	if err != nil {
		// Nacos 不可用时降级：从 trpc_go.yaml 读取（本地开发兼容）
		fmt.Printf("[WARN] nacos config unavailable, fallback to trpc_go.yaml: %v\n", err)
		cfg = fallbackConfig()
	}

	// ── 2. trpc-go 初始化（读取 trpc_go.yaml 中的服务/端口配置）────────────
	s := trpc.NewServer()

	// ── 3. 初始化 DB（使用 Nacos 配置的 DSN）────────────────────────────────
	// trpc-go gorm plugin 从 trpc_go.yaml client 段读 DSN，需动态覆盖
	if err := overrideMySQLDSN(cfg.MySQL.DSN); err != nil {
		panic("override mysql dsn: " + err.Error())
	}
	db, err := gormplugin.NewClientProxy("trpc.mysql.master.db")
	if err != nil {
		panic("mysql init failed: " + err.Error())
	}

	// ── 4. 依赖组装 ───────────────────────────────────────────────────────
	taskRepo    := infraMySQL.NewTaskRepo(db)
	scriptRepo  := infraMySQL.NewScriptRepo(db)
	versionRepo := infraMySQL.NewScriptVersionRepo(db)
	runRepo     := infraMySQL.NewTaskRunRepo(db)
	reportRepo  := infraMySQL.NewReportRepo(db)
	pointRepo   := infraMySQL.NewMetricPointRepo(db)
	apiRepo     := infraMySQL.NewAPIMetricsRepo(db)
	workerRepo  := infraMySQL.NewWorkerRepo(db)
	projectRepo := infraMySQL.NewProjectRepo(db)
	userRepo    := infraMySQL.NewUserRepo(db)
	auditRepo   := infraMySQL.NewAuditLogRepo(db)

	// ── 3.5. 初始化 Redis（使用 Nacos 配置的地址和密码）──────────────────────
	// trpc-database/goredis 从 trpc_go.yaml client 段读配置，需动态覆盖
	if err := overrideRedisConfig(cfg.Redis.Addr, cfg.Redis.Password); err != nil {
		panic("override redis config: " + err.Error())
	}
	execCache, errRedis := infraRedis.NewExecutionCache("trpc.redis.master.cache")
	if errRedis != nil {
		panic("redis init failed: " + errRedis.Error())
	}

	workerClient := rpc.NewWorkerClient()
	smsProv      := sms.NewProvider()

	jwtExpire := 24
	if cfg.JWT.ExpireHours > 0 {
		jwtExpire = cfg.JWT.ExpireHours
	}
	jwtSecret := cfg.JWT.Secret
	if jwtSecret == "" {
		jwtSecret = "jarvan4-secret-key"
	}

	auditSvc   := appAudit.NewService(auditRepo)
	projectSvc := appProject.NewService(projectRepo)
	taskSvc    := appTask.NewService(taskRepo, auditRepo)
	scriptSvc  := appScript.NewService(scriptRepo, versionRepo)
	workerSvc  := appWorker.NewService(workerRepo)
	reportSvc  := appReport.NewService(reportRepo, pointRepo, apiRepo, runRepo, taskRepo)
	execSvc    := appExec.NewService(runRepo, taskRepo, scriptRepo, workerRepo, reportRepo, pointRepo, apiRepo, workerClient, execCache)
	authSvc    := appAuth.NewService(userRepo, smsProv, infraJWT.NewIssuer(jwtSecret, jwtExpire))

	// ── 4.5. 启动 Worker 服务发现（Nacos 订阅 stress-worker）──────────────────
	nacosAddr := getEnv("NACOS_ADDR", "9.134.73.4:8848")
	nacosNS   := getEnv("NACOS_NAMESPACE", "7681a7b6-2c9a-4770-850f-b7c96bbdb7d1")
	discovery, discErr := nacos.NewWorkerDiscovery(nacosAddr, nacosNS, workerSvc)
	if discErr != nil {
		fmt.Printf("[WARN] worker discovery init failed: %v\n", discErr)
	} else {
		go func() {
			if err := discovery.Start(context.Background()); err != nil {
				fmt.Printf("[WARN] worker discovery start failed: %v\n", err)
			}
		}()
	}

	// ── 5. 注册 tRPC 内部服务（:8081，Worker → Master 指标上报）───────────
	pbinternal.RegisterMasterInternalService(
		s.Service("trpc.master.trpc.internal"),
		trpchandler.NewMasterInternalHandler(execSvc),
	)

	// ── 6. 注册路由 ───────────────────────────────────────────────────────
	r := mux.NewRouter()

	publicChain := func(h http.Handler) http.Handler {
		return middleware.CORS(middleware.Logger(h))
	}
	authChain := func(h http.Handler) http.Handler {
		return middleware.CORS(middleware.Logger(middleware.Auth(authSvc)(h)))
	}

	publicRouter := r.PathPrefix("").Subrouter()
	authHandler := handler.NewAuthHandler(authSvc, auditSvc)
	authHandler.RegisterPublic(publicRouter)

	workerHandlerInst := handler.NewWorkerHandler(workerSvc, auditSvc)
	workerHandlerInst.RegisterInternal(publicRouter) // 心跳接口无需 JWT

	protectedRouter := r.PathPrefix("").Subrouter()
	authHandler.RegisterProtected(protectedRouter)
	handler.NewProjectHandler(projectSvc, auditSvc).Register(protectedRouter)
	handler.NewTaskHandler(taskSvc, auditSvc).Register(protectedRouter)
	handler.NewScriptHandler(scriptSvc, auditSvc).Register(protectedRouter)
	handler.NewExecutionHandler(execSvc, auditSvc).Register(protectedRouter)
	handler.NewReportHandler(reportSvc).Register(protectedRouter)
	workerHandlerInst.Register(protectedRouter) // 已在上方实例化
	handler.NewAuditHandler(auditSvc).Register(protectedRouter)

	publicRouter.Use(func(next http.Handler) http.Handler { return publicChain(next) })
	protectedRouter.Use(func(next http.Handler) http.Handler { return authChain(next) })

	thttp.RegisterNoProtocolServiceMux(s.Service("trpc.master.http.api"), r)

	_ = s.Serve()
}

// overrideMySQLDSN 通过 trpc-go client.RegisterClientConfig 覆盖 MySQL 连接配置
// 必须在 trpc.NewServer() 之后、NewClientProxy() 之前调用
func overrideMySQLDSN(dsn string) error {
	if dsn == "" {
		return nil
	}
	return trpcclient.RegisterClientConfig("trpc.mysql.master.db", &trpcclient.BackendConfig{
		ServiceName: "trpc.mysql.master.db",
		Target:      "dsn://" + dsn,
	})
}

// overrideRedisConfig 通过 trpc-go client.RegisterClientConfig 覆盖 Redis 连接配置
// 必须在 trpc.NewServer() 之后、goredis.New() 之前调用
func overrideRedisConfig(addr, password string) error {
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	target := "redis://" + addr
	if password != "" {
		target = "redis://:" + password + "@" + addr
	}
	return trpcclient.RegisterClientConfig("trpc.redis.master.cache", &trpcclient.BackendConfig{
		ServiceName: "trpc.redis.master.cache",
		Target:      target,
	})
}

// fallbackConfig 本地开发降级：返回空配置（trpc_go.yaml 保留原值）
func fallbackConfig() *nacos.MasterConfig {
	cfg := &nacos.MasterConfig{}
	cfg.JWT.Secret = "jarvan4-secret-key"
	cfg.JWT.ExpireHours = 24
	return cfg
}
