package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// 转码队列指标（转码积压治理卡）。
//
// 为什么放在 pkg/metrics 而不是 video 模块：prometheus 的注册表是全局的，
// 谁注册谁负责清理；集中在 metrics 包注册可以避免同一个 collector 被两个包各注册一次而 panic。
// video 域只负责"喂值"（SetTranscodeQueue / ObserveTranscodeJob），不持有 collector。
var (
	// transcodeQueueDepth 各状态任务数（0待处理 1处理中 2成功 3失败）。
	// 用它回答「当前排队多少、正在转几个」：pending/running 两个标签值即可。
	transcodeQueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "queue_depth",
			Help:      "转码任务按状态计数（status=0 待处理 1 处理中 2 成功 3 失败）",
		},
		[]string{"status"},
	)
	// transcodeOldestPendingSeconds 最老待处理任务已等待秒数：「最老的等了多久」。
	transcodeOldestPendingSeconds = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "oldest_pending_seconds",
			Help:      "最老待处理转码任务已等待秒数（0 表示队列为空）",
		},
	)
	// transcodeJobDuration 单任务实际转码耗时：「平均耗时」由 histogram 的 _sum/_count 求得，
	// 比在进程内维护均值更准（多实例各自喂值也能合并）。
	transcodeJobDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "job_duration_seconds",
			Help:      "单个转码任务耗时分布（秒）",
			Buckets:   []float64{1, 5, 15, 30, 60, 180, 600, 1800, 3600, 7200},
		},
	)
	// transcodeJobsProcessed 处理结果计数：success / failed(重试) / dead（进终态失败）。
	transcodeJobsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "jobs_processed_total",
			Help:      "转码任务处理结果总量（success 成功 / retry 失败待重试 / dead 进终态）",
		},
		[]string{"result"},
	)
	// transcodeLeaseLost 租约丢失次数：>0 说明有 Worker 卡顿或进程被 kill，
	// 且这些任务可能已被其他 Worker 回收（排障关键信号）。
	transcodeLeaseLost = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "lease_lost_total",
			Help:      "租约丢失（任务已被其他 Worker 回收）次数",
		},
	)
	// transcodeReclaimed 过期任务回收数：进程崩溃留下的僵尸任务被拉回队列的次数。
	transcodeReclaimed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "dlidli",
			Subsystem: "transcode",
			Name:      "reclaimed_total",
			Help:      "租约过期被回收重排的任务数",
		},
	)
)

func init() {
	prometheus.MustRegister(
		transcodeQueueDepth, transcodeOldestPendingSeconds, transcodeJobDuration,
		transcodeJobsProcessed, transcodeLeaseLost, transcodeReclaimed,
	)
}

// SetTranscodeQueue 刷新队列深度与最老等待时长（由 Worker 轮询/任务结束/管理端查询时喂值）。
func SetTranscodeQueue(pending, running, success, failed int64, oldestPendingSeconds float64) {
	transcodeQueueDepth.WithLabelValues("0").Set(float64(pending))
	transcodeQueueDepth.WithLabelValues("1").Set(float64(running))
	transcodeQueueDepth.WithLabelValues("2").Set(float64(success))
	transcodeQueueDepth.WithLabelValues("3").Set(float64(failed))
	transcodeOldestPendingSeconds.Set(oldestPendingSeconds)
}

// ObserveTranscodeJob 记录单任务耗时与结果（result: success / retry / dead）。
func ObserveTranscodeJob(seconds float64, result string) {
	transcodeJobDuration.Observe(seconds)
	transcodeJobsProcessed.WithLabelValues(result).Inc()
}

// IncTranscodeLeaseLost 租约丢失计数 +1。
func IncTranscodeLeaseLost() { transcodeLeaseLost.Inc() }

// AddTranscodeReclaimed 过期回收计数 +n。
func AddTranscodeReclaimed(n int64) {
	if n > 0 {
		transcodeReclaimed.Add(float64(n))
	}
}
