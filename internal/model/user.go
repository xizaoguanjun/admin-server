// 【第1步：定义数据模型】
// model 层是接口开发的起点，用于定义数据结构。
// 开发一个接口，首先要明确：
//   - 数据库里存什么字段 → User 结构体
//   - 请求参数长什么样 → XxxReq 结构体
//   - 响应数据长什么样 → XxxResp 结构体
//
// 这些结构体会被 repository、handler 等各层共同引用。
package model

import "time"

// User 对应数据库 users 表的字段，是数据在程序中的载体。
// json tag 控制序列化时的字段名；Password 用 json:"-" 表示不对外暴露。
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // 密码哈希，响应时不输出
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Avatar    string    `json:"avatar"`
	Status    int       `json:"status"`
	Roles     string    `json:"roles"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UserListQuery 是列表查询的入参，binding tag 用于参数校验（from query string）。
type UserListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Username string `form:"username"`
	Status   *int   `form:"status"` // 用指针区分"未传"和"传了0"
}

// CreateUserReq 是创建用户接口的请求体。
// binding:"required,min=3,max=50" 会在 ShouldBindJSON 时自动校验。
type CreateUserReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	Avatar   string `json:"avatar"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Roles    string `json:"roles"`
	Status   int    `json:"status"`
}

// UpdateUserReq 是更新用户接口的请求体，字段均为可选（无 required）。
type UpdateUserReq struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
	Roles    string `json:"roles"`
	Status   *int   `json:"status"` // 用指针区分"未传"和"传了0（禁用）"
}

// LoginReq 是登录接口的请求体。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterReq 是注册接口的请求体。
type RegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	Avatar   string `json:"avatar"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
}

// LoginResp 是登录接口的响应体，包含 Token 和用户基本信息。
type LoginResp struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	Expires      time.Time `json:"expires"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	Roles        []string  `json:"roles"`
	Permissions  []string  `json:"permissions"`
}

// PageResult 是分页查询的通用响应结构，List 用 interface{} 可承载任意类型。
type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}
