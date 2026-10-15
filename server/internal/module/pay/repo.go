package pay

import (
	"errors"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	mysqldrv "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockingClause SELECT ... FOR UPDATE 行锁（现金余额强一致核心）。
var lockingClause = clause.Locking{Strength: "UPDATE"}

// txAttempts 事务重试次数（死锁/锁等待超时）。
const txAttempts = 5

// retryTx 事务重试包装：仅对可重试的并发错误（死锁 1213 / 锁等待超时 1205）重试。
//
// **为什么必须有它**：本模块有两个会并发改写 pay_order 的事务 —— 回调入账
// （`WHERE order_no=? AND status=?`，走 uk_order_no → PRIMARY，再维护
// `idx_status_expire`）与惰性关单（`WHERE status=? AND expire_at<NOW()`，走
// `idx_status_expire` → PRIMARY）。两者加锁顺序相反，InnoDB 判定死锁会**主动回滚其中
// 一个事务**（实测复现：`SHOW ENGINE INNODB STATUS` 显示一方持 PRIMARY 等
// idx_status_expire、另一方持 idx_status_expire 等 PRIMARY）。
//
// 死锁是 InnoDB 的正常仲裁结果而非缺陷，官方处置就是重试；关键在于**回滚是整体回滚**，
// 事务内不会留下部分副作用，故重试安全且幂等（MarkPaidTx 靠条件 UPDATE 保证
// 第二次尝试要么迁移成功、要么判定为「对手已生效」而幂等返回）。
func retryTx(fn func() error) error {
	var err error
	for i := 0; i < txAttempts; i++ {
		err = fn()
		if err == nil || !isRetryableTxErr(err) {
			return err
		}
		// 退避极短（毫秒级）：死锁后双方必然有一方已提交，重试即可拿到新状态；
		// 退避过长会放大调用方的响应时间（支付回调需在 5s 内处理完成，spec §4）。
		time.Sleep(time.Duration(i+1) * 5 * time.Millisecond)
	}
	return err
}

// isRetryableTxErr 判定是否为可重试的并发错误（MySQL 1213 死锁 / 1205 锁等待超时）。
func isRetryableTxErr(err error) bool {
	var me *mysqldrv.MySQLError
	if !errors.As(err, &me) {
		return false
	}
	return me.Number == 1213 || me.Number == 1205
}

// repo 支付域数据访问。**本文件的 SQL 是「钱」的唯一入口**：
// 任何绕过此处直接 UPDATE pay_order / cash_account 的写法都会绕开幂等与一致性保证。
type repo struct {
	db *gorm.DB
}

func newRepo(db *gorm.DB) *repo { return &repo{db: db} }

// NewRepo 支付域 repo（router 装配入口）。
func NewRepo(db *gorm.DB) *repo { return newRepo(db) }

// ---- 订单 ----

// CreateOrder 落订单（order_no 唯一由 DB 兜底）。
func (r *repo) CreateOrder(o *PayOrder) error { return r.db.Create(o).Error }

// CreateOrderWithSku 落订单 + 商品台账（**同一事务**，M4-VIP-01）。
//
// 若拆成两次写入：订单已建而台账缺失时，回调只知道订单号，无从判断该开哪档权益，
// 而用户已经付了钱——这是不可自愈的账实不符。故两者必须同事务。
func (r *repo) CreateOrderWithSku(o *PayOrder, sku string, h SkuHook) error {
	if sku == "" || h == nil {
		return r.db.Create(o).Error
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(o).Error; err != nil {
			return err
		}
		return h.CreateOrderTx(tx, o.OrderNo, o.UserID, sku)
	})
}

// GetOrder 按业务单号取订单；不存在返回 92001。
func (r *repo) GetOrder(orderNo string) (*PayOrder, error) {
	var o PayOrder
	err := r.db.Where("order_no = ?", orderNo).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrPayOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// GetUserOrder 取「某用户的」订单（越权访问返回 92001，不泄露他人订单是否存在）。
func (r *repo) GetUserOrder(orderNo string, userID int64) (*PayOrder, error) {
	var o PayOrder
	err := r.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrPayOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ListUserOrders 我的订单（按 id 倒序分页）。
func (r *repo) ListUserOrders(userID int64, page, size int) ([]PayOrder, int64, error) {
	var total int64
	q := r.db.Model(&PayOrder{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []PayOrder
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// OrderFilter 管理端订单查询条件（零值即不筛选）。
type OrderFilter struct {
	OrderNo string
	UserID  int64
	Channel string
	Status  *int8
	Page    int
	Size    int
}

// ListOrders 管理端订单查询。
func (r *repo) ListOrders(f OrderFilter) ([]PayOrder, int64, error) {
	q := r.db.Model(&PayOrder{})
	if f.OrderNo != "" {
		q = q.Where("order_no = ?", f.OrderNo)
	}
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.Channel != "" {
		q = q.Where("channel = ?", f.Channel)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []PayOrder
	err := q.Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// CloseExpired 惰性关单（plan §2E.3）：把已过期的待支付订单置为已关闭。
//
// 条件 UPDATE + RowsAffected 判定，天然并发安全（多实例重复执行只会有一方真正改动行）；
// 返回实际关闭笔数，供调用方记录与验收核对。不引入独立 Worker 的原因见 plan §2E.3。
//
// **LIMIT 批次**：无条件 UPDATE 会让本语句与「回调入账」形成死锁（加锁顺序相反，见 retryTx
// 注释）。分批后单次事务持有更少行锁、窗口更短，显著降低死锁概率；残余死锁由 retryTx 兜底
// （实测：20 并发回调下未加批次时必现 1 次死锁）。
func (r *repo) CloseExpired() (int64, error) {
	var closed int64
	err := retryTx(func() error {
		n, e := r.closeExpiredBatch()
		// 只采信**成功那一次**的计数：失败尝试（死锁回滚）的 RowsAffected 不可信，
		// 若跨次累加会虚报关单笔数，让验收核对与运维观测失真。
		if e == nil {
			closed = n
		}
		return e
	})
	return closed, err
}

// closeExpiredBatch 单批关单（最多 closeBatchSize 行）。
func (r *repo) closeExpiredBatch() (int64, error) {
	// MySQL 的 UPDATE ... ORDER BY ... LIMIT 需要显式限定（子查询取一批 id）。
	res := r.db.Exec(`UPDATE pay_order SET status = ?, closed_at = NOW()
		WHERE status = ? AND expire_at < NOW()
		  AND id IN (SELECT id FROM (
		      SELECT id FROM pay_order WHERE status = ? AND expire_at < NOW()
		      ORDER BY expire_at LIMIT ?
		  ) AS batch)`,
		StatusClosed, StatusPending, StatusPending, closeBatchSize)
	return res.RowsAffected, res.Error
}

// closeBatchSize 单批关单行数上限。
const closeBatchSize = 200

// ---- 资金与状态迁移（核心） ----

// MarkPaidTx 支付成功入账（spec §6.2/§6.4 一致性核心）。
//
// 单一事务内：① 条件 UPDATE 推进 0→1（RowsAffected 是唯一权威判据）；
// ② 行锁现金账户、加余额；③ 写现金流水（uk_biz_no 幂等）；④ 权益发放钩子（M4-VIP-01）。
//
// 返回 applied 表示**本次是否真正发生了迁移与入账**：
//   - applied=true：订单迁移 + 入账 + 流水三者同时生效。
//   - applied=false, err=nil：订单此前已支付（重放/并发对手已生效），本次未做任何副作用。
//   - err≠nil：被拒（订单不存在 92001 / 已关闭 92007 / 已退款 92002 / 权益发放失败）。
//
// 之所以必须把「金额核对」放在本函数之外（Service 层）：金额不符属于**回调报文的可信度**问题，
// 应当在验签后、入账前判定并留痕，而不是让资金层接受一个可疑金额。
func (r *repo) MarkPaidTx(orderNo, channelTradeNo string, hook PaidHook) (applied bool, err error) {
	err = retryTx(func() error {
		applied = false
		return r.markPaidOnce(orderNo, channelTradeNo, hook, &applied)
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

// markPaidOnce 单次尝试（由 MarkPaidTx 在死锁/锁超时时重试）。
func (r *repo) markPaidOnce(orderNo, channelTradeNo string, hook PaidHook, applied *bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// ① 条件 UPDATE：把「判定 + 变更」压成一条语句交给行锁串行化。
		res := tx.Model(&PayOrder{}).
			Where("order_no = ? AND status = ?", orderNo, StatusPending).
			Updates(map[string]any{
				"status":           StatusPaid,
				"channel_trade_no": channelTradeNo,
				"paid_at":          gorm.Expr("NOW()"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 未迁移：读当前状态给出可区分的结论（此处读取不参与竞争，仅作归因）。
			var cur PayOrder
			if e := tx.Where("order_no = ?", orderNo).First(&cur).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return errcode.ErrPayOrderNotFound
				}
				return e
			}
			switch cur.Status {
			case StatusPaid:
				return nil // 幂等：对手已生效，本次无副作用
			case StatusClosed:
				return errcode.ErrPayOrderExpired
			case StatusRefunded:
				return errcode.ErrPayOrderStatus
			default:
				return errcode.ErrPayOrderStatus
			}
		}
		// ② ③ 事务内入账：行锁账户 → 加余额 → 写流水。
		var o PayOrder
		if e := tx.Where("order_no = ?", orderNo).First(&o).Error; e != nil {
			return e
		}
		_, e := creditTx(tx, o.UserID, o.Amount, CashBizPay, "pay:"+orderNo, orderNo,
			"支付入账："+o.Subject)
		if e != nil {
			return e
		}
		// ④ 权益发放（M4-VIP-01）：**同一事务**。失败则整体回滚——绝不留下
		// 「已收钱但没开会员」；重放时本函数已在 RowsAffected==0 分支提前返回，
		// 故钩子只在真正迁移的那一次执行，天然不重复发放。
		if hook != nil {
			if e := hook(tx, orderNo, o.UserID); e != nil {
				return e
			}
		}
		*applied = true
		return nil
	})
}

// RefundTx 退款（spec §6.2）：条件 UPDATE 1→3，事务内扣回现金余额并写退款流水。
//
// refund_amount 累计写入；本次按「全额退款」语义处理（Amount = 订单金额），
// 部分退款的多笔累计归后续卡（本卡退款入口为全额，避免「部分退款 + 累计校验」半成品）。
func (r *repo) RefundTx(orderNo string) (applied bool, err error) {
	err = retryTx(func() error {
		applied = false
		return r.refundOnce(orderNo, &applied)
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

// refundOnce 单次退款尝试（由 RefundTx 在死锁/锁超时时重试）。
func (r *repo) refundOnce(orderNo string, applied *bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var o PayOrder
		if e := tx.Where("order_no = ?", orderNo).Clauses(lockingClause).First(&o).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return errcode.ErrPayOrderNotFound
			}
			return e
		}
		if o.Status != StatusPaid {
			return errcode.ErrPayOrderStatus
		}
		res := tx.Model(&PayOrder{}).
			Where("order_no = ? AND status = ?", orderNo, StatusPaid).
			Updates(map[string]any{
				"status":        StatusRefunded,
				"refund_amount": o.Amount,
				"refunded_at":   gorm.Expr("NOW()"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发对手已完成退款：幂等返回，不重复扣款。
			return nil
		}
		_, e := creditTx(tx, o.UserID, -o.Amount, CashBizRefund, "refund:"+orderNo, orderNo,
			"退款出账")
		if e != nil {
			return e
		}
		*applied = true
		return nil
	})
}

// MarkPaidManualTx 管理端手动补单（人工确认收款后入账，spec §6.7）。
//
// 与回调入账共用同一套保证（条件 UPDATE + RowsAffected + 事务内入账 + 幂等键）；
// 差别只在审计（remark 注明操作人）与「允许对已关闭订单补单」这一人工特权。
func (r *repo) MarkPaidManualTx(orderNo string, adminID int64, tradeNo string, hook PaidHook) (applied bool, err error) {
	err = retryTx(func() error {
		applied = false
		return r.markPaidManualOnce(orderNo, adminID, tradeNo, hook, &applied)
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

// markPaidManualOnce 单次补单尝试（由 MarkPaidManualTx 在死锁/锁超时时重试）。
func (r *repo) markPaidManualOnce(orderNo string, adminID int64, tradeNo string, hook PaidHook, applied *bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var o PayOrder
		if e := tx.Where("order_no = ?", orderNo).Clauses(lockingClause).First(&o).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return errcode.ErrPayOrderNotFound
			}
			return e
		}
		if o.Status == StatusPaid || o.Status == StatusRefunded {
			return errcode.ErrPayOrderStatus // 已支付/已退款不得补单
		}
		if tradeNo == "" {
			tradeNo = "manual-" + orderNo
		}
		res := tx.Model(&PayOrder{}).
			Where("order_no = ? AND status IN ?", orderNo, []int8{StatusPending, StatusClosed}).
			Updates(map[string]any{
				"status":           StatusPaid,
				"channel_trade_no": tradeNo,
				"paid_at":          gorm.Expr("NOW()"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errcode.ErrPayOrderStatus
		}
		_, e := creditTx(tx, o.UserID, o.Amount, CashBizPay, "pay:"+orderNo, orderNo,
			"手动补单（操作人 admin:"+itoa(adminID)+"）")
		if e != nil {
			return e
		}
		// 补单同样发放权益（M4-VIP-01）：管理员「人工确认收款」之后，用户买到的会员必须到手，
		// 否则补单只入账不发权益，用户看到的仍是「钱入了但会员没开」。
		if hook != nil {
			if e := hook(tx, orderNo, o.UserID); e != nil {
				return e
			}
		}
		*applied = true
		return nil
	})
}

// creditTx 资金变动（事务内）：行锁账户 → 变更余额 → 写流水。
//
// uk_biz_no 是资金幂等的最后一道闸：即使上层幂等判定被绕过（例如手工补单与回调
// 在不同事务里竞争），同一 biz_no 的第二次插入也会撞唯一键让整个事务回滚。
func creditTx(tx *gorm.DB, userID, delta int64, bizType int8, bizNo, orderNo, remark string) (int64, error) {
	acc, err := lockAccount(tx, userID)
	if err != nil {
		return 0, err
	}
	// 幂等：同 biz_no 已入账则不再变动（返回当前余额）。
	var cnt int64
	if err := tx.Model(&CashLog{}).Where("biz_no = ?", bizNo).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt > 0 {
		return acc.Balance, nil
	}
	balance := acc.Balance + delta
	if balance < 0 {
		// 现金余额不可为负（迁移注释的不变量）。当前无可消费路径，故正常流程不可达；
		// 一旦 PAY-02 引入「现金 → 虚拟币」兑换，退款需按应收处理而非直接扣负。
		return 0, errcode.ErrPayCashNotEnough
	}
	if err := tx.Model(&CashAccount{}).Where("id = ?", acc.ID).
		Update("balance", balance).Error; err != nil {
		return 0, err
	}
	if err := tx.Create(&CashLog{
		UserID: userID, Delta: delta, BalanceAfter: balance,
		BizType: bizType, BizNo: bizNo, OrderNo: orderNo, Remark: remark,
	}).Error; err != nil {
		return 0, err
	}
	return balance, nil
}

// lockAccount FOR UPDATE 行锁现金账户；无账户行则插入 0 余额行再重锁。
func lockAccount(tx *gorm.DB, userID int64) (*CashAccount, error) {
	var acc CashAccount
	err := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		acc = CashAccount{UserID: userID, Balance: 0}
		if err := tx.Create(&acc).Error; err != nil {
			// 并发首单：唯一键冲突说明对方已建，重读加锁
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

// CashBalance 查现金余额（懒创建口径：无账户即 0）。
func (r *repo) CashBalance(userID int64) (int64, error) {
	var acc CashAccount
	err := r.db.Where("user_id = ?", userID).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return acc.Balance, nil
}

// CashLogsByUser 用户现金流水（验收逐条核对用）。
func (r *repo) CashLogsByUser(userID int64) ([]CashLog, error) {
	var rows []CashLog
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// CashLogsByOrder 某订单的现金流水（对账「流水缺失/重复」判据）。
func (r *repo) CashLogsByOrder(orderNo string) ([]CashLog, error) {
	var rows []CashLog
	err := r.db.Where("order_no = ?", orderNo).Order("id ASC").Find(&rows).Error
	return rows, err
}

// ---- 回调日志 ----

// InsertCallbackLog 落回调日志（**验签失败也落库**，spec §6.4 取证要求）。
func (r *repo) InsertCallbackLog(l *PayCallbackLog) error { return r.db.Create(l).Error }

// CallbackCount 某订单的「验签通过」回调条数（重复回调判据，spec §6.5）。
func (r *repo) CallbackCount(orderNo string) (int64, error) {
	var n int64
	err := r.db.Model(&PayCallbackLog{}).
		Where("order_no = ? AND sign_ok = 1", orderNo).Count(&n).Error
	return n, err
}

// ---- 对账 ----

// ReconAgg 单渠道单日聚合结果（订单口径 + 流水口径）。
type ReconAgg struct {
	OrderCount   int64
	PaidCount    int64
	PaidAmount   int64
	RefundCount  int64
	RefundAmount int64
	CashLogPaid  int64
	CashLogRefnd int64
}

// ReconAggregate 聚合某渠道某日的订单口径与流水口径（金额单位：分，SUM 回 int64）。
//
// 口径刻意从两张独立的表分别取数（pay_order vs cash_log），而不是从一处派生——
// 对账的意义就在于「两条独立记录路径相互印证」，同源派生等于自己证明自己。
func (r *repo) ReconAggregate(channel, date string) (*ReconAgg, error) {
	var agg ReconAgg
	row := struct {
		Cnt    int64
		Amount int64
	}{}
	// 订单口径：当日下单
	if err := r.db.Model(&PayOrder{}).
		Where("channel = ? AND DATE(created_at) = ?", channel, date).
		Select("COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	agg.OrderCount = row.Cnt
	// 订单口径：当日支付成功（金额只在支付成功时计入收入）
	if err := r.db.Model(&PayOrder{}).
		Where("channel = ? AND status IN ? AND DATE(paid_at) = ?", channel, []int8{StatusPaid, StatusRefunded}, date).
		Select("COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	agg.PaidCount, agg.PaidAmount = row.Cnt, row.Amount
	// 订单口径：当日退款
	if err := r.db.Model(&PayOrder{}).
		Where("channel = ? AND status = ? AND DATE(refunded_at) = ?", channel, StatusRefunded, date).
		Select("COUNT(*) AS cnt, COALESCE(SUM(refund_amount),0) AS amount").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	agg.RefundCount, agg.RefundAmount = row.Cnt, row.Amount
	// 流水口径：当日现金流水（按 biz_type 分列）
	if err := r.db.Model(&CashLog{}).
		Where("biz_type = ? AND DATE(created_at) = ?", CashBizPay, date).
		Select("COALESCE(SUM(delta),0) AS amount").Scan(&row).Error; err != nil {
		return nil, err
	}
	agg.CashLogPaid = row.Amount
	if err := r.db.Model(&CashLog{}).
		Where("biz_type = ? AND DATE(created_at) = ?", CashBizRefund, date).
		Select("COALESCE(SUM(delta),0) AS amount").Scan(&row).Error; err != nil {
		return nil, err
	}
	agg.CashLogRefnd = row.Amount
	return &agg, nil
}

// DetectIssues 检测某渠道某日的异常订单（spec §6.5 四类）。
//
// 每类都是一条独立的 SQL 判据，不依赖前一轮对账结果，故可重复执行；
// 结果按 order_no 去重后交由 UpsertIssue 幂等登记。
func (r *repo) DetectIssues(channel, date string, stuckHours int) ([]PayReconIssue, error) {
	var issues []PayReconIssue
	reconDay := parseDate(date)
	add := func(orderNo string, typ int8, detail string) {
		issues = append(issues, PayReconIssue{
			ReconDate: reconDay, Channel: channel, OrderNo: orderNo,
			IssueType: typ, Detail: detail,
		})
	}

	// ① 金额不符：验签通过的回调所报金额 ≠ 订单金额（回调日志是唯一独立证据源）
	type mism struct {
		OrderNo string
		Amount  int64
		Need    int64
	}
	var ms []mism
	if err := r.db.Model(&PayCallbackLog{}).
		Joins("JOIN pay_order o ON o.order_no = pay_callback_log.order_no").
		Where("pay_callback_log.channel = ? AND pay_callback_log.sign_ok = 1 AND DATE(pay_callback_log.created_at) = ? AND pay_callback_log.amount <> o.amount",
			channel, date).
		Select("pay_callback_log.order_no AS order_no, pay_callback_log.amount AS amount, o.amount AS need").
		Scan(&ms).Error; err != nil {
		return nil, err
	}
	for _, m := range ms {
		add(m.OrderNo, IssueAmountMismatch,
			"回调金额 "+itoa(m.Amount)+" 分 ≠ 订单金额 "+itoa(m.Need)+" 分")
	}

	// ② 长期挂起：待支付且创建时间早于 now - stuckHours
	type stuckRow struct {
		OrderNo   string
		CreatedAt string
	}
	var st []stuckRow
	if err := r.db.Model(&PayOrder{}).
		Where("channel = ? AND status = ? AND created_at < DATE_SUB(NOW(), INTERVAL ? HOUR)",
			channel, StatusPending, stuckHours).
		Select("order_no, created_at").Scan(&st).Error; err != nil {
		return nil, err
	}
	for _, s := range st {
		add(s.OrderNo, IssueStuck, "订单自 "+s.CreatedAt+" 起长期处于待支付")
	}

	// ③ 流水缺失：已支付/已退款但无对应现金流水（理论不可能，出现即严重缺陷）
	type missingRow struct {
		OrderNo string
		Status  int8
		Amount  int64
	}
	var mi []missingRow
	if err := r.db.Model(&PayOrder{}).
		Where("pay_order.channel = ? AND pay_order.status IN ? AND NOT EXISTS (SELECT 1 FROM cash_log c WHERE c.biz_no = CONCAT('pay:', pay_order.order_no))",
			channel, []int8{StatusPaid, StatusRefunded}).
		Select("order_no, status, amount").Scan(&mi).Error; err != nil {
		return nil, err
	}
	for _, m := range mi {
		add(m.OrderNo, IssueMissingLog,
			"订单状态="+statusText(m.Status)+" 但无对应现金流水（应有 "+itoa(m.Amount)+" 分入账）")
	}

	// ④ 重复回调：同一订单在**本对账日**收到 > 1 次验签通过的回调。
	// 渠道重复投递是常态，故本项登记的是「发生了重复投递」这一事实，供人工确认幂等是否生效；
	// 若同时现金流水 > 1 条，才是幂等被击穿（detail 里写明，避免把正常重投误读为缺陷）。
	//
	// 日期口径：与 ①②③ 一致 —— ①②③ 是「当下状态」检查（订单现在是否仍挂起/仍缺流水），
	// 本项与前项「金额不符」是「发生在该日的事件」，故按回调时间圈定对账日，
	// 否则把历史上任意日期的重复投递都登记到今天名下，日对账单会失去日期含义。
	type dupRow struct {
		OrderNo string
		Cnt     int64
		CashCnt int64
	}
	var du []dupRow
	if err := r.db.Model(&PayCallbackLog{}).
		Where("channel = ? AND sign_ok = 1 AND order_no <> '' AND DATE(created_at) = ?", channel, date).
		Select("order_no, COUNT(*) AS cnt, (SELECT COUNT(*) FROM cash_log c WHERE c.biz_no = CONCAT('pay:', pay_callback_log.order_no)) AS cash_cnt").
		Group("order_no").Having("COUNT(*) > 1").
		Scan(&du).Error; err != nil {
		return nil, err
	}
	for _, d := range du {
		detail := "收到 " + itoa(d.Cnt) + " 次验签通过的回调，现金流水 " + itoa(d.CashCnt) + " 条"
		if d.CashCnt > 1 {
			detail += "（幂等被击穿，需人工介入）"
		}
		add(d.OrderNo, IssueDuplicateCb, detail)
	}
	return issues, nil
}

// UpsertReconDaily 写/覆盖日对账单（uk_date_channel 唯一，重跑幂等）。
func (r *repo) UpsertReconDaily(d *PayReconDaily) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "recon_date"}, {Name: "channel"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"order_count", "paid_count", "paid_amount", "refund_count",
			"refund_amount", "cash_log_amount", "diff_count", "status",
		}),
	}).Create(d).Error
}

// UpsertIssue 幂等登记异常（uk_date_order_type 唯一：重跑不重复膨胀清单）。
func (r *repo) UpsertIssue(i *PayReconIssue) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "recon_date"}, {Name: "order_no"}, {Name: "issue_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"detail", "channel"}),
	}).Create(i).Error
}

// ListDaily 日对账单查询。
func (r *repo) ListDaily(date, channel string, page, size int) ([]PayReconDaily, int64, error) {
	q := r.db.Model(&PayReconDaily{})
	if date != "" {
		q = q.Where("recon_date = ?", date)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []PayReconDaily
	err := q.Order("recon_date DESC, channel").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// IssueFilter 异常清单查询条件。
type IssueFilter struct {
	Date      string
	Channel   string
	Status    *int8
	IssueType *int8
	Page      int
	Size      int
}

// ListIssues 异常订单清单（人工处理台）。
func (r *repo) ListIssues(f IssueFilter) ([]PayReconIssue, int64, error) {
	q := r.db.Model(&PayReconIssue{})
	if f.Date != "" {
		q = q.Where("recon_date = ?", f.Date)
	}
	if f.Channel != "" {
		q = q.Where("channel = ?", f.Channel)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.IssueType != nil {
		q = q.Where("issue_type = ?", *f.IssueType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []PayReconIssue
	err := q.Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// HandleIssue 异常处理（备注 + 处理人 + 时间）；幂等：已处理的再次处理只更新备注。
func (r *repo) HandleIssue(id, adminID int64, note string) (*PayReconIssue, error) {
	var issue PayReconIssue
	if err := r.db.First(&issue, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, err
	}
	err := r.db.Model(&PayReconIssue{}).Where("id = ?", id).Updates(map[string]any{
		"status":      IssueHandled,
		"handled_by":  adminID,
		"handled_at":  gorm.Expr("NOW()"),
		"handle_note": note,
	}).Error
	if err != nil {
		return nil, err
	}
	if err := r.db.First(&issue, id).Error; err != nil {
		return nil, err
	}
	return &issue, nil
}
