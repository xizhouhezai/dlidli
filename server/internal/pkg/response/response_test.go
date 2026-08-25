package response

import (
	"net/http"
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
)

func TestHTTPStatusMapping(t *testing.T) {
	cases := []struct {
		name string
		err  *errcode.Error
		want int
	}{
		{"unauthorized", errcode.ErrUnauthorized, http.StatusUnauthorized},
		{"forbidden", errcode.ErrForbidden, http.StatusForbidden},
		{"notfound", errcode.ErrNotFound, http.StatusNotFound},
		{"tooMany", errcode.ErrTooManyRequests, http.StatusTooManyRequests},
		{"internal", errcode.ErrInternal, http.StatusInternalServerError},
		// 业务错误默认 200
		{"biz", errcode.ErrCoinNotEnough, http.StatusOK},
		{"custom", errcode.New(99999, "x"), http.StatusOK},
	}
	for _, c := range cases {
		if got := httpStatus(c.err); got != c.want {
			t.Errorf("%s: httpStatus=%d, want %d", c.name, got, c.want)
		}
	}
}

func TestBodyFields(t *testing.T) {
	b := Body{Code: 0, Message: "ok", Data: map[string]int{"a": 1}, TraceID: "tid"}
	if b.Code != 0 || b.Message != "ok" || b.TraceID != "tid" {
		t.Errorf("Body 字段异常: %+v", b)
	}
	if m, ok := b.Data.(map[string]int); !ok || m["a"] != 1 {
		t.Errorf("Data 异常: %+v", b.Data)
	}
}
