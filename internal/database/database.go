// 【第2步：建立数据库连接】
// database 层负责初始化并管理数据库连接池，是整个服务与数据库交互的基础。
// 开发顺序上，定义好 model 后，第二步就是建立数据库连接，
// 后续的 repository 层需要拿到这里提供的 *sql.DB 才能执行 SQL。
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/joho/godotenv/autoload"
)

// Service 是数据库操作的抽象接口，好处是：
//   - 上层代码（server）依赖接口而非具体实现，便于测试时替换为 mock
//   - 明确暴露哪些能力：健康检查、关闭连接、获取底层 *sql.DB
type Service interface {
	Health() map[string]string
	Close() error
	GetDB() *sql.DB
}

// service 是 Service 接口的具体实现，持有真实的 *sql.DB 连接。
type service struct {
	db *sql.DB
}

// 从环境变量读取数据库配置（通过 godotenv/autoload 自动加载 .env 文件）。
var (
	dbname     = os.Getenv("BLUEPRINT_DB_DATABASE")
	password   = os.Getenv("BLUEPRINT_DB_PASSWORD")
	username   = os.Getenv("BLUEPRINT_DB_USERNAME")
	port       = os.Getenv("BLUEPRINT_DB_PORT")
	host       = os.Getenv("BLUEPRINT_DB_HOST")
	dbInstance *service // 单例，避免重复创建连接池
)

// New 创建并返回数据库服务实例（单例模式）。
// 单例保证整个进程只有一个连接池，防止连接数爆炸。
// sql.Open 只解析 DSN 不真正连接，Ping 时才建立实际连接。
func New() Service {
	if dbInstance != nil {
		return dbInstance
	}

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4", username, password, host, port, dbname))
	if err != nil {
		log.Fatal(err)
	}
	// 连接池参数：最大空闲连接数和最大打开连接数都设为 50，
	// ConnMaxLifetime=0 表示连接永不超时（由数据库服务端控制）。
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(50)
	db.SetMaxOpenConns(50)

	dbInstance = &service{db: db}
	return dbInstance
}

// Health 检查数据库连通性，并收集连接池统计信息，供 /health 接口使用。
// 使用 1 秒超时的 context 防止 Ping 长时间阻塞。
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Fatalf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"

	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	if dbStats.OpenConnections > 40 {
		stats["message"] = "The database is experiencing heavy load."
	}
	if dbStats.WaitCount > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}
	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}

	return stats
}

// Close 关闭数据库连接，通常在服务优雅退出时调用。
func (s *service) Close() error {
	log.Printf("Disconnected from database: %s", dbname)
	return s.db.Close()
}

// GetDB 返回底层 *sql.DB，供 repository 层直接执行 SQL 语句。
func (s *service) GetDB() *sql.DB {
	return s.db
}
