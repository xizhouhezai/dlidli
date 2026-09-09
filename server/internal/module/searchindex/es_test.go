package searchindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeES 内存版 ES（httptest.Server），校验路径/方法并返回 ES 风格响应。
type fakeES struct {
	docs         map[int64]Doc
	mux          *http.ServeMux
	indexCreated bool
}

func newFakeES() (*fakeES, *httptest.Server) {
	f := &fakeES{docs: map[int64]Doc{}, mux: http.NewServeMux()}
	f.mux.HandleFunc("/dlidli_videos/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/dlidli_videos/")
		parts := strings.SplitN(p, "/", 3)
		if len(parts) >= 2 && parts[0] == "_doc" {
			var id int64
			fmt.Sscanf(parts[1], "%d", &id)
			switch r.Method {
			case http.MethodPut:
				var d Doc
				_ = json.NewDecoder(r.Body).Decode(&d)
				f.docs[id] = d
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"_id":"%d","result":"created"}`, id)
				return
			case http.MethodDelete:
				delete(f.docs, id)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"_id":"%d","result":"deleted"}`, id)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	f.mux.HandleFunc("/dlidli_videos/_search", func(w http.ResponseWriter, r *http.Request) {
		// 简易检索：title 包含关键字即命中，按 title 相关度（含关键字数）排序
		var q struct {
			From  int `json:"from"`
			Size  int `json:"size"`
			Query struct {
				MultiMatch struct {
					Query string `json:"query"`
				} `json:"multi_match"`
			} `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		var hits []map[string]any
		total := 0
		for _, d := range f.docs {
			if strings.Contains(d.Title, q.Query.MultiMatch.Query) ||
				strings.Contains(d.Description, q.Query.MultiMatch.Query) ||
				strings.Contains(d.OwnerName, q.Query.MultiMatch.Query) {
				total++
				if len(hits) < q.From+q.Size {
					hits = append(hits, map[string]any{"_source": d})
				}
			}
		}
		// from 之前的不返回，仅返回窗口
		from := q.From
		if from >= len(hits) {
			from = len(hits)
		}
		end := from + q.Size
		if end > len(hits) {
			end = len(hits)
		}
		win := hits[from:end]
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"hits":{"total":{"value":%d},"hits":%s}}`, total, mustJSON(win))
	})
	f.mux.HandleFunc("/dlidli_videos", func(w http.ResponseWriter, r *http.Request) {
		f.indexCreated = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"acknowledged":true}`)
	})
	f.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"fake-es"}`)
	})
	srv := httptest.NewServer(f.mux)
	return f, srv
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestESUpsertSearchDelete(t *testing.T) {
	f, srv := newFakeES()
	defer srv.Close()
	es := NewES(srv.URL, "dlidli_videos")

	ctx := context.Background()
	if err := es.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	if !f.indexCreated {
		t.Error("EnsureIndex 未调用建索引")
	}

	doc := Doc{VideoID: 1, Title: "Go 语言并发实战", Description: "goroutine", OwnerName: "up主甲"}
	if err := es.Upsert(ctx, doc); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	doc2 := Doc{VideoID: 2, Title: "Python 数据分析", OwnerName: "up主乙"}
	if err := es.Upsert(ctx, doc2); err != nil {
		t.Fatalf("Upsert2: %v", err)
	}

	ids, total, err := es.Search(ctx, "Go", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("Search 结果不符: ids=%v total=%d", ids, total)
	}

	if err := es.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// 幂等删除
	if err := es.Delete(ctx, 1); err != nil {
		t.Fatalf("幂等 Delete: %v", err)
	}
	ids, total, _ = es.Search(ctx, "Go", 1, 10)
	if total != 0 {
		t.Fatalf("删除后仍可检索到: ids=%v total=%d", ids, total)
	}
}

func TestESDisabled(t *testing.T) {
	es := NewES("", "dlidli_videos")
	if es.IsEnabled() {
		t.Error("空 baseURL 应视为未启用")
	}
	if _, _, err := es.Search(context.Background(), "x", 1, 10); err == nil {
		t.Error("未启用时 Search 应报错")
	}
	if err := es.Upsert(context.Background(), Doc{}); err == nil {
		t.Error("未启用时 Upsert 应报错")
	}
}
