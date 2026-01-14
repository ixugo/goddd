package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
)

const ResponseErr = "responseErr"

var defaultDebug = true

// IsRelease 是否是生产环境
func IsRelease() bool {
	return !defaultDebug
}

// SetRelease 设置为生产环境
// 接口不在输出 details 信息
func SetRelease() {
	defaultDebug = false
}

// SetDebug 设置为开发环境
// 接口输出 details 信息，details 会包含敏感信息
func SetDebug() {
	defaultDebug = true
}

// ResponseWriter ...
type ResponseWriter interface {
	JSON(code int, obj any)
	File(filepath string)
	Set(any, any)
	context.Context
	AbortWithStatusJSON(code int, obj any)
}

type HTTPContext interface {
	JSON(int, any)
	Header(key, value string)
	context.Context
}

// Success 通用成功返回
func Success(c HTTPContext, bean any) {
	c.JSON(http.StatusOK, bean)
}

type WithData func(map[string]any)

// Fail 通用错误返回
func Fail(c ResponseWriter, err error, fn ...WithData) {
	out := make(map[string]any)
	if traceID, ok := TraceID(c); ok {
		out["trace_id"] = traceID
	}

	code := 400

	if e1, ok := err.(reason.ErrorInfoer); ok {
		code = e1.GetHTTPCode()
		out["reason"] = e1.GetReason()
		out["msg"] = e1.GetMessage()

		// 是否需要添加 details
		if defaultDebug {
			d := e1.GetDetails()
			if len(d) > 0 {
				out["details"] = d
			}
		}
		// c.JSON(code, out)
		// c.Set(ResponseErr, err.Error())
		// return
	}

	for i := range fn {
		fn[i](out)
	}
	c.JSON(code, out)
	c.Set(ResponseErr, err.Error())
}

func AbortWithStatusJSON(c ResponseWriter, err error, fn ...WithData) {
	out := make(map[string]any)

	err1, ok := err.(reason.ErrorInfoer)

	var code int
	if ok {
		code = err1.GetHTTPCode()
		out["reason"] = err1.GetReason()
		out["msg"] = err1.GetMessage()

		d := err1.GetDetails()
		if defaultDebug && len(d) > 0 {
			out["details"] = d
		}
	}
	if traceID, ok := TraceID(c); ok {
		out["trace_id"] = traceID
	}
	for i := range fn {
		fn[i](out)
	}
	c.AbortWithStatusJSON(code, out)
	c.Set(ResponseErr, err.Error())
}

// WrapHs 包装业务处理函数的同时，支持多个中间件
func WrapHs[I any, O any](fn func(*gin.Context, *I) (O, error), mid ...gin.HandlerFunc) []gin.HandlerFunc {
	return slices.Concat(mid, []gin.HandlerFunc{WrapH(fn)})
}

// WrapH 让函数更专注于业务，一般入参和出参应该是指针类型
// 没有入参时，应该使用 *struct{}
func WrapH[I any, O any](fn func(*gin.Context, *I) (O, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in I
		if unsafe.Sizeof(in) > 0 { // nolint
			switch c.Request.Method {
			case http.MethodGet:
				if err := c.ShouldBindQuery(&in); err != nil {
					Fail(c, reason.ErrBadRequest.With(HanddleJSONErr(err).Error()))
					return
				}
			case http.MethodDelete:
				// https://google-cloud.gitbook.io/api-design-guide/standard_methods?q=delete#delete
				// delete 禁用 body 子句
				if c.Request.ContentLength > 0 {
					contentType := c.Request.Header.Get("Content-Type")
					if contentType == "" {
						Fail(c, reason.ErrBadRequest.With("Content-Type 不能为空"))
						return
					}
					if err := c.ShouldBind(&in); err != nil {
						Fail(c, reason.ErrBadRequest.With(HanddleJSONErr(err).Error()))
						return
					}
				} else {
					if err := c.ShouldBindQuery(&in); err != nil {
						Fail(c, reason.ErrBadRequest.With(HanddleJSONErr(err).Error()))
						return
					}
				}
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				if c.Request.ContentLength > 0 {
					contentType := c.Request.Header.Get("Content-Type")
					if contentType == "" {
						Fail(c, reason.ErrBadRequest.With("Content-Type 不能为空"))
						return
					}
					if err := c.ShouldBind(&in); err != nil {
						Fail(c, reason.ErrBadRequest.With(HanddleJSONErr(err).Error()))
						return
					}
				}
			}
		}
		out, err := fn(c, &in)
		if err != nil {
			Fail(c, err)
			return
		}
		Success(c, out)
	}
}

type ResponseMsg struct {
	Msg string `json:"msg"`
}

// HandlerResponseMsg 获取响应的结果
func HandlerResponseMsg(resp http.Response) error {
	if resp.StatusCode == 200 {
		return nil
	}
	var out ResponseMsg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return reason.ErrServer.SetMsg(out.Msg)
	}
	return reason.ErrServer.SetMsg(resp.Status)
}

func HanddleJSONErr(err error) error {
	if err == nil {
		return nil
	}

	var syntaxError *json.SyntaxError
	var unmarshalTypeError *json.UnmarshalTypeError
	var invalidUnmarshalError *json.InvalidUnmarshalError

	switch {
	case errors.As(err, &syntaxError):
		return fmt.Errorf("格式错误 (位于 %d)", syntaxError.Offset)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return fmt.Errorf("格式错误")
	case errors.As(err, &unmarshalTypeError):
		if unmarshalTypeError.Field != "" {
			return fmt.Errorf("正文包含不正确的格式类型 %q", unmarshalTypeError.Field)
		}
		return fmt.Errorf("正文包含不正确的格式类型 (位于 %d)", unmarshalTypeError.Offset)
	case errors.Is(err, io.EOF):
		return errors.New("正文不能为空")
	case errors.As(err, &invalidUnmarshalError):
		panic(err)
	default:
		return err
	}
}

// CustomMethods 自定义行为封装，其实现方式建议使用在叶子节点的路由上
// 一个最佳实践是 2~3 层路由上，例如 /rooms/:name/sound
//
// 设计参考谷歌 restful 设计指南:
// https://google-cloud.gitbook.io/api-design-guide/custom_methods#http-ying-she
//
// 示例：
// group := r.Group("/rooms", auth)
// CustomMethods(group, "/:name/sound", map[string]func(*gin.Context){
// "muted":   web.WrapH(api.muteRoom),
// "unmuted": web.WrapH(api.unmuteRoom),
// })
// 当找不到对应定义时，会响应 404 状态码
func CustomMethods(g gin.IRouter, relativePath string, data map[string]func(*gin.Context)) {
	for k, v := range data {
		k, _ := strings.CutPrefix(k, ":")
		data[k] = v
	}

	g.POST(relativePath+":method", func(c *gin.Context) {
		active := strings.TrimPrefix(c.Param("method"), ":")
		fn, ok := data[active]
		if ok {
			fn(c)
			return
		}
		c.AbortWithStatus(404)
	})
}
