package vip

import (
	"errors"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockingClause SELECT ... FOR UPDATE 行锁（会员到期时间强一致核心）。
var lockingClause = clause.Locking{Strength: "UPDATE"}

// repo 会员域数据访问。**本文件是会员到期时间与赠币的唯一写入入口**：
// 绕过此处直接 UPDATE membership.expire_at 会绕开顺延语义与赠币幂等。
type repo struct {
	db *gorm.DB
}

func newRepo(db *gorm.DB) *repo { return &repo{db: db} }

// NewRepo 会员域 repo（router 装配入口）。
func NewRepo(db *gorm.DB) *repo { return newRepo(db) }

// coinLog 硬币流水（coin_log）的本包最小映射。
//
// 刻意不 import account 包：会员赠币必须与会员状态在同一事务内落库
// （否则会出现「会员已生效但币没发」），而 account 的 AddCoins 自带独立事务。
// **这是硬币（coin_log），不是金瓜子（melon_seed_*）**——spec §2.1 里两者不同。
type coinLog struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64
	Delta     int
	Reason    string
	CreatedAt time.Time
}

func (coinLog) TableName() string { return "coin_log" }

// ---- 套餐 ----

// ListPlans 套餐列表（onlyOn=true 只返回上架项，C 端用）。
func (r *repo) ListPlans(onlyOn bool) ([]VipPlan, error) {
	var list []VipPlan
	q := r.db.Model(&VipPlan{})
	if onlyOn {
		q = q.Where("status = ?", PlanOn)
	}
	err := q.Order("sort ASC, id ASC").Find(&list).Error
	return list, err
}

// GetPlan 按 code 取套餐；不存在返回 92101。
func (r *repo) GetPlan(code string) (*VipPlan, error) {
	var p VipPlan
	err := r.db.Where("code = ?", code).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrVipPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPlanTx 事务内按 code 取套餐（续期扣款时锁价用，避免与改价并发撕裂）。
func (r *repo) getPlanTx(tx *gorm.DB, code string) (*VipPlan, error) {
	var p VipPlan
	err := tx.Where("code = ?", code).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrVipPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CreatePlan 新建套餐（管理端；code 冲突由 uk_code 兜底）。
func (r *repo) CreatePlan(p *VipPlan) error { return r.db.Create(p).Error }

// UpdatePlan 改价/上下架/改名等（仅传入的字段生效，零值字段也生效 —— 管理端语义）。
func (r *repo) UpdatePlan(code string, fields map[string]any) error {
	res := r.db.Model(&VipPlan{}).Where("code = ?", code).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// Updates 对「值未变」也返回 0，故需再确认存在性，否则改价成同价会误报 92101。
		if _, err := r.GetPlan(code); err != nil {
			return err
		}
	}
	return nil
}

// ---- 会员状态 ----

// GetMembership 取会员行；不存在返回 (nil, nil)（非会员是正常状态，不是错误）。
func (r *repo) GetMembership(userID int64) (*Membership, error) {
	var m Membership
	err := r.db.Where("user_id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// VipFlags 批量取会员标识（评论/弹幕等读模型拼装用，**单条 SQL 避免 N+1**）。
//
// 判定口径与 GetMembership 完全一致（expire_at > NOW()），不接受任何调用方传入的时间，
// 避免「读模型用请求时间、判定用 DB 时间」造成同一用户两处结论不一致。
func (r *repo) VipFlags(userIDs []int64) (map[int64]BadgeView, error) {
	out := make(map[int64]BadgeView, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []Membership
	err := r.db.Where("user_id IN ? AND expire_at > NOW()", userIDs).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].UserID] = BadgeView{IsVip: true, NickColor: "pink", Badge: "vip"}
	}
	return out, nil
}

// IsVip 单人会员判定（服务端权威口径：expire_at > NOW()）。
func (r *repo) IsVip(userID int64) (bool, error) {
	var n int64
	err := r.db.Model(&Membership{}).
		Where("user_id = ? AND expire_at > NOW()", userID).Count(&n).Error
	return n > 0, err
}

// ---- 订单 → 权益生效（核心；在 pay 的入账事务内调用） ----

// ApplyResult 权益生效结果（applied=false 表示幂等：此前已生效）。
type ApplyResult struct {
	Applied      bool
	PlanCode     string
	Months       int
	ExpireBefore *time.Time
	ExpireAfter  time.Time
}

// CreateOrderRow 落 vip_order 台账（下单时调用，months 为**下单时快照**）。
//
// 为什么要把 months 存在 vip_order 而不在生效时再查 vip_plan：套餐可能被改下架甚至删除，
// 若生效时再查，用户已付款的订单会因「套餐没了」而无法生效。下单快照是唯一不会反悔的来源。
func (r *repo) CreateOrderRow(orderNo string, userID int64, planCode string, months int) error {
	return r.db.Create(&VipOrder{
		OrderNo: orderNo, UserID: userID, PlanCode: planCode,
		Months: months, ApplyStatus: ApplyPending,
	}).Error
}

// GetOrderRow 取订单台账。
func (r *repo) GetOrderRow(orderNo string) (*VipOrder, error) {
	var o VipOrder
	err := r.db.Where("order_no = ?", orderNo).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ApplyOrderTx 在**调用方的事务**内把某订单的会员权益生效（顺延 + 赠币）。
//
// 必须在支付入账的同一事务内执行，否则会出现「钱已收、会员没开」或反之。
//
// 幂等三层：
//  1. vip_order.uk_order_no INSERT 冲突 → 读回；apply_status=1 即此前已生效，直接返回 applied=false。
//  2. 生效判定用条件 UPDATE（WHERE apply_status=0）并以 RowsAffected 裁决并发。
//  3. 赠币另有 (user_id, period_key) 唯一闸（月度幂等，与订单幂等彼此独立）。
//
// 顺延语义（用户已敲定）：
//
//	已是会员 → expire_at = max(now(), expire_at) + 时长（顺延，不覆盖剩余时长）
//	已过期/从未开通 → expire_at = now() + 时长
//
// 用 DB 的 NOW() 而非 Go 的 time.Now()：判定与写入必须同一时钟，
// 否则应用与 DB 时区/时钟漂移会让「刚过期的会员」在两者间得出相反结论。
func (r *repo) ApplyOrderTx(tx *gorm.DB, userID int64, orderNo, planCode string, months int) (*ApplyResult, error) {
	if months <= 0 {
		return nil, errcode.ErrVipMonthsInvalid
	}
	res := &ApplyResult{PlanCode: planCode, Months: months}

	// ① 台账行：不存在则建（下单路径已建）；已存在则读回判幂等。
	var row VipOrder
	err := tx.Where("order_no = ?", orderNo).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = VipOrder{OrderNo: orderNo, UserID: userID, PlanCode: planCode, Months: months, ApplyStatus: ApplyPending}
		if e := tx.Create(&row).Error; e != nil {
			if !errors.Is(e, gorm.ErrDuplicatedKey) {
				return nil, e
			}
			if e2 := tx.Where("order_no = ?", orderNo).First(&row).Error; e2 != nil {
				return nil, e2
			}
		}
	} else if err != nil {
		return nil, err
	}
	if row.ApplyStatus == ApplyDone {
		return res, nil // 幂等：此前已生效，无任何副作用
	}
	// 台账 owner 与订单 owner 不一致属数据异常，宁可报错也不给错人开会员。
	if row.UserID != userID {
		return nil, errcode.ErrVipOrderNotVip
	}
	applyMonths := row.Months
	if applyMonths <= 0 {
		applyMonths = months
	}

	// ② 行锁会员行 → 计算顺延（或首次开通）。
	var m Membership
	lockErr := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&m).Error
	switch {
	case errors.Is(lockErr, gorm.ErrRecordNotFound):
		// 首次开通：expire_at = NOW() + 时长（DB 时钟，与应用判定同源）。
		if e := tx.Exec(`INSERT INTO membership (user_id, level, expire_at, auto_renew, started_at, total_months, last_order_no)
			VALUES (?, ?, DATE_ADD(NOW(), INTERVAL ? MONTH), 0, NOW(), ?, ?)`,
			userID, LevelVip, applyMonths, applyMonths, orderNo).Error; e != nil {
			// 并发首单：唯一键冲突说明对手已建，回读加锁重算（顺延语义天然可重入）。
			if !errors.Is(e, gorm.ErrDuplicatedKey) {
				return nil, e
			}
			if e2 := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&m).Error; e2 != nil {
				return nil, e2
			}
			break
		}
		if e := tx.Where("user_id = ?", userID).First(&m).Error; e != nil {
			return nil, e
		}
		after := m.ExpireAt
		res.ExpireAfter = after
		return r.finishApply(tx, &row, nil, after, userID, orderNo, res)
	case lockErr != nil:
		return nil, lockErr
	}

	before := m.ExpireAt
	// 顺延核心：GREATEST(NOW(), expire_at) + 时长 —— 一条 SQL 完成「不覆盖剩余时长」。
	if e := tx.Model(&Membership{}).Where("user_id = ?", userID).Updates(map[string]any{
		"expire_at":     gorm.Expr("DATE_ADD(GREATEST(NOW(), expire_at), INTERVAL ? MONTH)", applyMonths),
		"total_months":  gorm.Expr("total_months + ?", applyMonths),
		"last_order_no": orderNo,
	}).Error; e != nil {
		return nil, e
	}
	if e := tx.Where("user_id = ?", userID).First(&m).Error; e != nil {
		return nil, e
	}
	return r.finishApply(tx, &row, &before, m.ExpireAt, userID, orderNo, res)
}

// finishApply 收尾：条件 UPDATE 推进台账（RowsAffected 裁决并发）+ 赠币。
func (r *repo) finishApply(tx *gorm.DB, row *VipOrder, before *time.Time, after time.Time,
	userID int64, orderNo string, res *ApplyResult) (*ApplyResult, error) {
	upd := tx.Model(&VipOrder{}).
		Where("order_no = ? AND apply_status = ?", orderNo, ApplyPending).
		Updates(map[string]any{
			"apply_status":  ApplyDone,
			"expire_before": before,
			"expire_after":  after,
			"applied_at":    gorm.Expr("NOW()"),
		})
	if upd.Error != nil {
		return nil, upd.Error
	}
	// 并发对手已推进：本次的会员行更新已在上面的 UPDATE 生效过（同一事务内不可能，
	// 但跨事务重放会走到这里）——此时不重复赠币，返回幂等结论。
	res.Applied = upd.RowsAffected == 1
	res.ExpireBefore = before
	res.ExpireAfter = after
	if !res.Applied {
		return res, nil
	}
	// ③ 赠币：按月发放，幂等键 (user_id, 周期键)。
	if _, err := r.GrantCoinsTx(tx, userID, GrantOnApply); err != nil {
		return nil, err
	}
	return res, nil
}

// GrantCoinsTx 在事务内发放会员月度硬币（幂等）。
//
// **幂等核心**：先插 vip_coin_grant（uk_user_period 唯一）；插入失败即本月已发过，
// 直接返回 (false, nil) 而不是报错 —— 「重复触发不重复发」是正常路径（定时补发会反复调用）。
//
// 币记在 coin_log（硬币），**不写 melon_seed_***（金瓜子是直播礼物资产，spec 中两者不同）。
func (r *repo) GrantCoinsTx(tx *gorm.DB, userID int64, source int8) (bool, error) {
	period := periodKey(time.Now())
	g := VipCoinGrant{UserID: userID, PeriodKey: period, Amount: defaultCoinPerMonth, Source: source}
	err := tx.Create(&g).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return false, nil // 本月已发放：幂等，非错误
	}
	if err != nil {
		return false, err
	}
	if err := tx.Exec("UPDATE `user` SET coin = coin + ? WHERE id = ?", g.Amount, userID).Error; err != nil {
		return false, err
	}
	cl := coinLog{UserID: userID, Delta: g.Amount, Reason: CoinReasonVIP}
	if err := tx.Create(&cl).Error; err != nil {
		return false, err
	}
	if err := tx.Model(&VipCoinGrant{}).Where("id = ?", g.ID).
		Update("coin_log_id", cl.ID).Error; err != nil {
		return false, err
	}
	return true, nil
}

// defaultCoinPerMonth 兜底枚数（spec §2.1：5；Service 构造时以配置覆盖）。
const defaultCoinPerMonth = 5

// ---- 连续包月签约 ----

// UpsertSubscription 签约/换套餐（同一用户一份签约，uk_user 唯一）。
func (r *repo) UpsertSubscription(userID int64, planCode, channel string, nextChargeAt time.Time) error {
	var s VipSubscription
	err := r.db.Where("user_id = ?", userID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(&VipSubscription{
			UserID: userID, PlanCode: planCode, Channel: channel, Status: SubActive,
			SignAt: time.Now(), NextChargeAt: nextChargeAt,
		}).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&VipSubscription{}).Where("id = ?", s.ID).Updates(map[string]any{
		"plan_code": planCode, "channel": channel, "status": SubActive,
		"next_charge_at": nextChargeAt, "cancel_at": nil, "fail_count": 0,
	}).Error
}

// GetSubscription 取签约；不存在返回 (nil, nil)。
func (r *repo) GetSubscription(userID int64) (*VipSubscription, error) {
	var s VipSubscription
	err := r.db.Where("user_id = ?", userID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CancelSubscription 解约（幂等：已解约再解约不算错）。
func (r *repo) CancelSubscription(userID int64) error {
	return r.db.Model(&VipSubscription{}).
		Where("user_id = ? AND status = ?", userID, SubActive).
		Updates(map[string]any{"status": SubCancelled, "cancel_at": gorm.Expr("NOW()")}).Error
}

// DueSubscriptions 取到期待扣款的签约（续期扫描驱动）。
func (r *repo) DueSubscriptions(limit int) ([]VipSubscription, error) {
	var list []VipSubscription
	err := r.db.Where("status = ? AND next_charge_at <= NOW()", SubActive).
		Order("next_charge_at ASC").Limit(limit).Find(&list).Error
	return list, err
}

// ClaimSubscription 用 CAS 抢下本轮续期权（status=1 且 next_charge_at 仍为读取值）。
//
// **为什么用 CAS 而不是分布式锁**：多实例并发扫描时，只有一个实例能把
// next_charge_at 从旧值推进到新值（RowsAffected=1），其余实例拿到 0 直接跳过。
// 这把「谁来做这次扣款」的仲裁交给数据库，与 pay 模块的条件 UPDATE 口径一致。
// 先推进时间再扣款：若扣款过程崩溃，本轮不会再被重复扣（宁可少扣一次，不可重复扣）。
func (r *repo) ClaimSubscription(userID int64, from, to time.Time) (bool, error) {
	res := r.db.Model(&VipSubscription{}).
		Where("user_id = ? AND status = ? AND next_charge_at = ?", userID, SubActive, from).
		Update("next_charge_at", to)
	return res.RowsAffected == 1, res.Error
}

// MarkCharged 记录扣款成功（顺延已完成，last_charge_at 与 fail_count 归零）。
func (r *repo) MarkCharged(userID int64) error {
	return r.db.Model(&VipSubscription{}).Where("user_id = ?", userID).
		Updates(map[string]any{"last_charge_at": gorm.Expr("NOW()"), "fail_count": 0}).Error
}

// MarkChargeFailed 记录扣款失败并自增失败次数，返回自增后的次数。
func (r *repo) MarkChargeFailed(userID int64) (int, error) {
	if err := r.db.Model(&VipSubscription{}).Where("user_id = ?", userID).
		UpdateColumn("fail_count", gorm.Expr("fail_count + 1")).Error; err != nil {
		return 0, err
	}
	var s VipSubscription
	if err := r.db.Where("user_id = ?", userID).First(&s).Error; err != nil {
		return 0, err
	}
	return s.FailCount, nil
}

// SetNextCharge 重排下次扣款时间（扣款失败后的短重试窗口）。
func (r *repo) SetNextCharge(userID int64, at time.Time) error {
	return r.db.Model(&VipSubscription{}).Where("user_id = ?", userID).
		Update("next_charge_at", at).Error
}

// ActiveMembershipsWithoutGrant 取「生效中但本期未发放赠币」的会员 uid（月度补发扫描）。
//
// 用 LEFT JOIN 一次 SQL 完成「会员 ∩ 未发」的集合运算；分页上限由调用方给定，
// 避免一次把全站会员拉进内存（会员数增长后这是唯一会 OOM 的地方）。
func (r *repo) ActiveMembershipsWithoutGrant(period string, limit int) ([]int64, error) {
	var uids []int64
	err := r.db.Model(&Membership{}).
		Joins("LEFT JOIN vip_coin_grant g ON g.user_id = membership.user_id AND g.period_key = ?", period).
		Where("membership.expire_at > NOW() AND g.id IS NULL").
		Order("membership.user_id ASC").Limit(limit).
		Pluck("membership.user_id", &uids).Error
	return uids, err
}

// ---- 管理端：会员查询与补发 ----

// MemberFilter 会员查询条件（零值即不筛选）。
type MemberFilter struct {
	UserID  int64
	OnlyVip bool // 只看生效中（expire_at > NOW()）
	Page    int
	Size    int
}

// ListMemberships 会员分页查询（管理端）。
func (r *repo) ListMemberships(f MemberFilter) ([]Membership, int64, error) {
	q := r.db.Model(&Membership{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.OnlyVip {
		q = q.Where("expire_at > NOW()")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Membership
	err := q.Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// SeedPlansIfEmpty 首次启动时播种默认套餐（管理端可改价/上下架）。
//
// 只在表为空时插入：已上线的价格是运营决策，任何自动播种都不得覆盖运营改过的价。
func (r *repo) SeedPlansIfEmpty(plans []VipPlan) (int, error) {
	var n int64
	if err := r.db.Model(&VipPlan{}).Count(&n).Error; err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, nil
	}
	if len(plans) == 0 {
		return 0, nil
	}
	if err := r.db.Create(&plans).Error; err != nil {
		return 0, err
	}
	return len(plans), nil
}

// DefaultPlans 默认套餐（月/季/年 + 连续包月首月优惠）。
//
// 价格是**可运营的初值**而非硬编码业务规则：管理端可改（spec §2 管理端要求「套餐配置」）。
func DefaultPlans() []VipPlan {
	return []VipPlan{
		{Code: "vip_month", Name: "大会员 月度", Months: 1, Price: 1500, OriginalPrice: 2000,
			AutoRenew: 1, FirstMonthPrice: 800, Sort: 1, Status: PlanOn},
		{Code: "vip_quarter", Name: "大会员 季度", Months: 3, Price: 4500, OriginalPrice: 6000, Sort: 2, Status: PlanOn},
		{Code: "vip_year", Name: "大会员 年度", Months: 12, Price: 14800, OriginalPrice: 24000, Sort: 3, Status: PlanOn},
	}
}
