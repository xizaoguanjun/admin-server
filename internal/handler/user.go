// 【第4步：Handler 层 - 用户 CRUD 接口】
// user.go 实现用户管理相关的 5 个接口：列表、创建、查询、更新、删除。
// 这些接口都需要 JWT 鉴权（在 routes.go 中通过中间件统一拦截）。
// handler 层遵循相同模式：解析参数 → 调用 repository → 返回响应。
package handler

import (
	"strconv"

	"admin-server/internal/model"
	"admin-server/internal/repository"

	"github.com/gin-gonic/gin"
)

// UserHandler 持有 userRepo，处理用户管理相关接口。
type UserHandler struct {
	userRepo *repository.UserRepository
}

// NewUserHandler 构造函数，在 routes.go 中调用。
func NewUserHandler(userRepo *repository.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

// List godoc
// @Summary      用户列表
// @Tags         用户管理
// @Security     BearerAuth
// @Produce      json
// @Param        page      query  int     false  "页码"
// @Param        pageSize  query  int     false  "每页数量"
// @Param        username  query  string  false  "用户名筛选"
// @Param        status    query  int     false  "状态筛选"
// @Success      200  {object}  handler.PageRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/users [get]
// List 处理 GET /api/users 请求，支持分页和条件筛选。
// ShouldBindQuery 从 URL query string（?page=1&pageSize=10）解析参数。
func (h *UserHandler) List(c *gin.Context) {
	var query model.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	users, total, err := h.userRepo.List(&query)
	if err != nil {
		ServerError(c, "查询失败: "+err.Error())
		return
	}

	// 返回分页结构，前端可据此渲染分页组件
	Success(c, model.PageResult{
		List:     users,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// Create godoc
// @Summary      创建用户
// @Tags         用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  model.CreateUserReq  true  "创建参数"
// @Success      200   {object}  handler.UserRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/users [post]
// Create 处理 POST /api/users 请求，创建新用户（管理员操作）。
// 与注册接口的区别：管理员可以指定角色和状态；注册接口角色固定为 "user"。
func (h *UserHandler) Create(c *gin.Context) {
	var req model.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 创建前检查用户名唯一性
	existing, err := h.userRepo.FindByUsername(req.Username)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if existing != nil {
		BadRequest(c, "用户名已存在")
		return
	}

	user, err := h.userRepo.Create(&req)
	if err != nil {
		ServerError(c, "创建失败: "+err.Error())
		return
	}

	SuccessMsg(c, "创建成功", user)
}

// GetByID godoc
// @Summary      查询用户详情
// @Tags         用户管理
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "用户 ID"
// @Success      200  {object}  handler.UserRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Failure      404  {object}  handler.ErrorRespWrapper
// @Router       /api/users/{id} [get]
// GetByID 处理 GET /api/users/:id 请求，查询单个用户详情。
// c.Param("id") 读取路由中的动态参数，需要转换为 int64 类型。
func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的用户 ID")
		return
	}

	user, err := h.userRepo.FindByID(id)
	if err != nil {
		ServerError(c, "查询失败")
		return
	}
	// 区分"查询出错"和"用户不存在"，返回不同状态码
	if user == nil {
		Fail(c, 404, "A0404", "用户不存在")
		return
	}

	Success(c, user)
}

// Update godoc
// @Summary      更新用户
// @Tags         用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "用户 ID"
// @Param        body  body  model.UpdateUserReq  true  "更新参数"
// @Success      200   {object}  handler.UserRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Failure      404   {object}  handler.ErrorRespWrapper
// @Router       /api/users/{id} [put]
// Update 处理 PUT /api/users/:id 请求，更新用户信息。
// 只更新请求中携带的字段，未传字段保持原值（repository 层动态拼接 SQL 实现）。
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的用户 ID")
		return
	}

	var req model.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	user, err := h.userRepo.Update(id, &req)
	if err != nil {
		ServerError(c, "更新失败: "+err.Error())
		return
	}
	if user == nil {
		Fail(c, 404, "A0404", "用户不存在")
		return
	}

	SuccessMsg(c, "更新成功", user)
}

// Delete godoc
// @Summary      删除用户
// @Tags         用户管理
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  int  true  "用户 ID"
// @Success      200  {object}  handler.MsgRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Failure      404  {object}  handler.ErrorRespWrapper
// @Router       /api/users/{id} [delete]
// Delete 处理 DELETE /api/users/:id 请求，软删除用户。
// 删除前先查询用户是否存在，存在才执行删除，给出准确的错误提示。
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的用户 ID")
		return
	}

	user, err := h.userRepo.FindByID(id)
	if err != nil {
		ServerError(c, "查询失败")
		return
	}
	if user == nil {
		Fail(c, 404, "A0404", "用户不存在")
		return
	}

	if err := h.userRepo.Delete(id); err != nil {
		ServerError(c, "删除失败: "+err.Error())
		return
	}

	SuccessMsg(c, "删除成功", nil)
}
