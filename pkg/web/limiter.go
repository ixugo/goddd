package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/reason"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器
// r 每秒允许发生的事件
// b 最大桶容量，处理突发事件
func RateLimiter(r rate.Limit, b int, ignoreFn ...IngoreOption) gin.HandlerFunc {
	l := rate.NewLimiter(rate.Limit(r), b)
	return func(c *gin.Context) {
		if !l.Allow() {
			// 达到限流时，可以放行某些路由，依然占用限流次数
			for _, fn := range ignoreFn {
				if fn(c) {
					c.Next()
					return
				}
			}
			AbortWithStatusJSON(c, reason.ErrRateLimit.SetMsg("服务器繁忙"))
			return
		}
		c.Next()
	}
}

// IPRateLimiter IP 限流器
// 可以在 filter 中执行 AbortWithStatusJSON 相关操作，用于替代默认行为
// r 每秒允许发生的事件
// b 最大桶容量，处理突发事件
// example:
//
//	IPRateLimiterForGin(1, 10, IgnorePrefix("/api/v1/login"))
func IPRateLimiterForGin(r rate.Limit, b int, ignoreFn ...IngoreOption) gin.HandlerFunc {
	limiter := IPRateLimiter(r, b)

	return func(c *gin.Context) {
		if !limiter(c.RemoteIP()) {

			for _, fn := range ignoreFn {
				if fn(c) {
					c.Next()
					return
				}
			}

			AbortWithStatusJSON(c, reason.ErrRateLimit)
			return
		}
		c.Next()
	}
}

// IPRateLimiter IP 限流器
func IPRateLimiter(r rate.Limit, b int) func(ip string) bool {
	cache := conc.NewTTLMap[string, *rate.Limiter]()
	return func(ip string) bool {
		v, ok := cache.Load(ip)
		if !ok {
			v, _ = cache.LoadOrStore(ip, rate.NewLimiter(r, b), 3*time.Minute)
		}
		return v.Allow()
	}
}

// LimitContentLength 限制请求体大小，比如限制 1MB，可以传入 1024*1024
func LimitContentLength(limit int, ignoreFn ...IngoreOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > int64(limit) {
			for _, fn := range ignoreFn {
				if fn(c) {
					c.Next()
					return
				}
			}
			AbortWithStatusJSON(c, reason.ErrContentTooLarge)
			return
		}
		c.Next()
	}
}
