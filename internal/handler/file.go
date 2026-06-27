package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"admin-server/internal/model"
	"admin-server/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FileHandler 文件管理接口。
type FileHandler struct {
	fileRepo   *repository.FileRepository
	folderRepo *repository.FolderRepository
	uploadDir  string
}

// NewFileHandler 构造函数。
func NewFileHandler(fileRepo *repository.FileRepository, folderRepo *repository.FolderRepository, uploadDir string) *FileHandler {
	dirs := []string{
		uploadDir,
		filepath.Join(uploadDir, "files"),
		filepath.Join(uploadDir, "chunks"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			panic("failed to create upload dir: " + err.Error())
		}
	}
	return &FileHandler{fileRepo: fileRepo, folderRepo: folderRepo, uploadDir: uploadDir}
}

func (h *FileHandler) getUserID(c *gin.Context) int64 {
	userID, _ := c.Get("userId")
	uid, _ := userID.(int64)
	return uid
}

func (h *FileHandler) validateFolder(folderID int64) error {
	folder, err := h.folderRepo.FindByID(folderID)
	if err != nil {
		return err
	}
	if folder == nil {
		return fmt.Errorf("文件夹不存在")
	}
	return nil
}

func (h *FileHandler) fileStoragePath(folderID int64, storedName string) string {
	return filepath.Join(h.uploadDir, "files", strconv.FormatInt(folderID, 10), storedName)
}

func (h *FileHandler) chunkDir(uploadID string) string {
	return filepath.Join(h.uploadDir, "chunks", uploadID)
}

func (h *FileHandler) chunkPath(uploadID string, chunkIndex int) string {
	return filepath.Join(h.chunkDir(uploadID), strconv.Itoa(chunkIndex))
}

func (h *FileHandler) storedNameFromOriginal(originalName string) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	return uuid.New().String() + ext
}

// List godoc
// @Summary      文件列表
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Param        folderId  query  int     true   "文件夹ID"
// @Param        keyword   query  string  false  "文件名关键词"
// @Param        page      query  int     false  "页码"
// @Param        pageSize  query  int     false  "每页数量"
// @Success      200  {object}  handler.PageRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files [get]
func (h *FileHandler) List(c *gin.Context) {
	var query model.FileListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	list, total, err := h.fileRepo.List(&query)
	if err != nil {
		ServerError(c, "查询失败: "+err.Error())
		return
	}
	Success(c, model.PageResult{
		List:     list,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// GetByID godoc
// @Summary      文件详情
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  int  true  "文件ID"
// @Success      200  {object}  handler.FileRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/{id} [get]
func (h *FileHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的文件ID")
		return
	}
	rec, err := h.fileRepo.FindByID(id)
	if err != nil {
		ServerError(c, "查询失败")
		return
	}
	if rec == nil {
		BadRequest(c, "文件不存在")
		return
	}
	Success(c, rec)
}

// Upload godoc
// @Summary      普通文件上传
// @Tags         文件管理
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file      formData  file   true   "文件"
// @Param        folderId  formData  int    true   "文件夹ID"
// @Success      200  {object}  handler.FileRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/upload [post]
func (h *FileHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择上传文件")
		return
	}
	defer file.Close()

	folderIDStr := c.PostForm("folderId")
	folderID, err := strconv.ParseInt(folderIDStr, 10, 64)
	if err != nil || folderID <= 0 {
		BadRequest(c, "请指定有效的 folderId")
		return
	}
	if err := h.validateFolder(folderID); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if header.Size > model.DirectUploadMaxSize {
		BadRequest(c, "文件超过 10MB，请使用分片上传")
		return
	}

	storedName := h.storedNameFromOriginal(header.Filename)
	savePath := h.fileStoragePath(folderID, storedName)
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		ServerError(c, "创建目录失败")
		return
	}

	out, err := os.Create(savePath)
	if err != nil {
		ServerError(c, "保存文件失败")
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		os.Remove(savePath)
		ServerError(c, "写入文件失败")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	rec, err := h.fileRepo.Create(&model.FileRecord{
		FolderID:     folderID,
		OriginalName: header.Filename,
		StoredName:   storedName,
		MimeType:     mimeType,
		Size:         written,
		UploaderID:   h.getUserID(c),
		Status:       model.FileStatusCompleted,
	})
	if err != nil {
		os.Remove(savePath)
		ServerError(c, "保存记录失败")
		return
	}
	SuccessMsg(c, "上传成功", rec)
}

// UploadChunk godoc
// @Summary      分片上传
// @Tags         文件管理
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData  file    true   "分片数据"
// @Param        fileId       formData  string  true   "上传会话ID"
// @Param        fileName     formData  string  true   "原始文件名"
// @Param        chunkIndex   formData  int     true   "分片索引"
// @Param        totalChunks  formData  int     true   "总分片数"
// @Param        folderId     formData  int     true   "文件夹ID"
// @Param        mimeType     formData  string  false  "MIME类型"
// @Success      200  {object}  handler.MsgRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/upload/chunk [post]
func (h *FileHandler) UploadChunk(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择分片文件")
		return
	}
	defer file.Close()

	uploadID := c.PostForm("fileId")
	fileName := c.PostForm("fileName")
	chunkIndexStr := c.PostForm("chunkIndex")
	totalChunksStr := c.PostForm("totalChunks")
	folderIDStr := c.PostForm("folderId")

	if uploadID == "" || fileName == "" {
		BadRequest(c, "缺少 fileId 或 fileName")
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		BadRequest(c, "无效的 chunkIndex")
		return
	}
	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil || totalChunks <= 0 {
		BadRequest(c, "无效的 totalChunks")
		return
	}
	folderID, err := strconv.ParseInt(folderIDStr, 10, 64)
	if err != nil || folderID <= 0 {
		BadRequest(c, "请指定有效的 folderId")
		return
	}
	if err := h.validateFolder(folderID); err != nil {
		BadRequest(c, err.Error())
		return
	}

	dir := h.chunkDir(uploadID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ServerError(c, "创建分片目录失败")
		return
	}

	chunkFile := h.chunkPath(uploadID, chunkIndex)
	out, err := os.Create(chunkFile)
	if err != nil {
		ServerError(c, "保存分片失败")
		return
	}
	if _, err = io.Copy(out, file); err != nil {
		out.Close()
		ServerError(c, "写入分片失败")
		return
	}
	out.Close()

	SuccessMsg(c, "分片上传成功", gin.H{
		"chunkIndex":  chunkIndex,
		"totalChunks": totalChunks,
	})
}

// ChunkStatus godoc
// @Summary      分片上传状态
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Param        fileId       query  string  true  "上传会话ID"
// @Param        totalChunks  query  int     true  "总分片数"
// @Success      200  {object}  handler.ChunkStatusRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/upload/chunk/status [get]
func (h *FileHandler) ChunkStatus(c *gin.Context) {
	uploadID := c.Query("fileId")
	totalChunksStr := c.Query("totalChunks")
	if uploadID == "" {
		BadRequest(c, "缺少 fileId")
		return
	}
	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil || totalChunks <= 0 {
		BadRequest(c, "无效的 totalChunks")
		return
	}

	var uploaded []int
	for i := 0; i < totalChunks; i++ {
		if _, err := os.Stat(h.chunkPath(uploadID, i)); err == nil {
			uploaded = append(uploaded, i)
		}
	}
	if uploaded == nil {
		uploaded = []int{}
	}
	Success(c, model.ChunkStatusResp{UploadedChunks: uploaded})
}

// Merge godoc
// @Summary      合并分片
// @Tags         文件管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  model.MergeFileReq  true  "合并参数"
// @Success      200   {object}  handler.FileRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/files/upload/merge [post]
func (h *FileHandler) Merge(c *gin.Context) {
	var req model.MergeFileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.validateFolder(req.FolderID); err != nil {
		BadRequest(c, err.Error())
		return
	}

	for i := 0; i < req.TotalChunks; i++ {
		if _, err := os.Stat(h.chunkPath(req.FileID, i)); os.IsNotExist(err) {
			BadRequest(c, fmt.Sprintf("分片 %d 缺失，无法合并", i))
			return
		}
	}

	storedName := h.storedNameFromOriginal(req.FileName)
	savePath := h.fileStoragePath(req.FolderID, storedName)
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		ServerError(c, "创建目录失败")
		return
	}

	out, err := os.Create(savePath)
	if err != nil {
		ServerError(c, "创建文件失败")
		return
	}

	var totalSize int64
	for i := 0; i < req.TotalChunks; i++ {
		chunkFile, err := os.Open(h.chunkPath(req.FileID, i))
		if err != nil {
			out.Close()
			os.Remove(savePath)
			ServerError(c, "读取分片失败")
			return
		}
		n, err := io.Copy(out, chunkFile)
		chunkFile.Close()
		if err != nil {
			out.Close()
			os.Remove(savePath)
			ServerError(c, "合并分片失败")
			return
		}
		totalSize += n
	}
	out.Close()

	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	rec, err := h.fileRepo.Create(&model.FileRecord{
		FolderID:     req.FolderID,
		OriginalName: req.FileName,
		StoredName:   storedName,
		MimeType:     mimeType,
		Size:         totalSize,
		UploaderID:   h.getUserID(c),
		Status:       model.FileStatusCompleted,
	})
	if err != nil {
		os.Remove(savePath)
		ServerError(c, "保存记录失败")
		return
	}

	os.RemoveAll(h.chunkDir(req.FileID))
	SuccessMsg(c, "上传成功", rec)
}

// Preview godoc
// @Summary      文件预览
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      octet-stream
// @Param        id  path  int  true  "文件ID"
// @Success      200  {file}  binary
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/{id}/preview [get]
func (h *FileHandler) Preview(c *gin.Context) {
	h.serveFile(c, "inline")
}

// Download godoc
// @Summary      文件下载
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      octet-stream
// @Param        id  path  int  true  "文件ID"
// @Success      200  {file}  binary
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/{id}/download [get]
func (h *FileHandler) Download(c *gin.Context) {
	h.serveFile(c, "attachment")
}

func (h *FileHandler) serveFile(c *gin.Context, disposition string) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的文件ID")
		return
	}
	rec, err := h.fileRepo.FindByID(id)
	if err != nil {
		ServerError(c, "查询失败")
		return
	}
	if rec == nil {
		BadRequest(c, "文件不存在")
		return
	}

	path := h.fileStoragePath(rec.FolderID, rec.StoredName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		BadRequest(c, "文件不存在于磁盘")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, rec.OriginalName))
	c.Header("Content-Type", rec.MimeType)
	c.File(path)
}

// Delete godoc
// @Summary      删除文件
// @Tags         文件管理
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  int  true  "文件ID"
// @Success      200  {object}  handler.MsgRespWrapper
// @Failure      400  {object}  handler.ErrorRespWrapper
// @Router       /api/files/{id} [delete]
func (h *FileHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的文件ID")
		return
	}

	rec, err := h.fileRepo.Delete(id)
	if err != nil {
		ServerError(c, "删除失败")
		return
	}
	if rec == nil {
		BadRequest(c, "文件不存在")
		return
	}

	path := h.fileStoragePath(rec.FolderID, rec.StoredName)
	_ = os.Remove(path)

	SuccessMsg(c, "删除成功", nil)
}
