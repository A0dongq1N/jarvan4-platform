package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	appProject "github.com/Aodongq1n/jarvan4-platform/master/internal/application/project"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	domainProject "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/project"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/gorilla/mux"
)

// ProjectHandler 项目相关 handler
type ProjectHandler struct {
	svc      appProject.ProjectUseCase
	auditSvc appAudit.AuditUseCase
}

func NewProjectHandler(svc appProject.ProjectUseCase, auditSvc appAudit.AuditUseCase) *ProjectHandler {
	return &ProjectHandler{svc: svc, auditSvc: auditSvc}
}

func (h *ProjectHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/projects", h.List).Methods(http.MethodGet)
	r.HandleFunc("/api/projects", h.Create).Methods(http.MethodPost)
	r.HandleFunc("/api/projects/{project_id}", h.Get).Methods(http.MethodGet)
	r.HandleFunc("/api/projects/{project_id}", h.Update).Methods(http.MethodPut)
	r.HandleFunc("/api/projects/{project_id}", h.Delete).Methods(http.MethodDelete)
}

func toProjectResp(p *domainProject.Project) dto.ProjectResp {
	var lastRunAt *string
	if p.LastRunAt != nil {
		s := p.LastRunAt.UTC().Format("2006-01-02T15:04:05Z")
		lastRunAt = &s
	}
	return dto.ProjectResp{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		TaskCount:   p.TaskCount,
		ScriptCount: p.ScriptCount,
		LastRunAt:   lastRunAt,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("page_size"), "20"))

	list, total, err := h.svc.ListProjects(r.Context(), page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.ProjectResp, 0, len(list))
	for _, p := range list {
		resps = append(resps, toProjectResp(p))
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	proj, err := h.svc.CreateProject(r.Context(), req.Name, req.Description, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toProjectResp(proj))
	writeAudit(r, h.auditSvc, audit.ActionCreateProject, audit.ResourceProject, proj.ID, proj.Name, "")
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	proj, err := h.svc.GetProject(r.Context(), mux.Vars(r)["project_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toProjectResp(proj))
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	proj, err := h.svc.UpdateProject(r.Context(), mux.Vars(r)["project_id"], req.Name, req.Description, middleware.CurrentUsername(r))
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toProjectResp(proj))
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["project_id"]
	if err := h.svc.DeleteProject(r.Context(), projectID, middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
	writeAudit(r, h.auditSvc, audit.ActionDeleteProject, audit.ResourceProject, projectID, "", "")
}
