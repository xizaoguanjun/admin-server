// 【第4步：Handler 层 - 处理 HTTP 请求】
// handler 是接口开发的核心业务层，每个函数对应一个 HTTP 接口。
// Handler 的职责：
//  1. 解析并校验请求参数（ShouldBindJSON / ShouldBindQuery）
//  2. 调用 repository 执行数据库操作
//  3. 处理业务逻辑（如密码校验、Token 生成）
//  4. 返回统一格式的响应（调用 response.go 中的方法）
//
// Handler 不直接操作数据库，只通过 repository 层间接访问，保持层次清晰。
package handler

import (
	"strings"

	"admin-server/internal/middleware"
	"admin-server/internal/model"
	"admin-server/internal/repository"

	"github.com/gin-gonic/gin"
)

// AuthHandler 持有 userRepo，通过依赖注入的方式获取数据访问能力。
// 依赖注入（而非直接 new）的好处是便于测试时传入 mock 对象。
type AuthHandler struct {
	userRepo *repository.UserRepository
}

// NewAuthHandler 构造函数，在 routes.go 中调用。
func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// Login godoc
// @Summary      用户登录
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      model.LoginReq  true  "登录参数"
// @Success      200   {object}  handler.LoginRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Failure      403   {object}  handler.ErrorRespWrapper
// @Router       /api/auth/login [post]
// Login 处理 POST /api/auth/login 请求。
// 流程：解析参数 → 查用户 → 校验密码 → 检查状态 → 生成 Token → 返回
func (h *AuthHandler) Login(c *gin.Context) {
	// 1. 解析请求体 JSON，binding tag 会自动校验 required 字段
	var req model.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 2. 查询用户是否存在
	user, err := h.userRepo.FindByUsername(req.Username)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	// 用户不存在或密码不匹配，统一返回"用户名或密码错误"（不暴露具体原因，防止用户枚举攻击）
	if user == nil || !h.userRepo.CheckPassword(user, req.Password) {
		BadRequest(c, "用户名或密码错误")
		return
	}

	// 3. 检查账号状态（status=0 表示已禁用）
	if user.Status == 0 {
		Fail(c, 403, "A0002", "账号已被禁用")
		return
	}

	// 4. 生成 accessToken 和 refreshToken
	accessToken, refreshToken, expires, err := middleware.GenerateTokens(user)
	if err != nil {
		ServerError(c, "生成 Token 失败")
		return
	}

	// 5. 返回登录成功响应
	roles := strings.Split(user.Roles, ",")
	Success(c, model.LoginResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expires:      expires,
		Username:     user.Username,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		Roles:        roles,
		Permissions:  []string{},
	})
}

// Register godoc
// @Summary      用户注册
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      model.RegisterReq  true  "注册参数"
// @Success      200   {object}  handler.RegisterRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/auth/register [post]
// Register 处理 POST /api/auth/register 请求。
// 流程：解析参数 → 检查用户名重复 → 创建用户 → 返回
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 检查用户名是否已被注册
	existing, err := h.userRepo.FindByUsername(req.Username)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if existing != nil {
		BadRequest(c, "用户名已存在")
		return
	}

	// nickname 为空时默认使用 username
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	// 注册用户默认角色为 "user"，状态为启用（1）
	user, err := h.userRepo.Create(&model.CreateUserReq{
		Username: req.Username,
		Password: req.Password,
		Avatar:   req.Avatar,
		Nickname: nickname,
		Phone:    req.Phone,
		Roles:    "user",
		Status:   1,
	})
	if err != nil {
		ServerError(c, "注册失败: "+err.Error())
		return
	}

	SuccessMsg(c, "注册成功", gin.H{"id": user.ID, "username": user.Username})
}
