package upload

import (
	"testing"

	"github.com/dlidli/server/internal/pkg/errcode"
)

func TestEnsureOwnership(t *testing.T) {
	file := &UploadFile{ID: 1, UserID: 100}
	// 本人文件放行
	if err := ensureOwnership(file, 100); err != nil {
		t.Errorf("本人文件应放行, got %v", err)
	}
	// 他人文件拒绝
	err := ensureOwnership(file, 200)
	if err == nil {
		t.Fatal("他人文件应拒绝")
	}
	if e, ok := err.(*errcode.Error); !ok || e.Code != errcode.ErrForbidden.Code {
		t.Errorf("应返回禁权错误, got %v", err)
	}
}
