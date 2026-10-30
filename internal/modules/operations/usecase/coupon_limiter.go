// A05 发放限流器：发放高峰时降级，避免把券库打穿。
package usecase

import (
	"sync"
	"time"
)

// IssueLimiter 是按批次维度的滑动窗口限流器。
//
// 它是进程内的：多实例部署时每个实例各限一份，实际总阈值会放大。这是刻意
// 的取舍——宁可多限一点，也不能让并发发放击穿库存校验。真正严格的全局限额
// 由批次自身的 totalCount 原子扣减兜底，限流只是第一道减压阀。
type IssueLimiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

// NewIssueLimiter 创建限流器。
func NewIssueLimiter() *IssueLimiter {
	return &IssueLimiter{events: make(map[string][]time.Time)}
}

// Allow 判断这一批 n 张是否还能发。窗口固定一分钟，计数按张数而非请求数，
// 这样"一次发 500 张"和"发 500 次每次 1 张"受到同样的约束。
func (l *IssueLimiter) Allow(key string, limitPerMinute int64, n int64, now time.Time) bool {
	if limitPerMinute <= 0 || n <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	windowStart := now.Add(-time.Minute)
	recent := make([]time.Time, 0, len(l.events[key])+1)
	var used int64
	for _, at := range l.events[key] {
		if at.After(windowStart) {
			recent = append(recent, at)
			used++
		}
	}
	// 一张券记一个事件点，窗口内用不完就淘汰掉最老的。
	if int64(len(recent))+n > limitPerMinute {
		l.events[key] = recent
		return false
	}
	for i := int64(0); i < n; i++ {
		recent = append(recent, now)
	}
	l.events[key] = recent
	return true
}

// Reset 清空计数，仅供测试使用。
func (l *IssueLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make(map[string][]time.Time)
}
