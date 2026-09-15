# DliDli API 镜像（M3-ENG-02）
# 多阶段构建：builder 编译静态二进制，runtime 用最小基础镜像。
# 构建（在 server/ 目录下执行）：
#   docker build -f deploy/Dockerfile -t dlidli/api:local .
FROM golang:1.25-alpine AS builder

WORKDIR /src

# 先只拷依赖清单，利用层缓存加速重复构建
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

COPY . .

# CGO_ENABLED=0 生成静态链接二进制，可直接跑在 scratch/alpine
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.20 AS runtime

# 时区与 CA 证书：访问微信接口/OTLP 需要
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 10001 -S dlidli && \
    adduser -u 10001 -S dlidli -G dlidli

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate
COPY configs /app/configs
COPY scripts/migrations /app/scripts/migrations

# 本地存储驱动需要写目录（生产应改对象存储/MinIO）
RUN mkdir -p /app/uploads && chown -R dlidli:dlidli /app

USER 10001

EXPOSE 8000

# 容器健康检查用 /livez（存活语义）。
# 不要用 /health：依赖不可用时它返回 503，会让编排系统误判容器已死。
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8000/livez || exit 1

ENTRYPOINT ["/app/api"]
CMD ["-config", "/app/configs/dev.yaml"]
