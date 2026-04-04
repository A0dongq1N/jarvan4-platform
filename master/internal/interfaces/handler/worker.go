package handler

import (
	"net/http"
	"net/netip"
	"strconv"

	appWorker "github.com/Aodongq1n/jarvan4-platform/master/internal/application/worker"
	domainWorker "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/worker"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/shared/constant"
	"github.com/gorilla/mux"
)

// WorkerHandler Worker 节点相关 handler
type WorkerHandler struct {
	svc appWorker.WorkerUseCase
}

func NewWorkerHandler(svc appWorker.WorkerUseCase) *WorkerHandler {
	return &WorkerHandler{svc: svc}
}

func (h *WorkerHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/workers", h.List).Methods(http.MethodGet)
	r.HandleFunc("/api/workers/{worker_id}/offline", h.Offline).Methods(http.MethodPost)
}

// toWorkerStatusStr 将 domain 枚举转为前端字符串
// online + 有正在执行的 run → "busy"；online + 无 run → "online"；offline → "offline"
func toWorkerStatusStr(w *domainWorker.WorkerNode) string {
	switch w.Status() {
	case constant.WorkerStatusOnline:
		if w.RunningRunID() != "" {
			return "busy"
		}
		return "online"
	case constant.WorkerStatusOffline:
		return "offline"
	default:
		return "offline"
	}
}

// toWorkerResp 将 domain WorkerNode 转为前端 DTO
// addr 格式为 "host:port"，拆分后分别填写 ip/port
func toWorkerResp(w *domainWorker.WorkerNode) dto.WorkerNodeResp {
	ip := w.Addr()
	port := 9090
	// 解析 addr = "ip:port"
	if ap, err := netip.ParseAddrPort(w.Addr()); err == nil {
		ip = ap.Addr().String()
		port = int(ap.Port())
	}

	return dto.WorkerNodeResp{
		ID:                 w.ID(),
		WorkerID:           w.ID(),
		Hostname:           ip, // domain 无独立 hostname，用 IP 代替
		IP:                 ip,
		Port:               port,
		Status:             toWorkerStatusStr(w),
		CPUCores:           w.CPUCores(),
		MemTotalGb:         w.MemTotalGB(),
		MaxConcurrency:     w.MaxConcurrency(),
		CPUUsage:           w.CPUUsage(),
		MemUsage:           w.MemUsage(),
		CurrentConcurrency: w.Concurrent(),
		RunningRunID:       w.RunningRunID(),
		LastHeartbeat:      w.UpdatedAt().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (h *WorkerHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "20"))
	statusFilter := q.Get("status")

	workers, err := h.svc.ListWorkers(r.Context())
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	// 状态过滤
	resps := make([]dto.WorkerNodeResp, 0, len(workers))
	for _, wn := range workers {
		resp := toWorkerResp(wn)
		if statusFilter != "" && resp.Status != statusFilter {
			continue
		}
		resps = append(resps, resp)
	}

	// 手动分页
	total := int64(len(resps))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(resps) {
		resps = []dto.WorkerNodeResp{}
	} else if end > len(resps) {
		resps = resps[start:]
	} else {
		resps = resps[start:end]
	}

	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *WorkerHandler) Offline(w http.ResponseWriter, r *http.Request) {
	workerID := mux.Vars(r)["worker_id"]
	if err := h.svc.OfflineWorker(r.Context(), workerID); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}
