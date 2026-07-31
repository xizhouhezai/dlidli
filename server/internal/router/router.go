// Package router 组装 gin 引擎与路由。业务模块路由自 M1 起在此注册。
package router

import (
	"context"
	"path/filepath"
	"time"

	"github.com/dlidli/server/internal/infra"
	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/admin"
	"github.com/dlidli/server/internal/module/danmaku"
	"github.com/dlidli/server/internal/module/dynamic"
	"github.com/dlidli/server/internal/module/interaction"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/module/relation"
	"github.com/dlidli/server/internal/module/search"
	"github.com/dlidli/server/internal/module/upload"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/metrics"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	_ "github.com/dlidli/server/docs" // swag 生成的 OpenAPI 文档（swag init）
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func New(cfg *config.Config, log *zap.Logger, res *infra.Resources) *gin.Engine {
	if cfg.App.Env == "prod" || cfg.App.Env == "staging" {
		gin.SetMode(gin.ReleaseMode)
	}

	e := gin.New()
	e.Use(
		middleware.TraceID(),
		metrics.Middleware(),
		middleware.AccessLog(log),
		middleware.Recovery(log),
		middleware.CORS(cfg.App.AllowOrigins),
	)

	// Prometheus 指标抓取端点（REL-02：Grafana 基础面板数据源）
	e.GET("/metrics", metrics.Handler())

	// Swagger 在线接口文档（仅非生产环境；访问 /swagger/index.html）
	if cfg.App.Env != "prod" {
		e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 健康检查（供负载均衡/监控拨测）
	e.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		response.OK(c, gin.H{
			"app":        cfg.App.Name,
			"env":        cfg.App.Env,
			"components": res.Health(ctx),
		})
	})

	// 本地存储静态资源（dev；生产由 CDN/对象存储直出）
	// 播放入口（videos 下 .m3u8/.mp4）需 HMAC 签名校验（VID-05），封面/头像放行
	if cfg.Storage.Driver == "local" || cfg.Storage.Driver == "" {
		e.Group("/static", middleware.PlaySignGuard(cfg.JWT.Secret)).Static("/", cfg.Storage.LocalDir)
	}

	v1 := e.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			response.OK(c, gin.H{"pong": time.Now().Unix()})
		})
	}

	authMW := middleware.Auth(cfg.JWT.Secret)
	optionalAuthMW := middleware.OptionalAuth(cfg.JWT.Secret)

	// 业务模块路由（依赖 MySQL/Redis，缺失时跳过并告警）
	if res.DB != nil && res.Redis != nil {
		accountSvc := account.NewService(account.NewRepo(res.DB), res.Redis, cfg, log)
		account.NewHandler(accountSvc, res.Storage).RegisterRoutes(v1, authMW)

		uploadTmp := filepath.Join(cfg.Storage.LocalDir, "chunks")
		uploadSvc := upload.NewService(upload.NewRepo(res.DB), res.Redis, res.Storage, uploadTmp, log)
		upload.NewHandler(uploadSvc).RegisterRoutes(v1, authMW)

		videoSvc := video.NewService(video.NewRepo(res.DB), uploadSvc, accountSvc, res.Redis, cfg, log)
		video.NewHandler(videoSvc, res.Storage).RegisterRoutes(v1, authMW, optionalAuthMW)

		// dev 内嵌转码 Worker（部署环境由独立 cmd/worker 承担）
		if cfg.Transcode.Enabled {
			videoSvc.StartTranscodeWorkers(context.Background(), res.Storage)
		}

		danmakuSvc := danmaku.NewService(danmaku.NewRepo(res.DB), videoSvc, accountSvc, res.Redis, log)
		danmaku.NewHandler(danmakuSvc).RegisterRoutes(v1, authMW)

		notifySvc := notify.NewService(notify.NewRepo(res.DB), accountSvc, log)
		notify.NewHandler(notifySvc).RegisterRoutes(v1, authMW)

		interactionSvc := interaction.NewService(interaction.NewRepo(res.DB), videoSvc, accountSvc, notifySvc, log)
		interaction.NewHandler(interactionSvc).RegisterRoutes(v1, authMW, optionalAuthMW)

		adminSvc := admin.NewService(admin.NewRepo(res.DB), videoSvc, accountSvc, cfg, log)
		admin.NewHandler(adminSvc).RegisterRoutes(v1, middleware.AdminAuth(cfg.JWT.Secret))

		relationSvc := relation.NewService(relation.NewRepo(res.DB), accountSvc, notifySvc)
		relation.NewHandler(relationSvc).RegisterRoutes(v1, authMW, optionalAuthMW)

		dynamicSvc := dynamic.NewService(dynamic.NewRepo(res.DB), accountSvc, videoSvc, relationSvc, log)
		dynamic.NewHandler(dynamicSvc).RegisterRoutes(v1, authMW)
		// 稿件发布 → 自动生成投稿动态（旁路钩子，失败仅日志）
		videoSvc.SetPublishHook(dynamicSvc.OnVideoPublished)

		search.NewHandler(videoSvc, accountSvc).RegisterRoutes(v1)
	} else {
		log.Warn("MySQL/Redis 未就绪，业务模块路由未注册")
	}

	return e
}
