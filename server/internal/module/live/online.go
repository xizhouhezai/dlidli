package live

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 在线人数口径（spec live §5B / plan live §2A.3）：
//   live:room:{id}:online —— INCR/DECR 计数（房间详情读它）；
//   live:room:{id}:hb:{uid} —— TTL 30s 心跳过期键（进房判重 + 懒剔除依据）。
// 心跳 ≤15s 一次；30s 无心跳视为离线，由「先到期的懒剔除」扣除。
const (
	hbTTL      = 30 * time.Second
	onlineFmt  = "live:room:%d:online"
	hbFmt      = "live:room:%d:hb:%d"
)

type Online struct {
	rdb *redis.Client
	db  *Repo
	log *zap.Logger
}

func NewOnline(rdb *redis.Client, db *Repo, log *zap.Logger) *Online {
	return &Online{rdb: rdb, db: db, log: log}
}

func onlineKey(roomID int64) string  { return fmt.Sprintf(onlineFmt, roomID) }
func hbKey(roomID, uid int64) string { return fmt.Sprintf(hbFmt, roomID, uid) }

// degraded Redis 不可用（客户端为 nil 或调用出错）：降级 DB 兜底计数。
func (o *Online) degraded(ctx context.Context, roomID int64, delta int64) error {
	o.log.Warn("在线人数 Redis 不可用，降级 DB 兜底", zap.Int64("room", roomID))
	return o.db.UpdateLiveCntByDelta(roomID, delta)
}

// Enter 进房：心跳键判重（已在线不重复计数），否则 INCR + DB 兜底 +1。
// Redis 不可用时仅落 DB 兜底计数（误差容忍，在线人数非资金数据）。
func (o *Online) Enter(ctx context.Context, roomID, uid int64) error {
	if o.rdb == nil {
		return o.degraded(ctx, roomID, 1)
	}
	ok, err := o.rdb.SetNX(ctx, hbKey(roomID, uid), 1, hbTTL).Result()
	if err != nil {
		return o.degraded(ctx, roomID, 1)
	}
	if !ok {
		return nil // 已在线（心跳键未过期），不重复计数
	}
	if err := o.rdb.Incr(ctx, onlineKey(roomID)).Err(); err != nil {
		return err
	}
	return o.db.UpdateLiveCntByDelta(roomID, 1)
}

// Heartbeat 心跳：刷新 TTL；心跳键已丢失（30s 无心跳判离线）时，
// 在线计数中残留的旧条目视为「到期」，扣减后按重新进房补记（净变化 0，口径自校准）。
func (o *Online) Heartbeat(ctx context.Context, roomID, uid int64) error {
	if o.rdb == nil {
		return nil // Redis 不可用心跳静默，计数维持现状
	}
	ok, err := o.rdb.Expire(ctx, hbKey(roomID, uid), hbTTL).Result()
	if err != nil {
		return nil // Redis 不可用心跳静默，计数维持现状
	}
	if !ok { // 键不存在：旧条目已到期 → 扣减陈旧计数后按进房补记
		_ = o.rdb.Decr(ctx, onlineKey(roomID)).Err()
		return o.Enter(ctx, roomID, uid)
	}
	return nil
}

// Leave 显式离房：DEL 心跳键 + DECR + DB 兜底 −1。幂等：键已不在则不重复扣。
func (o *Online) Leave(ctx context.Context, roomID, uid int64) error {
	if o.rdb == nil {
		return o.degraded(ctx, roomID, -1)
	}
	del, err := o.rdb.Del(ctx, hbKey(roomID, uid)).Result()
	if err != nil {
		return o.degraded(ctx, roomID, -1)
	}
	if del == 0 {
		return nil // 已不在在线集合（可能已被懒剔除），不重复扣
	}
	_ = o.rdb.Decr(ctx, onlineKey(roomID)).Err()
	return o.db.UpdateLiveCntByDelta(roomID, -1)
}

// Count 在线人数：Redis 计数；Redis 不可用降级读 DB 兜底 live_cnt。
func (o *Online) Count(ctx context.Context, roomID int64) (int64, error) {
	if o.rdb == nil {
		return o.dbLiveCnt(roomID)
	}
	n, err := o.rdb.Get(ctx, onlineKey(roomID)).Result()
	if err == redis.Nil {
		return o.dbLiveCnt(roomID)
	}
	if err != nil {
		o.log.Warn("在线人数 Redis 不可用，降级 DB 兜底", zap.Error(err), zap.Int64("room", roomID))
		return o.dbLiveCnt(roomID)
	}
	v, _ := strconv.ParseInt(n, 10, 64)
	if v < 0 {
		return 0, nil
	}
	return v, nil
}

// dbLiveCnt DB 兜底计数（GREATEST 约束下非负）。
func (o *Online) dbLiveCnt(roomID int64) (int64, error) {
	room, err := o.db.RoomByID(roomID)
	if err != nil || room == nil {
		return 0, err
	}
	return room.LiveCnt, nil
}

// Reset 开播/下播时清零（口径重建）。
func (o *Online) Reset(ctx context.Context, roomID int64) error {
	if o.rdb != nil {
		_ = o.rdb.Del(ctx, onlineKey(roomID)).Err()
	}
	return o.db.ResetLiveCnt(roomID)
}

// IsMuted 禁言判定（供 M4-LIV-03 弹幕通道调用）：记录存在且未到期即禁言中。
func (o *Online) IsMuted(ctx context.Context, roomID, uid int64) (bool, error) {
	until, err := o.db.MutedUntil(roomID, uid)
	if err != nil || until == nil {
		return false, err
	}
	return time.Now().Before(*until), nil
}

// ---- WS 生命周期计数原语（M4-LIV-03：live hub 断开即离房，plan live §2B.1）----

// EnterForce WS 进房：SETNX 心跳键判重后强制 INCR（连接生命周期即在线状态，
// 不依赖 HTTP 进房先行——WS 建连即进房）；键已存在（HTTP 进房未过期）不重复计数。
// 返回本次是否真正计数（供断开时按标志配对扣减——hb 键 TTL 30s 可能先于长连接
// 断开过期，届时 DEL 幂等会漏扣，故扣减不能依赖键存在性）。
func (o *Online) EnterForce(ctx context.Context, roomID, uid int64) bool {
	if o.rdb == nil {
		return false
	}
	ok, err := o.rdb.SetNX(ctx, hbKey(roomID, uid), 1, hbTTL).Result()
	if err != nil {
		o.log.Warn("WS 进房计数失败", zap.Int64("room", roomID), zap.Error(err))
		return false
	}
	if !ok {
		return false // 已在线（心跳键未过期），不重复计数
	}
	if err := o.rdb.Incr(ctx, onlineKey(roomID)).Err(); err != nil {
		o.log.Warn("WS 进房 INCR 失败", zap.Int64("room", roomID), zap.Error(err))
		return false
	}
	_ = o.db.UpdateLiveCntByDelta(roomID, 1)
	return true
}

// Beat 心跳续命：只刷 TTL（已在在线集合内，不判定、不增减计数）。
func (o *Online) Beat(ctx context.Context, roomID, uid int64) {
	if o.rdb == nil {
		return
	}
	if err := o.rdb.Expire(ctx, hbKey(roomID, uid), hbTTL).Err(); err != nil {
		o.log.Warn("WS 心跳续期失败", zap.Int64("room", roomID), zap.Error(err))
	}
}

// LeaveForce WS 离房（断开回调）：按进房时的计数标志配对扣减，并尽力 DEL 心跳键。
// counted=true 时无条件 DECR（即使 hb 键已过期——Enter 时确曾 +1，键过期
// 只是 TTL 先到，计数不因键消失而消失）；counted=false 不扣（判重路径进房）。
func (o *Online) LeaveForce(ctx context.Context, roomID, uid int64, counted bool) {
	if o.rdb == nil {
		return
	}
	_, _ = o.rdb.Del(ctx, hbKey(roomID, uid)).Result()
	if !counted {
		return
	}
	if err := o.rdb.Decr(ctx, onlineKey(roomID)).Err(); err != nil {
		o.log.Warn("WS 离房 DECR 失败", zap.Int64("room", roomID), zap.Error(err))
		return
	}
	_ = o.db.UpdateLiveCntByDelta(roomID, -1)
}
