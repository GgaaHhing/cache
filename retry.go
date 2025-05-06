package cache

import "time"

// RetryStrategy 重试策略，我们预计用户可能有自己的实现，所以做成接口
type RetryStrategy interface {
	// Next
	// 第一个返回值：重试的间隔
	// 第二个返回值：要不要继续重试
	Next() (time.Duration, bool)
}
