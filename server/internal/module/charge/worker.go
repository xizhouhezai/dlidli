package charge

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// StartSettleWorker 月度结算 Worker（进程内 ticker，与转码/截帧/续期 Worker 同风格）。
//
// 触发口径（plan §2H.5）：**每月 SettleDay 日结算上一自然月**。结算归属按
// `charge_log.paid_at`（支付成功时间），故「跨月订单」天然落在它真实发生的月份，
// 与「结算动作哪天被执行」无关——重跑/补跑不会改变金额归属。
//
// 幂等：`SettleMonth` 自身可重放（uk_user_month + 明细条件 UPDATE），
// 因此本 Worker 在触发日重复 tick 也不会重复计额，无需分布式锁或「已跑过」标志。
//
// dev 默认关闭（cfg.SettleEnabled=false）：周期任务会干扰其它测试，
// 与 VIP-01 的续期 Worker、LIV-05 的截帧 Worker 同一取舍。
func (s *Service) StartSettleWorker(ctx context.Context) {
	if !s.cfg.SettleEnabled {
		s.log.Info("月度结算 Worker 未启用（charge.settleEnabled=false）")
		return
	}
	interval := time.Duration(s.cfg.SettleIntervalSeconds) * time.Second
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		s.log.Info("月度结算 Worker 已启动",
			zap.Int("settle_day", s.cfg.SettleDay), zap.Duration("interval", interval))
		for {
			select {
			case <-ctx.Done():
				s.log.Info("月度结算 Worker 退出")
				return
			case <-t.C:
				s.settleTick(time.Now())
			}
		}
	}()
}

// settleTick 单次检查：仅在触发日结算上一自然月。
func (s *Service) settleTick(now time.Time) {
	if now.Day() != s.cfg.SettleDay {
		return
	}
	month := lastMonth(now)
	res, err := s.repo.SettleMonth(month)
	if err != nil {
		s.log.Warn("月度结算失败", zap.String("month", month), zap.Error(err))
		return
	}
	s.log.Info("月度结算已完成",
		zap.String("month", res.Month), zap.Int("users", res.Users),
		zap.Int64("charge_count", res.ChargeCount), zap.Int64("amount", res.Amount))
}

// SettleLastMonth 立即结算上一自然月（dev/运维手动入口，与 Worker 同一实现）。
func (s *Service) SettleLastMonth() (*SettleResult, error) {
	return s.repo.SettleMonth(lastMonth(time.Now()))
}
