package live

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ---- 直播回放素材登记（M4-LIV-14，迁移 0042）----

// CreateReplay 登记一条回放素材；同房间同场次唯一键冲突返回 (false, nil)（幂等）。
//
// 为什么要靠唯一键而不是先查后插：on_unpublish 回调可能被 SRS 重复投递
// （SRS 重启、回调超时重试），「先查后插」在并发投递下仍会双写；
// 唯一键 uk_room_session 是数据库层的最终保证。
func (r *Repo) CreateReplay(rep *LiveReplay) (bool, error) {
	err := r.db.Create(rep).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdateReplayFields 按字段更新回放登记。
func (r *Repo) UpdateReplayFields(id int64, fields map[string]any) error {
	return r.db.Model(&LiveReplay{}).Where("id = ?", id).Updates(fields).Error
}

// ReplayByID 按 ID 查回放；不存在返回 (nil, nil)。
func (r *Repo) ReplayByID(id int64) (*LiveReplay, error) {
	var rep LiveReplay
	err := r.db.First(&rep, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// ReplaysByAnchor 主播的已生成回放（含状态，供「我的回放」列表）。
func (r *Repo) ReplaysByAnchor(anchorID int64, limit int) ([]LiveReplay, error) {
	var list []LiveReplay
	err := r.db.Where("anchor_id = ? AND video_id > 0", anchorID).
		Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// ReplaysByRoom 房间的回放（合规追溯：某场直播留了哪些录制与稿件）。
func (r *Repo) ReplaysByRoom(roomID int64) ([]LiveReplay, error) {
	var list []LiveReplay
	err := r.db.Where("room_id = ?", roomID).Order("id DESC").Find(&list).Error
	return list, err
}

// ReplayByVideo 按稿件反查回放登记（发布/删除时校验归属用）；无记录返回 (nil, nil)。
func (r *Repo) ReplayByVideo(videoID int64) (*LiveReplay, error) {
	var rep LiveReplay
	err := r.db.Where("video_id = ?", videoID).First(&rep).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// ExpiredReplaySources 取出已超保留期、尚未清理的录制素材（清理任务用）。
//
// 两个必要条件缺一不可：
//   - dvr_key <> ''：未找到录制文件的失败行没有可删的东西，但它们的登记必须保留
//     （否则「这场直播为什么没有回放」就无从追溯）；
//   - status <> ReplayStatusPurged：**已清理的行必须排除**。少了这一条，已清理行会被
//     每一轮重新选中并再次「清理」（文件已不在，removed 为空），把首次写下
//     「已按保留期清理（dvr）」的原因覆盖成一串空括号，且每一轮都刷一条日志——
//     M4-LIV-14 实测踩到（清理任务每 15s 重复报 count=1）。
func (r *Repo) ExpiredReplaySources(before time.Time, limit int) ([]LiveReplay, error) {
	var list []LiveReplay
	err := r.db.Where("created_at < ? AND dvr_key <> '' AND status <> ?",
		before, ReplayStatusPurged).
		Order("id ASC").Limit(limit).Find(&list).Error
	return list, err
}
