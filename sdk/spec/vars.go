package spec

// VarStore 变量存储接口，goroutine 级别隔离。
// 每个 VU goroutine 持有独立实例，VU 间数据不共享。
// 跨迭代持久：同一 VU 的多次 Default 调用共享同一个 VarStore 实例。
type VarStore interface {
	// Set 存储 VU 私有变量。
	Set(key string, value interface{})

	// Get 读取 VU 私有变量，key 不存在时返回 nil。
	Get(key string) interface{}

	// GetString 读取字符串类型变量，key 不存在或类型不匹配时返回 ""。
	GetString(key string) string

	// GetInt 读取整数类型变量，key 不存在或类型不匹配时返回 0。
	GetInt(key string) int

	// Delete 删除 VU 私有变量。
	Delete(key string)

	// Env 读取 Master 下发的平台环境变量（只读）。
	// 来源：任务绑定的 Environment 配置，由 Master 在下发 SubTaskConfig 时注入。
	// key 不存在时返回 ""。
	Env(key string) string
}
