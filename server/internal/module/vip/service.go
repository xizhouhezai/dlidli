package vip

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/module/pay"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config 会员域配置（router 装配注入）。
type Config struct {
	CoinPerMonth     int  // 每月赠币枚数（spec §2.1：5）
	ExpireRemindDays int  // 到期前提醒天数（VIP-04：3）
	RenewEnabled     bool // 自动续期 Worker 总开关
	RenewIntervalSec int  // 续期扫描周期秒
	RenewBatchSize   int  // 单轮扫描上限
	MaxFailCount     int  // 连续失败多少次自动解约
}

// DefaultConfig 缺省配置。
func DefaultConfig() Config {
	return Config{
		CoinPerMonth: 5, ExpireRemindDays: 3,
		RenewEnabled: false, RenewIntervalSec: 60, RenewBatchSize: 100, MaxFailCount: 3,
	}
}

// PayGateway 会员域对支付域的最小依赖（**接口而非具体类型**，便于测试替身）。
type PayGateway interface {
	CreateOrder(ctx context.Context, req *pay.CreateOrderReq) (*pay.OrderView, error)
	DevCallback(orderNo string, amount int64, result string) (*pay.CallbackOutcome, error)
	GetOrder(ctx context.Context, orderNo string, userID int64) (*pay.OrderView, error)
}

// Service 会员域服务：套餐、开通/顺延、权益判定、连续包月与月度赠币。
type Service struct {
	repo *repo
	pay  PayGateway
	cfg  Config
	log  *zap.Logger
	// chargeFn 扣款实现（默认走 mock 渠道回调链；测试可替换以模拟扣款失败）。
	chargeFn func(ctx context.Context, userID int64, planCode string, amount int64) error
}

// NewService 构造（零值配置回落缺省）。
func NewService(r *repo, gateway PayGateway, cfg Config, log *zap.Logger) *Service {
	def := DefaultConfig()
	if cfg.CoinPerMonth <= 0 {
		cfg.CoinPerMonth = def.CoinPerMonth
	}
	if cfg.ExpireRemindDays <= 0 {
		cfg.ExpireRemindDays = def.ExpireRemindDays
	}
	if cfg.RenewIntervalSec <= 0 {
		cfg.RenewIntervalSec = def.RenewIntervalSec
	}
	if cfg.RenewBatchSize <= 0 {
		cfg.RenewBatchSize = def.RenewBatchSize
	}
	if cfg.MaxFailCount <= 0 {
		cfg.MaxFailCount = def.MaxFailCount
	}
	if log == nil {
		log = zap.NewNop()
	}
	s := &Service{repo: r, pay: gateway, cfg: cfg, log: log}
	s.chargeFn = s.mockCharge
	return s
}

// ---- 套餐 ----

// Plans C 端套餐列表（只返回上架项）。
func (s *Service) Plans() ([]*PlanView, error) {
	list, err := s.repo.ListPlans(true)
	if err != nil {
		return nil, err
	}
	out := make([]*PlanView, 0, len(list))
	for i := range list {
		out = append(out, toPlanView(&list[i]))
	}
	return out, nil
}

// allPlans 管理端套餐列表（含下架项，便于运营看到全貌后决定上下架）。
func (s *Service) allPlans() ([]*PlanView, error) {
	list, err := s.repo.ListPlans(false)
	if err != nil {
		return nil, err
	}
	out := make([]*PlanView, 0, len(list))
	for i := range list {
		out = append(out, toPlanView(&list[i]))
	}
	return out, nil
}

// ---- 商品（pay.SkuHook 实现） ----

// CheckSku 下单前校验：套餐存在、已上架、金额与**该用户当前应付价**一致。
//
// **为什么必须在服务端校验金额**：客户端传 amount，若只信客户端，用户可以用 1 分钱买年卡。
// 金额不一致一律拒绝（错误码可区分），不做「以服务端价覆盖客户端」的静默纠正——
// 静默纠正会让用户以为按自己看到的价格付了款。
//
// 应付价含首月优惠：**从未开通过**的用户签连续包月套餐时按首月价。
// 判据用「membership 行不存在」而非「当前不是会员」——否则退订后再签可无限刷首月价。
func (s *Service) CheckSku(sku string, userID int64, amount int64) error {
	p, err := s.repo.GetPlan(sku)
	if err != nil {
		return err
	}
	if p.Status != PlanOn {
		return errcode.ErrVipPlanNotFound
	}
	want := p.Price
	if p.AutoRenew == 1 && p.FirstMonthPrice > 0 {
		m, e := s.repo.GetMembership(userID)
		if e != nil {
			return e
		}
		if m == nil {
			want = p.FirstMonthPrice
		}
	}
	if want != amount {
		return errcode.ErrPayAmountInvalid.WithMsg("套餐价格不一致：应为 " + itoa(want) + " 分")
	}
	return nil
}

// CreateOrderTx 订单事务内落 vip_order 台账（时长快照）。
func (s *Service) CreateOrderTx(tx *gorm.DB, orderNo string, userID int64, sku string) error {
	p, err := s.repo.getPlanTx(tx, sku)
	if err != nil {
		return err
	}
	return tx.Create(&VipOrder{
		OrderNo: orderNo, UserID: userID, PlanCode: p.Code,
		Months: p.Months, ApplyStatus: ApplyPending,
	}).Error
}

// ---- 权益发放（pay.PaidHook 实现） ----

// ApplyPaidHook 支付成功后的权益发放，**在 pay 的入账事务内执行**。
//
// 非会员商品（sku 不在 vip_order 台账里）直接放行：pay 的钩子对所有支付都触发，
// 未来 PAY-02 的充电订单同样会走到这里，不能因为「不是会员订单」就报错。
func (s *Service) ApplyPaidHook(tx *gorm.DB, orderNo string, userID int64) error {
	row, err := s.getOrderRowTx(tx, orderNo)
	if err != nil {
		return err
	}
	if row == nil {
		return nil // 非会员订单，无权益
	}
	res, err := s.repo.ApplyOrderTx(tx, userID, orderNo, row.PlanCode, row.Months)
	if err != nil {
		return err
	}
	if !res.Applied {
		return nil // 幂等：该订单此前已生效
	}
	s.log.Info("会员权益已生效",
		zap.Int64("uid", userID), zap.String("order", orderNo),
		zap.String("plan", res.PlanCode), zap.Int("months", res.Months),
		zap.String("expire_after", fmtTime(res.ExpireAfter)))
	// 连续包月套餐：生效即建立/刷新签约，下次扣款时间 = 新生效的到期时间。
	if err := s.maybeSignTx(tx, userID, res.PlanCode, res.ExpireAfter); err != nil {
		return err
	}
	return nil
}

// maybeSignTx 若套餐支持连续包月则建立签约（同事务，避免「会员开了但签约丢了」）。
func (s *Service) maybeSignTx(tx *gorm.DB, userID int64, planCode string, expireAfter time.Time) error {
	p, err := s.repo.getPlanTx(tx, planCode)
	if err != nil {
		return err
	}
	if p.AutoRenew != 1 {
		return nil
	}
	var sub VipSubscription
	e := tx.Where("user_id = ?", userID).First(&sub).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return tx.Create(&VipSubscription{
			UserID: userID, PlanCode: planCode, Channel: pay.ChannelMock, Status: SubActive,
			SignAt: time.Now(), NextChargeAt: expireAfter,
		}).Error
	}
	if e != nil {
		return e
	}
	// 已有签约：换套餐/续期后同步计划与下次扣款时间（**不**因重新支付而重置失败计数）。
	return tx.Model(&VipSubscription{}).Where("id = ?", sub.ID).Updates(map[string]any{
		"plan_code": planCode, "status": SubActive, "cancel_at": nil,
		"next_charge_at": expireAfter,
	}).Error
}

// getOrderRowTx 事务内取订单台账；不存在返回 (nil, nil)。
func (s *Service) getOrderRowTx(tx *gorm.DB, orderNo string) (*VipOrder, error) {
	var o VipOrder
	err := tx.Where("order_no = ?", orderNo).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ---- 权益判定 ----

// Status 会员状态与权益位（`GET /vip/status`）。
//
// **服务端判定**：IsVip 完全由 DB 的 `expire_at > NOW()` 决定，不接受任何客户端输入，
// 前端无法通过改请求参数绕过（plan §2F.1）。
func (s *Service) Status(userID int64) (*StatusView, error) {
	m, err := s.repo.GetMembership(userID)
	if err != nil {
		return nil, err
	}
	v := &StatusView{IsVip: false, Level: 0, Rights: freeRights()}
	if m == nil {
		return v, nil
	}
	// 判定时钟取 DB：与应用时钟可能漂移，故再用一次 EXISTS 口径复核（单条 SQL）。
	active, err := s.repo.IsVip(userID)
	if err != nil {
		return nil, err
	}
	if !active {
		// 过期用户：显式判为免费（到期时间仍返回，便于前端提示「已过期」）。
		v.ExpireAt = fmtTime(m.ExpireAt)
		return v, nil
	}
	v.IsVip = true
	v.Level = m.Level
	v.ExpireAt = fmtTime(m.ExpireAt)
	v.AutoRenew = m.AutoRenew == 1
	v.Rights = vipRights(s.cfg.CoinPerMonth)
	// 剩余天数向上取整：不足一天也算 1 天，避免「今天到期」显示为 0 天引发误解。
	left := time.Until(m.ExpireAt)
	if left > 0 {
		v.ExpireDays = int((left + 24*time.Hour - 1) / (24 * time.Hour))
	}
	v.ExpiringSoon = v.ExpireDays > 0 && v.ExpireDays <= s.cfg.ExpireRemindDays
	// 签约状态以签约表为准（auto_renew 列是「生效中的连续包月」快照位）。
	if sub, _ := s.repo.GetSubscription(userID); sub != nil && sub.Status == SubActive {
		v.AutoRenew = true
	}
	return v, nil
}

// VipFlags 批量会员标识（评论/弹幕读模型用；实现 Badge 接口）。
func (s *Service) VipFlags(userIDs []int64) (map[int64]BadgeView, error) {
	return s.repo.VipFlags(userIDs)
}

// VipColors 批量取「生效中会员」的 uid → 昵称颜色（跨模块最小契约）。
//
// 刻意只返回原始类型而非 BadgeView 结构体：interaction / live 等消费方据此定义自己的
// 小接口即可完成注入，无需 import 会员域（避免读模型与会员域互相依赖）。
// 返回的 map 只含会员；非会员不在其中（absent = 免费用户）。
func (s *Service) VipColors(userIDs []int64) (map[int64]string, error) {
	flags, err := s.repo.VipFlags(userIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(flags))
	for uid, f := range flags {
		out[uid] = f.NickColor
	}
	return out, nil
}

// IsVip 单人会员判定（跨模块服务端判定入口，如 VIP-02 清晰度鉴权）。
func (s *Service) IsVip(userID int64) (bool, error) { return s.repo.IsVip(userID) }

// ---- 下单 ----

// CreateVipOrder 会员下单：校验套餐 → 走支付域下单（sku=套餐 code）。
//
// 复用 PAY-01 的订单与支付链路（spec VIP-01「下单复用订单链路」）：
// 本函数只做会员域的套餐校验与首次优惠判定，订单状态机、渠道、回调幂等全部复用支付域。
//
// **金额由服务端算**：客户端不传价格，只传 plan code（避免「信客户端金额」的口子）；
// 首月优惠的实付价在 CheckSku 里被同一套规则复核，故客户端无从篡改。
func (s *Service) CreateVipOrder(ctx context.Context, userID int64, planCode, channel string) (*pay.OrderView, error) {
	p, err := s.repo.GetPlan(planCode)
	if err != nil {
		return nil, err
	}
	if p.Status != PlanOn {
		return nil, errcode.ErrVipPlanNotFound
	}
	amount := p.Price
	// 连续包月首月优惠（spec VIP-01「首月优惠」）：仅对从未开通过的用户生效。
	if p.AutoRenew == 1 && p.FirstMonthPrice > 0 {
		m, err := s.repo.GetMembership(userID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			amount = p.FirstMonthPrice
		}
	}
	return s.pay.CreateOrder(ctx, &pay.CreateOrderReq{
		UserID: userID, Amount: amount, Channel: channel, Sku: p.Code,
		Subject: p.Name, ClientIP: "127.0.0.1",
	})
}

// ---- 连续包月 ----

// CancelSubscribe 解约（可随时解约，spec VIP-04）。
func (s *Service) CancelSubscribe(userID int64) error {
	sub, err := s.repo.GetSubscription(userID)
	if err != nil {
		return err
	}
	if sub == nil || sub.Status != SubActive {
		return errcode.ErrVipNoSubscription
	}
	if err := s.repo.CancelSubscription(userID); err != nil {
		return err
	}
	s.log.Info("连续包月已解约", zap.Int64("uid", userID))
	return nil
}

// SubscribeView 签约状态（无签约返回 nil）。
func (s *Service) SubscribeView(userID int64) (*SubscribeView, error) {
	sub, err := s.repo.GetSubscription(userID)
	if err != nil || sub == nil {
		return nil, err
	}
	v := &SubscribeView{
		PlanCode: sub.PlanCode, Status: sub.Status,
		SignAt: fmtTime(sub.SignAt), FailCount: sub.FailCount,
	}
	if p, e := s.repo.GetPlan(sub.PlanCode); e == nil {
		v.PlanName, v.Price = p.Name, p.Price
	}
	if sub.Status == SubActive {
		v.NextChargeAt = fmtTime(sub.NextChargeAt)
	}
	if sub.LastChargeAt != nil {
		v.LastChargeAt = fmtTime(*sub.LastChargeAt)
	}
	return v, nil
}

// mockCharge 走 mock 渠道扣款（**换入点**）。
//
// 真实免密代扣需要支付通道资质（当前不具备），故此处用「建订单 + mock 渠道回调」模拟一次
// 成功扣款——它**不是旁路实现**：订单状态机、回调验签与幂等、权益发放事务全部真实执行。
// 将来接入真实代扣，只需把本函数换成「渠道代扣 API + 代扣结果回调」，
// 其余逻辑（顺延、签约推进、失败计数）零改动（plan §2F.5）。
func (s *Service) mockCharge(ctx context.Context, userID int64, planCode string, _ int64) error {
	p, err := s.repo.GetPlan(planCode)
	if err != nil {
		return err
	}
	order, err := s.pay.CreateOrder(ctx, &pay.CreateOrderReq{
		UserID: userID, Amount: p.Price, Channel: pay.ChannelMock, Sku: p.Code,
		Subject: p.Name + "（连续包月自动续期）", ClientIP: "127.0.0.1",
	})
	if err != nil {
		return err
	}
	// 与「用户手动支付」走同一条回调链：验签 → 幂等 → 入账 → 权益发放（含签约推进）。
	res, err := s.pay.DevCallback(order.OrderNo, p.Price, "success")
	if err != nil {
		return err
	}
	if !res.Applied {
		// 理论上不可能（订单刚创建）；一旦出现说明幂等闸被误触发，必须报错而非静默放过。
		return errcode.ErrVipRenewFailed.WithMsg("续期订单未生效：" + res.Message)
	}
	return nil
}

// RenewOnce 自动续期单轮：扫描到期的签约 → CAS 抢权 → mock 扣款 → 成功顺延 / 失败计数。
//
// 返回 (候选数, 成功数, 失败数)。**通知简化为日志**（spec 允许）：
// 失败与解约都写 Warn 日志并带 uid/订单号，真实触达（站内信/短信）为换入点。
func (s *Service) RenewOnce(ctx context.Context) (due, ok, failed int, err error) {
	subs, e := s.repo.DueSubscriptions(s.cfg.RenewBatchSize)
	if e != nil {
		return 0, 0, 0, e
	}
	for i := range subs {
		sub := &subs[i]
		// CAS 抢权：把 next_charge_at 从读取值推进到下一周期，抢不到说明别的实例在做。
		to := sub.NextChargeAt.AddDate(0, monthsOf(s, sub.PlanCode), 0)
		if to.Before(time.Now()) {
			// 周期已错过（如长时间停机）时从当前时间起推，避免连续追赶多期把用户一次扣爆。
			to = time.Now().AddDate(0, monthsOf(s, sub.PlanCode), 0)
		}
		claimed, e := s.repo.ClaimSubscription(sub.UserID, sub.NextChargeAt, to)
		if e != nil {
			s.log.Error("续期抢权失败", zap.Int64("uid", sub.UserID), zap.Error(e))
			continue
		}
		if !claimed {
			continue // 对手已抢到本轮
		}
		due++
		price := int64(0)
		if p, e := s.repo.GetPlan(sub.PlanCode); e == nil {
			price = p.Price
		}
		if e := s.chargeFn(ctx, sub.UserID, sub.PlanCode, price); e != nil {
			failed++
			n, _ := s.repo.MarkChargeFailed(sub.UserID)
			s.log.Warn("连续包月自动续期扣款失败（通知换入点）",
				zap.Int64("uid", sub.UserID), zap.String("plan", sub.PlanCode),
				zap.Int("fail_count", n), zap.Error(e))
			// 失败后改到次日重试，而不是等到下个整月：给用户留出自助修复账号的时间窗。
			if e2 := s.repo.SetNextCharge(sub.UserID, time.Now().AddDate(0, 0, 1)); e2 != nil {
				s.log.Error("续期失败后重排时间失败", zap.Int64("uid", sub.UserID), zap.Error(e2))
			}
			if n >= s.cfg.MaxFailCount {
				// 连续失败达阈值 → 自动解约（签约不再生效，防止无限重试与骚扰）。
				if e2 := s.repo.CancelSubscription(sub.UserID); e2 == nil {
					s.log.Warn("连续包月因连续扣款失败已自动解约（通知换入点）",
						zap.Int64("uid", sub.UserID), zap.Int("fail_count", n))
				}
			}
			continue
		}
		if e := s.repo.MarkCharged(sub.UserID); e != nil {
			s.log.Error("续期成功标记失败", zap.Int64("uid", sub.UserID), zap.Error(e))
		}
		ok++
		s.log.Info("连续包月自动续期成功", zap.Int64("uid", sub.UserID),
			zap.String("plan", sub.PlanCode), zap.String("next_charge_at", fmtTime(to)))
	}
	return due, ok, failed, nil
}

// monthsOf 取套餐时长月数（套餐被删时回落 1 个月，宁可少发不中断服务）。
func monthsOf(s *Service, planCode string) int {
	if p, err := s.repo.GetPlan(planCode); err == nil && p.Months > 0 {
		return p.Months
	}
	return 1
}

// StartRenewWorker 启动自动续期定时器（与 live 截帧机审 / video 转码同风格：
// **立即返回**，循环在独立 goroutine 里跑，否则会阻塞 router 装配）。
func (s *Service) StartRenewWorker(ctx context.Context) {
	if !s.cfg.RenewEnabled {
		s.log.Info("连续包月自动续期未启用（vip.renewEnabled=false，dev 默认关闭）")
		return
	}
	go s.renewLoop(ctx)
}

// renewLoop 续期主循环：ticker + ctx.Done 退出，不引入新中间件。
func (s *Service) renewLoop(ctx context.Context) {
	interval := time.Duration(s.cfg.RenewIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.log.Info("连续包月自动续期 Worker 已启动", zap.Duration("interval", interval))
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 单轮 panic 自隔离：一轮异常不能拖死 Worker（与截帧机审同口径）。
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.log.Error("续期单轮 panic，已隔离", zap.Any("recover", r))
					}
				}()
				if _, _, _, err := s.RenewOnce(ctx); err != nil {
					s.log.Error("续期单轮失败", zap.Error(err))
				}
			}()
		}
	}
}

// ---- 月度赠币 ----

// GrantCoinsFor 给单个会员发放本月硬币（幂等：重复调用第二次返回 granted=false）。
func (s *Service) GrantCoinsFor(userID int64, source int8) (bool, error) {
	granted := false
	err := s.repo.db.Transaction(func(tx *gorm.DB) error {
		ok, e := s.repo.GrantCoinsTx(tx, userID, source)
		granted = ok
		return e
	})
	return granted, err
}

// GrantMonthlyOnce 月度补发扫描：给「生效中且本月未发」的会员补发硬币。
//
// 与开通时的即时发放共用同一幂等键，故两者谁先跑都不会重复发。
// **为什么还要一个定时补发**：即时发放只在开通/续期那一刻触发，
// 而「续费发生在月中、月月连续」的用户在跨月时不会有任何开通事件，
// 单靠即时发放会漏发——月赠币是订阅权益，不是首购礼品。
func (s *Service) GrantMonthlyOnce() (granted, skipped int, err error) {
	rows, e := s.repo.ActiveMembershipsWithoutGrant(periodKey(time.Now()), s.cfg.RenewBatchSize)
	if e != nil {
		return 0, 0, e
	}
	for _, uid := range rows {
		ok, e := s.GrantCoinsFor(uid, GrantByWorker)
		if e != nil {
			s.log.Error("月度赠币失败", zap.Int64("uid", uid), zap.Error(e))
			continue
		}
		if ok {
			granted++
		} else {
			skipped++
		}
	}
	return granted, skipped, nil
}

// ---- 管理端 ----

// AdminMembers 会员查询（挂 vip:view）。
func (s *Service) AdminMembers(f MemberFilter) (map[string]any, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 100 {
		f.Size = 20
	}
	list, total, err := s.repo.ListMemberships(f)
	if err != nil {
		return nil, err
	}
	type memberView struct {
		UserID      string `json:"user_id"`
		Level       int8   `json:"level"`
		ExpireAt    string `json:"expire_at"`
		IsVip       bool   `json:"is_vip"`
		AutoRenew   bool   `json:"auto_renew"`
		TotalMonths int    `json:"total_months"`
		StartedAt   string `json:"started_at"`
	}
	views := make([]memberView, 0, len(list))
	for i := range list {
		m := &list[i]
		views = append(views, memberView{
			UserID: itoa(m.UserID), Level: m.Level, ExpireAt: fmtTime(m.ExpireAt),
			IsVip: m.ExpireAt.After(time.Now()), AutoRenew: m.AutoRenew == 1,
			TotalMonths: m.TotalMonths, StartedAt: fmtTime(m.StartedAt),
		})
	}
	return map[string]any{"list": views, "total": total, "page": f.Page, "size": f.Size}, nil
}

// AdminGrantCoins 管理端手工补发本月硬币（挂 vip:grant；幂等，重复点不会多发）。
func (s *Service) AdminGrantCoins(userID int64, adminID int64) (bool, error) {
	granted, err := s.GrantCoinsFor(userID, GrantByAdmin)
	if err != nil {
		return false, err
	}
	s.log.Info("管理端补发会员硬币",
		zap.Int64("uid", userID), zap.Int64("admin", adminID), zap.Bool("granted", granted))
	return granted, nil
}

// AdminUpsertPlan 管理端套餐配置（挂 vip:plan；code 已存在则更新）。
func (s *Service) AdminUpsertPlan(p *VipPlan, adminID int64) error {
	if p.Code == "" || p.Name == "" || p.Months <= 0 || p.Price < 0 {
		return errcode.ErrVipPlanNotFound
	}
	if p.Status != PlanOn && p.Status != PlanOff {
		p.Status = PlanOff
	}
	one := int8(1)
	if p.AutoRenew != 0 && p.AutoRenew != one {
		p.AutoRenew = 0
	}
	existing, err := s.repo.GetPlan(p.Code)
	if err != nil {
		if !errors.Is(err, errcode.ErrVipPlanNotFound) {
			return err
		}
		if e := s.repo.CreatePlan(p); e != nil {
			return e
		}
	} else {
		if e := s.repo.UpdatePlan(existing.Code, map[string]any{
			"name": p.Name, "months": p.Months, "price": p.Price,
			"original_price": p.OriginalPrice, "first_month_price": p.FirstMonthPrice,
			"auto_renew": p.AutoRenew, "sort": p.Sort, "status": p.Status,
		}); e != nil {
			return e
		}
	}
	s.log.Info("管理端套餐配置已更新", zap.String("code", p.Code),
		zap.Int64("admin", adminID), zap.Int64("price", p.Price), zap.Int8("status", p.Status))
	return nil
}

// SeedDefaultPlans 首次启动播种默认套餐（表非空则不动，运营改过的价不会被覆盖）。
func (s *Service) SeedDefaultPlans() {
	n, err := s.repo.SeedPlansIfEmpty(DefaultPlans())
	if err != nil {
		s.log.Warn("默认套餐播种失败", zap.Error(err))
		return
	}
	if n > 0 {
		s.log.Info("已播种默认会员套餐", zap.Int("count", n))
	}
}

// itoa 十进制字符串（ID 输出统一字符串，避免 JS 精度丢失）。
func itoa(v int64) string { return strconv.FormatInt(v, 10) }
