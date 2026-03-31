package spec

import "time"

// HTTPClient HTTP 请求客户端接口。
// 实现由 Worker 注入，内部自动调用 ctx.Recorder 上报每次请求的指标。
type HTTPClient interface {
	Get(url string, opts ...RequestOption) (*HTTPResponse, error)
	Post(url string, body interface{}, opts ...RequestOption) (*HTTPResponse, error)
	Put(url string, body interface{}, opts ...RequestOption) (*HTTPResponse, error)
	Delete(url string, opts ...RequestOption) (*HTTPResponse, error)
	Do(req *HTTPRequest) (*HTTPResponse, error)
}

// RequestOption 请求选项函数。
type RequestOption func(*HTTPRequest)

// HTTPRequest HTTP 请求描述。
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   map[string]string
	Body    interface{}
	Timeout time.Duration
	// Name 显式指定指标 label（URL pattern），用于报告中接口维度统计归类。
	// 不指定时 Worker 自动将路径中的纯数字和 UUID 替换为 :id / :uuid。
	// 对于非数字动态段（slug、hash 等）必须手动指定，否则每个值都成为独立 pattern。
	Name string
}

// HTTPResponse HTTP 响应。
type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Duration   time.Duration // 请求耗时，平台自动记录

	skipped bool // 是否跳过指标统计，通过 Skip() 标记
}

// JSON 通过简单 key 路径提取 JSON 字段值（如 "data.token"）。
func (r *HTTPResponse) JSON(path string) interface{} {
	return jsonExtract(r.Body, path)
}

// Text 返回响应体字符串。
func (r *HTTPResponse) Text() string {
	return string(r.Body)
}

// Skip 标记本次请求不计入成功/失败统计。
// 适用于限流（429）、预热等不应影响压测结果的响应。
func (r *HTTPResponse) Skip() {
	r.skipped = true
}

// IsSkipped 返回是否已被标记为跳过（Worker 引擎内部使用）。
func (r *HTTPResponse) IsSkipped() bool {
	return r.skipped
}

// Checker HTTP/gRPC 断言接口。
// 断言失败不 panic，将本次迭代标记为 fail 并记录原因。
type Checker interface {
	// That 对 HTTP 响应进行链式断言。
	That(res *HTTPResponse) *Assertion
}

// Assertion HTTP 响应断言链。
type Assertion struct {
	res    *HTTPResponse
	failed bool
	reason string
}

// NewAssertion 创建断言链（供 Checker 实现调用）。
func NewAssertion(res *HTTPResponse) *Assertion {
	return &Assertion{res: res}
}

func (a *Assertion) Status(code int) *Assertion {
	if a.failed {
		return a
	}
	if a.res.StatusCode != code {
		a.failed = true
		a.reason = assertReason("status", code, a.res.StatusCode)
	}
	return a
}

func (a *Assertion) StatusIn(codes ...int) *Assertion {
	if a.failed {
		return a
	}
	for _, c := range codes {
		if a.res.StatusCode == c {
			return a
		}
	}
	a.failed = true
	a.reason = assertReasonIn("status", codes, a.res.StatusCode)
	return a
}

func (a *Assertion) BodyContains(s string) *Assertion {
	if a.failed {
		return a
	}
	if !contains(a.res.Body, s) {
		a.failed = true
		a.reason = "body does not contain: " + s
	}
	return a
}

func (a *Assertion) BodyJSON(path, val string) *Assertion {
	if a.failed {
		return a
	}
	got := toString(jsonExtract(a.res.Body, path))
	if got != val {
		a.failed = true
		a.reason = assertReason("body."+path, val, got)
	}
	return a
}

// RTLt 断言响应时间小于 ms 毫秒。
func (a *Assertion) RTLt(ms int) *Assertion {
	if a.failed {
		return a
	}
	if int(a.res.Duration.Milliseconds()) >= ms {
		a.failed = true
		a.reason = assertReason("rt_ms < ", ms, a.res.Duration.Milliseconds())
	}
	return a
}

func (a *Assertion) HeaderExists(key string) *Assertion {
	if a.failed {
		return a
	}
	if _, ok := a.res.Headers[key]; !ok {
		a.failed = true
		a.reason = "header not found: " + key
	}
	return a
}

// Failed 返回断言是否失败（Worker 引擎内部使用）。
func (a *Assertion) Failed() bool { return a.failed }

// Reason 返回失败原因（Worker 引擎内部使用）。
func (a *Assertion) Reason() string { return a.reason }
