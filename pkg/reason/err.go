package reason

// 常用错误
var (
	ErrBadRequest           = NewError("ErrBadRequest", "请求参数有误")
	ErrDB                   = NewError("ErrStore", "数据发生错误")
	ErrServer               = NewError("ErrServer", "服务器发生错误")
	ErrUnauthorizedToken    = NewError("ErrUnauthorizedToken", "用户已过期或错误").SetHTTPStatus(401)
	ErrJSON                 = NewError("ErrJSON", "JSON 编解码出错")
	ErrNotFound             = NewError("ErrNotFound", "资源未找到")
	ErrUsedLogic            = NewError("ErrUsedLogic", "使用逻辑错误")
	ErrLoginLimiter         = NewError("ErrLoginLimiter", "触发登录限制")
	ErrPermissionDenied     = NewError("ErrPermissionDenied", "没有该资源的权限")
	ErrTimeout              = NewError("ErrTimeout", "请求超时")
	ErrTooManyRequests      = NewError("ErrTooManyRequests", "请求频率过高")
	ErrServiceUnavailable   = NewError("ErrServiceUnavailable", "服务暂时不可用")
	ErrNetworkError         = NewError("ErrNetworkError", "网络连接错误")
	ErrFileUpload           = NewError("ErrFileUpload", "文件上传失败")
	ErrFileTooLarge         = NewError("ErrFileTooLarge", "文件大小超出限制")
	ErrUnsupportedMediaType = NewError("ErrUnsupportedMediaType", "不支持的媒体类型")
)

// 业务错误
var (
	ErrNameOrPasswd    = NewError("ErrNameOrPasswd", "用户名或密码错误")
	ErrCaptchaWrong    = NewError("ErrCaptchaWrong", "验证码错误")
	ErrAccountDisabled = NewError("ErrAccountDisabled", "登录限制")
)

var _ error = NewError("test_new_error", "")
