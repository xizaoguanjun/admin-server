// 【第5步：服务启动入口】
// server.go 是整个服务的"组装入口"，负责将所有组件连接在一起：
//   - 读取配置（端口）
//   - 初始化数据库连接
//   - 注册路由
//   - 配置 HTTP Server 参数（超时等）
//
// 最终返回一个标准的 *http.Server，由 main.go 调用 ListenAndServe 启动。
//
// 开发顺序的终点：model → database → repository → middleware → handler → routes → server
package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"admin-server/internal/database"
)

// Server 是应用核心结构体，聚合了端口配置和数据库服务。
// 路由注册方法（RegisterRoutes）挂载在 Server 上，可访问 s.db 进行健康检查等操作。
type Server struct {
	port int
	db   database.Service
}

// NewServer 组装并返回一个配置好的 *http.Server，供 main.go 直接启动。
// 注意：这里有两个变量都叫 NewServer，内层的是 *Server 应用结构体，
// 外层的 server 是标准库的 *http.Server，两者职责不同。
func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	NewServer := &Server{
		port: port,
		db:   database.New(), // 初始化数据库连接池（单例）
	}

	// 配置 HTTP Server：地址、路由处理器、各类超时时间
	// 生产环境必须配置超时，防止慢客户端拖垮服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(), // 注册所有路由
		IdleTimeout:  time.Minute,                // 空闲连接保持时间
		ReadTimeout:  10 * time.Second,           // 读取请求体的最长时间
		WriteTimeout: 30 * time.Second,           // 写入响应的最长时间
	}

	return server
}
