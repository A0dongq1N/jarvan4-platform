package spec

import "context"

// RunContext 是 Setup / Default / Teardown 三个生命周期方法统一使用的上下文。
//
// Setup 阶段：VUId=0，Iteration=0，SetupData=nil，可用 Vars/Log/HTTP 等。
// Default 阶段：VUId/Iteration 均有效，SetupData 为 Setup 返回值。
// Teardown 阶段：VUId=0，Iteration=0，SetupData 为 Setup 返回值。
type RunContext struct {
	context.Context

	// VUId 当前虚拟用户编号，从 1 开始，Setup/Teardown 阶段为 0。
	VUId int

	// Iteration 本 VU 已完成的迭代次数，从 0 开始递增，Setup/Teardown 阶段为 0。
	Iteration int64

	// WorkerID 执行本脚本的 Worker 节点 ID。
	WorkerID string

	// Vars 变量上下文，goroutine 级别隔离，VU 间不共享。
	// Env(key) 读取 Master 下发的平台环境变量（只读）。
	// Set/Get 读写当前 VU 的私有变量（跨迭代持久）。
	Vars VarStore

	// SetupData Setup 阶段返回的共享数据，所有 VU 只读访问。
	// Default/Teardown 阶段可用，Setup 阶段为 nil。
	SetupData interface{}

	// HTTP 客户端，最常用协议的快捷方式，内部自动调用 Recorder 上报指标。
	HTTP HTTPClient

	// Check 断言，失败不 panic，计入 fail 并记录原因。
	Check Checker

	// Log 日志，输出会上报到 Master，可在实时看板「日志流」查看。
	Log Logger

	// Sleep 睡眠，优先于 time.Sleep，可被引擎停止信号中断。
	Sleep Sleeper

	// Recorder 协议无关指标记录器。
	// HTTP/gRPC/Redis 等 SDK 内部已自动调用，通常脚本无需直接使用。
	// 仅在使用平台未内置协议时，手动调用记录自定义指标。
	Recorder MetricsRecorder
}
