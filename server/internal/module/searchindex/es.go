package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Indexer Elasticsearch 读写接口（便于测试注入 Fake）。
type Indexer interface {
	// IsEnabled ES 是否启用（未启用时搜索走 MySQL LIKE 兜底）。
	IsEnabled() bool
	// EnsureIndex 幂等创建索引与映射（IK 分词优先，未安装时降级标准分析器）。
	EnsureIndex(ctx context.Context) error
	// Upsert 写入/更新文档。
	Upsert(ctx context.Context, doc Doc) error
	// Delete 删除文档。
	Delete(ctx context.Context, videoID int64) error
	// Search 检索视频，返回命中的稿件 ID 列表与总数。
	Search(ctx context.Context, keyword string, page, size int) (ids []int64, total int64, err error)
}

// ES HTTP 客户端（go 1.25 net/http，无外部依赖）。
type ES struct {
	baseURL string // 如 http://127.0.0.1:9200
	index   string // 索引名
	client  *http.Client
}

// NewES 构建 ES 客户端；baseURL 为空时 IsEnabled 返回 false。
func NewES(baseURL, index string) *ES {
	if index == "" {
		index = "dlidli_videos"
	}
	return &ES{
		baseURL: strings.TrimRight(baseURL, "/"),
		index:   index,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// IsEnabled ES 是否启用。
func (e *ES) IsEnabled() bool { return e.baseURL != "" }

const indexBody = `{
  "settings": {
    "analysis": {
      "analyzer": {
        "ik_smart_search": { "type": "ik_smart" },
        "ik_max_word_search": { "type": "ik_max_word" }
      }
    }
  },
  "mappings": {
    "properties": {
      "video_id":     { "type": "long" },
      "bvid":         { "type": "keyword" },
      "title":        { "type": "text", "analyzer": "ik_max_word_search", "search_analyzer": "ik_smart_search" },
      "description":  { "type": "text", "analyzer": "ik_max_word_search", "search_analyzer": "ik_smart_search" },
      "cover":        { "type": "keyword", "index": false },
      "category_id":  { "type": "integer" },
      "tags":         { "type": "keyword" },
      "owner_id":     { "type": "long" },
      "owner_name":   { "type": "keyword" },
      "duration":     { "type": "integer" },
      "published_at": { "type": "date", "format": "epoch_second" },
      "view_cnt":     { "type": "long" },
      "like_cnt":     { "type": "long" }
    }
  }
}`

// EnsureIndex 幂等建索引；IK 未安装时 ES 会报错，此时降级为标准分析器重试一次。
func (e *ES) EnsureIndex(ctx context.Context) error {
	if !e.IsEnabled() {
		return nil
	}
	_, err := e.do(ctx, http.MethodPut, "/"+e.index, indexBody)
	if err == nil || isAlreadyExists(err) {
		return nil
	}
	// IK 未安装 → 降级标准分析器重试
	body := strings.ReplaceAll(indexBody, `"type": "ik_smart"`, `"type": "standard"`)
	body = strings.ReplaceAll(body, `"analyzer": "ik_max_word_search"`, `"analyzer": "standard"`)
	body = strings.ReplaceAll(body, `"search_analyzer": "ik_smart_search"`, `"search_analyzer": "standard"`)
	_, err2 := e.do(ctx, http.MethodPut, "/"+e.index, body)
	if err2 == nil || isAlreadyExists(err2) {
		return nil
	}
	return err2
}

func isAlreadyExists(err error) bool {
	esErr, ok := err.(*esError)
	return ok && (esErr.Status == http.StatusBadRequest || esErr.Status == http.StatusConflict) &&
		strings.Contains(esErr.Body, "resource_already_exists_exception")
}

// Upsert 文档 ID 用 video_id（幂等）。
func (e *ES) Upsert(ctx context.Context, doc Doc) error {
	if !e.IsEnabled() {
		return fmt.Errorf("ES 未启用")
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = e.do(ctx, http.MethodPut, fmt.Sprintf("/%s/_doc/%d", e.index, doc.VideoID), string(b))
	return err
}

// Delete 删除文档。
func (e *ES) Delete(ctx context.Context, videoID int64) error {
	if !e.IsEnabled() {
		return fmt.Errorf("ES 未启用")
	}
	_, err := e.do(ctx, http.MethodDelete, fmt.Sprintf("/%s/_doc/%d", e.index, videoID), "")
	// 404 视为已删除（幂等）
	if isNotFound(err) {
		return nil
	}
	return err
}

func isNotFound(err error) bool {
	esErr, ok := err.(*esError)
	return ok && esErr.Status == http.StatusNotFound
}

// Search 检索：multi_match（title^3 + description + owner_name^2）+ 分页 + 按相关度排序。
func (e *ES) Search(ctx context.Context, keyword string, page, size int) ([]int64, int64, error) {
	if !e.IsEnabled() {
		return nil, 0, fmt.Errorf("ES 未启用")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	q := fmt.Sprintf(`{
	  "from": %d,
	  "size": %d,
	  "query": {
	    "multi_match": {
	      "query": %s,
	      "fields": ["title^3", "description", "owner_name^2", "tags"]
	    }
	  },
	  "track_total_hits": true
	}`, (page-1)*size, size, strconv.Quote(keyword))
	raw, err := e.do(ctx, http.MethodPost, "/"+e.index+"/_search", q)
	if err != nil {
		return nil, 0, err
	}
	var resp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source Doc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(resp.Hits.Hits))
	for _, h := range resp.Hits.Hits {
		if h.Source.VideoID > 0 {
			ids = append(ids, h.Source.VideoID)
		}
	}
	return ids, resp.Hits.Total.Value, nil
}

// do 发送请求；2xx 返回响应体，否则返回 *esError。
func (e *ES) do(ctx context.Context, method, path, body string) ([]byte, error) {
	u := e.baseURL + path
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return raw, nil
	}
	return nil, &esError{Status: resp.StatusCode, Body: string(raw), URL: u}
}

type esError struct {
	Status int
	Body   string
	URL    string
}

func (e *esError) Error() string {
	return fmt.Sprintf("ES 请求失败: %d %s (%s)", e.Status, e.Body, e.URL)
}
