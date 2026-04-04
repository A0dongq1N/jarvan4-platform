package handler

import (
	"context"
	"net"
	"net/http"

	appAudit "github.com/Aodongq1n/jarvan4-platform/master/internal/application/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain/audit"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/interfaces/middleware"
)

// clientIP 从请求中提取客户端 IP，IPv6 本地回环映射为 127.0.0.1
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// ::1 是 IPv6 本地回环，映射为 IPv4 格式
	if host == "::1" {
		return "127.0.0.1"
	}
	// ::ffff:x.x.x.x 是 IPv4-mapped IPv6，提取 IPv4 部分
	if parsed := net.ParseIP(host); parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			return v4.String()
		}
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
