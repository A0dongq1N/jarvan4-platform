package dto

// ── 审计日志 ──────────────────────────────────────────────────────────────────

// AuditLogResp 对应前端 AuditLog 结构
type AuditLogResp struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Detail       string `json:"detail,omitempty"`
	IP           string `json:"ip"`
	CreatedAt    string `json:"createdAt"`
}
