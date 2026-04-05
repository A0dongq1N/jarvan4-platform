package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	appTask "github.com/Aodongq1n/jarvan4-platform/master/internal/application/task"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	domainTask "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/task"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/gorilla/mux"
)

// TaskHandler 任务相关 handler
type TaskHandler struct {
	svc      appTask.TaskUseCase
	auditSvc appAudit.AuditUseCase
}

func NewTaskHandler(svc appTask.TaskUseCase, auditSvc appAudit.AuditUseCase) *TaskHandler {
	return &TaskHandler{svc: svc, auditSvc: auditSvc}
}

func (h *TaskHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/tasks", h.List).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks", h.Create).Methods(http.MethodPost)
	r.HandleFunc("/api/tasks/{task_id}", h.Get).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks/{task_id}", h.Update).Methods(http.MethodPut)
	r.HandleFunc("/api/tasks/{task_id}", h.Delete).Methods(http.MethodDelete)
	r.HandleFunc("/api/tasks/{task_id}/scene", h.UpdateScene).Methods(http.MethodPut)
	r.HandleFunc("/api/tasks/{task_id}/scripts", h.BindScript).Methods(http.MethodPost)
	r.HandleFunc("/api/tasks/{task_id}/scripts/{script_id}", h.UnbindScript).Methods(http.MethodDelete)
	r.HandleFunc("/api/tasks/{task_id}/scripts/{script_id}/weight", h.UpdateScriptWeight).Methods(http.MethodPut)
}

// toTaskResp 将领域对象转为 DTO
func toTaskResp(t *domainTask.Task) dto.TaskResp {
	scripts := make([]dto.ScriptBindResp, 0, len(t.Scripts()))
	for _, s := range t.Scripts() {
		scripts = append(scripts, dto.ScriptBindResp{
			ScriptID:   s.ScriptID,
			ScriptName: s.ScriptName,
			Weight:     s.Weight,
		})
	}
	sc := t.SceneConfig()
	steps := make([]dto.StepItem, 0, len(sc.Steps))
	for _, s := range sc.Steps {
		steps = append(steps, dto.StepItem{Concurrent: s.Concurrent, RampTime: s.RampTime, Duration: s.Duration})
	}
	rpsSteps := make([]dto.RPSStepItem, 0, len(sc.RPSSteps))
	for _, s := range sc.RPSSteps {
		rpsSteps = append(rpsSteps, dto.RPSStepItem{RPS: s.RPS, Duration: s.Duration, RampTime: s.RampTime})
	}

	// 将 int8 mode 转为字符串
	modeStr := ""
	switch sc.Mode {
	case domainTask.SceneModeVUStep:
		modeStr = "step"
	case domainTask.SceneModeRPS:
		modeStr = "rps"
	}
	rpsModeStr := ""
	switch sc.RPSSubMode {
	case domainTask.RPSSubModeFixed:
		rpsModeStr = "fixed"
	case domainTask.RPSSubModeStep:
		rpsModeStr = "step"
	}

	scene := &dto.SceneResp{
		Mode:      modeStr,
		Duration:  sc.Duration,
		TimeoutMs: sc.TimeoutMs,
		EnvVars:   sc.EnvVars,
		Steps:     steps,
		RPSMode:   rpsModeStr,
		TargetRPS: sc.TargetRPS,
		RPSSteps:  rpsSteps,
		CircuitBreaker: &dto.CircuitBreakerItem{
			Enabled:                  sc.CircuitBreaker.Enabled,
			Rules:                    func() []dto.CircuitBreakerRule {
				rules := make([]dto.CircuitBreakerRule, 0, len(sc.CircuitBreaker.Rules))
				for _, r := range sc.CircuitBreaker.Rules {
					rules = append(rules, dto.CircuitBreakerRule{
						URLPattern:         r.URLPattern,
						ErrorRateThreshold: r.ErrorRateThreshold,
						WindowSeconds:      r.WindowSeconds,
						MinRequests:        r.MinRequests,
					})
				}
				return rules
			}(),
			GlobalErrorRateThreshold: sc.CircuitBreaker.GlobalErrorRateThreshold,
			GlobalWindowSeconds:      sc.CircuitBreaker.GlobalWindowSeconds,
			GlobalMinRequests:        sc.CircuitBreaker.GlobalMinRequests,
		},
	}

	// 任务状态：domain status=1 表示活跃，对前端暴露为 "inactive"（未执行）
	statusStr := "inactive"
	if t.Status() == 0 {
		statusStr = "deleted"
	}

	return dto.TaskResp{
		ID:             t.ID(),
		ProjectID:      t.ProjectID(),
		Name:           t.Name(),
		Description:    t.Description(),
		Status:         statusStr,
		Scripts:        scripts,
		ScenarioConfig: scene,
		CreatedBy:      t.CreatedBy(),
		CreatedAt:      t.CreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      t.UpdatedAt().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "20"))
	projectID := q.Get("projectId")

	list, total, err := h.svc.ListTasks(r.Context(), projectID, page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.TaskResp, 0, len(list))
	for _, t := range list {
		resps = append(resps, toTaskResp(t))
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	task, err := h.svc.CreateTask(r.Context(), req.ProjectID, req.Name, req.Description, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toTaskResp(task))
	writeAudit(r, h.auditSvc, audit.ActionCreateTask, audit.ResourceTask, task.ID(), task.Name(), "")
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	task, err := h.svc.GetTask(r.Context(), mux.Vars(r)["task_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toTaskResp(task))
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	task, err := h.svc.UpdateTask(r.Context(), mux.Vars(r)["task_id"], req.Name, req.Description, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toTaskResp(task))
	writeAudit(r, h.auditSvc, audit.ActionUpdateTask, audit.ResourceTask, task.ID(), task.Name(), "")
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["task_id"]
	if err := h.svc.DeleteTask(r.Context(), taskID, middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
	writeAudit(r, h.auditSvc, audit.ActionDeleteTask, audit.ResourceTask, taskID, "", "")
}

func (h *TaskHandler) UpdateScene(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var req dto.UpdateSceneReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	// 字符串 mode → domain SceneMode
	var sceneMode domainTask.SceneMode
	switch req.Mode {
	case "step":
		sceneMode = domainTask.SceneModeVUStep
	case "rps":
		sceneMode = domainTask.SceneModeRPS
	default:
		dto.WriteFail(w, http.StatusBadRequest, 400, "mode must be 'step' or 'rps'")
		return
	}

	// 字符串 rpsMode → domain RPSSubMode
	var rpsSubMode domainTask.RPSSubMode
	switch req.RPSMode {
	case "fixed":
		rpsSubMode = domainTask.RPSSubModeFixed
	case "step":
		rpsSubMode = domainTask.RPSSubModeStep
	default:
		rpsSubMode = 0 // 未设置
	}

	steps := make([]domainTask.StepConfig, 0, len(req.Steps))
	for _, s := range req.Steps {
		steps = append(steps, domainTask.StepConfig{Concurrent: s.Concurrent, RampTime: s.RampTime, Duration: s.Duration})
	}
	rpsSteps := make([]domainTask.RPSStepConfig, 0, len(req.RPSSteps))
	for _, s := range req.RPSSteps {
		rpsSteps = append(rpsSteps, domainTask.RPSStepConfig{RPS: s.RPS, Duration: s.Duration, RampTime: s.RampTime})
	}
	cb := domainTask.CircuitBreakerConfig{}
	if req.CircuitBreaker != nil {
		cbRules := make([]domainTask.CircuitBreakerRule, 0, len(req.CircuitBreaker.Rules))
		for _, r := range req.CircuitBreaker.Rules {
			cbRules = append(cbRules, domainTask.CircuitBreakerRule{
				URLPattern:         r.URLPattern,
				ErrorRateThreshold: r.ErrorRateThreshold,
				WindowSeconds:      r.WindowSeconds,
				MinRequests:        r.MinRequests,
			})
		}
		cb = domainTask.CircuitBreakerConfig{
			Enabled:                  req.CircuitBreaker.Enabled,
			Rules:                    cbRules,
			GlobalErrorRateThreshold: req.CircuitBreaker.GlobalErrorRateThreshold,
			GlobalWindowSeconds:      req.CircuitBreaker.GlobalWindowSeconds,
			GlobalMinRequests:        req.CircuitBreaker.GlobalMinRequests,
		}
	}
	cfg := domainTask.SceneConfig{
		Mode:           sceneMode,
		Duration:       req.Duration,
		TimeoutMs:      req.TimeoutMs,
		EnvVars:        req.EnvVars,
		Steps:          steps,
		RPSSubMode:     rpsSubMode,
		TargetRPS:      req.TargetRPS,
		RPSRampTime:    req.RPSRampTime,
		RPSSteps:       rpsSteps,
		CircuitBreaker: cb,
	}

	task, err := h.svc.UpdateScene(r.Context(), vars["task_id"], cfg, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toTaskResp(task))
}

func (h *TaskHandler) BindScript(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var req dto.BindScriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := h.svc.BindScript(r.Context(), vars["task_id"], req.ScriptID, "", req.Weight, middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}

func (h *TaskHandler) UnbindScript(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if err := h.svc.UnbindScript(r.Context(), vars["task_id"], vars["script_id"], middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}

func (h *TaskHandler) UpdateScriptWeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var req dto.UpdateScriptWeightReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := h.svc.UpdateScriptWeight(r.Context(), vars["task_id"], vars["script_id"], req.Weight, middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}
