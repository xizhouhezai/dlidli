package counter

import (
	"context"
	"net"
	"testing"
	"time"

	counterv1 "github.com/dlidli/server/internal/gen/counter/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// TestGRPCRoundTrip 真实起一个 gRPC server 并用客户端调用，
// 验证 proto 生成的代码、注册函数与适配层能端到端跑通
// （这是"骨架可编译"之外的额外证据：契约确实可用）。
func TestGRPCRoundTrip(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	srv := grpc.NewServer()
	counterv1.RegisterCounterServiceServer(srv, NewGRPCServer(svc))
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("建立连接失败: %v", err)
	}
	defer conn.Close()

	client := counterv1.NewCounterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const videoID = int64(900000030)
	db.Exec("DELETE FROM counter_event WHERE video_id = ?", videoID)
	db.Exec("DELETE FROM video_stat WHERE video_id = ?", videoID)

	// 1) 首次投递应生效
	resp, err := client.ApplyDelta(ctx, &counterv1.ApplyDeltaRequest{
		EventId: "grpc-evt-1", VideoId: videoID,
		Column: counterv1.CounterColumn_COUNTER_COLUMN_SHARE, Delta: 2,
	})
	if err != nil {
		t.Fatalf("ApplyDelta 失败: %v", err)
	}
	if !resp.GetApplied() || resp.GetCurrent() != 2 {
		t.Fatalf("首次应生效且 current=2: applied=%v current=%d", resp.GetApplied(), resp.GetCurrent())
	}

	// 2) 重复投递同一 event_id 应幂等
	resp, err = client.ApplyDelta(ctx, &counterv1.ApplyDeltaRequest{
		EventId: "grpc-evt-1", VideoId: videoID,
		Column: counterv1.CounterColumn_COUNTER_COLUMN_SHARE, Delta: 2,
	})
	if err != nil {
		t.Fatalf("重复 ApplyDelta 失败: %v", err)
	}
	if resp.GetApplied() {
		t.Fatalf("重复投递不应再次生效")
	}
	if resp.GetCurrent() != 2 {
		t.Fatalf("幂等命中后 current 应保持 2: got=%d", resp.GetCurrent())
	}

	// 3) GetStat 应反映累加结果
	snap, err := client.GetStat(ctx, &counterv1.GetStatRequest{VideoId: videoID})
	if err != nil {
		t.Fatalf("GetStat 失败: %v", err)
	}
	if snap.GetShareCnt() != 2 {
		t.Fatalf("share_cnt 应为 2: got=%d", snap.GetShareCnt())
	}

	// 4) 未登记列应返回 InvalidArgument（而非 Internal）
	_, err = client.ApplyDelta(ctx, &counterv1.ApplyDeltaRequest{
		EventId: "grpc-evt-bad", VideoId: videoID,
		Column: counterv1.CounterColumn_COUNTER_COLUMN_UNSPECIFIED, Delta: 1,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("未登记列应返回 InvalidArgument: got=%v err=%v", status.Code(err), err)
	}

	// 5) 非法 video_id 同样应是 InvalidArgument
	_, err = client.GetStat(ctx, &counterv1.GetStatRequest{VideoId: 0})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非法 video_id 应返回 InvalidArgument: got=%v err=%v", status.Code(err), err)
	}
}
