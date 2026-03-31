package spec

import "fmt"

// ScriptError 脚本可返回的结构化错误，用于向平台上报业务错误码。
// Default() 直接 return err（普通 Go error）时 Worker 归类为 system 错误；
// 需要上报被测服务返回的业务错误时，return &ScriptError{Type:"business", ...}。
type ScriptError struct {
	// Type 错误分类：
	//   "business" — 被测服务返回的应用层错误（如余额不足、库存不足）
	//   "system"   — 网络超时、连接拒绝、协议层错误
	Type string

	// Code 错误码，平台透传不解释语义。
	//   business: 被测服务的业务错误码，如 "10001"、"ORDER_FAIL"
	//   system:   标准化标识，如 "TIMEOUT"、"CONNECTION_REFUSED"
	Code string

	// Message 错误描述。平台按 (Type, Code) 聚合，Message 取同 Code 下第一条写入报告。
	Message string

	// API 可选，发生错误的接口 pattern（如 "/v1/order/create"）。
	// 填入后报告「接口维度指标」可精确归因；不填时 Worker 尝试从当前请求上下文推断。
	API string

	// LatencyMs 可选，发生错误时的请求耗时（ms）。
	// 填入后错误请求也计入 RT histogram，使分位值更准确。
	LatencyMs int
}

func (e *ScriptError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Code, e.Message)
}
