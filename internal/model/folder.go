package model

import "time"

// FileFolder 对应 file_folders 表。
type FileFolder struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ParentID  *int64    `json:"parentId"`
	CreatedBy int64     `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FolderTreeNode 文件夹树节点，含嵌套子节点。
type FolderTreeNode struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	ParentID *int64            `json:"parentId"`
	Children []*FolderTreeNode `json:"children"`
}

// CreateFolderReq 创建文件夹请求。
type CreateFolderReq struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	ParentID *int64 `json:"parentId"`
}

// UpdateFolderReq 更新文件夹（重命名或移动）。
type UpdateFolderReq struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parentId"`
}
