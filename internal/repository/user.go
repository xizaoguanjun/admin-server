// 【第3步：实现数据访问层（Repository）】
// repository 层专门负责与数据库打交道，封装所有 SQL 操作。
// 上层的 handler 不直接写 SQL，而是调用这里的方法，好处是：
//   - SQL 集中管理，改表结构只需改这一层
//   - handler 层更简洁，只关注业务逻辑
//   - 便于单独对数据库操作进行单元测试
package repository

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"admin-server/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// UserRepository 持有数据库连接，所有用户相关的 SQL 都在这里执行。
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository 是构造函数，在 routes.go 中调用，传入共享的 *sql.DB。
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// InitTable 在服务启动时调用，确保数据库表结构和初始数据就绪。
// 这是"数据库迁移"的简化版本，生产环境通常用专门的迁移工具（如 goose、migrate）。
func (r *UserRepository) InitTable() error {
	if err := r.ensureRolesColumn(); err != nil {
		log.Printf("[InitTable] ensureRolesColumn error: %v", err)
		return err
	}
	log.Println("[InitTable] users table ready")
	if err := r.ensureAdminUser(); err != nil {
		log.Printf("[InitTable] ensureAdminUser error: %v", err)
		return err
	}
	return nil
}

// ensureRolesColumn 检查 roles 字段是否存在，不存在则自动添加。
// 用于兼容旧版本数据库表结构（schema migration）。
func (r *UserRepository) ensureRolesColumn() error {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'roles'`,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		log.Println("[InitTable] adding roles column to users table")
		_, err = r.db.Exec(`ALTER TABLE users ADD COLUMN roles VARCHAR(255) DEFAULT 'user'`)
		return err
	}
	log.Println("[InitTable] roles column already exists")
	return nil
}

// ensureAdminUser 确保默认管理员账号存在。
// 若密码不是 bcrypt 格式（旧数据），则重置为 bcrypt 哈希，提升安全性。
func (r *UserRepository) ensureAdminUser() error {
	var count int
	var passwordHash string
	err := r.db.QueryRow(
		"SELECT COUNT(*), COALESCE(MAX(password_hash),'') FROM users WHERE username = 'admin' AND is_deleted = 0",
	).Scan(&count, &passwordHash)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if count == 0 {
		log.Println("[InitTable] creating default admin user (admin/admin123456)")
		_, err = r.db.Exec(
			"INSERT INTO users (username, password_hash, nickname, roles, status, is_deleted) VALUES (?, ?, ?, ?, ?, ?)",
			"admin", string(hash), "超级管理员", "admin", 1, 0,
		)
		return err
	}

	// 已存在 admin 用户，但密码不是 bcrypt 格式则重置
	if !strings.HasPrefix(passwordHash, "$2") {
		log.Println("[InitTable] resetting admin password to bcrypt (admin/admin123456)")
		_, err = r.db.Exec(
			"UPDATE users SET password_hash = ?, roles = 'admin' WHERE username = 'admin' AND is_deleted = 0",
			string(hash),
		)
		return err
	}

	log.Println("[InitTable] admin user already exists with bcrypt password")
	return nil
}

// Create 向数据库插入新用户记录。
// 密码在这里用 bcrypt 加密（cost=DefaultCost），永远不明文存储。
// 插入成功后调用 FindByID 返回完整用户信息（不含密码哈希）。
func (r *UserRepository) Create(req *model.CreateUserReq) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}
	roles := req.Roles
	if roles == "" {
		roles = "user"
	}
	status := req.Status
	if status == 0 {
		status = 1
	}
	result, err := r.db.Exec(
		"INSERT INTO users (username, password_hash, nickname, email, phone, avatar_url, roles, status, is_deleted) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.Username, string(hash), nickname, req.Email, req.Phone, req.Avatar, roles, status, 0,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return r.FindByID(id)
}

// FindByUsername 根据用户名查询用户，同时查出 password_hash 用于登录验证。
// COALESCE 处理 NULL 字段，避免 Scan 时出错。
// 返回 nil, nil 表示用户不存在（区别于查询出错）。
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, nickname,
		        COALESCE(email,''), COALESCE(phone,''), COALESCE(avatar_url,''),
		        status, COALESCE(roles,'user'), created_at, updated_at
		 FROM users WHERE username = ? AND is_deleted = 0`,
		username,
	).Scan(
		&user.ID, &user.Username, &user.Password,
		&user.Nickname, &user.Email, &user.Phone, &user.Avatar,
		&user.Status, &user.Roles, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindByID 根据 ID 查询用户，不查 password_hash（对外接口不需要密码）。
func (r *UserRepository) FindByID(id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, nickname,
		        COALESCE(email,''), COALESCE(phone,''), COALESCE(avatar_url,''),
		        status, COALESCE(roles,'user'), created_at, updated_at
		 FROM users WHERE id = ? AND is_deleted = 0`,
		id,
	).Scan(
		&user.ID, &user.Username, &user.Nickname,
		&user.Email, &user.Phone, &user.Avatar,
		&user.Status, &user.Roles, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// List 支持分页 + 条件查询。
// 先用 COUNT(*) 查总数，再用 LIMIT/OFFSET 查当前页数据，两次查询保持相同 WHERE 条件。
func (r *UserRepository) List(query *model.UserListQuery) ([]*model.User, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	conditions := []string{"is_deleted = 0"}
	args := []interface{}{}

	if query.Username != "" {
		conditions = append(conditions, "username LIKE ?")
		args = append(args, "%"+query.Username+"%")
	}
	if query.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *query.Status)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM users %s", where), countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset)

	rows, err := r.db.Query(
		fmt.Sprintf(`SELECT id, username, nickname,
		        COALESCE(email,''), COALESCE(phone,''), COALESCE(avatar_url,''),
		        status, COALESCE(roles,'user'), created_at, updated_at
		 FROM users %s ORDER BY id DESC LIMIT ? OFFSET ?`, where),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		err = rows.Scan(
			&u.ID, &u.Username, &u.Nickname,
			&u.Email, &u.Phone, &u.Avatar,
			&u.Status, &u.Roles, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []*model.User{}
	}
	return users, total, nil
}

// Update 动态拼接 SET 子句，只更新请求中携带的非空字段，避免覆盖未传字段。
func (r *UserRepository) Update(id int64, req *model.UpdateUserReq) (*model.User, error) {
	sets := []string{"updated_at = ?"}
	args := []interface{}{time.Now()}

	if req.Nickname != "" {
		sets = append(sets, "nickname = ?")
		args = append(args, req.Nickname)
	}
	if req.Email != "" {
		sets = append(sets, "email = ?")
		args = append(args, req.Email)
	}
	if req.Phone != "" {
		sets = append(sets, "phone = ?")
		args = append(args, req.Phone)
	}
	if req.Avatar != "" {
		sets = append(sets, "avatar_url = ?")
		args = append(args, req.Avatar)
	}
	if req.Roles != "" {
		sets = append(sets, "roles = ?")
		args = append(args, req.Roles)
	}
	if req.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
	}

	args = append(args, id)
	_, err := r.db.Exec(
		fmt.Sprintf("UPDATE users SET %s WHERE id = ? AND is_deleted = 0", strings.Join(sets, ", ")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByID(id)
}

// Delete 使用软删除（将 is_deleted 置为 1），数据不真正从数据库移除，便于审计和恢复。
func (r *UserRepository) Delete(id int64) error {
	_, err := r.db.Exec("UPDATE users SET is_deleted = 1, updated_at = ? WHERE id = ?", time.Now(), id)
	return err
}

// CheckPassword 用 bcrypt 比对用户输入的密码与数据库中存储的哈希值是否一致。
// bcrypt 内置了盐（salt），同一密码每次哈希结果不同，但 CompareHashAndPassword 能正确验证。
func (r *UserRepository) CheckPassword(user *model.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}
