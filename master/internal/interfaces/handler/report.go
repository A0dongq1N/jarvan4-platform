package handler

import (
	"net/http"
	"strconv"

	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	appReport "github.com/Aodongq1n/jarvan4-platform/master/internal/application/report"
	domainReport "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/report"
	"github.com/gorilla/mux"
)

// ReportHandler 报告相关 handler
type ReportHandler struct {
	svc appReport.ReportUseCase
}

func NewReportHandler(svc appReport.ReportUseCase) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/reports", h.List).Methods(http.MethodGet)
	r.HandleFunc("/api/reports/{report_id}", h.Get).Methods(http.MethodGet)
	r.HandleFunc("/api/reports/{report_id}/timeseries", h.GetTimeSeries).Methods(http.MethodGet)
	r.HandleFunc("/api/reports/{report_id}/api-metrics", h.GetAPIMetrics).Methods(http.MethodGet)
	r.HandleFunc("/api/reports/{report_id}", h.Delete).Methods(http.MethodDelete)
}

// toSummaryResp 从 domain ReportSummary 转换
func toSummaryResp(s domainReport.ReportSummary) dto.MetricsSummaryResp {
	successReqs := s.TotalReqs - s.FailReqs
	if successReqs < 0 {
		successReqs = 0
	}
	errorRate := 0.0
	if s.TotalReqs > 0 {
		errorRate = float64(s.FailReqs) / float64(s.TotalReqs)
	}
	return dto.MetricsSummaryResp{
		RPS:             s.AvgQPS,
		AvgResponseTime: s.AvgRT,
		P99ResponseTime: s.P99RT,
		ErrorRate:       errorRate,
		TotalRequests:   s.TotalReqs,
		SuccessRequests: successReqs,
		FailedRequests:  s.FailReqs,
	}
}

// toReportSummaryResp 列表页轻量摘要
func toReportSummaryResp(rp *domainReport.Report) dto.ReportSummaryResp {
	resp := dto.ReportSummaryResp{
		ID:        rp.ID,
		TaskID:    rp.TaskID,
		TaskName:  rp.Name,
		RunID:     rp.RunID,
		Status:    "success",
		Duration:  rp.Summary.Duration,
		Summary:   toSummaryResp(rp.Summary),
		CreatedAt: rp.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if rp.StartTime != nil {
		resp.StartTime = rp.StartTime.UTC().Format("2006-01-02T15:04:05Z")
	}
	if rp.EndTime != nil {
		resp.EndTime = rp.EndTime.UTC().Format("2006-01-02T15:04:05Z")
	}
	return resp
}

// toFullReportResp 详情页完整报告
func toFullReportResp(full *appReport.FullReport) dto.ReportResp {
	rp := full.Report
	resp := dto.ReportResp{
		ID:        rp.ID,
		TaskID:    rp.TaskID,
		TaskName:  full.TaskName,
		RunID:     rp.RunID,
		Status:    "success",
		Duration:  rp.Summary.Duration,
		Summary:   toSummaryResp(rp.Summary),
		CreatedAt: rp.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		// 初始化为空 slice，避免前端 JSON 解析到 null
		RpsData:          []dto.MetricPointResp{},
		ResponseTimeData: []dto.MetricPointResp{},
		ErrorRateData:    []dto.MetricPointResp{},
		ConcurrentData:   []dto.MetricPointResp{},
		Percentiles:      []dto.PercentileResp{},
		Errors:           []dto.ErrorDataResp{},
	}

	// 时间
	if full.StartTime != nil {
		t := full.StartTime.UTC().Format("2006-01-02T15:04:05Z")
		resp.StartTime = t
	}
	if full.EndTime != nil {
		t := full.EndTime.UTC().Format("2006-01-02T15:04:05Z")
		resp.EndTime = t
	}

	// 时序数据拆分为四条曲线
	for _, p := range full.Points {
		ts := p.Timestamp
		resp.RpsData = append(resp.RpsData, dto.MetricPointResp{Timestamp: ts, Value: p.QPS})
		resp.ResponseTimeData = append(resp.ResponseTimeData, dto.MetricPointResp{Timestamp: ts, Value: p.AvgRT})
		resp.ErrorRateData = append(resp.ErrorRateData, dto.MetricPointResp{Timestamp: ts, Value: p.FailRate * 100})
		resp.ConcurrentData = append(resp.ConcurrentData, dto.MetricPointResp{Timestamp: ts, Value: float64(p.Concurrent)})
	}

	// 接口级分位数
	var totalErrors int64
	for _, m := range full.APIMetrics {
		totalErrors += m.FailReqs
		errRate := 0.0
		if m.TotalReqs > 0 {
			errRate = float64(m.FailReqs) / float64(m.TotalReqs)
		}
		resp.Percentiles = append(resp.Percentiles, dto.PercentileResp{
			API:       m.Label,
			Requests:  m.TotalReqs,
			Errors:    m.FailReqs,
			ErrorRate: errRate,
			P50:       m.P50RT,
			P75:       0, // domain 无 P75，返回 0
			P90:       m.P90RT,
			P95:       m.P95RT,
			P99:       m.P99RT,
			Max:       m.MaxRT,
			Min:       m.MinRT,
		})
	}

	// 错误分析：从接口级指标聚合，超时单独列出
	if totalErrors > 0 {
		for _, m := range full.APIMetrics {
			if m.FailReqs == 0 {
				continue
			}
			pct := float64(m.FailReqs) / float64(totalErrors) * 100
			resp.Errors = append(resp.Errors, dto.ErrorDataResp{
				Code:       m.Label,
				Message:    m.Label + " 接口请求失败",
				ErrorType:  "business",
				Count:      m.FailReqs,
				Percentage: pct,
			})
		}
	}

	// 脚本快照
	for _, s := range full.ScriptSnapshots {
		resp.ScriptSnapshots = append(resp.ScriptSnapshots, dto.ScriptSnapshotResp{
			ScriptID:    s.ScriptID,
			ScriptName:  s.ScriptName,
			CommitHash:  s.CommitHash,
			ArtifactURL: s.ArtifactURL,
			Weight:      s.Weight,
		})
	}

	// Worker 快照
	for _, w := range full.WorkerSnapshots {
		resp.WorkerSnapshots = append(resp.WorkerSnapshots, dto.WorkerSnapshotResp{
			WorkerID: w.WorkerID,
			Hostname: w.WorkerID, // domain WorkerSnapshot 无 hostname，用 WorkerID 代替
			IP:       w.Addr,
			CPUCores: w.CPUCores,
		})
	}

	return resp
}

func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(defaultQuery(q.Get("page"), "1"))
	pageSize, _ := strconv.Atoi(defaultQuery(q.Get("pageSize"), "10"))

	list, total, err := h.svc.ListReports(r.Context(), q.Get("projectId"), page, pageSize)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.ReportSummaryResp, 0, len(list))
	for _, rp := range list {
		resps = append(resps, toReportSummaryResp(rp))
	}
	dto.WriteOK(w, dto.PageData{List: resps, Total: total, Page: page, PageSize: pageSize})
}

func (h *ReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	full, err := h.svc.GetFullReport(r.Context(), mux.Vars(r)["report_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	dto.WriteOK(w, toFullReportResp(full))
}

func (h *ReportHandler) GetTimeSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	startTs, _ := strconv.ParseInt(q.Get("startTs"), 10, 64)
	endTs, _ := strconv.ParseInt(q.Get("endTs"), 10, 64)

	points, err := h.svc.GetTimeSeries(r.Context(), mux.Vars(r)["report_id"], startTs, endTs)
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.MetricPointResp, 0, len(points))
	for _, p := range points {
		resps = append(resps, dto.MetricPointResp{Timestamp: p.Timestamp, Value: p.QPS})
	}
	dto.WriteOK(w, resps)
}

func (h *ReportHandler) GetAPIMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.svc.GetAPIMetrics(r.Context(), mux.Vars(r)["report_id"])
	if err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	resps := make([]dto.PercentileResp, 0, len(metrics))
	for _, m := range metrics {
		errRate := 0.0
		if m.TotalReqs > 0 {
			errRate = float64(m.FailReqs) / float64(m.TotalReqs)
		}
		resps = append(resps, dto.PercentileResp{
			API:       m.Label,
			Requests:  m.TotalReqs,
			Errors:    m.FailReqs,
			ErrorRate: errRate,
			P50:       m.P50RT,
			P90:       m.P90RT,
			P95:       m.P95RT,
			P99:       m.P99RT,
			Max:       m.MaxRT,
			Min:       m.MinRT,
		})
	}
	dto.WriteOK(w, resps)
}

func (h *ReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteReport(r.Context(), mux.Vars(r)["report_id"], ""); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}
