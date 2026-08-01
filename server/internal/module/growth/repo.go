package growth

import "gorm.io/gorm"

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// CreateExpLog 记录经验流水。
func (r *Repo) CreateExpLog(log *ExpLog) error {
	return r.db.Create(log).Error
}

// ListExpLogs 经验流水分页（新→旧）。
func (r *Repo) ListExpLogs(uid int64, page, size int) ([]ExpLog, int64, error) {
	q := r.db.Model(&ExpLog{}).Where("user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []ExpLog
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// LevelAndExp 返回用户当前等级与累计经验。
func (r *Repo) LevelAndExp(uid int64) (exp, level int, err error) {
	var u struct {
		Exp   int `gorm:"column:exp"`
		Level int `gorm:"column:level"`
	}
	if err := r.db.Table("user").Select("exp", "level").Where("id = ?", uid).Scan(&u).Error; err != nil {
		return 0, 0, err
	}
	return u.Exp, u.Level, nil
}

// AddExp 事务：经验累加 + 等级重算落库（user 表），返回最新 exp/level。
func (r *Repo) AddExp(uid int64, delta int) (exp, level int, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		var cur struct {
			Exp int `gorm:"column:exp"`
		}
		if err := tx.Table("user").Select("exp").Where("id = ?", uid).Scan(&cur).Error; err != nil {
			return err
		}
		exp = cur.Exp + delta
		if exp < 0 {
			exp = 0
		}
		level = int(LevelByExp(exp))
		return tx.Table("user").Where("id = ?", uid).Updates(map[string]any{
			"exp":   exp,
			"level": level,
		}).Error
	})
	return exp, level, err
}
