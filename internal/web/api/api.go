package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/web"
)

var startRuntime = time.Now()

func setupRouter(r *gin.Engine, uc *Usecase) {
	r.Use(
		web.Recover(),
		web.Metrics(),
		web.Logger(),
		// debug 环境中配合 debug 日志级别，记录请求体与响应体
		web.LoggerWithBody(web.DefaultBodyLimit, func(_ *gin.Context) bool {
			// true: 表示忽略记录日志
			// !debug 表示仅调试环境记录
			return !uc.Conf.Runtime.Debug
		}),
	)

	auth := web.AuthMiddleware(uc.Conf.Server.HTTP.JwtSecret)
	r.Any("/health", web.WrapH(uc.getHealth))
	r.GET("/app/metrics/api", web.WrapH(uc.getMetricsAPI))

	versionapi.Register(r, uc.Version, auth)
}
