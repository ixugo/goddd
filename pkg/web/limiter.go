package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/conc"
	"github.com/ixugo/goddd/pkg/reason"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器
// 可以在 filter 中执行 AbortWithStatusJSON 相关操作，用于替代默认行为
// r 每秒允许发生的事件
// b 最大桶容量，处理突发事件
// filter 达到限流时的处理，可以过滤某些路由，也可以自定义返回错误，如果 filter 不为空，则达到限流后不会有任何动作
func RateLimiter(r rate.Limit, b int, filter ...gin.HandlerFunc) gin.HandlerFunc {
	l := rate.NewLimiter(rate.Limit(r), b)

	var fn gin.HandlerFunc
	if len(filter) > 0 {
		fn = filter[0]
	}

	return func(c *gin.Context) {
		if !l.Allow() {
			if fn != nil {
				fn(c)
				return
			}
			c.AbortWithStatusJSON(400, gin.H{"msg": "服务器繁忙"})
			return
		}
		c.Next()
	}
}

// IPRateLimiter IP 限流器
// 可以在 filter 中执行 AbortWithStatusJSON 相关操作，用于替代默认行为
// r 每秒允许发生的事件
// b 最大桶容量，处理突发事件
// filter 达到限流时的处理，可以过滤某些路由，也可以自定义返回错误，如果 filter 不为空，则达到限流后不会有任何动作
// example:
//
//		IPRateLimiterForGin(1, 10, func(c *gin.Context) {
//	     	// 指定路由放行
//			if c.Request.URL.Path == "/api/v1/login" {
//				c.Next()
//				return
//			}
//			AbortWithStatusJSON(c, reason.ErrRateLimit)
//		})
func IPRateLimiterForGin(r rate.Limit, b int, filter ...gin.HandlerFunc) gin.HandlerFunc {
	limiter := IPRateLimiter(r, b)

	var fn gin.HandlerFunc
	if len(filter) > 0 {
		fn = filter[0]
	}

	return func(c *gin.Context) {
		if !limiter(c.RemoteIP()) {
			if fn != nil {
				fn(c)
				return
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
func LimitContentLength(limit int, filter ...gin.HandlerFunc) gin.HandlerFunc {
	var fn gin.HandlerFunc
	if len(filter) > 0 {
		fn = filter[0]
	}

	return func(c *gin.Context) {
		if c.Request.ContentLength > int64(limit) {
			if fn != nil {
				fn(c)
				return
			}
			AbortWithStatusJSON(c, reason.ErrContentTooLarge)
			return
		}
		c.Next()
	}
}
