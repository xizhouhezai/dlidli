package live

import (
	"errors"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/video"
	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ---- 房间 ----

// RoomByAnchor 按主播查房间；不存在返回 (nil, nil)。
func (r *Repo) RoomByAnchor(anchorID int64) (*LiveRoom, error) {
	var room LiveRoom
	err := r.db.Where("anchor_id = ?", anchorID).First(&room).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// RoomByID 按 ID 查房间；不存在返回 (nil, nil)。
func (r *Repo) RoomByID(id int64) (*LiveRoom, error) {
	var room LiveRoom
	err := r.db.First(&room, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// CreateRoom 创建常驻房间（一主播一房，uk_anchor 兜底并发）。
func (r *Repo) CreateRoom(room *LiveRoom) error {
	return r.db.Create(room).Error
}

// UpdateRoomFields 按字段更新房间（标题/封面/分区/状态/场次时间等）。
func (r *Repo) UpdateRoomFields(roomID int64, fields map[string]any) error {
	return r.db.Model(&LiveRoom{}).Where("id = ?", roomID).Updates(fields).Error
}

// ListLive 直播中房间（status=1，id 倒序分页）。
func (r *Repo) ListLive(page, size int) ([]LiveRoom, int64, error) {
	q := r.db.Model(&LiveRoom{}).Where("status = ?", RoomStatusLive)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []LiveRoom
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ListLiveByCategory 直播中房间按分区过滤。
func (r *Repo) ListLiveByCategory(categoryID int64, page, size int) ([]LiveRoom, int64, error) {
	q := r.db.Model(&LiveRoom{}).Where("status = ? AND category_id = ?", RoomStatusLive, categoryID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []LiveRoom
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// UpdateLiveCntByDelta DB 兜底在线计数增减（Redis 不可用时降级读它）。
func (r *Repo) UpdateLiveCntByDelta(roomID int64, delta int64) error {
	return r.db.Model(&LiveRoom{}).Where("id = ?", roomID).
		Update("live_cnt", gorm.Expr("GREATEST(live_cnt + ?, 0)", delta)).Error
}

// ResetLiveCnt 在线计数清零（开播/下播重建口径）。
func (r *Repo) ResetLiveCnt(roomID int64) error {
	return r.db.Model(&LiveRoom{}).Where("id = ?", roomID).Update("live_cnt", 0).Error
}

// ---- 白名单 ----

// Whitelisted 用户是否在开播白名单内（removed_at IS NULL）。
func (r *Repo) Whitelisted(uid int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&LiveWhitelist{}).
		Where("user_id = ? AND removed_at IS NULL", uid).Count(&cnt).Error
	return cnt > 0, err
}

// ---- 房管 ----

// IsAdmin 用户是否为指定房间房管。
func (r *Repo) IsAdmin(roomID, uid int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&LiveRoomAdmin{}).
		Where("room_id = ? AND user_id = ?", roomID, uid).Count(&cnt).Error
	return cnt > 0, err
}

// AddAdmin 任命房管（幂等：已在任返回 false）。
func (r *Repo) AddAdmin(roomID, uid, by int64) (bool, error) {
	res := r.db.Create(&LiveRoomAdmin{RoomID: roomID, UserID: uid, CreatedBy: by})
	if res.Error == nil {
		return true, nil
	}
	// GORM TranslateError（infra 与测试同配置）将 1062 转为 ErrDuplicatedKey
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return false, nil
	}
	return false, res.Error
}

// RemoveAdmin 撤销房管；返回是否确有删除。
func (r *Repo) RemoveAdmin(roomID, uid int64) (bool, error) {
	res := r.db.Where("room_id = ? AND user_id = ?", roomID, uid).Delete(&LiveRoomAdmin{})
	return res.RowsAffected > 0, res.Error
}

// ---- 禁言 ----

// UpsertMute 禁言/刷新禁言到期（未到期重复禁言覆盖 muted_until，spec §5C）。
func (r *Repo) UpsertMute(m *LiveRoomMute) error {
	return r.db.Exec(`
INSERT INTO live_room_mute (room_id, user_id, muted_until, reason, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, NOW(), NOW())
ON DUPLICATE KEY UPDATE muted_until = VALUES(muted_until), reason = VALUES(reason), created_by = VALUES(created_by)`,
		m.RoomID, m.UserID, m.MutedUntil, m.Reason, m.CreatedBy).Error
}

// MutedUntil 查询用户在房间的禁言到期时间；无记录返回 (nil, nil)。
func (r *Repo) MutedUntil(roomID, uid int64) (*time.Time, error) {
	var m LiveRoomMute
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, uid).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	until := m.MutedUntil
	return &until, nil
}

// Unmute 解除禁言；返回是否确有删除。
func (r *Repo) Unmute(roomID, uid int64) (bool, error) {
	res := r.db.Where("room_id = ? AND user_id = ?", roomID, uid).Delete(&LiveRoomMute{})
	return res.RowsAffected > 0, res.Error
}

// FanBadge 粉丝徽章等级（进场消息轻量信息）：关注主播的 relation.id 序（1..cap），
// 未关注返回 0；取不到时静默降级 0（徽章是尽力而为的展示信息）。
func (r *Repo) FanBadge(anchor, uid int64, cap int64) int64 {
	var id int64
	err := r.db.Table("relation").Select("id").
		Where("user_id = ? AND target_id = ?", uid, anchor).Take(&id).Error
	if err != nil {
		return 0
	}
	if id > cap {
		return 1
	}
	return id
}

// ---- 弹幕历史窗口（M4-LIV-03）----

// historyWindow 每房间保留的弹幕历史条数（进房拉最近 50 条）。
const historyWindow = 50

// AddRoomMsg 追加一条弹幕并裁剪窗口：仅保留该房间最近 historyWindow 条。
// MySQL 不支持 LIMIT 子查询，先取窗口内最小 id 作为 cutoff 再按整数键删除。
func (r *Repo) AddRoomMsg(m *LiveRoomMsg) error {
	if err := r.db.Create(m).Error; err != nil {
		return err
	}
	var cutoff int64
	err := r.db.Model(&LiveRoomMsg{}).Select("id").
		Where("room_id = ?", m.RoomID).
		Order("id DESC").Offset(historyWindow - 1).Limit(1).
		Take(&cutoff).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // 未超窗（不足 historyWindow 条）无需裁剪
	}
	if err != nil {
		return err
	}
	return r.db.Where("room_id = ? AND id < ?", m.RoomID, cutoff).
		Delete(&LiveRoomMsg{}).Error
}

// ListRoomMsg 进房历史：最近 historyWindow 条按时间正序回放。
func (r *Repo) ListRoomMsg(roomID int64) ([]LiveRoomMsg, error) {
	var rows []LiveRoomMsg
	err := r.db.Where("room_id = ?", roomID).
		Order("id DESC").Limit(historyWindow).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 { // 反转为正序
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

// ClearRoomMsg 清空房间弹幕历史（下播清理；口径见 plan live §2B.2）。
func (r *Repo) ClearRoomMsg(roomID int64) error {
	return r.db.Where("room_id = ?", roomID).Delete(&LiveRoomMsg{}).Error
}

// ---- 跨域只读（资格校验用）----

// CategoryExists 分区存在且启用（复用视频分区表）。
func (r *Repo) CategoryExists(id int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&video.Category{}).Where("id = ? AND status = 0", id).Count(&cnt).Error
	return cnt > 0, err
}

// CountFollowers 粉丝计数（LIV-01：粉丝 ≥ 100）。
func (r *Repo) CountFollowers(target int64) (int64, error) {
	var cnt int64
	err := r.db.Model(&struct{}{}).Table("relation").Where("target_id = ?", target).Count(&cnt).Error
	return cnt, err
}

// UserExists 用户存在（借用户表造数/校验）。
func (r *Repo) UserExists(uid int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&account.User{}).Where("id = ?", uid).Count(&cnt).Error
	return cnt > 0, err
}

// Realnamed 实名判定（LIV-01 第二闸，ACC-23 能力位）。
// 当前账号域尚无独立实名表：按「user_auth 存在手机号或邮箱已激活记录」视为完成实名基线；
// 接入真实实名单/第三方核验后此处替换为实名表查询（spec live §5A.3）。
func (r *Repo) Realnamed(uid int64) (bool, error) {
	var cnt int64
	err := r.db.Table("user_auth").
		Where("user_id = ? AND identity_type IN (?, ?)", uid, account.IdentityPhone, account.IdentityEmail).
		Count(&cnt).Error
	return cnt > 0, err
}
