package execution

import (
	"time"

	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
	"github.com/Aodongq1n/jarvan4-platform/shared/constant"
)

// TaskRun 压测执行聚合根（充血实体）。
// 状态机：PENDING → RUNNING → SUCCESS / STOPPED / FAILED / CIRCUIT_BROKEN
// 字段通过行为方法修改，外部无法直接赋值，保证不变式。
type TaskRun struct {
	id          string
	taskID      string
	triggerType constant.TriggerType
	triggeredBy string // 操作人用户名

	status    constant.TaskRunStatus
	errorMsg  string
	warningMsg string

	workerSnapshots []WorkerSnapshot
	scriptSnapshots []ScriptSnapshot
	initSteps       map[string]*InitStep

	startTime *time.Time
	endTime   *time.Time
	createdAt time.Time
	updatedAt time.Time
}

// NewTaskRun 工厂函数，创建 PENDING 状态的执行记录
func NewTaskRun(id, taskID, triggeredBy string, triggerType constant.TriggerType) *TaskRun {
	now := time.Now()
	return &TaskRun{
		id:          id,
		taskID:      taskID,
		triggerType: triggerType,
		triggeredBy: triggeredBy,
		status:      constant.TaskRunStatusPending,
		initSteps:   make(map[string]*InitStep),
		createdAt:   now,
		updatedAt:   now,
	}
}

// Reconstruct 从持久化数据重建实体（供 infra 层调用）
func Reconstruct(
	id, taskID, triggeredBy string,
	triggerType constant.TriggerType,
	status constant.TaskRunStatus,
	errorMsg, warningMsg string,
	workers []WorkerSnapshot,
	scripts []ScriptSnapshot,
	steps map[string]*InitStep,
	startTime, endTime *time.Time,
	createdAt, updatedAt time.Time,
) *TaskRun {
	return &TaskRun{
		id:              id,
		taskID:          taskID,
		triggerType:     triggerType,
		triggeredBy:     triggeredBy,
		status:          status,
		errorMsg:        errorMsg,
		warningMsg:      warningMsg,
		workerSnapshots: workers,
		scriptSnapshots: scripts,
		initSteps:       steps,
		startTime:       startTime,
		endTime:         endTime,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

// ── 状态转换 ──────────────────────────────────────────────────────────────

// Start 从 PENDING 转为 RUNNING，写入 Worker/Script 快照（写入后不可变）
func (r *TaskRun) Start(workers []WorkerSnapshot, scripts []ScriptSnapshot, warningMsg string) error {
	if r.status != constant.TaskRunStatusPending {
		return domain.ErrInvalidStateTransition
	}
	now := time.Now()
	r.workerSnapshots = workers
	r.scriptSnapshots = scripts
	r.warningMsg = warningMsg
	r.status = constant.TaskRunStatusRunning
	r.startTime = &now
	r.updatedAt = now
	return nil
}

// Complete 从 RUNNING 转为 SUCCESS
func (r *TaskRun) Complete() error {
	if r.status != constant.TaskRunStatusRunning {
		return domain.ErrInvalidStateTransition
	}
	now := time.Now()
	r.status = constant.TaskRunStatusSuccess
	r.endTime = &now
	r.updatedAt = now
	return nil
}

// Stop 从 RUNNING 转为 STOPPED（手动停止）
func (r *TaskRun) Stop() error {
	if r.status != constant.TaskRunStatusRunning {
		return domain.ErrInvalidStateTransition
	}
	now := time.Now()
	r.status = constant.TaskRunStatusStopped
	r.endTime = &now
	r.updatedAt = now
	return nil
}

// Fail 从 RUNNING 转为 FAILED
func (r *TaskRun) Fail(msg string) error {
	if r.status != constant.TaskRunStatusRunning {
		return domain.ErrInvalidStateTransition
	}
	now := time.Now()
	r.status = constant.TaskRunStatusFailed
	r.errorMsg = msg
	r.endTime = &now
	r.updatedAt = now
	return nil
}

// CircuitBreak 从 RUNNING 转为 CIRCUIT_BROKEN（熔断触发）
func (r *TaskRun) CircuitBreak(reason string) error {
	if r.status != constant.TaskRunStatusRunning {
		return domain.ErrInvalidStateTransition
	}
	now := time.Now()
	r.status = constant.TaskRunStatusCircuitBroken
	r.errorMsg = reason
	r.endTime = &now
	r.updatedAt = now
	return nil
}

// UpdateInitStep 追加或更新初始化步骤状态（PENDING 阶段使用）
func (r *TaskRun) UpdateInitStep(key, label string, status constant.InitStepStatus, detail string, items []string) {
	if r.initSteps == nil {
		r.initSteps = make(map[string]*InitStep)
	}
	if step, ok := r.initSteps[key]; ok {
		step.Status = status
		step.Detail = detail
		step.Items = items
	} else {
		r.initSteps[key] = &InitStep{Key: key, Label: label, Status: status, Detail: detail, Items: items}
	}
	r.updatedAt = time.Now()
}

// ── 只读访问器 ────────────────────────────────────────────────────────────

func (r *TaskRun) ID() string                        { return r.id }
func (r *TaskRun) TaskID() string                    { return r.taskID }
func (r *TaskRun) TriggeredBy() string               { return r.triggeredBy }
func (r *TaskRun) TriggerType() constant.TriggerType { return r.triggerType }
func (r *TaskRun) Status() constant.TaskRunStatus    { return r.status }
func (r *TaskRun) ErrorMsg() string                  { return r.errorMsg }
func (r *TaskRun) WarningMsg() string                { return r.warningMsg }
func (r *TaskRun) StartTime() *time.Time             { return r.startTime }
func (r *TaskRun) EndTime() *time.Time               { return r.endTime }
func (r *TaskRun) CreatedAt() time.Time              { return r.createdAt }
func (r *TaskRun) UpdatedAt() time.Time              { return r.updatedAt }

func (r *TaskRun) WorkerSnapshots() []WorkerSnapshot { return r.workerSnapshots }
func (r *TaskRun) ScriptSnapshots() []ScriptSnapshot { return r.scriptSnapshots }
func (r *TaskRun) InitSteps() map[string]*InitStep   { return r.initSteps }

// IsTerminated 是否已终态
func (r *TaskRun) IsTerminated() bool {
	return r.status == constant.TaskRunStatusSuccess ||
		r.status == constant.TaskRunStatusStopped ||
		r.status == constant.TaskRunStatusFailed ||
		r.status == constant.TaskRunStatusCircuitBroken
}
