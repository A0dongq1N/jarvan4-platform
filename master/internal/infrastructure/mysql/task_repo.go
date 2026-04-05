package mysql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
	domainTask "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/task"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/mysql/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TaskRepo 实现 domain/task.TaskRepo
type TaskRepo struct {
	db *gorm.DB
}

func NewTaskRepo(db *gorm.DB) *TaskRepo { return &TaskRepo{db: db} }

// Save 新增或全量更新 task、scene_config、task_script（事务）
func (r *TaskRepo) Save(ctx context.Context, task *domainTask.Task) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. upsert task
		tm := &model.TaskModel{
			BizID:       task.ID(),
			ProjectID:   task.ProjectID(),
			Name:        task.Name(),
			Description: task.Description(),
			CreatedBy:   task.CreatedBy(),
			Status:      task.Status(),
			CreatedAt:   task.CreatedAt(),
			UpdatedAt:   task.UpdatedAt(),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "biz_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "status", "updated_at"}),
		}).Create(tm).Error; err != nil {
			return err
		}

		// 2. upsert scene_config
		sc := task.SceneConfig()
		stepsJSON, err := json.Marshal(sc.Steps)
		if err != nil {
			return err
		}
		rpsStepsJSON, err := json.Marshal(sc.RPSSteps)
		if err != nil {
			return err
		}
		cbEnabled := int8(0)
		if sc.CircuitBreaker.Enabled {
			cbEnabled = 1
		}
		cbRulesJSON, err := json.Marshal(sc.CircuitBreaker.Rules)
		if err != nil {
			return err
		}
		envVarsJSON, err := json.Marshal(sc.EnvVars)
		if err != nil {
			return err
		}
		scm := &model.SceneConfigModel{
			TaskBizID:           task.ID(),
			Mode:                int8(sc.Mode),
			Duration:            sc.Duration,
			RPS:                 sc.TargetRPS,
			RPSRampTime:         sc.RPSRampTime,
			RPSMode:             int8(sc.RPSSubMode),
			StepsJSON:           string(stepsJSON),
			RPSStepsJSON:        string(rpsStepsJSON),
			CBEnabled:           cbEnabled,
			CBRulesJSON:         string(cbRulesJSON),
			CBGlobalErrorRate:   sc.CircuitBreaker.GlobalErrorRateThreshold,
			CBGlobalWindowSec:   sc.CircuitBreaker.GlobalWindowSeconds,
			CBGlobalMinRequests: sc.CircuitBreaker.GlobalMinRequests,
			TimeoutMs:           sc.TimeoutMs,
			EnvVarsJSON:         string(envVarsJSON),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"mode", "duration", "rps", "rps_ramp_time", "rps_mode",
				"steps_json", "rps_steps_json", "cb_enabled", "cb_rules_json",
				"cb_global_error_rate", "cb_global_window_sec", "cb_global_min_requests",
				"timeout_ms", "env_vars_json", "updated_at",
			}),
		}).Create(scm).Error; err != nil {
			return err
		}

		// 3. 全量重建 task_script（先删后插）
		if err := tx.Where("task_id = ?", task.ID()).Delete(&model.TaskScriptModel{}).Error; err != nil {
			return err
		}
		for _, ts := range task.Scripts() {
			tsm := &model.TaskScriptModel{
				TaskBizID:   task.ID(),
				ScriptBizID: ts.ScriptID,
				ScriptName:  ts.ScriptName,
				Weight:      ts.Weight,
			}
			if err := tx.Create(tsm).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// hydrate 从三张表数据重建领域对象
func (r *TaskRepo) hydrate(ctx context.Context, tm *model.TaskModel) (*domainTask.Task, error) {
	var scm model.SceneConfigModel
	if err := r.db.WithContext(ctx).Where("task_id = ?", tm.BizID).First(&scm).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var tsms []model.TaskScriptModel
	if err := r.db.WithContext(ctx).Where("task_id = ?", tm.BizID).Find(&tsms).Error; err != nil {
		return nil, err
	}

	var steps []domainTask.StepConfig
	if scm.StepsJSON != "" {
		_ = json.Unmarshal([]byte(scm.StepsJSON), &steps)
	}
	var rpsSteps []domainTask.RPSStepConfig
	if scm.RPSStepsJSON != "" {
		_ = json.Unmarshal([]byte(scm.RPSStepsJSON), &rpsSteps)
	}

	var cbRules []domainTask.CircuitBreakerRule
	if scm.CBRulesJSON != "" && scm.CBRulesJSON != "null" {
		_ = json.Unmarshal([]byte(scm.CBRulesJSON), &cbRules)
	}
	if cbRules == nil {
		cbRules = []domainTask.CircuitBreakerRule{}
	}

	var envVars map[string]string
	if scm.EnvVarsJSON != "" && scm.EnvVarsJSON != "null" {
		_ = json.Unmarshal([]byte(scm.EnvVarsJSON), &envVars)
	}

	sc := domainTask.SceneConfig{
		Mode:        domainTask.SceneMode(scm.Mode),
		Duration:    scm.Duration,
		TimeoutMs:   scm.TimeoutMs,
		EnvVars:     envVars,
		Steps:       steps,
		RPSSubMode:  domainTask.RPSSubMode(scm.RPSMode),
		TargetRPS:   scm.RPS,
		RPSRampTime: scm.RPSRampTime,
		RPSSteps:    rpsSteps,
		CircuitBreaker: domainTask.CircuitBreakerConfig{
			Enabled:                  scm.CBEnabled == 1,
			Rules:                    cbRules,
			GlobalErrorRateThreshold: scm.CBGlobalErrorRate,
			GlobalWindowSeconds:      scm.CBGlobalWindowSec,
			GlobalMinRequests:        scm.CBGlobalMinRequests,
		},
	}

	scripts := make([]domainTask.TaskScript, 0, len(tsms))
	for _, ts := range tsms {
		scripts = append(scripts, domainTask.TaskScript{
			ScriptID:   ts.ScriptBizID,
			ScriptName: ts.ScriptName,
			Weight:     ts.Weight,
		})
	}

	return domainTask.Reconstruct(
		tm.BizID, tm.ProjectID, tm.Name, tm.Description, tm.CreatedBy,
		tm.Status, scripts, sc, tm.CreatedAt, tm.UpdatedAt,
	), nil
}

// FindByID 按 BizID 查询。未找到返回 domain.ErrNotFound。
func (r *TaskRepo) FindByID(ctx context.Context, id string) (*domainTask.Task, error) {
	var tm model.TaskModel
	if err := r.db.WithContext(ctx).Where("biz_id = ? AND status = 1", id).First(&tm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return r.hydrate(ctx, &tm)
}

// ListByProjectID 分页查询（按 created_at DESC），仅返回 status=1 的任务。
func (r *TaskRepo) ListByProjectID(ctx context.Context, projectID string, page, pageSize int) ([]*domainTask.Task, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.TaskModel{}).Where("project_id = ? AND status = 1", projectID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	var tms []model.TaskModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tms).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*domainTask.Task, 0, len(tms))
	for i := range tms {
		t, err := r.hydrate(ctx, &tms[i])
		if err != nil {
			return nil, 0, err
		}
		result = append(result, t)
	}
	return result, total, nil
}

// Delete 软删除（status=0）
func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.TaskModel{}).
		Where("biz_id = ?", id).
		Update("status", 0).Error
}
