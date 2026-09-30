package upload

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// FindByUserHash 按「用户 + 文件 hash」查已完成文件（秒传依据，VID-24）；不存在返回 (nil, nil)。
// 秒传必须限定属主：upload_file 的属主即投稿人，跨用户复用会拿到他人 file_id。
//
// 注：早期另有不限定用户的 FindByHash，它正是 2026-09-30 缺陷①（跨用户返回他人 file_id）
// 的载体，已随 0033 迁移一并删除——需要「该内容是否已存在于库中」时请查 video_stream.play_path。
func (r *Repo) FindByUserHash(uid int64, hash string) (*UploadFile, error) {
	var f UploadFile
	err := r.db.Where("user_id = ? AND file_hash = ?", uid, hash).First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindByID 按 ID 查文件；不存在返回 (nil, nil)。
func (r *Repo) FindByID(id int64) (*UploadFile, error) {
	var f UploadFile
	err := r.db.First(&f, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Create 登记完成文件；唯一键 (user_id, file_hash) 冲突时返回【该用户自己的】已存在记录。
//
// 语义：同一用户并发合并同一文件时，后到者复用先到者的记录（幂等）。
// 若命中的记录属主不是 f.UserID，说明唯一键与归属语义不一致（见 0033 迁移前的
// uk_hash 全局唯一缺陷），此时**必须报错**而不是返回他人记录——否则调用方会拿到
// 不属于自己的 file_id，直到投稿才以 10004 暴露。
func (r *Repo) Create(f *UploadFile) (*UploadFile, error) {
	err := r.db.Create(f).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		existing, findErr := r.FindByUserHash(f.UserID, f.FileHash)
		if findErr != nil {
			return nil, findErr
		}
		if existing == nil {
			// 唯一键冲突但按「用户+hash」查不到：唯一键与归属语义不一致
			return nil, fmt.Errorf("文件登记冲突：hash=%s 在用户 %d 下无记录（唯一键定义与归属校验不一致）", f.FileHash, f.UserID)
		}
		return existing, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}
