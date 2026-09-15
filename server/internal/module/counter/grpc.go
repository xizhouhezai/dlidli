package counter

import (
	"context"
	"errors"

	counterv1 "github.com/dlidli/server/internal/gen/counter/v1"
	"github.com/dlidli/server/internal/pkg/errcode"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCServer 把 Service 适配为 counter.v1.CounterServiceServer。
type GRPCServer struct {
	counterv1.UnimplementedCounterServiceServer
	svc *Service
}

// NewGRPCServer 构建 gRPC 适配层。
func NewGRPCServer(svc *Service) *GRPCServer { return &GRPCServer{svc: svc} }

// ApplyDelta 实现 counter.v1.CounterServiceServer。
func (g *GRPCServer) ApplyDelta(ctx context.Context, req *counterv1.ApplyDeltaRequest) (*counterv1.ApplyDeltaResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "请求为空")
	}
	applied, current, err := g.svc.ApplyDelta(ctx, req.GetEventId(), req.GetVideoId(), req.GetColumn(), req.GetDelta())
	if err != nil {
		return nil, toStatus(err)
	}
	return &counterv1.ApplyDeltaResponse{Applied: applied, Current: current}, nil
}

// GetStat 实现 counter.v1.CounterServiceServer。
func (g *GRPCServer) GetStat(ctx context.Context, req *counterv1.GetStatRequest) (*counterv1.StatSnapshot, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "请求为空")
	}
	snap, err := g.svc.GetStat(ctx, req.GetVideoId())
	if err != nil {
		return nil, toStatus(err)
	}
	return snap, nil
}

// BatchGetStats 实现 counter.v1.CounterServiceServer。
func (g *GRPCServer) BatchGetStats(ctx context.Context, req *counterv1.BatchGetStatsRequest) (*counterv1.BatchGetStatsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "请求为空")
	}
	stats, err := g.svc.BatchGetStats(ctx, req.GetVideoIds())
	if err != nil {
		return nil, toStatus(err)
	}
	return &counterv1.BatchGetStatsResponse{Stats: stats}, nil
}

// toStatus 把业务错误映射为 gRPC status。
//
// 统一复用 errcode.Error 的 code 字段，避免在适配层重复维护一套映射表：
// 参数类（1xxxx 中 10002/10005）→ InvalidArgument/NotFound，鉴权类 → Unauthenticated/PermissionDenied，
// 其余按 Internal 处理，不回传内部细节。
func toStatus(err error) error {
	if err == nil {
		return nil
	}
	var e *errcode.Error
	if errors.As(err, &e) {
		switch e.Code {
		case errcode.ErrInvalidParams.Code:
			return status.Error(codes.InvalidArgument, e.Msg)
		case errcode.ErrNotFound.Code:
			return status.Error(codes.NotFound, e.Msg)
		case errcode.ErrUnauthorized.Code:
			return status.Error(codes.Unauthenticated, e.Msg)
		case errcode.ErrForbidden.Code:
			return status.Error(codes.PermissionDenied, e.Msg)
		case errcode.ErrTooManyRequests.Code:
			return status.Error(codes.ResourceExhausted, e.Msg)
		}
	}
	return status.Error(codes.Internal, err.Error())
}
