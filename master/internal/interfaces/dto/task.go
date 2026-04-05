package dto

// ── 项目 ──────────────────────────────────────────────────────────────────

type CreateProjectReq struct {
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description" binding:"max=512"`
}

type UpdateProjectReq struct {
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description" binding:"max=512"`
}

type ProjectResp struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	TaskCount   int     `json:"taskCount"`
	ScriptCount int     `json:"scriptCount"`
	LastRunAt   *string `json:"lastRunAt"` // ISO 时间字符串，null=从未压测
	CreatedBy   string  `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"` // ISO 时间字符串
	UpdatedAt   string  `json:"updatedAt"` // ISO 时间字符串
}

// ── 任务 ──────────────────────────────────────────────────────────────────

type CreateTaskReq struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description" binding:"max=512"`
}

type UpdateTaskReq struct {
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description" binding:"max=512"`
}

type BindScriptReq struct {
	ScriptID string `json:"scriptId"  binding:"required"`
	Weight   int    `json:"weight"    binding:"required,min=1"`
}

type UpdateScriptWeightReq struct {
	Weight int `json:"weight" binding:"required,min=1"`
}

type UpdateSceneReq struct {
	Mode           string            `json:"mode"` // "step" | "rps"
	Duration       int               `json:"duration"`
	TimeoutMs      int               `json:"timeoutMs"`
	EnvVars        map[string]string `json:"envVars"` // 运行时环境变量（透传给脚本）
	Steps          []StepItem        `json:"steps"`
	RPSMode        string            `json:"rpsMode"` // "fixed" | "step"
	TargetRPS      int               `json:"targetRps"`
	RPSRampTime    int               `json:"rpsRampTime"`
	RPSSteps       []RPSStepItem     `json:"rpsSteps"`
	CircuitBreaker *CircuitBreakerItem `json:"circuitBreaker"`
}

type StepItem struct {
	Concurrent int `json:"concurrent"`
	RampTime   int `json:"rampTime"`
	Duration   int `json:"duration"`
}

type RPSStepItem struct {
	RPS      int `json:"rps"`
	Duration int `json:"duration"`
	RampTime int `json:"rampTime"`
}

type CircuitBreakerRule struct {
	URLPattern         string  `json:"urlPattern"`
	ErrorRateThreshold float64 `json:"errorRateThreshold"`
	WindowSeconds      int     `json:"windowSeconds"`
	MinRequests        int     `json:"minRequests"`
}

type CircuitBreakerItem struct {
	Enabled                  bool                  `json:"enabled"`
	Rules                    []CircuitBreakerRule  `json:"rules"`
	GlobalErrorRateThreshold float64               `json:"globalErrorRateThreshold"`
	GlobalWindowSeconds      int                   `json:"globalWindowSeconds"`
	GlobalMinRequests        int                   `json:"globalMinRequests"`
}

type TaskResp struct {
	ID             string           `json:"id"`
	ProjectID      string           `json:"projectId"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Status         string           `json:"status"`
	Scripts        []ScriptBindResp `json:"scripts"`
	ScenarioConfig *SceneResp       `json:"scenarioConfig,omitempty"`
	CreatedBy      string           `json:"createdBy"`
	CreatedAt      string           `json:"createdAt"` // ISO 时间字符串
	UpdatedAt      string           `json:"updatedAt"` // ISO 时间字符串
}

type ScriptBindResp struct {
	ScriptID   string `json:"scriptId"`
	ScriptName string `json:"scriptName"`
	Weight     int    `json:"weight"`
}

type SceneResp struct {
	Mode           string            `json:"mode"` // "step" | "rps"
	Duration       int               `json:"duration"`
	TimeoutMs      int               `json:"timeoutMs"`
	EnvVars        map[string]string `json:"envVars"` // 运行时环境变量
	Steps          []StepItem        `json:"steps,omitempty"`
	RPSMode        string            `json:"rpsMode,omitempty"` // "fixed" | "step"
	TargetRPS      int               `json:"targetRps,omitempty"`
	RPSSteps       []RPSStepItem     `json:"rpsSteps,omitempty"`
	CircuitBreaker *CircuitBreakerItem `json:"circuitBreaker"`
}
