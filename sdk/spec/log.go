package spec

import "time"

// Logger 日志接口。
// 日志输出会批量上报到 Master，可在实时看板「日志流」查看。
// 高频日志（如每次迭代都打）建议使用 Debug 级别，避免上报量过大。
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Sleeper 睡眠接口。
// 优先于标准库 time.Sleep，可被引擎停止信号中断，避免停止压测时卡在 sleep 上。
type Sleeper interface {
	// Sleep 暂停当前 VU，duration 期间若收到停止信号会提前返回。
	Sleep(duration time.Duration)
}
