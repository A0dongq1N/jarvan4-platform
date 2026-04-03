// Package audit 审计日志聚合
package audit

import "time"

// AuditLog 审计日志实体
type AuditLog struct {
	ID           string
	UserID       string
	Username     string
	Action       AuditAction
	ResourceType ResourceType
	ResourceID   string
	ResourceName string
	Detail       string // JSON 扩展字段
	IP           string
	CreatedAt    time.Time
}

// AuditAction 操作类型枚举（与前端 AuditAction 保持一致）
type AuditAction string

const (
	ActionLogin          AuditAction = "login"
	ActionLogout         AuditAction = "logout"
	ActionCreateTask     AuditAction = "create_task"
	ActionUpdateTask     AuditAction = "update_task"
	ActionDeleteTask     AuditAction = "delete_task"
	ActionCopyTask       AuditAction = "copy_task"
	ActionStartExecution AuditAction = "start_execution"
	ActionStopExecution  AuditAction = "stop_execution"
	ActionCreateScript   AuditAction = "create_script"
	ActionDeleteScript   AuditAction = "delete_script"
	ActionCreateProject  AuditAction = "create_project"
	ActionDeleteProject  AuditAction = "delete_project"
	ActionCreateUser     AuditAction = "create_user"
	ActionUpdateUser     AuditAction = "update_user"
	ActionDeleteUser     AuditAction = "delete_user"
	ActionRegisterWorker AuditAction = "register_worker"
	ActionOfflineWorker  AuditAction = "offline_worker"
)

// ResourceType 资源类型枚举（与前端 AuditResourceType 保持一致）
type ResourceType string

const (
	ResourceTask      ResourceType = "task"
	ResourceScript    ResourceType = "script"
	ResourceExecution ResourceType = "execution"
	ResourceProject   ResourceType = "project"
	ResourceUser      ResourceType = "user"
	ResourceWorker    ResourceType = "worker"
	ResourceSystem    ResourceType = "system"
)

// New 审计日志工厂函数
func New(userID, username, ip string, action AuditAction, resourceType ResourceType,
	resourceID, resourceName, detail string) *AuditLog {
	return &AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Detail:       detail,
		IP:           ip,
		CreatedAt:    time.Now(),
	}
}
