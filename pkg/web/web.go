package web

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

// ScrollPageOutput 滚动翻页
type ScrollPageOutput[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next"`
}

// PageOutput 分页数据
type PageOutput[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// PagerFilter 分页过滤
type PagerFilter struct {
	Page         int      `form:"page"`
	Size         int      `form:"size"`
	Sort         string   `form:"sort"`
	SortSafelist []string `json:"-"`
}

func NewPagerFilterMaxSize() PagerFilter {
	return PagerFilter{
		Size: 99999,
	}
}

// DateFilter 日期区间过滤
type DateFilter struct {
	StartMs int64 `form:"start_ms"`
	EndMs   int64 `form:"end_ms"`
}

// StartAt 开始时间
func (d DateFilter) StartAt() time.Time {
	return time.UnixMilli(d.StartMs)
}

// EndAt 结束时间
func (d DateFilter) EndAt() time.Time {
	return time.UnixMilli(d.EndMs)
}

// DefaultStartAt 当为零值或不符合规则时，返回提供的默认值
func (d DateFilter) DefaultStartAt(date time.Time) time.Time {
	if d.StartMs <= 0 || d.StartMs > d.EndMs {
		return date
	}
	return time.UnixMilli(d.StartMs)
}

// DefaultEndAt 当为零值或不符合规则时，返回提供的默认值
func (d DateFilter) DefaultEndAt(date time.Time) time.Time {
	if d.EndMs <= 0 || d.EndMs < d.StartMs {
		return date
	}
	return time.UnixMilli(d.EndMs)
}

// MustSortColumn 忽略安全问题
// 失败如果是空串，则不做排序处理
func (f PagerFilter) MustSortColumn() string {
	column, ok := f.SortColumn()
	if !ok {
		return ""
	}
	return column + " " + f.SortDirection()
}

// SortColumn 通过对 SortColumn 设置值，仅对允许的值做排序处理
func (f PagerFilter) SortColumn() (string, bool) {
	if f.Sort != "" && slices.Contains(f.SortSafelist, f.Sort) {
		return strings.TrimPrefix(f.Sort, "-"), true
	}
	return "", false
}

// SortDirection 如果 sort 携带负号返回倒序，否则返回正序
func (f PagerFilter) SortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// Offset 计算偏离数值
func (f PagerFilter) Offset() int {
	if f.Page < 1 {
		f.Page = 1
	}
	return (f.Page - 1) * f.Size
}

// Limit 每页 10~100 区间
func (f PagerFilter) Limit() int {
	if f.Size <= 1 {
		return 10
	}
	if f.Size > 10000 {
		return 10000
	}
	return f.Size
}

// Limit 限制数值在 min 和 max 之间
func Limit(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func Offset(page, size int) int {
	if page < 1 {
		return 1
	}
	return (page - 1) * size
}

// GetBaseURL 提取请求地址
// 例如 http://127.0.0.1:8080/health 提取出 http://127.0.0.1:8080
func GetBaseURL(req *http.Request) string {
	return fmt.Sprintf("%s://%s", GetScheme(req), req.Host)
}

// GetHost 提取主机 IP 或域名
// 例如 http://127.0.0.1:8080/health 提取出 127.0.0.1
func GetHost(req *http.Request) string {
	host := req.Host
	if l := strings.Split(host, ":"); len(l) == 2 {
		host = l[0]
	}
	return host
}

// GetScheme 获取请求协议
// 例如 http://127.0.0.1:8080/health 提取出 http
func GetScheme(req *http.Request) string {
	if req.URL.Scheme != "" {
		return req.URL.Scheme
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

// XForwardedPrefix 解决反向代理路由问题
func XForwardedPrefix(req *http.Request, path string) string {
	return strings.TrimSuffix(req.Header.Get("X-Forwarded-Prefix"), "/") + path
}
