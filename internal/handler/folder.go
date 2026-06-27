package handler

import (
	"strconv"

	"admin-server/internal/model"
	"admin-server/internal/repository"

	"github.com/gin-gonic/gin"
)

// FolderHandler 文件夹管理接口。
type FolderHandler struct {
	folderRepo *repository.FolderRepository
	fileRepo   *repository.FileRepository
}

// NewFolderHandler 构造函数。
func NewFolderHandler(folderRepo *repository.FolderRepository, fileRepo *repository.FileRepository) *FolderHandler {
	return &FolderHandler{folderRepo: folderRepo, fileRepo: fileRepo}
}

// GetTree godoc
// @Summary      文件夹树
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  handler.FolderTreeRespWrapper
// @Failure      500  {object}  handler.ErrorRespWrapper
// @Router       /api/folders/tree [get]
func (h *FolderHandler) GetTree(c *gin.Context) {
	tree, err := h.folderRepo.GetTree()
	if err != nil {
		ServerError(c, "查询失败: "+err.Error())
		return
	}
	Success(c, tree)
}

// Create godoc
// @Summary      创建文件夹
// @Tags         文件管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  model.CreateFolderReq  true  "创建参数"
// @Success      200   {object}  handler.FolderRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/folders [post]
func (h *FolderHandler) Create(c *gin.Context) {
	var req model.CreateFolderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(int64)

	if req.ParentID != nil {
		parent, err := h.folderRepo.FindByID(*req.ParentID)
		if err != nil {
			ServerError(c, "服务器错误")
			return
		}
		if parent == nil {
			BadRequest(c, "父目录不存在")
			return
		}
	}

	exists, err := h.folderRepo.ExistsSiblingName(req.Name, req.ParentID, 0)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if exists {
		BadRequest(c, "同级目录下已存在同名文件夹")
		return
	}

	folder, err := h.folderRepo.Create(&req, uid)
	if err != nil {
		ServerError(c, "创建失败: "+err.Error())
		return
	}
	SuccessMsg(c, "创建成功", folder)
}

// Update godoc
// @Summary      更新文件夹
// @Tags         文件管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "文件夹ID"
// @Param        body  body  model.UpdateFolderReq  true  "更新参数"
// @Success      200   {object}  handler.FolderRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/folders/{id} [put]
func (h *FolderHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的文件夹ID")
		return
	}

	folder, err := h.folderRepo.FindByID(id)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if folder == nil {
		BadRequest(c, "文件夹不存在")
		return
	}
	if folder.ParentID == nil {
		BadRequest(c, "根目录不可修改")
		return
	}

	var req model.UpdateFolderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if req.Name == "" && req.ParentID == nil {
		BadRequest(c, "请提供 name 或 parentId")
		return
	}

	var result *model.FileFolder

	if req.Name != "" {
		exists, err := h.folderRepo.ExistsSiblingName(req.Name, folder.ParentID, id)
		if err != nil {
			ServerError(c, "服务器错误")
			return
		}
		if exists {
			BadRequest(c, "同级目录下已存在同名文件夹")
			return
		}
		result, err = h.folderRepo.UpdateName(id, req.Name)
		if err != nil {
			ServerError(c, "更新失败: "+err.Error())
			return
		}
	}

	if req.ParentID != nil {
		if *req.ParentID == id {
			BadRequest(c, "不能将文件夹移动到自身")
			return
		}
		isDesc, err := h.folderRepo.IsDescendant(id, *req.ParentID)
		if err != nil {
			ServerError(c, "服务器错误")
			return
		}
		if isDesc {
			BadRequest(c, "不能将文件夹移动到其子目录下")
			return
		}
		parent, err := h.folderRepo.FindByID(*req.ParentID)
		if err != nil {
			ServerError(c, "服务器错误")
			return
		}
		if parent == nil {
			BadRequest(c, "目标父目录不存在")
			return
		}
		result, err = h.folderRepo.Move(id, req.ParentID)
		if err != nil {
			ServerError(c, "移动失败: "+err.Error())
			return
		}
	}

	if result == nil {
		result = folder
	}
	SuccessMsg(c, "更新成功", result)
}

// Delete godoc
// @Summary      删除文件夹
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  int  true  "文件夹ID"
// @Success      200  {object}  handler.MsgRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/folders/{id} [delete]
func (h *FolderHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的文件夹ID")
		return
	}

	folder, err := h.folderRepo.FindByID(id)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if folder == nil {
		BadRequest(c, "文件夹不存在")
		return
	}
	if folder.ParentID == nil {
		BadRequest(c, "根目录不可删除")
		return
	}

	childCount, err := h.folderRepo.CountChildren(id)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if childCount > 0 {
		BadRequest(c, "目录非空，请先删除子文件夹")
		return
	}

	fileCount, err := h.fileRepo.CountByFolder(id)
	if err != nil {
		ServerError(c, "服务器错误")
		return
	}
	if fileCount > 0 {
		BadRequest(c, "目录非空，请先删除文件")
		return
	}

	if err := h.folderRepo.Delete(id); err != nil {
		ServerError(c, "删除失败: "+err.Error())
		return
	}
	SuccessMsg(c, "删除成功", nil)
}
