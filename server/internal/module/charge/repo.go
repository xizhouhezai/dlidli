package charge

import (
	"errors"
	"fmt"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockingClause SELECT ... FOR UPDATE 行锁（余额强一致核心，与 live/gift_repo.go 同写法）。
var lockingClause = clause.Locking{Strength: "UPDATE"}

// Repo 充电域数据访问。一切余额变更必须在事务内先锁余额行。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// DB 暴露底层句柄（月结 worker / 验收脚本只读聚合用）。
func (r *Repo) DB() *gorm.DB { return r.db }

// ---- 档位 ----

// TierList 上架档位（按 sort）。
func (r *Repo) TierList() ([]ChargeTier, error) {
	var rows []ChargeTier
	err := r.db.Where("status = ?", TierOn).Order("sort ASC").Find(&rows).Error
	return rows, err
}

// TierByCode 取档位（含下架，由调用方判定状态）。
func (r *Repo) TierByCode(code string) (*ChargeTier, error) {
	var t ChargeTier
	err := r.db.Where("code = ?", code).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrChargeTierNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ---- 充电落账（在支付入账事务内调用） ----

// CreateCharge 落充电记录（status=0 待支付；下单链路调用）。
//
// 时序（plan §2H.3）：本行在**把支付凭据返回给客户端之前**落库——客户端在此之前
// 既不知道订单号也无法完成支付，故「已支付但无充电记录」结构上不可达。
// uk_order_no 是第二道闸：同订单第二次插入撞唯一键回滚。
func (r *Repo) CreateCharge(c *ChargeLog) error {
	if c.ShareRate == 0 {
		c.ShareRate = int(ShareRateCreator)
	}
	return r.db.Create(c).Error
}

// MarkChargedTx 支付成功后把充电记录置为已支付（在支付入账事务内调用）。
//
// 条件 UPDATE + RowsAffected：只有恰好把 status 从 0 推成 1 的那一次才返回 applied=true，
// 重复回调第二次 RowsAffected=0 → applied=false，不会重复记分成收益。
func (r *Repo) MarkChargedTx(tx *gorm.DB, orderNo string) (applied bool, err error) {
	res := tx.Model(&ChargeLog{}).
		Where("order_no = ? AND status = ?", orderNo, ChargeUnpaid).
		Updates(map[string]any{"status": ChargePaid, "paid_at": gorm.Expr("NOW()")})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ---- 分成入账（在支付入账事务内调用） ----

// creditIncomeTx 记创作者分成收益（**同一事务内**调用）：
// 写 creator_income 明细（uk_source 幂等）+ 行锁余额账户加分。
//
// 资金口径：这是平台对创作者的**现金负债**增加，不流入消费者的 cash_account。
func (r *Repo) creditIncomeTx(tx *gorm.DB, c *ChargeLog) error {
	// 幂等闸：同一笔充电只入账一次（uk_source 也会兜底，此处提前返回更省一次事务重试）。
	var cnt int64
	if err := tx.Model(&CreatorIncome{}).
		Where("source_type = ? AND source_id = ?", IncomeSourceCharge, c.ID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	platform := c.Amount - c.Amount*int64(c.ShareRate)/1000
	income := &CreatorIncome{
		UserID: c.AnchorID, SourceType: IncomeSourceCharge, SourceID: c.ID,
		OrderNo: c.OrderNo, VideoID: c.VideoID,
		Amount: c.Amount * int64(c.ShareRate) / 1000, PlatformAmount: platform,
		ShareRate: c.ShareRate, SettleStatus: IncomeUnsettled, PaidAt: c.PaidAt,
	}
	if err := tx.Create(income).Error; err != nil {
		return err
	}
	_, err := r.creditBalanceTx(tx, c.AnchorID, income.Amount, 0, BalBizIncome,
		fmt.Sprintf("income:%d", c.ID), "充电分成入账")
	return err
}

// CreditIncomeByOrder 支付成功后按订单号记分成（幂等；重复调用无副作用）。
//
// 返回 (是否本次实际入账, 分成金额)。**由支付域 PaidHook 在入账事务内调用**。
// 先条件推进 charge_log.status 0→1，只有真正推进的那一次才记收益——
// 这样「重复回调」在两层（pay_order 状态机 / charge_log.status）都不会重复入账。
func (r *Repo) CreditIncomeByOrder(tx *gorm.DB, orderNo string) (bool, int64, error) {
	var c ChargeLog
	err := tx.Where("order_no = ?", orderNo).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, 0, nil // 非充电订单（会员/纯支付），无分成
	}
	if err != nil {
		return false, 0, err
	}
	if c.Status == ChargePaid {
		return false, 0, nil // 幂等：该笔充电已入账
	}
	applied, err := r.MarkChargedTx(tx, orderNo)
	if err != nil {
		return false, 0, err
	}
	if !applied {
		return false, 0, nil // 并发对手已推进
	}
	var cnt int64
	if err := tx.Model(&CreatorIncome{}).
		Where("source_type = ? AND source_id = ?", IncomeSourceCharge, c.ID).Count(&cnt).Error; err != nil {
		return false, 0, err
	}
	if cnt > 0 {
		return false, 0, nil // uk_source 已存在（理论不可达，防御）
	}
	// 归属月依据：以**支付成功时刻**为准（本事务刚写入的 NOW()），而非回调到达时刻。
	var paid ChargeLog
	if e := tx.Where("id = ?", c.ID).First(&paid).Error; e != nil {
		return false, 0, e
	}
	if err := r.creditIncomeTx(tx, &paid); err != nil {
		return false, 0, err
	}
	return true, paid.Amount * int64(paid.ShareRate) / 1000, nil
}

// creditBalanceTx 余额入账（事务内）：行锁 → 变更 → 写流水（biz_no 幂等）。
func (r *Repo) creditBalanceTx(tx *gorm.DB, userID, delta, frozenDelta int64, bizType int8, bizNo, remark string) (*CreatorBalance, error) {
	acc, err := r.lockBalanceTx(tx, userID)
	if err != nil {
		return nil, err
	}
	var cnt int64
	if err := tx.Model(&CreatorBalanceLog{}).Where("biz_no = ?", bizNo).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return acc, nil // 幂等：已入账
	}
	balance := acc.Balance + delta
	frozen := acc.Frozen + frozenDelta
	if balance < 0 || frozen < 0 {
		return nil, errcode.ErrCreatorBalanceNotEnough
	}
	if err := tx.Model(&CreatorBalance{}).Where("id = ?", acc.ID).
		Updates(map[string]any{"balance": balance, "frozen": frozen}).Error; err != nil {
		return nil, err
	}
	if err := tx.Create(&CreatorBalanceLog{
		UserID: userID, Delta: delta, FrozenDelta: frozenDelta,
		BalanceAfter: balance, FrozenAfter: frozen,
		BizType: bizType, BizNo: bizNo, Remark: remark,
	}).Error; err != nil {
		return nil, err
	}
	acc.Balance, acc.Frozen = balance, frozen
	return acc, nil
}

// lockBalanceTx FOR UPDATE 行锁创作者余额；无行则插入 0 余额行再返回
// （与 live.lockAccount / pay.lockAccount 同构，含并发首单的唯一键冲突重读）。
func (r *Repo) lockBalanceTx(tx *gorm.DB, userID int64) (*CreatorBalance, error) {
	var acc CreatorBalance
	err := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		acc = CreatorBalance{UserID: userID}
		if err := tx.Create(&acc).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				if e := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&acc).Error; e != nil {
					return nil, e
				}
				return &acc, nil
			}
			return nil, err
		}
		return &acc, nil
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// ---- 余额查询 ----

// Balance 创作者账户视图（懒创建口径：无账户视为 0）。
func (r *Repo) Balance(userID int64) (*CreatorBalanceView, error) {
	var acc CreatorBalance
	err := r.db.Where("user_id = ?", userID).First(&acc).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	v := &CreatorBalanceView{UserID: itoa(userID), Balance: acc.Balance, Frozen: acc.Frozen}
	row := struct {
		Unsettled int64
		Total     int64
	}{}
	if err := r.db.Model(&CreatorIncome{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(CASE WHEN settle_status = 0 THEN amount ELSE 0 END),0) AS unsettled, COALESCE(SUM(amount),0) AS total").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	v.Unsettled, v.TotalIncome = row.Unsettled, row.Total
	var paid int64
	if err := r.db.Model(&WithdrawOrder{}).
		Where("user_id = ? AND status = ?", userID, WithdrawPaid).
		Select("COALESCE(SUM(amount),0)").Scan(&paid).Error; err != nil {
		return nil, err
	}
	v.Withdrawn = paid
	return v, nil
}

// ---- 提现 ----

// CountRecentWithdraws 近 24h 提现申请笔数（频次限制判据）。
func (r *Repo) CountRecentWithdraws(userID int64) (int64, error) {
	var n int64
	err := r.db.Model(&WithdrawOrder{}).
		Where("user_id = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 24 HOUR)", userID).
		Count(&n).Error
	return n, err
}

// ApplyWithdrawTx 提现申请（**防超提核心**）。
//
// 单事务内：行锁创作者余额 → 校验频次 → 校验可用余额 → 可用余额转冻结 → 写余额流水 → 落提现单。
// 行锁是「并发提现不得超提」的唯一保证：N 个并发请求在该行上串行化，
// 每个请求读到的余额都是前一个请求扣减后的结果，故 Σ成功金额 不可能超过可用余额。
// 与 LIV-04 的 SendGiftTx 同写法（monetization spec 明确要求同一口径）。
//
// **频次限制为什么也必须在事务内**：若在事务外先查「24h 内已提几次」再进事务，
// 并发请求会同时读到旧计数而全部通过——限制形同虚设（本卡实测：并发下余额只够少数几笔，
// 却仍可能落出超过日限的提现单）。放进同一事务、复用同一把行锁后，
// 计数与冻结在同一次串行化中完成，日限才真正成立。
func (r *Repo) ApplyWithdrawTx(w *WithdrawOrder, maxPerDay int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		acc, err := r.lockBalanceTx(tx, w.UserID)
		if err != nil {
			return err
		}
		if maxPerDay > 0 {
			var n int64
			if err := tx.Model(&WithdrawOrder{}).
				Where("user_id = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 24 HOUR)", w.UserID).
				Count(&n).Error; err != nil {
				return err
			}
			if n >= int64(maxPerDay) {
				return errcode.ErrWithdrawTooFrequent
			}
		}
		if acc.Balance < w.Amount {
			return errcode.ErrCreatorBalanceNotEnough
		}
		// 可用 → 冻结（同一事务，余额与冻结金额强一致）。
		if _, err := r.creditBalanceTx(tx, w.UserID, -w.Amount, w.Amount, BalBizWithdrawFreeze,
			"wd-freeze:"+w.WithdrawNo, "提现申请冻结"); err != nil {
			return err
		}
		return tx.Create(w).Error
	})
}

// AuditWithdrawTx 审核（通过/驳回）。
//
// 用**条件 UPDATE + RowsAffected** 保证并发安全：只有恰好把状态从「审核中」推走的那个请求
// 才执行后续副作用（驳回时解冻），重复审核第二个请求 RowsAffected=0 直接报状态错误，
// 不会二次解冻（否则余额会被凭空增加，这是最危险的资金缺陷）。
func (r *Repo) AuditWithdrawTx(withdrawNo string, approve bool, adminID int64, note string) (*WithdrawOrder, error) {
	var out *WithdrawOrder
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var w WithdrawOrder
		if e := tx.Where("withdraw_no = ?", withdrawNo).Clauses(lockingClause).First(&w).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return errcode.ErrWithdrawNotFound
			}
			return e
		}
		if w.Status != WithdrawAuditing {
			return errcode.ErrWithdrawStatus
		}
		target := WithdrawApproved
		if !approve {
			target = WithdrawRejected
		}
		res := tx.Model(&WithdrawOrder{}).
			Where("withdraw_no = ? AND status = ?", withdrawNo, WithdrawAuditing).
			Updates(map[string]any{
				"status": target, "audit_by": adminID,
				"audit_at": gorm.Expr("NOW()"), "audit_note": note,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errcode.ErrWithdrawStatus // 并发对手已处理
		}
		if !approve {
			// 驳回：冻结金额解冻返还可用余额。
			if _, e := r.creditBalanceTx(tx, w.UserID, w.Amount, -w.Amount, BalBizWithdrawReject,
				"wd-reject:"+w.WithdrawNo, "提现驳回解冻"); e != nil {
				return e
			}
		}
		out = &w
		out.Status = target
		out.AuditNote = note
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.WithdrawByNo(withdrawNo)
}

// PayoutWithdrawTx 打款（mock 通道；真实出款为换入点，plan §2H.7）。
//
// 双闸防重复出款：
//
//	① 条件 UPDATE status 2→4 + RowsAffected（第二个请求为 0 → 92303）；
//	② withdraw_payout_log.uk_withdraw_no 唯一键（即使闸①被绕过也撞唯一键回滚）。
//
// 打款成功 = 冻结金额真正扣减（钱已经出去了）。失败（fail=true）则解冻返还，可重新申请。
func (r *Repo) PayoutWithdrawTx(withdrawNo string, fail bool, tradeNo, remark string) (*WithdrawOrder, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var w WithdrawOrder
		if e := tx.Where("withdraw_no = ?", withdrawNo).Clauses(lockingClause).First(&w).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return errcode.ErrWithdrawNotFound
			}
			return e
		}
		if w.Status != WithdrawApproved {
			return errcode.ErrWithdrawStatus
		}
		fields := map[string]any{"status": WithdrawPaid, "paid_at": gorm.Expr("NOW()")}
		if fail {
			fields = map[string]any{"status": WithdrawPayFail, "fail_reason": remark}
		} else {
			fields["channel_trade_no"] = tradeNo
		}
		res := tx.Model(&WithdrawOrder{}).
			Where("withdraw_no = ? AND status = ?", withdrawNo, WithdrawApproved).
			Updates(fields)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errcode.ErrWithdrawStatus
		}
		// 打款流水（闸②）；重复打款在此撞唯一键 → 整笔事务回滚。
		result := int8(0)
		if fail {
			result = 1
		}
		if e := tx.Create(&WithdrawPayoutLog{
			WithdrawNo: w.WithdrawNo, UserID: w.UserID, Amount: w.Amount,
			Channel: w.Channel, ChannelTradeNo: tradeNo, Result: result, Remark: remark,
		}).Error; e != nil {
			return e
		}
		// 冻结金额出清：成功=钱已出，失败=解冻返还。
		if fail {
			if _, e := r.creditBalanceTx(tx, w.UserID, w.Amount, -w.Amount, BalBizPayoutFail,
				"wd-payout-fail:"+w.WithdrawNo, "打款失败解冻"); e != nil {
				return e
			}
		} else {
			if _, e := r.creditBalanceTx(tx, w.UserID, 0, -w.Amount, BalBizPayout,
				"wd-payout:"+w.WithdrawNo, "提现打款扣减"); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.WithdrawByNo(withdrawNo)
}

// WithdrawByNo 取提现单。
func (r *Repo) WithdrawByNo(withdrawNo string) (*WithdrawOrder, error) {
	var w WithdrawOrder
	err := r.db.Where("withdraw_no = ?", withdrawNo).First(&w).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrWithdrawNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// PayoutLogCount 打款流水条数（验收核对「重复打款被拦」用）。
func (r *Repo) PayoutLogCount(withdrawNo string) (int64, error) {
	var n int64
	err := r.db.Model(&WithdrawPayoutLog{}).Where("withdraw_no = ?", withdrawNo).Count(&n).Error
	return n, err
}

// WithdrawFilter 提现单查询条件。
type WithdrawFilter struct {
	UserID int64
	Status *int8
	Page   int
	Size   int
}

// ListWithdraws 提现单分页（管理端审核台 / C 端我的提现共用）。
func (r *Repo) ListWithdraws(f WithdrawFilter) ([]WithdrawOrder, int64, error) {
	q := r.db.Model(&WithdrawOrder{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]WithdrawOrder, 0)
	err := q.Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// ---- 充电记录与鸣谢 ----

// ChargeByOrder 按订单号取充电记录。
func (r *Repo) ChargeByOrder(orderNo string) (*ChargeLog, error) {
	var c ChargeLog
	err := r.db.Where("order_no = ?", orderNo).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrChargeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ChargeFilter 充电记录查询条件（管理端 / 鸣谢名单共用）。
type ChargeFilter struct {
	UserID   int64
	AnchorID int64
	VideoID  int64
	// OnlyPaid 只返回已支付的充电（鸣谢名单口径：未支付不算打赏）。
	OnlyPaid bool
	Page     int
	Size     int
}

// ListCharges 充电记录分页（按时间倒序）。
func (r *Repo) ListCharges(f ChargeFilter) ([]ChargeLog, int64, error) {
	q := r.db.Model(&ChargeLog{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.AnchorID > 0 {
		q = q.Where("anchor_id = ?", f.AnchorID)
	}
	if f.VideoID > 0 {
		q = q.Where("video_id = ?", f.VideoID)
	}
	if f.OnlyPaid {
		q = q.Where("status = ?", ChargePaid)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]ChargeLog, 0)
	err := q.Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// IncomeSum 创作者分成累计（验收核对 70/30 精确值用）。
func (r *Repo) IncomeSum(userID int64) (int64, int64, error) {
	row := struct {
		Creator  int64
		Platform int64
	}{}
	err := r.db.Model(&CreatorIncome{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount),0) AS creator, COALESCE(SUM(platform_amount),0) AS platform").
		Scan(&row).Error
	return row.Creator, row.Platform, err
}

// IncomeByOrder 某订单的分成明细（逐笔核对）。
func (r *Repo) IncomeByOrder(orderNo string) (*CreatorIncome, error) {
	var inc CreatorIncome
	err := r.db.Where("order_no = ?", orderNo).First(&inc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrChargeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

// BalanceLogs 余额流水（对账与取证）。
func (r *Repo) BalanceLogs(userID int64) ([]CreatorBalanceLog, error) {
	var rows []CreatorBalanceLog
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// CreditBalanceForTest 测试专用：按真实入账路径给余额加钱（走流水与幂等闸）。
//
// 之所以提供这个导出入口而不是让测试直接 UPDATE 余额：直接改余额会绕过
// creator_balance_log 与不变量检查，测出来的「并发安全」可能是假的。
func (r *Repo) CreditBalanceForTest(userID, delta int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		_, err := r.creditBalanceTx(tx, userID, delta, 0, BalBizIncome,
			fmt.Sprintf("test-seed:%d:%d", userID, time.Now().UnixNano()), "测试造余额")
		return err
	})
}

// ---- 月度结算 ----

// SettleResult 单轮月结结果。
type SettleResult struct {
	Month       string `json:"month"`
	Users       int    `json:"users"`
	ChargeCount int64  `json:"charge_count"`
	Amount      int64  `json:"amount"`
	Platform    int64  `json:"platform_amount"`
}

// SettleMonth 结算某自然月（幂等，可重放）：聚合充电流水 → 生成结算单 + 回写明细状态。
//
// 归属口径（plan §2H.5）：**按 charge_log.paid_at（支付成功时间）归属自然月**，
// 而不是结算动作发生的时刻——否则「上月的钱记到本月」会让结算单与账单期不符。
//
// 幂等：creator_charge_settlement.uk_user_month 唯一 + 明细 settle_status 条件 UPDATE，
// 重复跑同一月不会重复计额（已结算明细不再被聚合）。
func (r *Repo) SettleMonth(month string) (*SettleResult, error) {
	start, end, err := monthRange(month)
	if err != nil {
		return nil, err
	}
	res := &SettleResult{Month: month}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		// 该月待结算的分成明细（按创作者聚合）。
		type aggRow struct {
			UserID      int64
			Cnt         int64
			Amount      int64
			PlatformAmt int64
		}
		// 聚合**该月全部分成明细**（不按 settle_status 过滤）：
		// 只聚合「待结算」的行虽然不会重复入账，但重跑会返回 0/0 并与已生成的结算单不一致，
		// 让人无法判断「重跑是否算出同一个数」。按整月重算 + upsert 覆盖，才使
		// 「结算单金额 = Σ该月各笔分成」这条验收判据在任意次重跑后都成立。
		var rows []aggRow
		if e := tx.Model(&CreatorIncome{}).
			Where("paid_at >= ? AND paid_at < ?", start, end).
			Select("user_id, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount, COALESCE(SUM(platform_amount),0) AS platform_amt").
			Group("user_id").Scan(&rows).Error; e != nil {
			return e
		}
		for _, a := range rows {
			settle := &CreatorChargeSettlement{
				SettleMonth: month, UserID: a.UserID, ChargeCount: int(a.Cnt),
				Amount: a.Amount, PlatformAmount: a.PlatformAmt,
			}
			// upsert 结算单（同月同创作者唯一，重跑覆盖为最新聚合值）。
			if e := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}, {Name: "settle_month"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"charge_count", "amount", "platform_amount",
				}),
			}).Create(settle).Error; e != nil {
				return e
			}
			var saved CreatorChargeSettlement
			if e := tx.Where("user_id = ? AND settle_month = ?", a.UserID, month).First(&saved).Error; e != nil {
				return e
			}
			// 明细打标（只动本轮的待结算行，条件 UPDATE 保证可重放）。
			if e := tx.Model(&CreatorIncome{}).
				Where("settle_status = ? AND paid_at >= ? AND paid_at < ? AND user_id = ?",
					IncomeUnsettled, start, end, a.UserID).
				Updates(map[string]any{
					"settle_status": IncomeSettled, "settle_month": month, "settlement_id": saved.ID,
				}).Error; e != nil {
				return e
			}
			res.Users++
			res.ChargeCount += a.Cnt
			res.Amount += a.Amount
			res.Platform += a.PlatformAmt
		}
		// 结算单内按稿聚合 → 复用 creator_settlement（source='charge'，plan §23 决策：
		// 与创作激励共用结算通道，不另起一套结算体系）。
		if e := r.settleByVideoTx(tx, start, end); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// settleByVideoTx 把该月充电分成按（创作者 × 稿件）写入 creator_settlement。
//
// 同样按 paid_at 区间重算（而非按已打标的 settle_month），保证重跑得到同一组行与同一金额。
func (r *Repo) settleByVideoTx(tx *gorm.DB, start, end time.Time) error {
	type vRow struct {
		UserID      int64
		VideoID     int64
		Cnt         int64
		Amount      int64
		PlatformAmt int64
	}
	var rows []vRow
	if err := tx.Model(&CreatorIncome{}).
		Where("paid_at >= ? AND paid_at < ?", start, end).
		Select("user_id, video_id, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount, COALESCE(SUM(platform_amount),0) AS platform_amt").
		Group("user_id, video_id").Scan(&rows).Error; err != nil {
		return err
	}
	settleDate := start.Format("2006-01-02") // 该自然月 1 号
	for _, v := range rows {
		// 冲突键与迁移 0041 的 uk_source_date_user_video 一致：必须显式带上 user_id，
		// 否则 video_id=0 的空间页充电会让不同创作者互相覆盖（见迁移内注释）。
		if err := tx.Exec(`INSERT INTO creator_settlement
			(settle_date, user_id, video_id, source, valid_views, amount, charge_count, settlement_id)
			VALUES (?, ?, ?, 'charge', 0, ?, ?, 0)
			ON DUPLICATE KEY UPDATE amount = VALUES(amount), charge_count = VALUES(charge_count)`,
			settleDate, v.UserID, v.VideoID, v.Amount, v.Cnt).Error; err != nil {
			return err
		}
	}
	return nil
}

// SettlementList 结算单分页（管理端 / 创作者端）。
func (r *Repo) SettlementList(userID int64, month string, page, size int) ([]CreatorChargeSettlement, int64, error) {
	q := r.db.Model(&CreatorChargeSettlement{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if month != "" {
		q = q.Where("settle_month = ?", month)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]CreatorChargeSettlement, 0)
	err := q.Order("settle_month DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// SettledChargeSum 结算单内充电笔数对应的分成合计（验收：金额 = Σ各笔分成）。
func (r *Repo) SettledChargeSum(userID int64, month string) (int64, error) {
	var n int64
	err := r.db.Model(&CreatorIncome{}).
		Where("user_id = ? AND settle_month = ?", userID, month).
		Select("COALESCE(SUM(amount),0)").Scan(&n).Error
	return n, err
}

// monthRange 解析 YYYY-MM 为 [start, end) 区间。
func monthRange(month string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errcode.ErrSettleMonthInvalid
	}
	return t, t.AddDate(0, 1, 0), nil
}

// itoa 数字转字符串（避免各文件重复引入 strconv）。
func itoa(v int64) string { return fmt.Sprintf("%d", v) }
