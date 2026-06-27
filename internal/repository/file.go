package repository

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"admin-server/internal/model"
)

// FileRepository 文件数据访问层。
type FileRepository struct {
	db *sql.DB
}

// NewFileRepository 构造函数。
func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

// InitTable 初始化 files 表。
func (r *FileRepository) InitTable() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			folder_id BIGINT NOT NULL,
			original_name VARCHAR(255) NOT NULL,
			stored_name VARCHAR(255) NOT NULL,
			mime_type VARCHAR(128) NOT NULL DEFAULT '',
			size BIGINT NOT NULL DEFAULT 0,
			uploader_id BIGINT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'completed',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			is_deleted TINYINT NOT NULL DEFAULT 0,
			INDEX idx_folder_id (folder_id),
			INDEX idx_is_deleted (is_deleted)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return err
	}
	log.Println("[InitTable] files table ready")
	return nil
}

// Create 创建文件记录。
func (r *FileRepository) Create(rec *model.FileRecord) (*model.FileRecord, error) {
	result, err := r.db.Exec(
		`INSERT INTO files (folder_id, original_name, stored_name, mime_type, size, uploader_id, status, is_deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		rec.FolderID, rec.OriginalName, rec.StoredName, rec.MimeType, rec.Size, rec.UploaderID, rec.Status,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return r.FindByID(id)
}

// FindByID 根据 ID 查询文件。
func (r *FileRepository) FindByID(id int64) (*model.FileRecord, error) {
	rec := &model.FileRecord{}
	err := r.db.QueryRow(
		`SELECT id, folder_id, original_name, stored_name, mime_type, size, uploader_id, status, created_at, updated_at
		 FROM files WHERE id = ? AND is_deleted = 0`,
		id,
	).Scan(
		&rec.ID, &rec.FolderID, &rec.OriginalName, &rec.StoredName,
		&rec.MimeType, &rec.Size, &rec.UploaderID, &rec.Status,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.URL = fmt.Sprintf("/api/files/%d/preview", rec.ID)
	return rec, nil
}

// List 分页查询文件列表。
func (r *FileRepository) List(query *model.FileListQuery) ([]*model.FileRecord, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	conditions := []string{"is_deleted = 0", "folder_id = ?", "status = ?"}
	args := []interface{}{query.FolderID, model.FileStatusCompleted}

	if query.Keyword != "" {
		conditions = append(conditions, "original_name LIKE ?")
		args = append(args, "%"+query.Keyword+"%")
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM files %s", where), countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset)

	rows, err := r.db.Query(
		fmt.Sprintf(`SELECT id, folder_id, original_name, stored_name, mime_type, size, uploader_id, status, created_at, updated_at
		 FROM files %s ORDER BY id DESC LIMIT ? OFFSET ?`, where),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.FileRecord
	for rows.Next() {
		rec := &model.FileRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.FolderID, &rec.OriginalName, &rec.StoredName,
			&rec.MimeType, &rec.Size, &rec.UploaderID, &rec.Status,
			&rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		rec.URL = fmt.Sprintf("/api/files/%d/preview", rec.ID)
		list = append(list, rec)
	}
	if list == nil {
		list = []*model.FileRecord{}
	}
	return list, total, nil
}

// CountByFolder 统计文件夹下文件数量。
func (r *FileRepository) CountByFolder(folderID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE folder_id = ? AND is_deleted = 0`,
		folderID,
	).Scan(&count)
	return count, err
}

// Delete 软删除文件记录。
func (r *FileRepository) Delete(id int64) (*model.FileRecord, error) {
	rec, err := r.FindByID(id)
	if err != nil || rec == nil {
		return rec, err
	}
	_, err = r.db.Exec(
		`UPDATE files SET is_deleted = 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	return rec, err
}
