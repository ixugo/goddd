package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ixugo/goddd/pkg/reason"
)

const (
	KeyUserID      = "uid"
	KeyLevel       = "level"
	KeyRoleID      = "role_id"
	KeyUsername    = "username"
	KeyTokenString = "token"
)

// Claims ...
type Claims struct {
	Data map[string]any
	jwt.RegisteredClaims
}

type ClaimsData map[string]any

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

func (c ClaimsData) SetRole(role string) ClaimsData {
	c[KeyRoleID] = role
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
func GetUID(c *gin.Context) int {
	return c.GetInt(KeyUserID)
}

// GetUsername 获取用户名
func GetUsername(c *gin.Context) string {
	return c.GetString(KeyUsername)
}

// GetRole 获取用户角色
func GetRoleID(c *gin.Context) int {
	return c.GetInt(KeyRoleID)
}

func GetLevel(c *gin.Context) int {
	v, exist := c.Get(KeyLevel)
	if exist {
		return v.(int)
	}
	return 12
}

func AuthLevel(level int) gin.HandlerFunc {
	// 等级从1开始，等级越小，权限越大
	return func(c *gin.Context) {
		l := c.GetInt("level")
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
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, fmt.Errorf("解析失败")
	}
	c, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("令牌类型错误")
	}
	return c, nil
}

type TokenOptions func(*Claims)

// WithExpiresAt 设置过期时间
func WithExpiresAt(expiresAt time.Time) TokenOptions {
	return func(c *Claims) {
		c.ExpiresAt = jwt.NewNumericDate(expiresAt)
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
