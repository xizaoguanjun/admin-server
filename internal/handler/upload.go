package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	uploadDir string
}

func NewUploadHandler(uploadDir string) *UploadHandler {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic("failed to create upload dir: " + err.Error())
	}
	return &UploadHandler{uploadDir: uploadDir}
}

// UploadAvatar godoc
// @Summary      上传头像
// @Tags         文件上传
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "头像文件"
// @Success      200   {object}  handler.UploadRespWrapper
// @Failure      400   {object}  handler.ErrorRespWrapper
// @Router       /api/upload/avatar [post]
func (h *UploadHandler) UploadAvatar(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择上传文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true,
		".png": true, ".gif": true, ".webp": true,
	}
	if !allowed[ext] {
		BadRequest(c, "只支持 jpg/jpeg/png/gif/webp 格式")
		return
	}

	if header.Size > 5*1024*1024 {
		BadRequest(c, "文件大小不能超过 5MB")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(h.uploadDir, filename)

	out, err := os.Create(savePath)
	if err != nil {
		ServerError(c, "保存文件失败")
		return
	}
	defer out.Close()

	if _, err = io.Copy(out, file); err != nil {
		ServerError(c, "写入文件失败")
		return
	}

	Success(c, gin.H{"url": "/uploads/avatars/" + filename})
}
