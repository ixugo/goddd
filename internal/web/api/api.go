package api

import (
	"expvar"
	"log/slog"
	"net/http"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/web"
)

var startRuntime = time.Now()

func setupRouter(r *gin.Engine, uc *Usecase) {
	r.Use(
		// 格式化输出到控制台，然后记录到日志
		// 此处不做 recover，底层 http.server 也会 recover，但不会输出方便查看的格式
		gin.CustomRecovery(func(c *gin.Context, err any) {
			slog.Error("panic", "err", err, "stack", string(debug.Stack()))
			c.AbortWithStatus(http.StatusInternalServerError)
		}),
		web.Metrics(),
		web.Logger(slog.Default(), func(_ *gin.Context) bool {
			// true:记录请求响应报文
			return uc.Conf.Server.Debug
		}),
	)
	go web.CountGoroutines(10*time.Minute, 20)

	auth := web.AuthMiddleware(uc.Conf.Server.HTTP.JwtSecret)
	r.GET("/health", web.WarpH(uc.getHealth))
	r.GET("/app/metrics/api", web.WarpH(uc.getMetricsAPI))

	versionapi.Register(r, uc.Version, auth)
}

type getHealthOutput struct {
	Version   string    `json:"version"`
	StartAt   time.Time `json:"start_at"`
	GitBranch string    `json:"git_branch"`
	GitHash   string    `json:"git_hash"`
}

func (uc *Usecase) getHealth(_ *gin.Context, _ *struct{}) (getHealthOutput, error) {
	return getHealthOutput{
		Version:   uc.Conf.BuildVersion,
		GitBranch: strings.Trim(expvar.Get("git_branch").String(), `"`),
		GitHash:   strings.Trim(expvar.Get("git_hash").String(), `"`),
		StartAt:   startRuntime,
	}, nil
}

type getMetricsAPIOutput struct {
	RealTimeRequests int64  `json:"real_time_requests"` // 实时请求数
	TotalRequests    int64  `json:"total_requests"`     // 总请求数
	TotalResponses   int64  `json:"total_responses"`    // 总响应数
	RequestTop10     []KV   `json:"request_top10"`      // 请求TOP10
	StatusCodeTop10  []KV   `json:"status_code_top10"`  // 状态码TOP10
	Goroutines       any    `json:"goroutines"`         // 协程数量
	NumGC            uint32 `json:"num_gc"`             // gc 次数
	SysAlloc         uint64 `json:"sys_alloc"`          // 内存占用
	StartAt          string `json:"start_at"`           // 运行时间
}

func (uc *Usecase) getMetricsAPI(_ *gin.Context, _ *struct{}) (*getMetricsAPIOutput, error) {
	req := expvar.Get("request").(*expvar.Int).Value()
	reqs := expvar.Get("requests").(*expvar.Int).Value()
	resps := expvar.Get("responses").(*expvar.Int).Value()
	urls := expvar.Get(`requestURLs`).(*expvar.Map)
	status := expvar.Get(`statusCodes`).(*expvar.Map)
	u := sortExpvarMap(urls, 10)
	s := sortExpvarMap(status, 10)
	g := expvar.Get("goroutine_num").(expvar.Func)

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	return &getMetricsAPIOutput{
		RealTimeRequests: req,
		TotalRequests:    reqs,
		TotalResponses:   resps,
		RequestTop10:     u,
		StatusCodeTop10:  s,
		Goroutines:       g(),
		NumGC:            stats.NumGC,
		SysAlloc:         stats.Sys,
		StartAt:          startRuntime.Format(time.DateTime),
	}, nil
}

type KV struct {
	Key   string
	Value int64
}

func sortExpvarMap(data *expvar.Map, top int) []KV {
	kvs := make([]KV, 0, 8)
	data.Do(func(kv expvar.KeyValue) {
		kvs = append(kvs, KV{
			Key:   kv.Key,
			Value: kv.Value.(*expvar.Int).Value(),
		})
	})

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value > kvs[j].Value
	})

	idx := top
	if l := len(kvs); l < top {
		idx = len(kvs)
	}
	return kvs[:idx]
}
