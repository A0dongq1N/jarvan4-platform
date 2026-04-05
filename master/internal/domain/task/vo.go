package task

// SceneMode 压测模式
type SceneMode int8

const (
	SceneModeVUStep SceneMode = 1 // VU 阶梯模式
	SceneModeRPS    SceneMode = 2 // 定速 RPS 模式
)

// RPSSubMode RPS 子模式
type RPSSubMode int8

const (
	RPSSubModeFixed RPSSubMode = 1 // 固定速率
	RPSSubModeStep  RPSSubMode = 2 // 阶梯爬升
)

// SceneConfig 场景配置值对象（整体替换，无局部修改）
type SceneConfig struct {
	Mode      SceneMode
	Duration  int // 总时长(秒)，VU阶梯模式下由 steps 各阶段累加
	TimeoutMs int // 单次请求超时(ms)

	// 运行时环境变量（透传给脚本，脚本通过 ctx.Vars.Env(key) 读取）
	EnvVars map[string]string

	// VU 阶梯模式
	Steps []StepConfig

	// RPS 模式
	RPSSubMode  RPSSubMode
	TargetRPS   int
	RPSRampTime int // fixed 子模式爬坡时长(秒)
	RPSSteps    []RPSStepConfig

	// 熔断
	CircuitBreaker CircuitBreakerConfig
}

// StepConfig VU 阶梯一个阶段
type StepConfig struct {
	Concurrent int // 目标并发数
	RampTime   int // 爬坡时长(秒)，0=瞬变
	Duration   int // 本阶段持续时长(秒)
}

// RPSStepConfig RPS 阶梯一个阶段
type RPSStepConfig struct {
	RPS      int
	Duration int
	RampTime int
}

// CircuitBreakerRule 接口级熔断规则
type CircuitBreakerRule struct {
	URLPattern         string  // 接口 pattern，支持 * 通配符
	ErrorRateThreshold float64 // 该接口错误率阈值(%)
	WindowSeconds      int     // 滑动统计窗口(秒)
	MinRequests        int     // 窗口内最少请求数
}

// CircuitBreakerConfig 熔断参数值对象
type CircuitBreakerConfig struct {
	Enabled bool
	// 接口级规则（优先级高于全局兜底）
	Rules []CircuitBreakerRule
	// 全局兜底
	GlobalErrorRateThreshold float64
	GlobalWindowSeconds      int
	GlobalMinRequests        int
}

// TaskScript 任务脚本绑定关系（值对象，通过 Task 行为方法管理）
type TaskScript struct {
	ScriptID   string
	ScriptName string
	Weight     int
}
