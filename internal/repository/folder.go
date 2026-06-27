package repository

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"admin-server/internal/model"
)

const rootFolderName = "全部文件"

// FolderRepository 文件夹数据访问层。
type FolderRepository struct {
	db *sql.DB
}

// NewFolderRepository 构造函数。
func NewFolderRepository(db *sql.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

// InitTable 初始化 file_folders 表并确保根目录存在。
func (r *FolderRepository) InitTable() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS file_folders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			parent_id BIGINT NULL,
			created_by BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			is_deleted TINYINT NOT NULL DEFAULT 0,
			INDEX idx_parent_id (parent_id),
			INDEX idx_is_deleted (is_deleted)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return err
	}
	if err := r.ensureRootFolder(); err != nil {
		return err
	}
	log.Println("[InitTable] file_folders table ready")
	return nil
}

func (r *FolderRepository) ensureRootFolder() error {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM file_folders WHERE parent_id IS NULL AND is_deleted = 0`,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	log.Println("[InitTable] creating root folder")
	_, err = r.db.Exec(
		`INSERT INTO file_folders (name, parent_id, created_by, is_deleted) VALUES (?, NULL, 0, 0)`,
		rootFolderName,
	)
	return err
}

// GetRootID 返回根目录 ID。
func (r *FolderRepository) GetRootID() (int64, error) {
	var id int64
	err := r.db.QueryRow(
		`SELECT id FROM file_folders WHERE parent_id IS NULL AND is_deleted = 0 ORDER BY id ASC LIMIT 1`,
	).Scan(&id)
	return id, err
}

// GetTree 返回嵌套文件夹树。
func (r *FolderRepository) GetTree() ([]*model.FolderTreeNode, error) {
	rows, err := r.db.Query(
		`SELECT id, name, parent_id FROM file_folders WHERE is_deleted = 0 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make(map[int64]*model.FolderTreeNode)
	var roots []*model.FolderTreeNode

	for rows.Next() {
		var id int64
		var name string
		var parentID sql.NullInt64
		if err := rows.Scan(&id, &name, &parentID); err != nil {
			return nil, err
		}
		node := &model.FolderTreeNode{
			ID:       id,
			Name:     name,
			Children: []*model.FolderTreeNode{},
		}
		if parentID.Valid {
			pid := parentID.Int64
			node.ParentID = &pid
		}
		nodes[id] = node
	}

	for _, node := range nodes {
		if node.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[*node.ParentID]
		if ok {
			parent.Children = append(parent.Children, node)
		}
	}
	if roots == nil {
		roots = []*model.FolderTreeNode{}
	}
	return roots, nil
}

// FindByID 根据 ID 查询文件夹。
func (r *FolderRepository) FindByID(id int64) (*model.FileFolder, error) {
	f := &model.FileFolder{}
	var parentID sql.NullInt64
	err := r.db.QueryRow(
		`SELECT id, name, parent_id, created_by, created_at, updated_at
		 FROM file_folders WHERE id = ? AND is_deleted = 0`,
		id,
	).Scan(&f.ID, &f.Name, &parentID, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		pid := parentID.Int64
		f.ParentID = &pid
	}
	return f, nil
}

// Create 创建文件夹。
func (r *FolderRepository) Create(req *model.CreateFolderReq, createdBy int64) (*model.FileFolder, error) {
	var parentID interface{}
	if req.ParentID != nil {
		parentID = *req.ParentID
	}
	result, err := r.db.Exec(
		`INSERT INTO file_folders (name, parent_id, created_by, is_deleted) VALUES (?, ?, ?, 0)`,
		req.Name, parentID, createdBy,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return r.FindByID(id)
}

// UpdateName 重命名文件夹。
func (r *FolderRepository) UpdateName(id int64, name string) (*model.FileFolder, error) {
	_, err := r.db.Exec(
		`UPDATE file_folders SET name = ?, updated_at = ? WHERE id = ? AND is_deleted = 0`,
		name, time.Now(), id,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByID(id)
}

// Move 移动文件夹到新的父目录。
func (r *FolderRepository) Move(id int64, parentID *int64) (*model.FileFolder, error) {
	var parent interface{}
	if parentID != nil {
		parent = *parentID
	}
	_, err := r.db.Exec(
		`UPDATE file_folders SET parent_id = ?, updated_at = ? WHERE id = ? AND is_deleted = 0`,
		parent, time.Now(), id,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByID(id)
}

// CountChildren 统计子文件夹数量。
func (r *FolderRepository) CountChildren(id int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM file_folders WHERE parent_id = ? AND is_deleted = 0`,
		id,
	).Scan(&count)
	return count, err
}

// Delete 软删除文件夹（调用方需确保目录为空）。
func (r *FolderRepository) Delete(id int64) error {
	_, err := r.db.Exec(
		`UPDATE file_folders SET is_deleted = 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	return err
}

// IsDescendant 检查 targetID 是否为 ancestorID 的子孙节点（防止循环移动）。
func (r *FolderRepository) IsDescendant(ancestorID, targetID int64) (bool, error) {
	if ancestorID == targetID {
		return true, nil
	}
	rows, err := r.db.Query(
		`SELECT id, parent_id FROM file_folders WHERE is_deleted = 0`,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	parentMap := make(map[int64]*int64)
	for rows.Next() {
		var id int64
		var parentID sql.NullInt64
		if err := rows.Scan(&id, &parentID); err != nil {
			return false, err
		}
		if parentID.Valid {
			pid := parentID.Int64
			parentMap[id] = &pid
		}
	}

	// 从 targetID 向上遍历，若遇到 ancestorID 则说明 target 是 ancestor 的子孙
	current := targetID
	visited := make(map[int64]bool)
	for {
		if visited[current] {
			break
		}
		visited[current] = true
		if current == ancestorID {
			return true, nil
		}
		parent, ok := parentMap[current]
		if !ok || parent == nil {
			break
		}
		current = *parent
	}
	return false, nil
}

// ExistsSiblingName 检查同级目录下是否存在同名文件夹。
func (r *FolderRepository) ExistsSiblingName(name string, parentID *int64, excludeID int64) (bool, error) {
	conditions := []string{"is_deleted = 0", "name = ?"}
	args := []interface{}{name}
	if parentID == nil {
		conditions = append(conditions, "parent_id IS NULL")
	} else {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, *parentID)
	}
	if excludeID > 0 {
		conditions = append(conditions, "id != ?")
		args = append(args, excludeID)
	}
	var count int
	err := r.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM file_folders WHERE %s", strings.Join(conditions, " AND ")),
		args...,
	).Scan(&count)
	return count > 0, err
}
