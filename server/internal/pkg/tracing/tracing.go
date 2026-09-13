// Package tracing 提供可选的 OpenTelemetry HTTP 链路追踪（M3-ENG-04）。
// endpoint 为空时返回 noop provider，不改变现有服务行为；配置 OTLP HTTP 地址后启用批量导出。
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/dlidli/server/internal/pkg/tracing"

// Provider 根据 endpoint 创建 TracerProvider。endpoint 为空时返回 noop provider。
// endpoint 可传完整 URL，也可传 host:port；当前使用 OTLP/HTTP protobuf。
func Provider(ctx context.Context, serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
	if strings.TrimSpace(endpoint) == "" {
		return sdktrace.NewTracerProvider(), nil
	}
	endpoint = strings.TrimSpace(endpoint)
	options := []otlptracehttp.Option{}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		options = append(options, otlptracehttp.WithEndpointURL(endpoint))
	} else {
		options = append(options, otlptracehttp.WithEndpoint(endpoint))
		options = append(options, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("创建 OTLP trace exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("deployment.environment", "configured"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 trace resource: %w", err)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	), nil
}

// Setup 创建并设置全局 provider；endpoint 为空时只设置 noop provider。
func Setup(ctx context.Context, serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "dlidli-api"
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	provider, err := Provider(ctx, serviceName, endpoint)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(provider)
	return provider, nil
}

// Middleware 为每个 Gin 请求创建 server span，并记录 HTTP 与现有 request-id 信息。
func Middleware() gin.HandlerFunc {
	tracer := otel.Tracer(instrumentationName)
	return func(c *gin.Context) {
		started := time.Now()
		path := c.FullPath()
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		if path == "" {
			path = c.Request.URL.Path
		}
		ctx, span := tracer.Start(ctx, c.Request.Method+" "+path,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		span.SetAttributes(
			attribute.String(string(semconv.HTTPRequestMethodKey), c.Request.Method),
			attribute.String(string(semconv.URLPathKey), c.Request.URL.Path),
			attribute.String("http.route", path),
		)
		c.Next()
		span.SetAttributes(
			attribute.Int(string(semconv.HTTPResponseStatusCodeKey), c.Writer.Status()),
			attribute.Int64("http.server.duration_ms", time.Since(started).Milliseconds()),
		)
		if tid := c.GetString("trace_id"); tid != "" {
			span.SetAttributes(attribute.String("dlidli.request_id", tid))
		}
		if c.Writer.Status() >= http.StatusInternalServerError {
			span.RecordError(fmt.Errorf("HTTP %d", c.Writer.Status()))
		}
	}
}
