package live

import (
	"errors"
	"strconv"
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

// ---- 截帧机审与违规流水（M4-LIV-05）----

// ListLiveRoomsAll 全部直播中房间（截帧机审遍历用，不分页）。
func (r *Repo) ListLiveRoomsAll() ([]LiveRoom, error) {
	var list []LiveRoom
	err := r.db.Where("status = ?", RoomStatusLive).Order("id ASC").Find(&list).Error
	return list, err
}

// CreateFrameAudit 落一条截帧机审记录（spec §5E.5 合规存档凭据）。
func (r *Repo) CreateFrameAudit(rec *LiveFrameAudit) error {
	return r.db.Create(rec).Error
}

// LatestFrameAudit 房间最近一次截帧判定（巡查列表摘要用）；无记录返回 (nil, nil)。
func (r *Repo) LatestFrameAudit(roomID int64) (*LiveFrameAudit, error) {
	var rec LiveFrameAudit
	err := r.db.Where("room_id = ?", roomID).Order("id DESC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// CreateViolation 落一条违规流水（先于切断动作写入，保证可审计）。
func (r *Repo) CreateViolation(log *LiveViolationLog) error {
	return r.db.Create(log).Error
}

// SetViolationCutMode 回填违规流水的实际切断方式（1 srs_api / 2 degraded）。
func (r *Repo) SetViolationCutMode(id int64, mode int8) error {
	return r.db.Model(&LiveViolationLog{}).Where("id = ?", id).
		Update("cut_mode", mode).Error
}

// ListViolationsByRoom 房间违规流水（巡查/追溯用，倒序分页）。
func (r *Repo) ListViolationsByRoom(roomID int64, limit int) ([]LiveViolationLog, error) {
	var list []LiveViolationLog
	err := r.db.Where("room_id = ?", roomID).Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// ViolationRecord 违规流水行 + 操作人用户名（管理端「违规记录」展示用）。
//
// 单独 DTO 而非直接序列化 LiveViolationLog：表里只存 cut_by（管理员 ID），
// 而「谁切断的」正是巡台要看的字段 —— ID 对人不可读，故在查询里 JOIN 出用户名。
type ViolationRecord struct {
	ID        int64  `json:"id"`
	RoomID    int64  `json:"room_id"`
	AnchorID  string `json:"anchor_id"`
	Source    int8   `json:"source"`
	Reason    string `json:"reason"`
	CutMode   int8   `json:"cut_mode"`
	CutBy     string `json:"cut_by"`
	Operator  string `json:"operator"` // 管理员用户名；人工切断必有，机审为空
	CreatedAt string `json:"created_at"`
}

// ListViolations 全站违规流水（倒序分页，roomID>0 时按房间过滤）。
func (r *Repo) ListViolations(roomID int64, page, size int) ([]ViolationRecord, int64, error) {
	q := r.db.Model(&LiveViolationLog{})
	if roomID > 0 {
		q = q.Where("room_id = ?", roomID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []struct {
		LiveViolationLog
		Username string
	}
	// LEFT JOIN：机审流水 cut_by=0 无对应管理员，不能因它丢行。
	err := q.Select("live_violation_log.*, admin_user.username AS username").
		Joins("LEFT JOIN admin_user ON admin_user.id = live_violation_log.cut_by").
		Order("live_violation_log.id DESC").
		Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]ViolationRecord, 0, len(rows))
	for i := range rows {
		v := rows[i]
		rec := ViolationRecord{
			ID: v.ID, RoomID: v.RoomID, AnchorID: strconv.FormatInt(v.AnchorID, 10),
			Source: v.Source, Reason: v.Reason, CutMode: v.CutMode,
			CutBy: strconv.FormatInt(v.CutBy, 10), Operator: v.Username,
			CreatedAt: v.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		out = append(out, rec)
	}
	return out, total, nil
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

// ListAdmins 房间房管列表（任命时间正序）。
func (r *Repo) ListAdmins(roomID int64) ([]LiveRoomAdmin, error) {
	var rows []LiveRoomAdmin
	err := r.db.Where("room_id = ?", roomID).Order("created_at ASC").Find(&rows).Error
	return rows, err
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

// ListMutes 房间禁言列表（未到期在前，其余按到期时间正序）。
func (r *Repo) ListMutes(roomID int64) ([]LiveRoomMute, error) {
	var rows []LiveRoomMute
	err := r.db.Where("room_id = ?", roomID).Order("muted_until ASC").Find(&rows).Error
	return rows, err
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
