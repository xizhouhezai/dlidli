package searchindex

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// fakeRepo 内存版 OutboxRepo。
type fakeRepo struct {
	mu      sync.Mutex
	rows    []Outbox
	nextID  int64
	attempt map[int64]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{attempt: map[int64]int{}}
}

func (f *fakeRepo) Enqueue(_ context.Context, videoID int64, action string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 同 video 待处理折叠
	kept := f.rows[:0]
	for _, r := range f.rows {
		if r.VideoID == videoID && r.Status == OutboxPending {
			continue
		}
		kept = append(kept, r)
	}
	f.rows = kept
	f.nextID++
	f.rows = append(f.rows, Outbox{ID: f.nextID, VideoID: videoID, Action: action, Status: OutboxPending})
	return nil
}

func (f *fakeRepo) PendingBatch(_ context.Context, limit int) ([]Outbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Outbox
	for _, r := range f.rows {
		if r.Status == OutboxPending {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) MarkDone(_ context.Context, ids []int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	set := map[int64]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for i := range f.rows {
		if set[f.rows[i].ID] {
			f.rows[i].Status = OutboxDone
		}
	}
	return nil
}

func (f *fakeRepo) MarkFailed(_ context.Context, ids []int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	set := map[int64]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for i := range f.rows {
		if set[f.rows[i].ID] {
			f.rows[i].Status = OutboxFailed
		}
	}
	return nil
}

func (f *fakeRepo) IncrAttempts(_ context.Context, id int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempt[id]++
	return f.attempt[id], nil
}

func (f *fakeRepo) statuses() map[int64]int8 {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := map[int64]int8{}
	for _, r := range f.rows {
		m[r.ID] = r.Status
	}
	return m
}

// fakeIndexer 记录调用并可选失败。
type fakeIndexer struct {
	upserts  []Doc
	deletes  []int64
	failNext int // >0 时前 N 次调用失败
	calls    int
}

func (f *fakeIndexer) IsEnabled() bool                   { return true }
func (f *fakeIndexer) EnsureIndex(context.Context) error { return nil }
func (f *fakeIndexer) Upsert(_ context.Context, d Doc) error {
	f.calls++
	if f.failNext > 0 {
		f.failNext--
		return errFail
	}
	f.upserts = append(f.upserts, d)
	return nil
}
func (f *fakeIndexer) Delete(_ context.Context, id int64) error {
	f.calls++
	if f.failNext > 0 {
		f.failNext--
		return errFail
	}
	f.deletes = append(f.deletes, id)
	return nil
}
func (f *fakeIndexer) Search(_ context.Context, _ string, _, _ int) ([]int64, int64, error) {
	return nil, 0, nil
}

var errFail = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "boom" }

// fakeLoader 返回固定文档；missing 集合返回 nil。
type fakeLoader struct {
	docs    map[int64]*Doc
	missing map[int64]bool
}

func (f *fakeLoader) IndexDoc(_ context.Context, videoID int64) (*Doc, error) {
	if f.missing[videoID] {
		return nil, nil
	}
	return f.docs[videoID], nil
}

func TestServiceDrainUpsertAndDelete(t *testing.T) {
	repo := newFakeRepo()
	idx := &fakeIndexer{}
	loader := &fakeLoader{docs: map[int64]*Doc{1: {VideoID: 1, Title: "t1"}, 2: {VideoID: 2, Title: "t2"}}}
	svc := NewService(repo, idx, loader, zap.NewNop())

	ctx := context.Background()
	if err := svc.Enqueue(ctx, 1, ActionUpsert); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enqueue(ctx, 2, ActionDelete); err != nil {
		t.Fatal(err)
	}
	// 同稿件最新覆盖：先 upsert 再 delete → 折叠为 delete
	if err := svc.Enqueue(ctx, 1, ActionDelete); err != nil {
		t.Fatal(err)
	}

	svc.drain(ctx)

	st := repo.statuses()
	for id, s := range st {
		if s != OutboxDone {
			t.Errorf("outbox %d 应已完成, got %d", id, s)
		}
	}
	if len(idx.deletes) != 2 {
		t.Fatalf("应删除 2 条（video2 + video1 折叠后）, got %d: %v", len(idx.deletes), idx.deletes)
	}
	if len(idx.upserts) != 0 {
		t.Fatalf("不应 upsert（1 被折叠为 delete）, got %v", idx.upserts)
	}
}

func TestServiceDrainRetryThenFail(t *testing.T) {
	repo := newFakeRepo()
	idx := &fakeIndexer{failNext: 2}
	loader := &fakeLoader{docs: map[int64]*Doc{1: {VideoID: 1, Title: "t"}}}
	svc := NewService(repo, idx, loader, zap.NewNop())
	svc.maxAttempts = 3

	ctx := context.Background()
	_ = svc.Enqueue(ctx, 1, ActionUpsert)

	// 前两次失败（attempts 1、2），第 3 次成功
	svc.drain(ctx)
	svc.drain(ctx)
	svc.drain(ctx)

	st := repo.statuses()
	for id, s := range st {
		if s != OutboxDone {
			t.Errorf("3 次尝试后应成功, outbox %d status=%d", id, s)
		}
	}
	if len(idx.upserts) != 1 {
		t.Fatalf("最终应 upsert 1 次, got %d", len(idx.upserts))
	}
}

func TestServiceDrainExhausted(t *testing.T) {
	repo := newFakeRepo()
	idx := &fakeIndexer{failNext: 10}
	loader := &fakeLoader{docs: map[int64]*Doc{1: {VideoID: 1, Title: "t"}}}
	svc := NewService(repo, idx, loader, zap.NewNop())
	svc.maxAttempts = 3

	ctx := context.Background()
	_ = svc.Enqueue(ctx, 1, ActionUpsert)

	for i := 0; i < 5; i++ {
		svc.drain(ctx)
	}

	st := repo.statuses()
	for id, s := range st {
		if s != OutboxFailed {
			t.Errorf("超限应标记失败, outbox %d status=%d", id, s)
		}
	}
}

func TestServiceEnqueueDisabled(t *testing.T) {
	repo := newFakeRepo()
	idx := &fakeIndexer{}
	disabled := &Service{repo: repo, indexer: &disabledIndexer{}, loader: nil, log: zap.NewNop()}
	if err := disabled.Enqueue(context.Background(), 1, ActionUpsert); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 0 {
		t.Fatal("ES 未启用时不应产生 Outbox 记录")
	}
	_ = idx
}

type disabledIndexer struct{}

func (disabledIndexer) IsEnabled() bool                     { return false }
func (disabledIndexer) EnsureIndex(context.Context) error   { return nil }
func (disabledIndexer) Upsert(context.Context, Doc) error   { return nil }
func (disabledIndexer) Delete(context.Context, int64) error { return nil }
func (disabledIndexer) Search(context.Context, string, int, int) ([]int64, int64, error) {
	return nil, 0, nil
}
