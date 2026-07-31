package upload

import (
	"errors"

	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// FindByHash 按文件 hash 查已完成文件（秒传依据）；不存在返回 (nil, nil)。
func (r *Repo) FindByHash(hash string) (*UploadFile, error) {
	var f UploadFile
	err := r.db.Where("file_hash = ?", hash).First(&f).Error
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

// Create 登记完成文件；hash 冲突（并发合并同一文件）时返回已存在记录。
func (r *Repo) Create(f *UploadFile) (*UploadFile, error) {
	err := r.db.Create(f).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return r.FindByHash(f.FileHash)
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}
