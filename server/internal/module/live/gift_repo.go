package live

import (
	"errors"
	"fmt"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockingClause SELECT ... FOR UPDATE 行锁（余额强一致核心，spec LIV-13）。
var lockingClause = clause.Locking{Strength: "UPDATE"}

// giftShareDefault 主播分成比例（‰，plan 预研值 70%，对齐 monetization spec CHG-02 的 70/30）。
const giftShareDefault int64 = 700

// giftRepo 礼物域数据访问：金瓜子资产强一致（事务 + SELECT ... FOR UPDATE 行锁）。
type giftRepo struct {
	db *gorm.DB
}

func newGiftRepo(db *gorm.DB) *giftRepo { return &giftRepo{db: db} }

// NewGiftRepo 礼物域 repo（router 装配入口）。
func NewGiftRepo(db *gorm.DB) *giftRepo { return newGiftRepo(db) }

// MelonBalance 读余额（懒创建：账户不存在视为 0，首次扣款/入账时落账户行）。
func (r *giftRepo) MelonBalance(userID int64) (int64, error) {
	var acc MelonSeedAccount
	err := r.db.Where("user_id = ?", userID).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return acc.Balance, nil
}

// RechargeTx 充值入账（dev 直充 / 将来 PAY-01 支付回调共用）：
// 事务内行锁余额账户、加余额、写流水；biz_no 唯一键承载幂等——
// 同一支付单号重复回调第二次插入撞 uk_biz_no，事务回滚、余额不变（PAY-01 替换点）。
// 返回 (当前余额, 本次是否实际入账)。
func (r *giftRepo) RechargeTx(userID int64, delta int64, bizNo, remark string) (int64, bool, error) {
	var balance int64
	applied := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		acc, err := lockAccount(tx, userID)
		if err != nil {
			return err
		}
		// 幂等：同 biz_no 已入账则不再变动（返回当前余额）
		var cnt int64
		if err := tx.Model(&MelonSeedLog{}).Where("biz_no = ?", bizNo).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			balance = acc.Balance
			return nil
		}
		if err := tx.Model(&MelonSeedAccount{}).Where("id = ?", acc.ID).
			Update("balance", gorm.Expr("balance + ?", delta)).Error; err != nil {
			return err
		}
		balance = acc.Balance + delta
		applied = true
		return tx.Create(&MelonSeedLog{
			UserID: userID, Delta: delta, BalanceAfter: balance,
			BizType: MelonBizRecharge, BizNo: bizNo, Remark: remark,
		}).Error
	})
	if err != nil {
		return 0, false, err
	}
	return balance, applied, nil
}

// SendGiftTx 赠送礼物强一致核心（spec LIV-13）：
// 事务内 FOR UPDATE 锁观众余额 → 校验充足并扣减 → 观众流水 → 礼物流水 → 主播分成流水，
// 任一步失败整体回滚，余额不可能为负；广播在事务提交成功后进行（Service 层）。
// 返回扣款后余额与礼物流水 ID。
func (r *giftRepo) SendGiftTx(senderID int64, room *LiveRoom, gift *Gift, count int64) (int64, int64, error) {
	total := gift.Price * count
	share := giftShareDefault
	var logID int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 行锁观众余额账户（先确保账户行存在，避免锁空）
		acc, err := lockAccount(tx, senderID)
		if err != nil {
			return err
		}
		if acc.Balance < total {
			return errcode.ErrLiveMelonNotEnough
		}
		if err := tx.Model(&MelonSeedAccount{}).Where("id = ?", acc.ID).
			Update("balance", gorm.Expr("balance - ?", total)).Error; err != nil {
			return err
		}
		balance := acc.Balance - total
		// 观众扣减流水
		glog := &GiftLog{
			ID: snowflake.NextID(), RoomID: room.ID, SenderID: senderID,
			AnchorID: room.AnchorID, GiftID: gift.ID, Count: count,
			UnitPrice: gift.Price, TotalPrice: total, ShareRate: share,
		}
		if err := tx.Create(glog).Error; err != nil {
			return err
		}
		logID = glog.ID
		if err := tx.Create(&MelonSeedLog{
			UserID: senderID, Delta: -total, BalanceAfter: balance,
			BizType: MelonBizGiftSpend, BizNo: fmt.Sprintf("gift-%d", glog.ID),
			Remark: fmt.Sprintf("房间%d赠送%s×%d", room.ID, gift.Name, count),
		}).Error; err != nil {
			return err
		}
		// 主播分成入账（记流水；月度结算归 creator 体系，PAY-02 消费）
		anchorAmount := total * share / 1000
		return tx.Create(&GiftShareLog{
			GiftLogID: glog.ID, RoomID: room.ID, AnchorID: room.AnchorID,
			Amount: anchorAmount, PlatformAmount: total - anchorAmount,
		}).Error
	})
	if err != nil {
		return 0, 0, err
	}
	return logID, total, nil
}

// AnchorShareSum 主播分成累计（gift_share_log，验收核对用）。
func (r *giftRepo) AnchorShareSum(anchorID int64) (int64, error) {
	var sum int64
	err := r.db.Model(&GiftShareLog{}).Where("anchor_id = ?", anchorID).
		Select("COALESCE(SUM(amount), 0)").Take(&sum).Error
	return sum, err
}

// GiftLogsByRoom 房间礼物流水（验收核对用）。
func (r *giftRepo) GiftLogsByRoom(roomID int64) ([]GiftLog, error) {
	var rows []GiftLog
	err := r.db.Where("room_id = ?", roomID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// lockAccount FOR UPDATE 行锁余额账户；无账户行则插入初始 0 余额行再重锁。
func lockAccount(tx *gorm.DB, userID int64) (*MelonSeedAccount, error) {
	var acc MelonSeedAccount
	err := tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		acc = MelonSeedAccount{UserID: userID, Balance: 0}
		if err := tx.Create(&acc).Error; err != nil {
			// 并发首单：唯一键冲突说明对方已建，重读加锁
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				err = tx.Where("user_id = ?", userID).Clauses(lockingClause).First(&acc).Error
				if err != nil {
					return nil, err
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
