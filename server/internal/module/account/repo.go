package account

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

// FindAuth 按认证方式+标识查找凭据；不存在返回 (nil, nil)。
func (r *Repo) FindAuth(identityType int8, identifier string) (*UserAuth, error) {
	var auth UserAuth
	err := r.db.Where("identity_type = ? AND identifier = ?", identityType, identifier).First(&auth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

// FindUserByID 按 ID 查用户；不存在返回 (nil, nil)。
func (r *Repo) FindUserByID(id int64) (*User, error) {
	var u User
	err := r.db.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindUsersByIDs 批量查用户。
func (r *Repo) FindUsersByIDs(ids []int64) ([]User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []User
	if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// CreateUserWithAuth 事务创建用户与认证凭据。
func (r *Repo) CreateUserWithAuth(u *User, auth *UserAuth) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		auth.UserID = u.ID
		return tx.Create(auth).Error
	})
}

// FindAuthByUser 按用户与认证类型查认证记录。
func (r *Repo) FindAuthByUser(uid int64, identityType int8) (*UserAuth, error) {
	var auth UserAuth
	err := r.db.Where("user_id = ? AND identity_type = ?", uid, identityType).First(&auth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

// UpdateCredentialByUser 更新认证凭证（密码 bcrypt）。
func (r *Repo) UpdateCredentialByUser(uid int64, identityType int8, credential string) error {
	return r.db.Model(&UserAuth{}).
		Where("user_id = ? AND identity_type = ?", uid, identityType).
		Update("credential", credential).Error
}

// UpdateUserFields 按字段更新用户资料。
func (r *Repo) UpdateUserFields(uid int64, fields map[string]any) error {
	return r.db.Model(&User{}).Where("id = ?", uid).Updates(fields).Error
}

// SearchByNickname 昵称模糊搜索（MVP LIKE）。
func (r *Repo) SearchByNickname(keyword string, page, size int) ([]User, int64, error) {
	q := r.db.Model(&User{}).
		Where("nickname LIKE ? AND status = ?", "%"+keyword+"%", UserStatusNormal)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []User
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AdminSearchUsers 后台用户查询：keyword 纯数字时按 UID/手机号精确匹配，否则按昵称模糊；
// status 传 -1 表示全部状态。
func (r *Repo) AdminSearchUsers(keyword string, status int, page, size int) ([]User, int64, error) {
	q := r.db.Model(&User{})
	if keyword != "" {
		isDigits := true
		for _, c := range keyword {
			if c < '0' || c > '9' {
				isDigits = false
				break
			}
		}
		if isDigits {
			q = q.Where("id = ? OR id IN (SELECT user_id FROM user_auth WHERE identity_type = ? AND identifier = ?)",
				keyword, IdentityPhone, keyword)
		} else {
			q = q.Where("nickname LIKE ?", "%"+keyword+"%")
		}
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []User
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AddCoins 硬币增减（事务：余额不可为负 + 流水留痕）；余额不足返回 (false, nil)。
func (r *Repo) AddCoins(uid int64, delta int, reason string) (ok bool, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&User{}).
			Where("id = ? AND coin + ? >= 0", uid, delta).
			UpdateColumn("coin", gorm.Expr("coin + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			ok = false
			return nil
		}
		ok = true
		return tx.Create(&CoinLog{UserID: uid, Delta: delta, Reason: reason}).Error
	})
	return ok, err
}
