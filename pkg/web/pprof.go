package web

import (
	"expvar"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/arl/statsviz"
	"github.com/gin-gonic/gin"
)

// remoteIP 返回 TCP 直连对端 IP。X-Forwarded-For 等请求头可被客户端伪造，
// 白名单校验必须基于协议栈给出的 RemoteAddr，不能依赖 ClientIP
func remoteIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// debugAccess 授权指定 ip 访问
// 校验规则: TCP 直连对端 IP(RemoteAddr, 由协议栈给出不可伪造) 必须在 ips 内;
// 若请求经反向代理而来(携带 X-Forwarded-For), 该头部最右段(由直连代理追加, 可信) 也必须在 ips 内
func debugAccess(ips *[]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		lips := *ips
		if !strings.HasPrefix(c.Request.URL.Path, "/debug/") || len(lips) == 0 {
			c.Next()
			return
		}
		if trustAccess(c, lips) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(400, gin.H{"msg": fmt.Sprintf("%s 无权访问", remoteIP(c))})
	}
}

// trustAccess 判定请求是否通过白名单: 先锚定不可伪造的直连对端, 再核验代理追加的最右段
func trustAccess(c *gin.Context, ips []string) bool {
	if !slices.Contains(ips, remoteIP(c)) {
		return false
	}
	xff := c.GetHeader("X-Forwarded-For")
	if xff == "" {
		// 无 XFF 说明客户端直连, 直连对端已过白名单, 放行
		return true
	}
	last := rightmostIP(xff)
	return last != "" && slices.Contains(ips, last)
}

// rightmostIP 取 X-Forwarded-For 最右非空段。每跳代理只能向尾部追加,
// 故最右段是直连代理所追加的上游 IP, 链中唯一不可被客户端伪造的一段。
// 最右段非法则返回空串, 调用方按拒绝处理(宁可错杀)
func rightmostIP(xff string) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		s := strings.TrimSpace(parts[i])
		if s == "" {
			continue
		}
		if net.ParseIP(s) == nil {
			return ""
		}
		return s
	}
	return ""
}

// SetupPProf 注册 pprof 与 expvar 调试路由, 按 ips 白名单鉴权
//
// 前置要求: 若服务前置反向代理, 代理必须以追加模式设置 X-Forwarded-For, nginx 配置:
//
//	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
//
// 鉴权规则: 直连对端 IP 须在 ips 内; 经代理时 XFF 最右段(直连代理追加的可信段)也须在 ips 内。
// 注意: 多级代理链场景下最右段是直连代理的上游 IP, 按照白名单规则全部允许访问。
func SetupPProf(r gin.IRouter, ips *[]string) {
	debug := r.Group("/debug", debugAccess(ips))
	debug.GET("/pprof/", gin.WrapF(pprof.Index))
	debug.GET("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	debug.GET("/pprof/profile", gin.WrapF(pprof.Profile))
	debug.GET("/pprof/symbol", gin.WrapF(pprof.Symbol))
	debug.POST("/pprof/symbol", gin.WrapF(pprof.Symbol))
	debug.GET("/pprof/trace", gin.WrapF(pprof.Trace))
	debug.GET("/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
	debug.GET("/pprof/block", gin.WrapH(pprof.Handler("block")))
	debug.GET("/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	debug.GET("/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	debug.GET("/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
	debug.GET("/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	debug.GET("/pprof/goroutineleak", gin.WrapH(pprof.Handler("goroutineleak")))
	debug.GET("/vars", gin.WrapH(expvar.Handler()))

	setupStatsviz(debug)
}

// setupStatsviz 延迟初始化 statsviz：首次访问才启动采集协程，避免无人查看时的持续内存开销。
// 采集频率 5s（行业生产级 15s，此处为交互式调试面板适当提高）。
func setupStatsviz(r gin.IRouter) {
	var (
		once sync.Once
		ws   http.HandlerFunc
		idx  http.HandlerFunc
	)

	r.GET("/statsviz/*path", func(ctx *gin.Context) {
		once.Do(func() {
			srv, _ := statsviz.NewServer(statsviz.SendFrequency(5 * time.Second))
			if srv != nil {
				ws = srv.Ws()
				idx = srv.Index()
			}
		})
		if ws == nil {
			ctx.Status(503)
			return
		}
		if ctx.Param("path") == "/ws" {
			ws(ctx.Writer, ctx.Request)
			return
		}
		idx(ctx.Writer, ctx.Request)
	})
}

// SetupMutexProfile 启用互斥锁采样，rate=1 开启采样, rate<=0 关闭采样
func SetupMutexProfile(rate int) {
	runtime.SetBlockProfileRate(rate)
	runtime.SetMutexProfileFraction(rate)
}
