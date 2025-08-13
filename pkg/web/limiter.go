package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/conc"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器
// 可以在 handler 中执行 AbortWithStatusJSON 相关操作，用于替代默认行为
func RateLimiter(r rate.Limit, b int, handler ...gin.HandlerFunc) gin.HandlerFunc {
	l := rate.NewLimiter(rate.Limit(r), b)

	var fn gin.HandlerFunc
	if len(handler) > 0 {
		fn = handler[0]
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
// 可以在 handler 中执行 AbortWithStatusJSON 相关操作，用于替代默认行为
func IPRateLimiterForGin(r rate.Limit, b int, handler ...gin.HandlerFunc) gin.HandlerFunc {
	limiter := IPRateLimiter(r, b)

	var fn gin.HandlerFunc
	if len(handler) > 0 {
		fn = handler[0]
	}

	return func(c *gin.Context) {
		if !limiter(c.RemoteIP()) {
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
