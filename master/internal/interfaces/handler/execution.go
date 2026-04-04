package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	appExec "github.com/Aodongq1n/jarvan4-platform/master/internal/application/execution"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	domainExec "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/execution"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/Aodongq1n/jarvan4-platform/shared/constant"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	"github.com/gorilla/mux"
)

// ExecutionHandler 执行相关 handler（前端 ↔ Master HTTP 接口）
type ExecutionHandler struct {
	svc      appExec.ExecutionUseCase
	auditSvc appAudit.AuditUseCase
}

func NewExecutionHandler(svc appExec.ExecutionUseCase, auditSvc appAudit.AuditUseCase) *ExecutionHandler {
	return &ExecutionHandler{svc: svc, auditSvc: auditSvc}
}

func (h *ExecutionHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/executions", h.Start).Methods(http.MethodPost)
	r.HandleFunc("/api/executions/{execution_id}/stop", h.Stop).Methods(http.MethodPost)
	r.HandleFunc("/api/executions/{execution_id}", h.GetStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/executions/{execution_id}/metrics", h.GetMetrics).Methods(http.MethodGet)
	r.HandleFunc("/api/executions/{execution_id}/api-metrics", h.GetAPIMetrics).Methods(http.MethodGet)
	r.HandleFunc("/api/executions/{execution_id}/logs", h.GetLogs).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks/{task_id}/executions", h.ListRuns).Methods(http.MethodGet)
}

// toRunStatusStr 将常量转为前端期望的字符串
func toRunStatusStr(s constant.TaskRunStatus) string {
	switch s {
	case constant.TaskRunStatusPending:
		return "pending"
	case constant.TaskRunStatusRunning:
		return "running"
	case constant.TaskRunStatusSuccess:
		return "success"
	case constant.TaskRunStatusStopped:
		return "stopped"
	case constant.TaskRunStatusFailed:
		return "failed"
	case constant.TaskRunStatusCircuitBroken:
		return "circuit_broken"
	default:
		return "unknown"
	}
}

// toInitStepStatusStr 将 InitStepStatus 常量转为前端字符串
func toInitStepStatusStr(s constant.InitStepStatus) string {
	switch s {
	case constant.InitStepPending:
		return "waiting"
	case constant.InitStepRunning:
		return "running"
	case constant.InitStepDone:
		return "done"
	case constant.InitStepFailed:
		return "error"
	default:
		return "waiting"
	}
}

// toExecutionStateResp 将领域对象转为前端 ExecutionState 结构
func toExecutionStateResp(run *domainExec.TaskRun) dto.ExecutionStateResp {
	resp := dto.ExecutionStateResp{
		ID:         run.ID(),
		TaskID:     run.TaskID(),
		TaskName:   "",  // 需要从 task 查询，暂时空字符串
		Status:     toRunStatusStr(run.Status()),
		WarningMsg: run.WarningMsg(),
		ErrorMsg:   run.ErrorMsg(),
	}

	if run.StartTime() != nil {
		t := run.StartTime().Format(time.RFC3339)
		resp.StartTime = &t
		if run.Status() == constant.TaskRunStatusRunning {
			resp.ElapsedSeconds = int64(time.Since(*run.StartTime()).Seconds())
		} else if run.EndTime() != nil {
			resp.ElapsedSeconds = int64(run.EndTime().Sub(*run.StartTime()).Seconds())
		}
	}
	if run.EndTime() != nil {
		t := run.EndTime().Format(time.RFC3339)
		resp.EndTime = &t
	}

	// script snapshots
	snaps := run.ScriptSnapshots()
	if len(snaps) > 0 {
		resp.ScriptSnapshots = make([]dto.ScriptSnapshotResp, 0, len(snaps))
		for _, s := range snaps {
			resp.ScriptSnapshots = append(resp.ScriptSnapshots, dto.ScriptSnapshotResp{
				ScriptID:    s.ScriptID,
				ScriptName:  s.ScriptName,
				CommitHash:  s.CommitHash,
				ArtifactURL: s.ArtifactURL,
				Weight:      s.Weight,
			})
		}
	}

	// init steps — 保持顺序，map 转 slice 按固定 key 顺序
	steps := run.InitSteps()
	if len(steps) > 0 {
		stepOrder := []string{"select_worker", "download_script", "load_plugin", "inject_start"}
		resp.InitSteps = make([]dto.InitStepResp, 0, len(stepOrder))
		for _, key := range stepOrder {
			if step, ok := steps[key]; ok {
				resp.InitSteps = append(resp.InitSteps, dto.InitStepResp{
					Key:    step.Key,
					Label:  step.Label,
					Status: toInitStepStatusStr(step.Status),
					Detail: step.Detail,
					Items:  step.Items,
				})
			}
		}
	}

	return resp
}

// toMetricsSummaryResp 将 protobuf metrics 转为前端 MetricsSummary
func toMetricsSummaryResp(m *pbinternal.MetricsPayload) dto.MetricsSummaryResp {
	if m == nil {
		return dto.MetricsSummaryResp{}
	}
	successReqs := m.TotalReqs - m.FailReqs
	if successReqs < 0 {
		successReqs = 0
	}
	errorRate := 0.0
	if m.TotalReqs > 0 {
		errorRate = float64(m.FailReqs) / float64(m.TotalReqs)
	}
	return dto.MetricsSummaryResp{
		RPS:             m.Qps,
		AvgResponseTime: m.AvgRtMs,
		P99ResponseTime: m.P99RtMs,
		ErrorRate:       errorRate,
		TotalRequests:   m.TotalReqs,
		SuccessRequests: successReqs,
		FailedRequests:  m.FailReqs,
		Concurrent:      int(m.Concurrent),
	}
}

func (h *ExecutionHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req dto.StartExecutionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if req.TaskID == "" {
		dto.WriteFail(w, http.StatusBadRequest, 400, "taskId is required")
		return
	}
	run, err := h.svc.StartExecution(r.Context(), req.TaskID, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toExecutionStateResp(run))
	writeAudit(r, h.auditSvc, audit.ActionStartExecution, audit.ResourceExecution, run.ID(), run.TaskID(), "")
}

func (h *ExecutionHandler) Stop(w http.ResponseWriter, r *http.Request) {
	runID := mux.Vars(r)["execution_id"]
	if err := h.svc.StopExecution(r.Context(), runID, middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	// 返回最新状态（前端期望 ExecutionState）
	run, _, err := h.svc.GetRunStatus(r.Context(), runID)
	if err != nil {
		dto.WriteOK(w, nil)
		return
	}
	dto.WriteOK(w, toExecutionStateResp(run))
	writeAudit(r, h.auditSvc, audit.ActionStopExecution, audit.ResourceExecution, runID, "", "")
}

func (h *ExecutionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	run, _, err := h.svc.GetRunStatus(r.Context(), mux.Vars(r)["execution_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toExecutionStateResp(run))
}

func (h *ExecutionHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	_, metrics, err := h.svc.GetRunStatus(r.Context(), mux.Vars(r)["execution_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toMetricsSummaryResp(metrics))
}

func (h *ExecutionHandler) GetAPIMetrics(w http.ResponseWriter, r *http.Request) {
	// 从 Redis 或 DB 获取接口级实时指标，暂返回空列表（待 execution service 扩展）
	dto.WriteOK(w, map[string]interface{}{
		"percentiles": []interface{}{},
		"errors":      []interface{}{},
	})
}

func (h *ExecutionHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	// 实时日志暂返回空（Worker 日志推送功能待实现）
	dto.WriteOK(w, []interface{}{})
}

func (h *ExecutionHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "10"))
	taskID := mux.Vars(r)["task_id"]

	list, total, err := h.svc.ListRuns(r.Context(), taskID, page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.ExecutionRecordResp, 0, len(list))
	for _, run := range list {
		rec := dto.ExecutionRecordResp{
			ID:              run.ID(),
			TaskID:          run.TaskID(),
			Status:          toRunStatusStr(run.Status()),
			TriggerType:     int(run.TriggerType()),
			TriggeredByName: run.TriggeredBy(),
			ErrorMsg:        run.ErrorMsg(),
		}
		if run.StartTime() != nil {
			t := run.StartTime().Format(time.RFC3339)
			rec.StartTime = &t
		}
		if run.EndTime() != nil {
			t := run.EndTime().Format(time.RFC3339)
			rec.EndTime = &t
		}
		if run.StartTime() != nil && run.EndTime() != nil {
			dur := int64(run.EndTime().Sub(*run.StartTime()).Seconds())
			rec.DurationSec = &dur
		}
		resps = append(resps, rec)
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}
