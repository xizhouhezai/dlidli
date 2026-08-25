package errcode

import "testing"

func TestWithMsgReturnsCopy(t *testing.T) {
	base := ErrInvalidParams
	copy := base.WithMsg("自定义文案")
	if copy.Code != base.Code {
		t.Errorf("copy.Code=%d, want %d", copy.Code, base.Code)
	}
	if copy.Msg != "自定义文案" {
		t.Errorf("copy.Msg=%q, want 自定义文案", copy.Msg)
	}
	if base.Msg == "自定义文案" {
		t.Error("WithMsg 不应修改原对象")
	}
}

func TestErrorImplementsError(t *testing.T) {
	var _ error = ErrInternal
}

func TestDistinctCodes(t *testing.T) {
	// 通用错误码唯一
	codes := map[int]bool{}
	for _, e := range []*Error{
		ErrInternal, ErrInvalidParams, ErrUnauthorized, ErrForbidden, ErrNotFound, ErrTooManyRequests,
	} {
		if codes[e.Code] {
			t.Errorf("错误码重复: %d", e.Code)
		}
		codes[e.Code] = true
	}
}

func TestAllMessagesNonEmpty(t *testing.T) {
	for _, e := range allErrors() {
		if e.Msg == "" {
			t.Errorf("错误 %d 缺少文案", e.Code)
		}
	}
}

// allErrors 汇集本包全部公开错误，便于统一断言。
func allErrors() []*Error {
	return []*Error{
		ErrInternal, ErrInvalidParams, ErrUnauthorized, ErrForbidden, ErrNotFound, ErrTooManyRequests,
		ErrSmsTooFrequent, ErrSmsCodeInvalid, ErrPasswordMismatch, ErrLoginLocked, ErrRefreshInvalid,
		ErrUserBanned, ErrNicknameTaken, ErrAccountNotExists, ErrCoinNotEnough, ErrUserMuted,
		ErrUploadNotFound, ErrChunkIndexInvalid, ErrUploadIncomplete, ErrFileHashMismatch,
		ErrFileTypeNotAllowed, ErrFileTooLarge,
		ErrDanmakuTooFrequent, ErrDanmakuLevelLow, ErrDanmakuPrivilege, ErrDanmakuDuplicate,
		ErrAlreadyCoined, ErrCoinSelf, ErrCoinCount,
		ErrFollowSelf,
	}
}
