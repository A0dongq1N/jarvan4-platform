package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	appScript "github.com/Aodongq1n/jarvan4-platform/master/internal/application/script"
	domainScript "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/script"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/gorilla/mux"
)

// ScriptHandler 脚本相关 handler
type ScriptHandler struct {
	svc appScript.ScriptUseCase
}

func NewScriptHandler(svc appScript.ScriptUseCase) *ScriptHandler {
	return &ScriptHandler{svc: svc}
}

func (h *ScriptHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/scripts", h.List).Methods(http.MethodGet)
	r.HandleFunc("/api/scripts/{script_id}", h.Get).Methods(http.MethodGet)
	r.HandleFunc("/api/scripts/{script_id}/versions", h.ListVersions).Methods(http.MethodGet)
	r.HandleFunc("/api/scripts/{script_id}", h.Offline).Methods(http.MethodDelete)
	r.HandleFunc("/api/internal/scripts/publish", h.Publish).Methods(http.MethodPost)
}

func toScriptResp(s *domainScript.Script) dto.ScriptResp {
	statusStr := "active"
	if s.Status == 0 {
		statusStr = "offline"
	}
	return dto.ScriptResp{
		ID:          s.ID,
		ProjectID:   s.ProjectID,
		Name:        s.Name,
		Description: s.Description,
		Language:    s.Lang,
		CommitHash:  s.CommitHash,
		ArtifactURL: s.ArtifactURL,
		CommitMsg:   s.CommitMsg,
		Author:      s.Author,
		Status:      statusStr,
		CreatedAt:   s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   s.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toScriptVersionResp(v *domainScript.ScriptVersion) dto.ScriptVersionResp {
	return dto.ScriptVersionResp{
		ID:          v.ID,
		CommitHash:  v.CommitHash,
		ArtifactURL: v.ArtifactURL,
		CommitMsg:   v.CommitMsg,
		Author:      v.Author,
		CreatedAt:   v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (h *ScriptHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "20"))
	projectID := q.Get("projectId")

	list, total, err := h.svc.ListScripts(r.Context(), projectID, page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.ScriptResp, 0, len(list))
	for _, s := range list {
		resps = append(resps, toScriptResp(s))
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *ScriptHandler) Get(w http.ResponseWriter, r *http.Request) {
	script, err := h.svc.GetScript(r.Context(), mux.Vars(r)["script_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toScriptResp(script))
}

func (h *ScriptHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "20"))

	list, total, err := h.svc.ListVersions(r.Context(), mux.Vars(r)["script_id"], page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.ScriptVersionResp, 0, len(list))
	for _, v := range list {
		resps = append(resps, toScriptVersionResp(v))
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *ScriptHandler) Offline(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.OfflineScript(r.Context(), mux.Vars(r)["script_id"], middleware.CurrentUsername(r)); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}

func (h *ScriptHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req dto.PublishScriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	script, err := h.svc.PublishScript(r.Context(), req.ProjectID, req.Name, req.Description, req.CommitHash, req.ArtifactURL, req.CommitMsg, req.Author)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, toScriptResp(script))
}
