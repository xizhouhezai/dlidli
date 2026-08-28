package account

import (
	"errors"

	"github.com/dlidli/server/internal/pkg/encrypt"
	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// FindAuth 按认证方式+标识查找凭据；不存在返回 (nil, nil)。
// 优先按确定性哈希精确匹配（ACC-43 密文存储）；未命中时回退旧明文列，
// 兼容尚未惰性迁移的存量手机号/邮箱账号。
func (r *Repo) FindAuth(identityType int8, identifier string) (*UserAuth, error) {
	hash := encrypt.IdentifierHash(identityType, identifier)
	var auth UserAuth
	err := r.db.Where("identity_type = ? AND identifier_hash = ?", identityType, hash).First(&auth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 存量明文回退：命中后由调用方做惰性加密回填（见 service migrateAuth）
		err = r.db.Where("identity_type = ? AND identifier = ?", identityType, identifier).First(&auth).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

// FindAuthByHash 纯按哈希查找（新数据路径，无需明文回退）。
func (r *Repo) FindAuthByHash(identityType int8, hash string) (*UserAuth, error) {
	var auth UserAuth
	err := r.db.Where("identity_type = ? AND identifier_hash = ?", identityType, hash).First(&auth).Error
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

// UpdateIdentifierEncrypted 存量明文标识加密回填：写密文 + 哈希（ACC-43）。
func (r *Repo) UpdateIdentifierEncrypted(authID int64, ciphertext, hash string) error {
	return r.db.Model(&UserAuth{}).
		Where("id = ?", authID).
		Updates(map[string]any{"identifier": ciphertext, "identifier_hash": hash}).Error
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
// status 传 -1 表示全部状态。phoneHash 为手机号确定性哈希（ACC-43 密文存储下按哈希匹配），
// 由 service 层计算传入；空串表示该分支不参与（纯 UID 匹配时不传哈希亦可）。
func (r *Repo) AdminSearchUsers(keyword string, phoneHash string, status int, page, size int) ([]User, int64, error) {
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
			cond := "id = ?"
			args := []any{keyword}
			if phoneHash != "" {
				cond += " OR id IN (SELECT user_id FROM user_auth WHERE identity_type = ? AND identifier_hash = ?)"
				args = append(args, IdentityPhone, phoneHash)
			}
			// 兼容历史明文：仍未回填的存量手机号按明文 identifier 继续可查（两阶段迁移期间）
			cond += " OR id IN (SELECT user_id FROM user_auth WHERE identity_type = ? AND identifier = ?)"
			args = append(args, IdentityPhone, keyword)
			q = q.Where(cond, args...)
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

// PhoneByUsers 批量取用户手机号（identity_type=1），返回 uid→手机号 映射（未绑定不出现）。
// 返回的为密文（ACC-43），由调用方解密展示。
func (r *Repo) PhoneByUsers(ids []int64) map[int64]string {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var auths []UserAuth
	if err := r.db.Where("identity_type = ? AND user_id IN ?", IdentityPhone, ids).Find(&auths).Error; err != nil {
		return out
	}
	for _, a := range auths {
		out[a.UserID] = a.Identifier
	}
	return out
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

// ListCoinLogs 硬币流水分页（新→旧）。
func (r *Repo) ListCoinLogs(uid int64, page, size int) ([]CoinLog, int64, error) {
	q := r.db.Model(&CoinLog{}).Where("user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []CoinLog
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}
