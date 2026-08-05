package abtest

import "gorm.io/gorm"
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// EnabledOf 指定场景的启用实验（同一场景仅一个）。
func (r *Repo) EnabledOf(target string) (*Experiment, error) {
	var e Experiment
	err := r.db.Where("target = ? AND status = 0", target).Order("id DESC").First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListAll 全部实验（新→旧）。
func (r *Repo) ListAll() ([]Experiment, error) {
	var list []Experiment
	err := r.db.Order("id DESC").Find(&list).Error
	return list, err
}

func (r *Repo) Create(e *Experiment) error {
	return r.db.Create(e).Error
}

func (r *Repo) Update(id int64, fields map[string]any) error {
	return r.db.Model(&Experiment{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) Delete(id int64) error {
	return r.db.Delete(&Experiment{}, id).Error
}
