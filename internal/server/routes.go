// 【第5步：路由注册 - 将 URL 与 Handler 绑定】
// routes.go 是接口开发的"最后一步"，将前面所有层连接成完整的请求处理链：
//
//	HTTP 请求
//	  → CORS 中间件（跨域处理）
//	  → 路由匹配（找到对应 handler）
//	  → JWT 中间件（鉴权，仅部分路由）
//	  → Handler 函数（业务逻辑）
//	  → Repository（数据库操作）
//	  → HTTP 响应
//
// 开发新接口的完整步骤：
//  1. model/    → 定义请求/响应结构体
//  2. repository/ → 实现数据库操作方法
//  3. handler/  → 实现 HTTP 处理函数
//  4. routes.go → 在这里注册新路由
package server

import (
	"log"
	"net/http"

	"admin-server/internal/handler"
	"admin-server/internal/middleware"
	"admin-server/internal/repository"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterRoutes 注册所有路由，返回 http.Handler 供 http.Server 使用。
func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default() // 默认包含 Logger 和 Recovery 中间件

	// 全局 CORS 中间件：允许指定来源的跨域请求
	// AllowCredentials=true 配合前端 withCredentials 使用，允许携带 Cookie/Authorization
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8859", "http://localhost:5173", "http://localhost:3000", "http://172.17.87.70:8859"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// 静态文件服务：将 ./uploads 目录映射到 /uploads URL 路径
	// 上传的头像可通过 http://host/uploads/avatars/xxx.jpg 直接访问
	r.Static("/uploads", "./uploads")

	// Swagger API 文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查和根路径，无需鉴权
	r.GET("/", s.HelloWorldHandler)
	r.GET("/health", s.healthHandler)

	// 初始化 Repository 层（注入数据库连接）
	userRepo := repository.NewUserRepository(s.db.GetDB())
	// 确保数据库表结构和初始数据就绪，失败则终止服务启动
	if err := userRepo.InitTable(); err != nil {
		log.Fatalf("failed to init users table: %v", err)
	}

	// 初始化 Handler 层（注入 Repository）
	authHandler := handler.NewAuthHandler(userRepo)
	userHandler := handler.NewUserHandler(userRepo)
	uploadHandler := handler.NewUploadHandler("./uploads/avatars")

	folderRepo := repository.NewFolderRepository(s.db.GetDB())
	if err := folderRepo.InitTable(); err != nil {
		log.Fatalf("failed to init file_folders table: %v", err)
	}
	fileRepo := repository.NewFileRepository(s.db.GetDB())
	if err := fileRepo.InitTable(); err != nil {
		log.Fatalf("failed to init files table: %v", err)
	}
	folderHandler := handler.NewFolderHandler(folderRepo, fileRepo)
	fileHandler := handler.NewFileHandler(fileRepo, folderRepo, "./uploads")

	// 所有业务接口统一挂载在 /api 前缀下，便于区分 API 和静态资源
	api := r.Group("/api")
	{
		// 认证相关接口：无需 JWT，允许未登录访问
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
		}

		// 文件上传接口：需要 JWT 鉴权（middleware.JWTAuth() 会拦截无效 Token）
		upload := api.Group("/upload")
		upload.Use(middleware.JWTAuth())
		{
			upload.POST("/avatar", uploadHandler.UploadAvatar)
		}

		// 用户 CRUD 接口：需要 JWT 鉴权
		// /:id 是路由参数，handler 中用 c.Param("id") 读取
		users := api.Group("/users")
		users.Use(middleware.JWTAuth())
		{
			users.GET("", userHandler.List)          // 分页查询用户列表
			users.POST("", userHandler.Create)       // 创建用户
			users.GET("/:id", userHandler.GetByID)   // 查询单个用户
			users.PUT("/:id", userHandler.Update)    // 更新用户信息
			users.DELETE("/:id", userHandler.Delete) // 删除用户（软删除）
		}

		// 文件夹管理接口
		folders := api.Group("/folders")
		folders.Use(middleware.JWTAuth())
		{
			folders.GET("/tree", folderHandler.GetTree)
			folders.POST("", folderHandler.Create)
			folders.PUT("/:id", folderHandler.Update)
			folders.DELETE("/:id", folderHandler.Delete)
		}

		// 文件管理接口
		files := api.Group("/files")
		files.Use(middleware.JWTAuth())
		{
			files.GET("", fileHandler.List)
			files.POST("/upload", fileHandler.Upload)
			files.POST("/upload/chunk", fileHandler.UploadChunk)
			files.GET("/upload/chunk/status", fileHandler.ChunkStatus)
			files.POST("/upload/merge", fileHandler.Merge)
			files.GET("/:id", fileHandler.GetByID)
			files.GET("/:id/preview", fileHandler.Preview)
			files.GET("/:id/download", fileHandler.Download)
			files.DELETE("/:id", fileHandler.Delete)
		}
	}

	return r
}

// HelloWorldHandler godoc
// @Summary      根路径
// @Tags         系统
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       / [get]
// HelloWorldHandler 根路径处理，用于验证服务是否正常运行。
func (s *Server) HelloWorldHandler(c *gin.Context) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"
	c.JSON(http.StatusOK, resp)
}

// healthHandler godoc
// @Summary      健康检查
// @Tags         系统
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
// healthHandler 健康检查接口，返回数据库连接状态，供运维监控使用。
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, s.db.Health())
}
