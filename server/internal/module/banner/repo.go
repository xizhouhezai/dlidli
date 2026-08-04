package banner

import "gorm.io/gorm"

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListEnabled 启用的 Banner（按 sort），最多 8 条。
func (r *Repo) ListEnabled() ([]Banner, error) {
	var list []Banner
	err := r.db.Where("status = 0").Order("sort, id").Limit(8).Find(&list).Error
	return list, err
}

// ListAll 全部 Banner（admin，按 sort）。
func (r *Repo) ListAll() ([]Banner, error) {
	var list []Banner
	err := r.db.Order("sort, id").Find(&list).Error
	return list, err
}

func (r *Repo) FindByID(id int64) (*Banner, error) {
	var b Banner
	err := r.db.First(&b, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *Repo) Create(b *Banner) error {
	return r.db.Create(b).Error
}

func (r *Repo) Update(id int64, fields map[string]any) error {
	return r.db.Model(&Banner{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) Delete(id int64) error {
	return r.db.Delete(&Banner{}, id).Error
}
