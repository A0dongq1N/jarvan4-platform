package model

import "time"

// TaskModel GORM model for `task` table
type TaskModel struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	BizID       string    `gorm:"column:biz_id;type:varchar(64);not null;uniqueIndex"`
	ProjectID   string    `gorm:"column:project_id;type:varchar(64);not null;index"`
	Name        string    `gorm:"column:name;type:varchar(128);not null"`
	Description string    `gorm:"column:description;type:varchar(512)"`
	CreatedBy   string    `gorm:"column:created_by;type:varchar(64);not null"`
	Status      int8      `gorm:"column:status;not null;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TaskModel) TableName() string { return "task" }

// SceneConfigModel GORM model for `scene_config` table
type SceneConfigModel struct {
	ID            uint64  `gorm:"primaryKey;autoIncrement"`
	TaskBizID     string  `gorm:"column:task_id;type:varchar(64);not null;uniqueIndex"`
	Mode          int8    `gorm:"column:mode;not null"`
	Duration      int     `gorm:"column:duration;not null;default:60"`
	RPS           int     `gorm:"column:rps;default:0"`
	RPSRampTime   int     `gorm:"column:rps_ramp_time;default:0"`
	RPSMode       int8    `gorm:"column:rps_mode;default:1"`
	StepsJSON     string  `gorm:"column:steps_json;type:json"`
	RPSStepsJSON  string  `gorm:"column:rps_steps_json;type:json"`
	CBEnabled                int8    `gorm:"column:cb_enabled;not null;default:0"`
	CBRulesJSON              string  `gorm:"column:cb_rules_json;type:json"`
	CBGlobalErrorRate        float64 `gorm:"column:cb_global_error_rate;type:decimal(5,2);default:10.00"`
	CBGlobalWindowSec        int     `gorm:"column:cb_global_window_sec;default:30"`
	CBGlobalMinRequests      int     `gorm:"column:cb_global_min_requests;default:100"`
	TimeoutMs     int     `gorm:"column:timeout_ms;not null;default:5000"`
	EnvVarsJSON   string  `gorm:"column:env_vars_json;type:json"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (SceneConfigModel) TableName() string { return "scene_config" }

// TaskScriptModel GORM model for `task_script` table
type TaskScriptModel struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	TaskBizID  string    `gorm:"column:task_id;type:varchar(64);not null;index"`
	ScriptBizID string   `gorm:"column:script_id;type:varchar(64);not null"`
	ScriptName string    `gorm:"column:script_name;type:varchar(128)"`
	Weight     int       `gorm:"column:weight;not null;default:1"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (TaskScriptModel) TableName() string { return "task_script" }
