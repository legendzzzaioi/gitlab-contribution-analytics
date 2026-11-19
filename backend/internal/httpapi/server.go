package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"gitlab-analytics/internal/config"
	"gitlab-analytics/internal/model"
	"gitlab-analytics/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Server HTTP服务器
type Server struct {
	db        *gorm.DB
	cfg       config.Config
	syncer    *service.SyncService
	scheduler *service.Scheduler
}

// New 创建HTTP服务器
func New(db *gorm.DB, cfg config.Config, syncer *service.SyncService) *Server {
	return &Server{db: db, cfg: cfg, syncer: syncer}
}

// SetScheduler 设置调度器
func (s *Server) SetScheduler(scheduler *service.Scheduler) { s.scheduler = scheduler }

// Router 创建路由
func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors())
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.POST("/api/auth/login", s.login)
	api := r.Group("/api")
	api.Use(s.auth())
	api.GET("/me", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"username": s.cfg.AdminUsername}) })
	api.GET("/settings", s.getSettings)
	api.PUT("/settings", s.updateSettings)
	api.POST("/settings/test", s.testSettings)
	api.GET("/groups", s.listGroups)
	api.GET("/projects", s.listProjects)
	api.GET("/users", s.listUsers)
	api.PATCH("/users/:id", s.updateUser)
	api.POST("/users/:id/aliases", s.createAlias)
	api.DELETE("/aliases/:id", s.deleteAlias)
	api.GET("/dashboard", s.dashboard)
	api.GET("/contributions", s.contributions)
	api.GET("/commits", s.commits)
	api.GET("/export/commits.csv", s.exportCommits)
	api.GET("/sync/runs", s.syncRuns)
	api.GET("/sync/status", s.syncStatus)
	api.POST("/sync/run", s.startSync)
	api.GET("/permissions/tree", s.getPermissionsTree)
	api.GET("/permissions/users", s.getUsersWithPermissions)
	api.GET("/permissions/users/:userId", s.getUserPermissions)
	return r
}

// cors 跨域中间件
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// login 登录
func (s *Server) login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "请输入账号和密码")
		return
	}
	validUser := subtle.ConstantTimeCompare([]byte(input.Username), []byte(s.cfg.AdminUsername)) == 1
	validPassword := subtle.ConstantTimeCompare([]byte(input.Password), []byte(s.cfg.AdminPassword)) == 1
	if !validUser || !validPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "账号或密码错误"})
		return
	}
	claims := jwt.MapClaims{"sub": s.cfg.AdminUsername, "exp": time.Now().Add(2 * time.Hour).Unix(), "iat": time.Now().Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expiresIn": 43200})
}

// auth 认证中间件
func (s *Server) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "请先登录"})
			c.Abort()
			return
		}
		token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(s.cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// badRequest 请求参数错误
func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"message": message})
}

// internalError 服务器内部错误
func internalError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"message": "服务器处理失败", "detail": err.Error()})
}

// settingResponse 设置响应
func settingResponse(setting model.Setting) gin.H {
	masked := ""
	if setting.GitLabToken != "" {
		masked = "********" + setting.GitLabToken[max(0, len(setting.GitLabToken)-4):]
	}
	return gin.H{"gitlabUrl": setting.GitLabURL, "tokenConfigured": setting.GitLabToken != "", "maskedToken": masked, "rootGroupIds": setting.RootGroupIDs, "syncCron": setting.SyncCron, "lastSuccessAt": setting.LastSuccessAt}
}

// max 返回两个整数中的最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
