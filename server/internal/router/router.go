// Package router 组装 gin 引擎与路由。业务模块路由自 M1 起在此注册。
package router

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/dlidli/server/internal/infra"
	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/module/abtest"
	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/admin"
	"github.com/dlidli/server/internal/module/banner"
	"github.com/dlidli/server/internal/module/collection"
	"github.com/dlidli/server/internal/module/creator"
	"github.com/dlidli/server/internal/module/danmaku"
	"github.com/dlidli/server/internal/module/dynamic"
	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/module/im"
	"github.com/dlidli/server/internal/module/interaction"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/module/recommend"
	"github.com/dlidli/server/internal/module/relation"
	"github.com/dlidli/server/internal/module/report"
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
	// 服务存活(进程运行)恒返回 200；业务就绪(MySQL/Redis 可用、业务路由已注册)返回 ready=true，
	// 否则返回 503，便于监控区分"进程活着但业务不可用"。
	e.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		ready := res.DB != nil && res.Redis != nil
		code := http.StatusOK
		msg := "ok"
		if !ready {
			code = http.StatusServiceUnavailable
			msg = "service unavailable: business dependencies not ready"
		}
		c.JSON(code, gin.H{
			"code":    0,
			"message": msg,
			"data": gin.H{
				"app":        cfg.App.Name,
				"env":        cfg.App.Env,
				"ready":      ready,
				"components": res.Health(ctx),
			},
			"trace_id": c.GetString(response.CtxTraceID),
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

	optionalAuthMW := middleware.OptionalAuth(cfg.JWT.Secret)

	// 需登录路由组：鉴权 + 写接口限流（合并为单一中间件，保证限流先于业务执行；
	// 仅统计 POST/PUT/PATCH/DELETE，依赖 Redis 不可用时放行）
	rateLimiter := middleware.NewRateLimiter(res.Redis, cfg.RateLimit.PerMinute, log)
	authedRateLimited := middleware.AuthedRateLimited(cfg.JWT.Secret, rateLimiter)

	// 业务模块路由（依赖 MySQL/Redis，缺失时跳过并告警）
	if res.DB != nil && res.Redis != nil {
		growthSvc := growth.NewService(growth.NewRepo(res.DB), res.Redis, log)

		accountSvc := account.NewService(account.NewRepo(res.DB), res.Redis, cfg, log, growthSvc)
		account.NewHandler(accountSvc, res.Storage).RegisterRoutes(v1, authedRateLimited)

		uploadTmp := filepath.Join(cfg.Storage.LocalDir, "chunks")
		uploadSvc := upload.NewService(upload.NewRepo(res.DB), res.Redis, res.Storage, uploadTmp, log)
		upload.NewHandler(uploadSvc).RegisterRoutes(v1, authedRateLimited)
		// 周期性清理未完成上传的孤儿分片目录（Redis 会话过期后的磁盘兜底）
		uploadSvc.StartCleanupWorker(context.Background(), 30*time.Minute)

		videoSvc := video.NewService(video.NewRepo(res.DB), uploadSvc, accountSvc, growthSvc, res.Redis, cfg, log)
		video.NewHandler(videoSvc, res.Storage).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		// dev 内嵌转码 Worker（部署环境由独立 cmd/worker 承担）
		if cfg.Transcode.Enabled {
			videoSvc.StartTranscodeWorkers(context.Background(), res.Storage)
		}

		danmakuHub := danmaku.NewHub(cfg.App.AllowOrigins, log)
		danmakuSvc := danmaku.NewService(danmaku.NewRepo(res.DB), videoSvc, accountSvc, growthSvc, res.Redis, danmakuHub, cfg.JWT.Secret, log)
		danmaku.NewHandler(danmakuSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		notifySvc := notify.NewService(notify.NewRepo(res.DB), accountSvc, log)
		notify.NewHandler(notifySvc).RegisterRoutes(v1, authedRateLimited)

		interactionSvc := interaction.NewService(interaction.NewRepo(res.DB), videoSvc, accountSvc, notifySvc, growthSvc, log)
		interaction.NewHandler(interactionSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		growth.NewHandler(growthSvc).RegisterRoutes(v1, authedRateLimited)

		relationSvc := relation.NewService(relation.NewRepo(res.DB), accountSvc, notifySvc)
		relation.NewHandler(relationSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		dynamicSvc := dynamic.NewService(dynamic.NewRepo(res.DB), accountSvc, videoSvc, relationSvc, log)
		dynamic.NewHandler(dynamicSvc).RegisterRoutes(v1, authedRateLimited)
		// 稿件发布 → 自动生成投稿动态（旁路钩子，失败仅日志）
		videoSvc.SetPublishHook(dynamicSvc.OnVideoPublished)

		adminSvc := admin.NewService(admin.NewRepo(res.DB), videoSvc, accountSvc, cfg, log)
		admin.NewHandler(adminSvc).WithInviteGen(accountSvc.GenerateInviteCodes).RegisterRoutes(v1, middleware.AdminAuth(cfg.JWT.Secret))

		// 举报体系（M2-AUD-03）：C 端提交 + 后台队列处理
		reportSvc := report.NewService(report.NewRepo(res.DB), videoSvc, accountSvc,
			interactionSvc, danmakuSvc, dynamicSvc, notifySvc, log)
		report.NewHandler(reportSvc).RegisterRoutes(v1, authedRateLimited, middleware.AdminAuth(cfg.JWT.Secret),
			func(code string) gin.HandlerFunc { return middleware.RequirePerm(adminSvc.HasPerm, code) })

		// 推荐系统（M3-REC）：热度榜 + 混合召回 + 行为采集 + 负反馈 + 推荐开关
		recommendSvc := recommend.NewService(recommend.NewRepo(res.DB), videoSvc, res.Redis, log)
		recommend.NewHandler(recommendSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		// A/B 实验（M3-OPS-03）：分流框架 + 推荐策略变体接入
		abtestSvc := abtest.NewService(abtest.NewRepo(res.DB))
		recommendSvc.SetABTest(abtestSvc)
		abtest.NewHandler(abtestSvc).RegisterRoutes(v1, middleware.AdminAuth(cfg.JWT.Secret),
			func(code string) gin.HandlerFunc { return middleware.RequirePerm(adminSvc.HasPerm, code) })

		// UP 主合集（M3-CRT-05 合集部分）
		collectionSvc := collection.NewService(collection.NewRepo(res.DB), videoSvc)
		collection.NewHandler(collectionSvc).RegisterRoutes(v1, authedRateLimited)

		// 私信 IM（M3-IM）：会话/消息 + 发送限制 + 机审 + WS 实时推送
		imHub := im.NewHub(cfg.App.AllowOrigins, log)
		imSvc := im.NewService(im.NewRepo(res.DB), accountSvc, relationSvc, imHub, log)
		im.NewHandler(imSvc).RegisterRoutes(v1, authedRateLimited)

		// 创作者中心（M3-CRT）：数据看板 + 激励结算
		creatorSvc := creator.NewService(creator.NewRepo(res.DB), log)
		creator.NewHandler(creatorSvc).RegisterRoutes(v1, authedRateLimited)

		// 运营位（M3-OPS-01）：首页轮播 Banner
		bannerSvc := banner.NewService(banner.NewRepo(res.DB), videoSvc)
		banner.NewHandler(bannerSvc).RegisterRoutes(v1, middleware.AdminAuth(cfg.JWT.Secret),
			func(code string) gin.HandlerFunc { return middleware.RequirePerm(adminSvc.HasPerm, code) })

		search.NewHandler(videoSvc, accountSvc).RegisterRoutes(v1)
	} else {
		log.Warn("MySQL/Redis 未就绪，业务模块路由未注册")
	}

	return e
}
