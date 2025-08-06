package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ScrollPager 滚动翻页
type ScrollPager[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next"`
}

// PageOutput 分页数据
type PageOutput struct {
	Total int64 `json:"total"`
	Items any   `json:"items"`
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
func (f PagerFilter) MustSortColumn() string {
	return strings.TrimLeft(f.Sort, "-")
}

// SortColumn 通过对 SortColumn 设置值，仅对允许的值做排序处理
func (f PagerFilter) SortColumn() (string, error) {
	for _, v := range f.SortSafelist {
		if f.Sort == v {
			return strings.TrimLeft(f.Sort, "-"), nil
		}
	}
	return "", fmt.Errorf("%s 不支持排序", f.Sort)
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
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	host := req.Host
	return fmt.Sprintf("%s://%s", scheme, host)
}

// GetHost 提取主机 IP 或域名
// 例如 http://127.0.0.1:8080/health 提取出 127.0.0.1
func GetHost(c *http.Request) string {
	host := c.Host
	if l := strings.Split(host, ":"); len(l) == 2 {
		host = l[0]
	}
	return host
}
