package handler

import (
	"context"
	"net"
	"net/http"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
)

// clientIP 从请求中提取客户端 IP
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// writeAudit 异步写入审计日志，不阻塞主流程
func writeAudit(
	r *http.Request,
	auditSvc appAudit.AuditUseCase,
	action audit.AuditAction,
	resourceType audit.ResourceType,
	resourceID, resourceName, detail string,
) {
	userID := middleware.CurrentUserID(r)
	username := middleware.CurrentUsername(r)
	ip := clientIP(r)
	go func() {
		_ = auditSvc.Write(context.Background(),
			userID, username, ip,
			action, resourceType,
			resourceID, resourceName, detail,
		)
	}()
}
