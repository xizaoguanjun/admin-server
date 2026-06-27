package model

import "time"

const (
	FileStatusUploading = "uploading"
	FileStatusCompleted = "completed"

	// DirectUploadMaxSize 普通上传最大 10MB。
	DirectUploadMaxSize = 10 * 1024 * 1024
)

// FileRecord 对应 files 表。
type FileRecord struct {
	ID           int64     `json:"id"`
	FolderID     int64     `json:"folderId"`
	OriginalName string    `json:"originalName"`
	StoredName   string    `json:"storedName"`
	MimeType     string    `json:"mimeType"`
	Size         int64     `json:"size"`
	UploaderID   int64     `json:"uploaderId"`
	Status       string    `json:"status"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// FileListQuery 文件列表查询参数。
type FileListQuery struct {
	FolderID int64  `form:"folderId" binding:"required"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

// MergeFileReq 分片合并请求。
type MergeFileReq struct {
	FileID       string `json:"fileId" binding:"required"`
	FileName     string `json:"fileName" binding:"required"`
	TotalChunks  int    `json:"totalChunks" binding:"required,min=1"`
	FolderID     int64  `json:"folderId" binding:"required"`
	MimeType     string `json:"mimeType"`
}

// ChunkStatusResp 分片上传状态响应。
type ChunkStatusResp struct {
	UploadedChunks []int `json:"uploadedChunks"`
}
