// 【中间件层：请求鉴权】
// middleware 是请求到达 handler 之前的"拦截器"，用于处理横切关注点（如鉴权、日志、限流）。
// JWT 鉴权中间件的作用：
//   - 验证请求头中的 Bearer Token 是否合法
//   - 将 Token 中的用户信息注入 gin.Context，供后续 handler 读取
//   - 验证失败则直接拒绝请求，不进入业务逻辑
//
// 开发顺序上，中间件在路由注册时挂载（routes.go），因此需要在 handler 之前理解它。
package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"admin-server/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 是 JWT Payload 的自定义结构，嵌入标准的 RegisteredClaims（含过期时间等）。
// 将用户 ID、用户名、角色写入 Token，后续接口可从 Token 中直接读取，无需再查库。
type Claims struct {
	UserID   int64    `json:"userId"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// getSecret 从环境变量读取 JWT 签名密钥，未配置时使用默认值（仅用于开发环境）。
// 生产环境务必通过环境变量配置强密钥。
func getSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "admin-manage-secret-key-2024"
	}
	return []byte(secret)
}

// GenerateTokens 登录成功后调用，生成一对 Token：
//   - accessToken：2小时有效，用于接口鉴权
//   - refreshToken：7天有效，用于无感刷新 accessToken（前端静默续期）
//
// 两个 Token 都使用 HS256 算法签名，签名密钥相同。
func GenerateTokens(user *model.User) (accessToken, refreshToken string, expires time.Time, err error) {
	roles := strings.Split(user.Roles, ",")

	expires = time.Now().Add(2 * time.Hour)
	refreshExpires := time.Now().Add(7 * 24 * time.Hour)

	accessClaims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshClaims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = t.SignedString(getSecret())
	if err != nil {
		return
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = rt.SignedString(getSecret())
	return
}

// ParseToken 解析并验证 Token 字符串，返回其中的 Claims 数据。
// jwt 库会自动校验签名和过期时间，任意一项不通过都会返回 error。
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return getSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// JWTAuth 返回一个 gin 中间件函数，挂载到需要登录才能访问的路由组上。
// 执行流程：
//  1. 读取 Authorization 请求头
//  2. 校验格式是否为 "Bearer <token>"
//  3. 调用 ParseToken 验证 Token 合法性
//  4. 验证通过：将用户信息写入 Context，调用 c.Next() 放行
//  5. 验证失败：返回 401，调用 c.Abort() 终止后续处理
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "A0001", "message": "未授权，请先登录", "data": nil})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "A0001", "message": "Token 格式错误", "data": nil})
			c.Abort()
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "A0001", "message": "Token 无效或已过期", "data": nil})
			c.Abort()
			return
		}

		// 将解析出的用户信息存入 Context，handler 中通过 c.Get("userId") 等方式读取
		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}
