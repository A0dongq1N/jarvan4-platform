package spec

// ScriptEntry 是每个压测脚本必须实现并导出的接口。
// 导出符号名固定为 "Script"，Worker 通过 plugin.Lookup("Script") 获取。
//
// 三个生命周期方法均接收 *RunContext，统一通过 ctx.Vars.Env() 读取平台下发的环境变量。
// Setup/Teardown 阶段 ctx.VUId=0、ctx.Iteration=0，ctx.SetupData=nil（Setup 中）。
type ScriptEntry interface {
	// Setup 在压测开始前执行一次（全局，非每个 goroutine）。
	// 返回的 data 会传递给每个 VU goroutine 的 Default，以及最终的 Teardown。
	// 典型用途：初始化连接池、预加载测试数据、登录获取共享 token。
	// 若无需前置逻辑，实现为空方法返回 (nil, nil) 即可。
	Setup(ctx *RunContext) (data interface{}, err error)

	// Default 是压测的核心逻辑，每次迭代调用一次。
	// 每次调用对应一个虚拟用户（VU）的一次完整行为。
	// 返回非 nil error 时，本次迭代计入失败；panic 会被引擎 recover，同样计入失败。
	Default(ctx *RunContext) error

	// Teardown 在所有 VU goroutine 结束后执行一次。
	// data 是 Setup 的返回值。
	// 典型用途：清理测试数据、关闭连接池、输出统计摘要。
	Teardown(ctx *RunContext, data interface{}) error
}
