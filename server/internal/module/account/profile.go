package account

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/storage"
	"gorm.io/gorm"
)

const avatarMaxSize = 2 << 20 // 2MB

var avatarExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// UpdateProfileReq 资料更新请求；nil 字段表示不修改。
type UpdateProfileReq struct {
	Nickname  *string `json:"nickname"`
	Signature *string `json:"signature"`
	Gender    *int8   `json:"gender"`
}

// UpdateProfile 更新资料并返回最新 Profile。
func (s *Service) UpdateProfile(ctx context.Context, uid int64, req *UpdateProfileReq) (*Profile, error) {
	fields := map[string]any{}

	if req.Nickname != nil {
		nick := strings.TrimSpace(*req.Nickname)
		if n := utf8.RuneCountInString(nick); n < 2 || n > 24 {
			return nil, errcode.ErrInvalidParams.WithMsg("昵称长度须为 2-24 个字符")
		}
		// TODO(M2-AUD): 昵称接入内容安全机审
		fields["nickname"] = nick
	}
	if req.Signature != nil {
		if utf8.RuneCountInString(*req.Signature) > 200 {
			return nil, errcode.ErrInvalidParams.WithMsg("签名最长 200 个字符")
		}
		fields["signature"] = *req.Signature
	}
	if req.Gender != nil {
		if *req.Gender < 0 || *req.Gender > 2 {
			return nil, errcode.ErrInvalidParams
		}
		fields["gender"] = *req.Gender
	}
	if len(fields) == 0 {
		return s.Me(ctx, uid)
	}

	if err := s.repo.UpdateUserFields(uid, fields); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errcode.ErrNicknameTaken
		}
		return nil, err
	}
	return s.Me(ctx, uid)
}

// UploadAvatar 上传头像并更新用户资料，返回头像 URL。
func (s *Service) UploadAvatar(ctx context.Context, uid int64, fh *multipart.FileHeader, store storage.Storage) (string, error) {
	if store == nil {
		return "", errcode.ErrInternal.WithMsg("存储服务未就绪")
	}
	if fh.Size <= 0 || fh.Size > avatarMaxSize {
		return "", errcode.ErrInvalidParams.WithMsg("头像大小须在 2MB 以内")
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !avatarExts[ext] {
		return "", errcode.ErrInvalidParams.WithMsg("仅支持 jpg / png / webp 格式")
	}

	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	// TODO(M2-AUD): 头像接入图片内容安全机审后再生效
	key := fmt.Sprintf("avatars/%d_%d%s", uid, time.Now().UnixMilli(), ext)
	url, err := store.Save(ctx, key, io.LimitReader(f, avatarMaxSize))
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdateUserFields(uid, map[string]any{"avatar": url}); err != nil {
		return "", err
	}
	return url, nil
}
