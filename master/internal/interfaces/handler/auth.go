package handler

import (
	"context"
	"encoding/json"
	"net/http"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	appAuth "github.com/Aodongq1n/jarvan4-platform/master/internal/application/auth"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/dto"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
	"github.com/gorilla/mux"
)

// AuthHandler 认证相关 handler
type AuthHandler struct {
	svc      appAuth.AuthUseCase
	auditSvc appAudit.AuditUseCase
}

func NewAuthHandler(svc appAuth.AuthUseCase, auditSvc appAudit.AuditUseCase) *AuthHandler {
	return &AuthHandler{svc: svc, auditSvc: auditSvc}
}

// RegisterPublic 注册不需要鉴权的 auth 路由（登录、发验证码）
func (h *AuthHandler) RegisterPublic(r *mux.Router) {
	r.HandleFunc("/api/auth/login", h.Login).Methods(http.MethodPost)
	r.HandleFunc("/api/auth/login/sms", h.LoginBySMS).Methods(http.MethodPost)
	r.HandleFunc("/api/auth/sms/send", h.SendSMSCode).Methods(http.MethodPost)
}

// RegisterProtected 注册需要鉴权的 auth 路由（登出、获取当前用户）
func (h *AuthHandler) RegisterProtected(r *mux.Router) {
	r.HandleFunc("/api/auth/logout", h.Logout).Methods(http.MethodPost)
	r.HandleFunc("/api/auth/me", h.Me).Methods(http.MethodGet)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if req.Username == "" || req.Password == "" {
		dto.WriteFail(w, http.StatusBadRequest, 400, "username and password are required")
		return
	}
	token, user, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		dto.WriteFail(w, http.StatusUnauthorized, 401, err.Error())
		return
	}
	dto.WriteOK(w, dto.LoginResp{
		Token: token,
		UserInfo: dto.UserInfoResp{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.Username,
			Role:        roleToStr(user.Role),
		},
	})
	// 登录成功写审计（userID 从 user 对象取，不依赖 context）
	ip := clientIP(r)
	go func() {
		_ = h.auditSvc.Write(context.Background(),
			user.ID, user.Username, ip,
			audit.ActionLogin, audit.ResourceSystem,
			user.ID, user.Username, "",
		)
	}()
}

func (h *AuthHandler) LoginBySMS(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginBySMSReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if req.Phone == "" || req.Code == "" {
		dto.WriteFail(w, http.StatusBadRequest, 400, "phone and code are required")
		return
	}
	token, user, err := h.svc.LoginBySMS(r.Context(), req.Phone, req.Code)
	if err != nil {
		dto.WriteFail(w, http.StatusUnauthorized, 401, err.Error())
		return
	}
	dto.WriteOK(w, dto.LoginResp{
		Token: token,
		UserInfo: dto.UserInfoResp{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.Username,
			Role:        roleToStr(user.Role),
		},
	})
	ip := clientIP(r)
	go func() {
		_ = h.auditSvc.Write(context.Background(),
			user.ID, user.Username, ip,
			audit.ActionLogin, audit.ResourceSystem,
			user.ID, user.Username, "SMS登录",
		)
	}()
}

func (h *AuthHandler) SendSMSCode(w http.ResponseWriter, r *http.Request) {
	var req dto.SendSMSCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.WriteFail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if req.Phone == "" {
		dto.WriteFail(w, http.StatusBadRequest, 400, "phone is required")
		return
	}
	if err := h.svc.SendSMSCode(r.Context(), req.Phone); err != nil {
		dto.WriteFail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	dto.WriteOK(w, nil)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	writeAudit(r, h.auditSvc, audit.ActionLogout, audit.ResourceSystem, "", middleware.CurrentUsername(r), "")
	dto.WriteOK(w, nil)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.CurrentUserID(r)
	username := middleware.CurrentUsername(r)
	role := middleware.CurrentRole(r)
	dto.WriteOK(w, dto.UserInfoResp{
		ID:          userID,
		Username:    username,
		DisplayName: username,
		Role:        roleToStr(role),
	})
}

// roleToStr 将 int8 角色值转为字符串
func roleToStr(role int8) string {
	if role == 1 {
		return "admin"
	}
	return "user"
}
