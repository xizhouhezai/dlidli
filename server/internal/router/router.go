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
	"github.com/dlidli/server/internal/module/live"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/module/pay"
	"github.com/dlidli/server/internal/module/recommend"
	"github.com/dlidli/server/internal/module/relation"
	"github.com/dlidli/server/internal/module/report"
	"github.com/dlidli/server/internal/module/search"
	"github.com/dlidli/server/internal/module/searchindex"
	"github.com/dlidli/server/internal/module/upload"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/module/vip"
	"github.com/dlidli/server/internal/module/wechat"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/metrics"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/dlidli/server/internal/pkg/tracing"
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
		tracing.Middleware(),
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

	// 存活探针（M3-ENG-02）：只表示"进程还能服务"，恒返回 200。
	// 必须与 /health 区分：/health 在依赖不可用时返回 503，那是**就绪**语义；
	// 若把它当 liveness 探针，MySQL/Redis 抖动会导致 Pod 被反复重启（雪崩）。
	e.GET("/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":     0,
			"message":  "ok",
			"trace_id": c.GetString(response.CtxTraceID),
		})
	})

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

		// 微信 JSSDK 签名（M2-H5-06）+ 小程序 code2session（M3-MP-01）：
		// 凭据未配置时对应能力返回未启用，前端静默降级。需先于 account 创建，
		// 以便把 code→openid 能力注入账号登录（单向桥接，account 不反向依赖 wechat）。
		wechatSvc := wechat.NewServiceWithMP(
			cfg.WeChat.AppID, cfg.WeChat.AppSecret,
			cfg.WeChat.MPAppID, cfg.WeChat.MPAppSecret,
			res.Redis, log,
		)

		accountSvc := account.NewService(account.NewRepo(res.DB), res.Redis, cfg, log, growthSvc)
		account.NewHandler(accountSvc, res.Storage).
			WithWxSession(func(ctx context.Context, code string) (string, error) {
				sess, err := wechatSvc.Code2Session(ctx, code)
				if err != nil {
					return "", err
				}
				return sess.OpenID, nil
			}).
			RegisterRoutes(v1, authedRateLimited)

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

		// 通知实时推送 Hub（M2-MSG-02 comet）：按用户房间推送，Origin 白名单防跨站
		notifyHub := notify.NewHub(cfg.App.AllowOrigins, log)
		notifySvc := notify.NewService(notify.NewRepo(res.DB), accountSvc, log)
		notifySvc.SetHub(notifyHub)
		notify.NewHandler(notifySvc).RegisterRoutes(v1, authedRateLimited)

		// 微信 JSSDK 签名路由（M2-H5-06；wechatSvc 已在 account 之前创建）
		wechat.NewHandler(wechatSvc).RegisterRoutes(v1)

		interactionSvc := interaction.NewService(interaction.NewRepo(res.DB), videoSvc, accountSvc, notifySvc, growthSvc, log)
		interaction.NewHandler(interactionSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		growth.NewHandler(growthSvc).RegisterRoutes(v1, authedRateLimited)

		relationSvc := relation.NewService(relation.NewRepo(res.DB), accountSvc, notifySvc)
		relation.NewHandler(relationSvc).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		dynamicSvc := dynamic.NewService(dynamic.NewRepo(res.DB), accountSvc, videoSvc, relationSvc, log)
		dynamic.NewHandler(dynamicSvc).RegisterRoutes(v1, authedRateLimited)
		// 稿件发布 → 自动生成投稿动态（旁路钩子，失败仅日志）
		videoSvc.SetPublishHook(dynamicSvc.OnVideoPublished)

		// 搜索索引同步（M2-SRH-01/02）：MySQL Outbox → ES Worker；
		// ESURL 未配置时搜索降级 MySQL LIKE，Worker 不启动
		searchIdx := searchindex.NewService(
			searchindex.NewRepo(res.DB),
			searchindex.NewES(cfg.Search.ESURL, cfg.Search.Index),
			videoSvc,
			log,
		)
		videoSvc.SetSearchHook(func(videoID int64, action string) {
			_ = searchIdx.Enqueue(context.Background(), videoID, action)
		})
		searchIdx.StartWorker(context.Background(), time.Duration(cfg.Search.PollSeconds)*time.Second)

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

		searchHandler := search.NewHandler(videoSvc, accountSvc)
		searchHandler.SetIndex(searchIdx)
		searchHandler.RegisterRoutes(v1)

		// 直播间服务（M4-LIV-02）：房间生命周期 + 开播资格 + 在线人数 + 房管/禁言
		liveRepo := live.NewRepo(res.DB)
		liveOnline := live.NewOnline(res.Redis, liveRepo, log)
		liveSvc := live.NewService(liveRepo, liveOnline, cfg, log)
		// 直播弹幕（M4-LIV-03）：独立 live hub（不复用 danmaku hub，plan live §2A.2），
		// 弹幕校验链与在线计数回调经 SetDeps 注入（live 不反向构造 account）
		liveHub := live.NewHub(cfg.App.AllowOrigins, log)
		// SendDanmaku 签名为 (uid, roomID)，hub 回调按 (roomID, uid) 传入——此处适配
		liveHub.SetDeps(
			func(ctx context.Context, roomID, uid int64, content string) error {
				return liveSvc.SendDanmaku(ctx, uid, roomID, content)
			},
			liveOnline.Beat,
			func(ctx context.Context, roomID, uid int64, counted bool) {
				liveOnline.LeaveForce(ctx, roomID, uid, counted)
			})
		liveSvc.SetHub(liveHub)
		liveSvc.SetLiveDeps(&live.LiveDeps{Account: accountSvc, RDB: res.Redis})
		live.NewHandler(liveSvc).WithHub(liveHub).RegisterRoutes(v1, authedRateLimited, optionalAuthMW)

		// 礼物系统（M4-LIV-04）：金瓜子资产强一致（事务+行锁）、赠送与分成、WS 特效帧；
		// dev 直充挂 admin 中间件（开发期临时通道，PAY-01 接入后下线，spec LIV-13 替换点）
		giftSvc := live.NewGiftService(live.NewGiftRepo(res.DB), liveHub)
		giftSvc.SetLiveDeps(&live.LiveDeps{Account: accountSvc, RDB: res.Redis})
		giftHandler := live.NewGiftHandler(giftSvc)
		giftHandler.RegisterGiftRoutes(v1.Group("/live", optionalAuthMW))
		permGift := func(code string) gin.HandlerFunc { return middleware.RequirePerm(adminSvc.HasPerm, code) }
		giftHandler.RegisterDevRoutes(v1.Group("/admin", middleware.AdminAuth(cfg.JWT.Secret), permGift("config:edit")))

		// 截帧机审与快速切断（M4-LIV-05，spec §5E / plan §2D）：
		// 进程内定时器截帧 → contentmod 机审 → 命中走 SRS 踢流 + 封禁；
		// 人工巡查接口挂 admin 中间件（AdminAuth + 权限点 live:audit）。
		if cfg.Live.Audit.Enabled {
			liveSvc.StartAuditWorker(context.Background())
		} else {
			log.Info("截帧机审未启用（live.audit.enabled=false，spec §5E.1 dev 默认关闭）")
		}
		live.NewPatrolHandler(liveSvc).RegisterPatrolRoutes(
			v1.Group("/admin", middleware.AdminAuth(cfg.JWT.Secret), permGift("live:audit")))

		// 支付基础设施（M4-PAY-01，spec §6 / plan §2E）：统一网关（适配器模式 + mock 渠道）、
		// 订单状态机（条件 UPDATE + RowsAffected）、回调验签与幂等、现金账户、日对账与异常处理台。
		// 资质现实：真实支付宝/微信渠道为换入点（92004），全链路以 mock 渠道跑通。
		payCfg := pay.Config{
			OrderTimeoutMinutes: cfg.Pay.OrderTimeoutMinutes,
			StuckHours:          cfg.Pay.StuckHours,
			MockSecret:          cfg.Pay.MockSecret,
			MaxAmount:           cfg.Pay.MaxAmount,
			NotifyURLBase:       cfg.Pay.NotifyURLBase,
		}
		payRepo := pay.NewRepo(res.DB)
		paySvc := pay.NewService(payRepo, payCfg, log)
		payHandler := pay.NewHandler(paySvc, pay.NewReconService(payRepo, payCfg, log))
		// C 端：资金接口需登录（authedRateLimited 已含鉴权 + 写接口限流）
		payHandler.RegisterRoutes(v1.Group("/pay", optionalAuthMW), authedRateLimited)
		// 回调端点单独注册：渠道服务器没有用户令牌，鉴权靠验签（spec §6.4）。
		// 挂 authedRateLimited 会被 401 挡死真实回调。
		payHandler.RegisterCallbackRoute(v1.Group("/pay", optionalAuthMW))
		// 管理端：财务敏感，权限点 pay:view/pay:order/pay:recon 已落 builtinPermissions
		payHandler.RegisterAdminRoutes(
			v1.Group("/admin/pay", middleware.AdminAuth(cfg.JWT.Secret)), permGift)

		// 大会员（M4-VIP-01，spec §2.1/§2.2/§2.7 / plan §2F）：套餐、开通/顺延、权益判定、
		// 连续包月（自动续期走 mock 渠道，真实免密代扣为换入点）与月度赠币。
		//
		// 与支付域的接法（**本卡的关键结构决策**）：把会员域的两个钩子注入支付域，
		// 而不是让 vip 去调 pay 之后再补记权益 —— 前者保证「订单迁移 + 入账 + 开会员 + 赠币」
		// 在同一事务内，任一失败整体回滚，不会出现「已收钱但没开会员」（plan §2F.4）。
		vipCfg := vip.Config{
			CoinPerMonth:     cfg.Vip.CoinPerMonth,
			ExpireRemindDays: cfg.Vip.ExpireRemindDays,
			RenewEnabled:     cfg.Vip.RenewEnabled,
			RenewIntervalSec: cfg.Vip.RenewIntervalSeconds,
			RenewBatchSize:   cfg.Vip.RenewBatchSize,
			MaxFailCount:     cfg.Vip.MaxFailCount,
		}
		vipSvc := vip.NewService(vip.NewRepo(res.DB), paySvc, vipCfg, log)
		paySvc.SetSkuHook(vipSvc)
		paySvc.SetPaidHook(vipSvc.ApplyPaidHook)
		vipHandler := vip.NewHandler(vipSvc)
		// C 端：plans/status 匿名可访问（价格公开；游客必须能拿到「不是会员」而不是 401，
		// 否则首页无法决定是否显示会员入口）；下单/签约需登录。
		vipHandler.RegisterRoutes(v1.Group("/vip", optionalAuthMW), authedRateLimited)
		// 管理端：vip:view / vip:plan / vip:grant 已落 builtinPermissions
		vipHandler.RegisterAdminRoutes(
			v1.Group("/admin/vip", middleware.AdminAuth(cfg.JWT.Secret)), permGift)
		// 首次启动播种默认套餐（表非空则不动：运营改过的价不会被自动播种覆盖）。
		vipSvc.SeedDefaultPlans()
		// 连续包月自动续期 Worker（dev 默认关闭，vip.renewEnabled=true 打开）。
		vipSvc.StartRenewWorker(context.Background())

		// 会员身份标识落地点（M4-VIP-01 权益落地，plan §2F.6）：
		//   ① 视频评论：UserBrief.is_vip / nickname_color
		//   ② 直播弹幕（实时帧 + 历史回放）：DanmakuInfo.is_vip / nickname_color
		// 两处都只注入**最小只读接口**（VipColors），读模型不 import 会员域。
		// 清晰度权益位（max_resolution）本卡只下发不鉴权，消费方为 M4-VIP-02。
		interactionSvc.SetVipBadge(vipSvc)
		liveSvc.SetVipBadge(vipSvc)
	} else {
		log.Warn("MySQL/Redis 未就绪，业务模块路由未注册")
	}

	return e
}
