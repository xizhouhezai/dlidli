// Package counter 计数服务（M3-ENG-01）：视频统计的幂等累加与批量读取。
//
// 设计要点（见 docs/architecture/adr-m3-eng-01-grpc-split.md）：
//   - 计数列使用白名单映射，禁止把调用方输入带入 SQL 列名（与 M3-ENG-03 表名白名单同一防御思路）；
//   - ApplyDelta 以 event_id 幂等去重，保证"至少一次投递"下计数不重复；
//   - 不反向依赖互动模块，避免循环依赖。
//
// 本包同时提供 gRPC 服务端实现（通过 internal/gen/counter/v1 的注册函数挂载）。
package counter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	counterv1 "github.com/dlidli/server/internal/gen/counter/v1"
	"github.com/dlidli/server/internal/pkg/errcode"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 计数事件去重表名（幂等键登记处）。
const eventTable = "counter_event"

// Column 白名单列名（数据库列），与 counter.v1.CounterColumn 一一对应。
// 注意：播放列在既有 schema 中叫 view_cnt（见 0001_init.up.sql），
// 而 proto 枚举名为 COUNTER_COLUMN_PLAY，映射时必须落到 view_cnt。
var allowColumns = map[string]string{
	"view_cnt":    "view_cnt",
	"like_cnt":    "like_cnt",
	"coin_cnt":    "coin_cnt",
	"fav_cnt":     "fav_cnt",
	"comment_cnt": "comment_cnt",
	"danmaku_cnt": "danmaku_cnt",
	"share_cnt":   "share_cnt",
}

// ColumnFromProto 把 proto 枚举翻译成白名单列名。
// 未登记（含 UNSPECIFIED）时返回错误，调用方不得执行任何 DML。
func ColumnFromProto(c counterv1.CounterColumn) (string, error) {
	switch c {
	case counterv1.CounterColumn_COUNTER_COLUMN_PLAY:
		return "view_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_LIKE:
		return "like_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_COIN:
		return "coin_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_FAV:
		return "fav_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_COMMENT:
		return "comment_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_DANMAKU:
		return "danmaku_cnt", nil
	case counterv1.CounterColumn_COUNTER_COLUMN_SHARE:
		return "share_cnt", nil
	default:
		return "", fmt.Errorf("未登记的计数列: %v", c)
	}
}

// Service 计数服务。
type Service struct {
	db *gorm.DB
}

// NewService 构建计数服务。
func NewService(db *gorm.DB) *Service { return &Service{db: db} }

// counterEvent 幂等键登记行（counter_event 表）。
// 注意：column 是 MySQL 保留字，GORM 会自动加反引号；此处显式声明列名以避免歧义。
type counterEvent struct {
	EventID   string `gorm:"primaryKey;size:128"`
	VideoID   int64  `gorm:"index"`
	Column    string `gorm:"column:column;size:32"`
	Delta     int64
	CreatedAt time.Time
}

// TableName 固定表名（本表不参与分表）。
func (counterEvent) TableName() string { return eventTable }

// statColumns 统计读取的显式列清单（避免 SELECT *，遵循 SQL 规范）。
// 与 video_stat 的列名一一对应；播放列是 view_cnt 而非 play_cnt。
const statColumns = "video_id, view_cnt, like_cnt, coin_cnt, fav_cnt, comment_cnt, danmaku_cnt, share_cnt"

// stat 仅取计数列（避免 SELECT *，遵循 SQL 规范）。
// 字段名必须与 video_stat 实际列名一致：播放列是 view_cnt（非 play_cnt）。
type stat struct {
	VideoID    int64
	ViewCnt    int64
	LikeCnt    int64
	CoinCnt    int64
	FavCnt     int64
	CommentCnt int64
	DanmakuCnt int64
	ShareCnt   int64
}

// TableName 复用既有 video_stat 表（计数服务拥有该表）。
func (stat) TableName() string { return "video_stat" }

// ApplyDelta 幂等累加：同一 event_id 只生效一次。
//
// 返回 applied=false 表示该事件此前已处理（幂等命中），current 为该列当前值。
//
// 并发说明：多个调用方同时投递同一 event_id 时，InnoDB 的唯一键冲突处理
// 可能与 video_stat 的行锁形成死锁（错误 1213）。这里用有界重试收敛，
// 而不是把死锁暴露给调用方——重试是这类"幂等键 + 计数行"写法的标准做法。
func (s *Service) ApplyDelta(ctx context.Context, eventID string, videoID int64, col counterv1.CounterColumn, delta int64) (applied bool, current int64, err error) {
	if strings.TrimSpace(eventID) == "" {
		return false, 0, errcode.ErrInvalidParams.WithMsg("event_id 不能为空")
	}
	if videoID <= 0 {
		return false, 0, errcode.ErrInvalidParams.WithMsg("video_id 非法")
	}
	column, err := ColumnFromProto(col)
	if err != nil {
		return false, 0, errcode.ErrInvalidParams.WithMsg(err.Error())
	}
	// 双保险：即使 ColumnFromProto 被改错，也不允许未登记列进入 SQL。
	if _, ok := allowColumns[column]; !ok {
		return false, 0, errcode.ErrInvalidParams.WithMsg("非法计数列: " + column)
	}

	for attempt := 0; ; attempt++ {
		applied, err = s.applyOnce(ctx, eventID, videoID, column, delta)
		if err == nil {
			break
		}
		if !isDeadlock(err) || attempt >= deadlockMaxRetries {
			return false, 0, err
		}
		// 指数退避（10ms / 20ms / 40ms），避免重试风暴
		select {
		case <-ctx.Done():
			return false, 0, ctx.Err()
		case <-time.After(deadlockBaseBackoff << attempt):
		}
	}

	current, err = s.columnValue(ctx, videoID, column)
	if err != nil {
		return applied, 0, err
	}
	return applied, current, nil
}

const (
	// deadlockMaxRetries 死锁重试次数上限。
	deadlockMaxRetries = 3
	// deadlockBaseBackoff 首次重试等待时间，之后按 2 的幂递增。
	deadlockBaseBackoff = 10 * time.Millisecond
	// mysqlErrDeadlock MySQL 死锁错误码。
	mysqlErrDeadlock = 1213
)

// isDeadlock 判断是否为 MySQL 死锁错误（1213）。
func isDeadlock(err error) bool {
	var me *mysqldriver.MySQLError
	if errors.As(err, &me) {
		return me.Number == mysqlErrDeadlock
	}
	return false
}

// applyOnce 单次幂等累加尝试（在事务内完成"登记事件 + 累加计数"）。
func (s *Service) applyOnce(ctx context.Context, eventID string, videoID int64, column string, delta int64) (applied bool, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) 幂等键登记：DoNothing 让重复投递变成 0 行影响，而非报错。
		ev := counterEvent{EventID: eventID, VideoID: videoID, Column: column, Delta: delta, CreatedAt: time.Now()}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ev)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 幂等命中：本次不累加
			applied = false
			return nil
		}
		applied = true

		// 2) 原子 upsert 计数行：不存在则建行，存在则累加并做下限 0 保护。
		// column 来自白名单映射（见 allowColumns），不存在注入风险。
		insertValue := delta
		if insertValue < 0 {
			insertValue = 0 // 新行不接受负的初始值
		}
		sql := "INSERT INTO video_stat (video_id, `" + column + "`) VALUES (?, ?) " +
			"ON DUPLICATE KEY UPDATE `" + column + "` = GREATEST(CAST(`" + column + "` AS SIGNED) + ?, 0)"
		return tx.Exec(sql, videoID, insertValue, delta).Error
	})
	return applied, err
}

// columnValue 读取单个视频的某列当前值。
func (s *Service) columnValue(ctx context.Context, videoID int64, column string) (int64, error) {
	var value int64
	err := s.db.WithContext(ctx).Model(&stat{}).Where("video_id = ?", videoID).
		Select(column).Scan(&value).Error
	if err != nil {
		return 0, err
	}
	return value, nil
}

// GetStat 读取单个视频统计快照。
func (s *Service) GetStat(ctx context.Context, videoID int64) (*counterv1.StatSnapshot, error) {
	if videoID <= 0 {
		return nil, errcode.ErrInvalidParams.WithMsg("video_id 非法")
	}
	var row stat
	err := s.db.WithContext(ctx).
		Select(statColumns).
		Where("video_id = ?", videoID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 统计行尚未建立时返回零值快照，调用方无需区分
		return &counterv1.StatSnapshot{VideoId: videoID}, nil
	}
	if err != nil {
		return nil, err
	}
	return toSnapshot(row), nil
}

// BatchGetStats 批量读取（首页/列表页，避免 N+1 往返）。
// 返回结果按 video_ids 顺序补齐，缺失的视频返回零值快照。
func (s *Service) BatchGetStats(ctx context.Context, videoIDs []int64) ([]*counterv1.StatSnapshot, error) {
	if len(videoIDs) == 0 {
		return []*counterv1.StatSnapshot{}, nil
	}
	var rows []stat
	err := s.db.WithContext(ctx).
		Select(statColumns).
		Where("video_id IN ?", videoIDs).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*counterv1.StatSnapshot, len(rows))
	for _, r := range rows {
		byID[r.VideoID] = toSnapshot(r)
	}
	out := make([]*counterv1.StatSnapshot, 0, len(videoIDs))
	for _, id := range videoIDs {
		if snap, ok := byID[id]; ok {
			out = append(out, snap)
			continue
		}
		out = append(out, &counterv1.StatSnapshot{VideoId: id})
	}
	return out, nil
}

// toSnapshot 行 → proto 快照。
func toSnapshot(r stat) *counterv1.StatSnapshot {
	return &counterv1.StatSnapshot{
		VideoId:    r.VideoID,
		PlayCnt:    r.ViewCnt,
		LikeCnt:    r.LikeCnt,
		CoinCnt:    r.CoinCnt,
		FavCnt:     r.FavCnt,
		CommentCnt: r.CommentCnt,
		DanmakuCnt: r.DanmakuCnt,
		ShareCnt:   r.ShareCnt,
	}
}
