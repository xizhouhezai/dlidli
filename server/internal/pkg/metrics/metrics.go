// Package metrics 提供 Prometheus 指标采集：HTTP 请求量/耗时直方图，供 Grafana 基础面板消费。
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequests 按方法/路由/状态码统计请求总量
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dlidli",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "HTTP 请求总量",
		},
		[]string{"method", "path", "status"},
	)
	// httpDuration 请求耗时直方图（秒）
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dlidli",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP 请求耗时分布",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)
	// inFlight 当前处理中的请求数
	inFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "dlidli",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "当前处理中的 HTTP 请求数",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequests, httpDuration, inFlight)
}

// Middleware 采集每个请求的量/耗时；用 gin 路由模板（FullPath）作为 path 标签，避免高基数。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = "unmatched" // 未匹配路由归并，防止 404 扫描撑爆标签基数
		}
		inFlight.Inc()
		start := time.Now()

		c.Next()

		inFlight.Dec()
		elapsed := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		httpRequests.WithLabelValues(c.Request.Method, path, status).Inc()
		httpDuration.WithLabelValues(c.Request.Method, path).Observe(elapsed)
	}
}

// Handler 返回 /metrics 处理器（Prometheus 抓取端点）。
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
