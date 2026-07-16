package reason

// 客户端错误（HTTP 400）
//
// 参考：https://cloud.google.com/apis/design/errors
var (
	ErrBadRequest           = NewError("ErrBadRequest", "请求参数有误")
	ErrNotFound             = NewError("ErrNotFound", "资源未找到")
	ErrConflict             = NewError("ErrConflict", "操作冲突，请稍后重试")
	ErrAborted              = NewError("ErrAborted", "操作被中止")
	ErrJSON                 = NewError("ErrJSON", "JSON 编解码出错")
	ErrUsedLogic            = NewError("ErrUsedLogic", "使用逻辑错误")
	ErrLoginLimiter         = NewError("ErrLoginLimiter", "触发登录限制")
	ErrFileUpload           = NewError("ErrFileUpload", "文件上传失败")
	ErrUnsupportedMediaType = NewError("ErrUnsupportedMediaType", "不支持的媒体类型")
)

// 客户端错误（非 400 状态码）
var (
	ErrUnauthorized     = NewError("ErrUnauthorized", "未登录或凭证已过期").WithHTTPStatus(401)
	ErrPermissionDenied = NewError("ErrPermissionDenied", "没有该资源的权限").WithHTTPStatus(403)
	ErrFileTooLarge     = NewError("ErrFileTooLarge", "文件大小超出限制").WithHTTPStatus(413)
	ErrContentTooLarge  = NewError("ErrContentTooLarge", "请求体过大").WithHTTPStatus(413)
	ErrTooManyRequests  = NewError("ErrTooManyRequests", "请求频率过高").WithHTTPStatus(429)
)

// 服务端错误（HTTP 500）
var (
	ErrInternal     = NewError("ErrInternal", "服务器内部错误").WithHTTPStatus(500)
	ErrDB           = NewError("ErrStore", "数据发生错误").WithHTTPStatus(500)
	ErrServer       = NewError("ErrServer", "服务器发生错误").WithHTTPStatus(500)
	ErrNetworkError = NewError("ErrNetworkError", "网络连接错误").WithHTTPStatus(500)
)

// 服务端错误（非 500 状态码）
var (
	ErrUnimplemented      = NewError("ErrUnimplemented", "功能尚未实现").WithHTTPStatus(501)
	ErrServiceUnavailable = NewError("ErrServiceUnavailable", "服务暂时不可用").WithHTTPStatus(503)
	ErrTimeout            = NewError("ErrTimeout", "请求超时").WithHTTPStatus(504)
)

// Deprecated: 使用 ErrUnauthorized 代替。
var ErrUnauthorizedToken = NewError("ErrUnauthorizedToken", "用户已过期或错误").WithHTTPStatus(401)

// Deprecated: 使用 ErrTooManyRequests 代替。
var ErrRateLimit = NewError("ErrRateLimit", "请求频率过高").WithHTTPStatus(429)

// 业务错误
var (
	ErrNameOrPasswd    = NewError("ErrNameOrPasswd", "用户名或密码错误")
	ErrCaptchaWrong    = NewError("ErrCaptchaWrong", "验证码错误")
	ErrAccountDisabled = NewError("ErrAccountDisabled", "登录限制")
)

var _ error = NewError("test_new_error", "")
