package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ixugo/goddd/pkg/reason"
)

// 为确保兼容性，以下值不可更改
// uid 和 role_id 是 int 类型，其余未明确标识的是字符串类型
const (
	KeyUserID      = "uid"
	KeyLevel       = "level"
	KeyRoleID      = "role_id"
	KeyUsername    = "username"
	KeyTokenString = "token"
)

// Claims ...
// 注意 int 类型在 json 反序列化后会是 float64
// 即通过 gin.context 获取的数字参数，都要用 GetFloat64
type Claims struct {
	Data map[string]any
	jwt.RegisteredClaims
}

type ClaimsData map[string]any

type Geter interface {
	GetString(key string) string
	Get(key string) (value any, exists bool)
}

type IngoreOption func(*gin.Context) bool

type TokenOptions func(*Claims)

// NewClaimsData 提供了一些默认的设置，例如 SetUserID
// 提供的不够用时，请使用 Set(k,v)，并实现对应的 GetK() 函数
// 也可以匿名嵌套实现更多
func NewClaimsData() ClaimsData {
	return make(ClaimsData)
}

func (c ClaimsData) SetUserID(uid int) ClaimsData {
	c[KeyUserID] = uid
	return c
}

func (c ClaimsData) SetLevel(level int) ClaimsData {
	c[KeyLevel] = level
	return c
}

func (c ClaimsData) SetRoleID(roleID int) ClaimsData {
	c[KeyRoleID] = roleID
	return c
}

func (c ClaimsData) SetUsername(username string) ClaimsData {
	c[KeyUsername] = username
	return c
}

func (c ClaimsData) Set(key string, value any) ClaimsData {
	c[key] = value
	return c
}

// AuthMiddleware 鉴权
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.Request.Header.Get("Authorization")
		// header 中没有时，尝试从 query 参数中取
		if auth == "" {
			auth = c.Query("token")
		}
		const prefix = "Bearer "
		if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
			AbortWithStatusJSON(c, reason.ErrUnauthorizedToken.SetMsg("身份验证失败"))
			return
		}
		claims, err := ParseToken(auth[len(prefix):], secret)
		if err != nil {
			AbortWithStatusJSON(c, reason.ErrUnauthorizedToken.SetMsg("身份验证失败"))
			return
		}
		if err := claims.Valid(); err != nil {
			AbortWithStatusJSON(c, reason.ErrUnauthorizedToken.SetMsg("请重新登录"))
			return
		}

		c.Set(KeyTokenString, auth)
		for k, v := range claims.Data {
			c.Set(k, v)
		}
		c.Next()
	}
}

// GetUID 获取用户 ID
func GetUID(c Geter) int {
	return GetInt(c, KeyUserID)
}

// GetUsername 获取用户名
func GetUsername(c Geter) string {
	return c.GetString(KeyUsername)
}

// GetRole 获取用户角色
func GetRoleID(c Geter) int {
	return GetInt(c, KeyRoleID)
}

// GetToken 获取 token
func GetToken(c Geter) string {
	return c.GetString(KeyTokenString)
}

func GetInt(c Geter, key string) int {
	v, exist := c.Get(key)
	if !exist {
		return 0
	}
	switch v := v.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func GetLevel(c Geter) int {
	return GetInt(c, KeyLevel)
}

// AuthLevel 类似日志，可以使用
// IgnorePrefix,IgnoreMethod,IgnoreBool,IgoreContains 等方法
// 用于限制特定级别的访问，当访问级别大于 level 时响应权限不足，<= level 时放行
// 等级从1开始，等级越小，权限越大
func AuthLevel(level int, ignoreFn ...IngoreOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, fn := range ignoreFn {
			if fn(c) {
				c.Next()
				return
			}
		}

		l := GetLevel(c)
		if l > level || l == 0 {
			Fail(c, reason.ErrBadRequest.SetMsg("权限不足"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// ParseToken 解析 token
func ParseToken(tokenString string, secret string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithoutClaimsValidation())
	return &claims, err
}

// WithExpiresAt 设置指定过期时间
func WithExpiresAt(expiresAt time.Time) TokenOptions {
	return func(c *Claims) {
		c.ExpiresAt = jwt.NewNumericDate(expiresAt)
	}
}

// WithExpires 设置多久过期
func WithExpires(duration time.Duration) TokenOptions {
	return func(c *Claims) {
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(duration))
	}
}

// WithIssuedAt 设置签发时间
func WithIssuedAt(issuedAt time.Time) TokenOptions {
	return func(c *Claims) {
		c.IssuedAt = jwt.NewNumericDate(issuedAt)
	}
}

// WithIssuer 设置签发人
func WithIssuer(issuer string) TokenOptions {
	return func(c *Claims) {
		c.Issuer = issuer
	}
}

// WithNotBefore 设置生效时间
func WithNotBefore(notBefore time.Time) TokenOptions {
	return func(c *Claims) {
		c.NotBefore = jwt.NewNumericDate(notBefore)
	}
}

// NewToken 创建 token
// 秘钥不能为空，默认过期时间是 6 个小时
// WithExpires() 指定过期时间
func NewToken(data map[string]any, secret string, opts ...TokenOptions) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret is required")
	}
	now := time.Now()
	claims := Claims{
		Data: data,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(6 * time.Hour)), // 失效时间
			IssuedAt:  jwt.NewNumericDate(now),                    // 签发时间
			Issuer:    "xx@golang.space",                          // 签发人
		},
	}
	for _, opt := range opts {
		opt(&claims)
	}
	tc := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tc.SignedString([]byte(secret))
}
