// Package config 负责加载多环境配置（yaml + 环境变量覆盖）。
// 环境变量前缀 DLIDLI_，如 DLIDLI_MYSQL_DSN 覆盖 mysql.dsn。
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App       App
	Log       Log
	MySQL     MySQL
	Redis     Redis
	JWT       JWT
	Storage   Storage
	RateLimit RateLimit
	Transcode Transcode
	WeChat    WeChat
	Search    Search
	Tracing   Tracing
	Live      Live
}

// Live 直播域配置（M4-LIV-01/02）：推拉流寻址与 SRS 回调鉴权。
//
// 寻址契约（plan liv01 D1/D4）：RTMP 地址 rtmp://<pushDomain>/<app>/room_<id>?key=<streamKey>，
// 拉流地址 http://<playDomain>/<app>/room_<id>.m3u8|.flv；流名公开不含密钥，密钥只走查询串
// （SRS 把它放进 on_publish 回调体的 param 字段）。
type Live struct {
	PushDomain string // RTMP 推流域名，如 127.0.0.1:1935；为空时推流地址返回占位域
	PlayDomain string // HLS/FLV 拉流域名，如 127.0.0.1:8080；为空时拉流地址返回占位域
	StreamApp  string // RTMP 应用名（流路径 /<app>/<stream>），缺省 live
	// CallbackToken SRS http_hooks 共享 token（?token= 或 X-Live-Callback-Token）。
	// 为空时回调一律拒绝（fail-closed）：宁可推不了流，也不能让伪造回调改房间状态。
	CallbackToken string
	Audit         LiveAudit // 截帧机审与快速切断（M4-LIV-05，spec §5E / plan §2D）
}

// LiveAudit 直播截帧机审配置（M4-LIV-05）。
//
// dev 默认 Enabled=false：机审会周期拉起 ffmpeg 进程，默认关闭避免影响其它测试；
// 上线/联调时用 DLIDLI_LIVE_AUDIT_ENABLED=true 或改 dev.yaml 打开。
type LiveAudit struct {
	Enabled              bool    // 截帧机审总开关（dev 默认 false）
	FrameIntervalSeconds int     // 截帧周期秒（spec §3 硬要求：默认 10s）
	FlvTimeoutSeconds    int     // 单房间截帧超时秒（ffmpeg 挂死兜底）
	SrsAPIBase           string  // SRS HTTP API 基址（查流/踢流），如 http://127.0.0.1:1985
	FrameDir             string  // 截帧图落盘目录（合规存档，spec §4 ≥60 天）
	BlackYAvgMax         float64 // 黑屏判据：灰度均值 ≤ 此值视为黑屏（plan §2D.5 实测标定）
	FlatRangeMax         float64 // 纯色遮挡判据：YMAX-YMIN ≤ 此值视为动态范围坍缩
}

type App struct {
	Name         string
	Env          string // dev / test / staging / prod
	Port         int
	AllowOrigins []string
	// InviteCodeRequired 内测开关：开启后新注册（短信自动注册/邮箱注册）须携带一次性邀请码（ACC-44）
	InviteCodeRequired bool
	// AutoApprove 仅限开发环境：稿件提交后自动过审（审核工作台 M1-ADM 上线前的联调便利）
	AutoApprove bool
}

// WeChat 微信公众号/小程序配置：
//   - appId/appSecret：公众号，用于 H5 分享 JSSDK 的 jsapi_ticket 签名（M2-H5-06）
//   - mpAppId/mpAppSecret：小程序，用于 code2session 静默授权登录（M3-MP-01）
//
// 各自未配置时对应能力返回未启用错误，前端静默降级。
type WeChat struct {
	AppID     string // 公众号 appId
	AppSecret string // 公众号 appSecret
	// MPAppID/MPAppSecret 小程序凭据（M3-MP-01）。未配置时 /auth/login/wechat 返回"微信登录未启用"。
	MPAppID     string
	MPAppSecret string
}

// Search 搜索索引配置（M2-SRH-01/02）：ESURL 为空时 ES 关闭，
// 搜索自动降级 MySQL LIKE；索引同步 Worker 不启动。
type Search struct {
	ESURL       string // Elasticsearch 地址，如 http://127.0.0.1:9200
	Index       string // 索引名，默认 dlidli_videos
	PollSeconds int    // Worker 轮询间隔（秒），默认 3
	BatchSize   int    // 每批消费条数，默认 100
}

// Tracing OpenTelemetry 链路追踪配置（M3-ENG-04）。Endpoint 为空时关闭导出。
type Tracing struct {
	Endpoint    string
	ServiceName string
}

type Log struct {
	Level string // debug / info / warn / error
}

type MySQL struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

type Redis struct {
	Addr     string
	Password string
	DB       int
}

type JWT struct {
	Secret       string
	AccessTTLMin int
}

type Storage struct {
	Driver   string // local / minio（M1-VID-01 接入）
	LocalDir string // local 驱动：磁盘目录
	BaseURL  string // 对外访问前缀，如 http://localhost:8000/static
}

type RateLimit struct {
	Enabled   bool // 全局写接口限流开关
	PerMinute int  // 每个 key（用户/IP）每分钟写请求上限；<=0 表示禁用
}

type Transcode struct {
	Enabled     bool   // dev 内嵌 Worker；部署环境由独立 cmd/worker 承担
	Workers     int    // 并发转码协程数
	FfmpegPath  string // 缺省 "ffmpeg"（依赖 PATH）
	FfprobePath string // 缺省 "ffprobe"
}

// Load 读取指定路径的 yaml 配置，并允许环境变量覆盖。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DLIDLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setSchemaDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.App.Port == 0 {
		cfg.App.Port = 8000
	}
	if cfg.RateLimit.PerMinute == 0 && cfg.RateLimit.Enabled {
		cfg.RateLimit.PerMinute = 30 // 默认每个写接口 30 次/分钟
	}
	if cfg.Transcode.Workers <= 0 {
		cfg.Transcode.Workers = 1
	}
	if cfg.Transcode.FfmpegPath == "" {
		cfg.Transcode.FfmpegPath = "ffmpeg"
	}
	if cfg.Transcode.FfprobePath == "" {
		cfg.Transcode.FfprobePath = "ffprobe"
	}
	if cfg.Search.Index == "" {
		cfg.Search.Index = "dlidli_videos"
	}
	if cfg.Search.PollSeconds <= 0 {
		cfg.Search.PollSeconds = 3
	}
	if cfg.Search.BatchSize <= 0 {
		cfg.Search.BatchSize = 100
	}
	if cfg.Live.StreamApp == "" {
		cfg.Live.StreamApp = "live" // 与 SRS conf 的 app 名对齐（部署侧可用 DLIDLI_LIVE_STREAMAPP 覆盖）
	}
	// 截帧机审（M4-LIV-05）：周期/超时给零值兜底（配置写 0 或负数时回落默认，避免 ticker panic）
	audit := &cfg.Live.Audit
	if audit.FrameIntervalSeconds <= 0 {
		audit.FrameIntervalSeconds = 10
	}
	if audit.FlvTimeoutSeconds <= 0 {
		audit.FlvTimeoutSeconds = 15
	}
	if audit.SrsAPIBase == "" {
		audit.SrsAPIBase = "http://127.0.0.1:1985"
	}
	if audit.FrameDir == "" {
		audit.FrameDir = "./uploads/live-frames"
	}
	if audit.BlackYAvgMax <= 0 {
		audit.BlackYAvgMax = 24
	}
	if audit.FlatRangeMax <= 0 {
		audit.FlatRangeMax = 12
	}
	if err := validateProd(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// setSchemaDefaults 为全量配置项注册零值缺省。viper 的 AutomaticEnv 只对
// AllKeys 里已存在的键生效（yaml 缺省的键不会查环境变量），注册缺省值后
// Unmarshal 才能感知并应用 DLIDLI_ 环境变量覆盖。
func setSchemaDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "")
	v.SetDefault("app.env", "")
	v.SetDefault("app.port", 0)
	v.SetDefault("app.autoapprove", false)
	v.SetDefault("app.alloworigins", []string{})
	v.SetDefault("log.level", "")
	v.SetDefault("mysql.dsn", "")
	v.SetDefault("mysql.maxopenconns", 0)
	v.SetDefault("mysql.maxidleconns", 0)
	v.SetDefault("redis.addr", "")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.accessttlmin", 0)
	v.SetDefault("storage.driver", "")
	v.SetDefault("storage.localdir", "")
	v.SetDefault("storage.baseurl", "")
	v.SetDefault("ratelimit.enabled", false)
	v.SetDefault("ratelimit.perminute", 0)
	v.SetDefault("transcode.enabled", false)
	v.SetDefault("transcode.workers", 0)
	v.SetDefault("transcode.ffmpegpath", "")
	v.SetDefault("transcode.ffprobepath", "")
	v.SetDefault("search.esurl", "")
	v.SetDefault("search.index", "")
	v.SetDefault("search.pollseconds", 0)
	v.SetDefault("search.batchsize", 0)
	v.SetDefault("wechat.appid", "")
	v.SetDefault("wechat.appsecret", "")
	v.SetDefault("wechat.mpappid", "")
	v.SetDefault("wechat.mpappsecret", "")
	v.SetDefault("tracing.endpoint", "")
	v.SetDefault("tracing.servicename", "dlidli-api")
	v.SetDefault("live.pushdomain", "")
	v.SetDefault("live.playdomain", "")
	v.SetDefault("live.streamapp", "live")
	v.SetDefault("live.callbacktoken", "")
	// 截帧机审（M4-LIV-05）：默认关闭，周期 10s（spec §3），阈值取自 plan §2D.5 实测标定
	v.SetDefault("live.audit.enabled", false)
	v.SetDefault("live.audit.frameintervalseconds", 10)
	v.SetDefault("live.audit.flvtimeoutseconds", 15)
	v.SetDefault("live.audit.srsapibase", "http://127.0.0.1:1985")
	v.SetDefault("live.audit.framedir", "./uploads/live-frames")
	v.SetDefault("live.audit.blackyavgmax", 24.0)
	v.SetDefault("live.audit.flatrangemax", 12.0)
}

// validateProd 拦截 prod 环境的危险缺省：密钥/DSN 必须显式注入，禁止占位值上线。
func validateProd(cfg *Config) error {
	if cfg.App.Env != "prod" {
		return nil
	}
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "dev-secret-do-not-use-in-prod" {
		return fmt.Errorf("prod 环境必须通过 DLIDLI_JWT_SECRET 注入正式密钥")
	}
	if cfg.MySQL.DSN == "" {
		return fmt.Errorf("prod 环境必须通过 DLIDLI_MYSQL_DSN 注入数据库连接串")
	}
	return nil
}
