package handler

import (
	"net/http"
	"strconv"
	"time"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/gorilla/mux"
)

// AuditHandler 审计日志 handler
type AuditHandler struct {
	svc appAudit.AuditUseCase
}

func NewAuditHandler(svc appAudit.AuditUseCase) *AuditHandler {
	return &AuditHandler{svc: svc}
}

func (h *AuditHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/audit-logs", h.List).Methods(http.MethodGet)
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "20"))

	// 时间参数：前端传 YYYY-MM-DD 字符串，转为 Unix 秒
	var startTs, endTs int64
	if s := q.Get("startTime"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startTs = t.Unix()
		}
	}
	if e := q.Get("endTime"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			endTs = t.Add(24*time.Hour - time.Second).Unix()
		}
	}

	filter := audit.ListFilter{
		Keyword:      q.Get("keyword"),
		UserID:       q.Get("userId"),
		Action:       audit.AuditAction(q.Get("action")),
		ResourceType: audit.ResourceType(q.Get("resourceType")),
		StartTime:    startTs,
		EndTime:      endTs,
	}

	list, total, err := h.svc.List(r.Context(), filter, page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	resps := make([]dto.AuditLogResp, 0, len(list))
	for _, l := range list {
		resps = append(resps, dto.AuditLogResp{
			ID:           l.ID,
			UserID:       l.UserID,
			Username:     l.Username,
			Action:       string(l.Action),
			ResourceType: string(l.ResourceType),
			ResourceID:   l.ResourceID,
			ResourceName: l.ResourceName,
			Detail:       l.Detail,
			IP:           l.IP,
			CreatedAt:    l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}
