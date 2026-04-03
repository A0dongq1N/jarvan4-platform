// Master 服务入口（trpc-go 框架）
package main

import (
	"net/http"

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
	infraRedis  "github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/redis"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/rpc"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/sms"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"

	"github.com/gorilla/mux"
	trpc "trpc.group/trpc-go/trpc-go"
	thttp "trpc.group/trpc-go/trpc-go/http"
	gormplugin "trpc.group/trpc-go/trpc-database/gorm"
)

func main() {
	// ── trpc-go 初始化（读取 trpc_go.yaml）────────────────────────────────
	s := trpc.NewServer()

	// ── 初始化 DB（从 trpc_go.yaml client 段读取 DSN）────────────────────
	db, err := gormplugin.NewClientProxy("trpc.mysql.master.db")
	if err != nil {
		panic("mysql init failed: " + err.Error())
	}

	// ── 依赖组装（依赖注入）───────────────────────────────────────────────
	// infra — MySQL repos
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

	// infra — Redis cache
	execCache, errRedis := infraRedis.NewExecutionCache("trpc.redis.master.cache")
	if errRedis != nil {
		panic("redis init failed: " + errRedis.Error())
	}

	// infra — RPC / SMS
	workerClient := rpc.NewWorkerClient()
	smsProv      := sms.NewProvider()

	// app services
	auditSvc   := appAudit.NewService(auditRepo)
	projectSvc := appProject.NewService(projectRepo)
	taskSvc    := appTask.NewService(taskRepo, auditRepo)
	scriptSvc  := appScript.NewService(scriptRepo, versionRepo)
	workerSvc  := appWorker.NewService(workerRepo)
	reportSvc  := appReport.NewService(reportRepo, pointRepo, apiRepo, runRepo, taskRepo)
	execSvc    := appExec.NewService(runRepo, taskRepo, scriptRepo, workerRepo, reportRepo, pointRepo, apiRepo, workerClient, execCache)
	authSvc    := appAuth.NewService(userRepo, smsProv, infraJWT.NewIssuer("jarvan4-secret-key", 24))

	// ── 注册 tRPC 内部服务（:8081，Worker → Master 指标上报）─────────────
	pbinternal.RegisterMasterInternalService(
		s.Service("trpc.master.trpc.internal"),
		trpchandler.NewMasterInternalHandler(execSvc),
	)

	// ── 注册路由（gorilla/mux，:8080，前端 ↔ Master）─────────────────────
	r := mux.NewRouter()

	// 公开路由：仅 CORS + Logger，不过 Auth
	publicChain := func(h http.Handler) http.Handler {
		return middleware.CORS(middleware.Logger(h))
	}
	// 受保护路由：CORS + Logger + Auth
	authChain := func(h http.Handler) http.Handler {
		return middleware.CORS(middleware.Logger(middleware.Auth(authSvc)(h)))
	}

	// 公开路由（登录/发送验证码，无需 token）
	publicRouter := r.PathPrefix("").Subrouter()
	authHandler := handler.NewAuthHandler(authSvc)
	authHandler.RegisterPublic(publicRouter)

	// 受保护路由
	protectedRouter := r.PathPrefix("").Subrouter()
	authHandler.RegisterProtected(protectedRouter)
	handler.NewProjectHandler(projectSvc).Register(protectedRouter)
	handler.NewTaskHandler(taskSvc).Register(protectedRouter)
	handler.NewScriptHandler(scriptSvc).Register(protectedRouter)
	handler.NewExecutionHandler(execSvc).Register(protectedRouter)
	handler.NewReportHandler(reportSvc).Register(protectedRouter)
	handler.NewWorkerHandler(workerSvc).Register(protectedRouter)
	handler.NewAuditHandler(auditSvc).Register(protectedRouter)

	// 将两个 subrouter 分别包装中间件后挂到主路由
	// gorilla/mux subrouter 共享同一底层路由表，需要用 Use() 而非包装 handler
	publicRouter.Use(func(next http.Handler) http.Handler { return publicChain(next) })
	protectedRouter.Use(func(next http.Handler) http.Handler { return authChain(next) })

	// 将 mux 注册到 trpc-go HTTP service（不再额外包 chain）
	thttp.RegisterNoProtocolServiceMux(s.Service("trpc.master.http.api"), r)

	_ = s.Serve()
}
