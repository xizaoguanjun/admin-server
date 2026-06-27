// 【第4步辅助：统一响应格式】
// response.go 定义了所有接口响应的标准结构和快捷方法。
// 统一响应格式的好处：
//   - 前端只需处理一种 JSON 结构，降低对接成本
//   - 错误码（code）统一管理，便于排查问题
//   - 各 handler 直接调用 Success/Fail 等函数，无需重复拼 JSON
//
// 约定：code "00000" 表示成功，"A" 开头表示客户端错误，"B" 开头表示服务端错误。
package handler

import (
	"net/http"

	"admin-server/internal/model"

	"github.com/gin-gonic/gin"
)

// ApiResponse 是所有接口统一的响应体结构。
type ApiResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Success 返回业务成功响应，HTTP 状态码 200，code="00000"。
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, ApiResponse{Code: "00000", Message: "success", Data: data})
}

// SuccessMsg 同 Success，但允许自定义 message（如"创建成功"、"删除成功"）。
func SuccessMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, ApiResponse{Code: "00000", Message: msg, Data: data})
}

// Fail 返回业务失败响应，支持自定义 HTTP 状态码和业务错误码。
func Fail(c *gin.Context, httpStatus int, code, msg string) {
	c.JSON(httpStatus, ApiResponse{Code: code, Message: msg, Data: nil})
}

// BadRequest 快捷方法：请求参数错误，HTTP 400，业务码 "A0400"。
func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, "A0400", msg)
}

// ServerError 快捷方法：服务器内部错误，HTTP 500，业务码 "B0001"。
func ServerError(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, "B0001", msg)
}

// LoginRespWrapper 登录接口响应包装，供 swag 生成文档使用。
type LoginRespWrapper struct {
	Code    string          `json:"code" example:"00000"`
	Message string          `json:"message" example:"success"`
	Data    model.LoginResp `json:"data"`
}

// RegisterRespWrapper 注册接口响应包装。
type RegisterRespWrapper struct {
	Code    string `json:"code" example:"00000"`
	Message string `json:"message" example:"注册成功"`
	Data    struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"data"`
}

// UserRespWrapper 单个用户响应包装。
type UserRespWrapper struct {
	Code    string      `json:"code" example:"00000"`
	Message string      `json:"message" example:"success"`
	Data    model.User  `json:"data"`
}

// PageRespWrapper 分页列表响应包装。
type PageRespWrapper struct {
	Code    string           `json:"code" example:"00000"`
	Message string           `json:"message" example:"success"`
	Data    model.PageResult `json:"data"`
}

// UploadRespWrapper 文件上传响应包装。
type UploadRespWrapper struct {
	Code    string `json:"code" example:"00000"`
	Message string `json:"message" example:"success"`
	Data    struct {
		URL string `json:"url" example:"/uploads/avatars/xxx.png"`
	} `json:"data"`
}

// MsgRespWrapper 仅返回消息的响应包装。
type MsgRespWrapper struct {
	Code    string `json:"code" example:"00000"`
	Message string `json:"message" example:"操作成功"`
	Data    any    `json:"data"`
}

// ErrorRespWrapper 错误响应包装。
type ErrorRespWrapper struct {
	Code    string `json:"code" example:"A0400"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// HealthRespWrapper 健康检查响应包装。
type HealthRespWrapper struct {
	Message string `json:"message" example:"OK"`
}

// FolderRespWrapper 单个文件夹响应包装。
type FolderRespWrapper struct {
	Code    string           `json:"code" example:"00000"`
	Message string           `json:"message" example:"success"`
	Data    model.FileFolder `json:"data"`
}

// FolderTreeRespWrapper 文件夹树响应包装。
type FolderTreeRespWrapper struct {
	Code    string                  `json:"code" example:"00000"`
	Message string                  `json:"message" example:"success"`
	Data    []*model.FolderTreeNode `json:"data"`
}

// FileRespWrapper 单个文件响应包装。
type FileRespWrapper struct {
	Code    string           `json:"code" example:"00000"`
	Message string           `json:"message" example:"success"`
	Data    model.FileRecord `json:"data"`
}

// ChunkStatusRespWrapper 分片状态响应包装。
type ChunkStatusRespWrapper struct {
	Code    string               `json:"code" example:"00000"`
	Message string               `json:"message" example:"success"`
	Data    model.ChunkStatusResp `json:"data"`
}
