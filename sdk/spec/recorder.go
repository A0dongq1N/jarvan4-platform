package spec

import "time"

// MetricsRecorder 协议无关的指标上报接口。
// 所有协议 SDK（http/grpc/redis/kafka/ws/tcp）内部统一通过此接口上报，
// Worker 侧实现负责按 label 分桶聚合后推送给 Master。
// 脚本通常无需直接调用，除非使用平台未内置的自定义协议。
type MetricsRecorder interface {
	// Record 记录一次操作的指标。
	//   label    - 操作标识，建议格式 "协议.操作" 或 gRPC method 路径。
	//              示例："redis.GET"、"kafka.Produce"、"/order.OrderService/Create"
	//   duration - 操作耗时
	//   err      - nil 表示成功；非 nil 计入失败，并按 err.Error() 归类错误原因
	Record(label string, duration time.Duration, err error)

	// Skip 标记最近一次 Record 的结果不计入成功/失败统计。
	// 适用于限流（429）、预热跳过等不应影响压测结果的场景。
	Skip()
}
