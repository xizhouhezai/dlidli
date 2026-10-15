package charge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/pay"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config 充电域配置（router 装配注入；零值由 DefaultConfig 兜底）。
type Config struct {
	// MinWithdrawAmount 最低提现金额（分，默认 10000 即 100 元）。
	// 取值理由：低于此额度的出款手续费占比过高，且小额高频提现会放大渠道与人工审核成本。
	MinWithdrawAmount int64
	// MaxWithdrawPerDay 单用户 24h 内提现申请次数上限（默认 3）。
	// 取值理由：提现是人工审核环节，日限 3 次足以覆盖正常创作者，同时限制刷单/试错。
	MaxWithdrawPerDay int
	// SettleEnabled 月度结算 Worker 总开关（dev 默认 false，避免周期任务干扰测试）。
	SettleEnabled bool
	// SettleIntervalSeconds 结算扫描周期秒（默认 3600）。
	SettleIntervalSeconds int
	// SettleDay 自动结算触发日（默认 1=每月 1 号结算上一自然月）。
	SettleDay int
}

// DefaultConfig 缺省配置。
func DefaultConfig() Config {
	return Config{
		MinWithdrawAmount:     10_000,
		MaxWithdrawPerDay:     3,
		SettleEnabled:         false,
		SettleIntervalSeconds: 3600,
		SettleDay:             1,
	}
}

// PayGateway 支付域最小接口（本包只用到下单与 dev 回调两件事）。
//
// 用接口而非直接依赖 *pay.Service：便于单测替换，也让「充电只是支付域的一个消费者」
// 这一依赖方向显式化（charge → pay；pay 不反向依赖 charge，收益发放经 PaidHook 注入）。
type PayGateway interface {
	CreateOrder(ctx context.Context, req *pay.CreateOrderReq) (*pay.OrderView, error)
	DevCallback(orderNo string, amount int64, result string) (*pay.CallbackOutcome, error)
}

// Service 充电域服务。
type Service struct {
	repo *Repo
	pay  PayGateway
	cfg  Config
	log  *zap.Logger
}

// NewService 构造（cfg 零值项用 DefaultConfig 兜底）。
func NewService(repo *Repo, pgw PayGateway, cfg Config, log *zap.Logger) *Service {
	def := DefaultConfig()
	if cfg.MinWithdrawAmount <= 0 {
		cfg.MinWithdrawAmount = def.MinWithdrawAmount
	}
	if cfg.MaxWithdrawPerDay <= 0 {
		cfg.MaxWithdrawPerDay = def.MaxWithdrawPerDay
	}
	if cfg.SettleIntervalSeconds <= 0 {
		cfg.SettleIntervalSeconds = def.SettleIntervalSeconds
	}
	if cfg.SettleDay <= 0 || cfg.SettleDay > 28 {
		cfg.SettleDay = def.SettleDay
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, pay: pgw, cfg: cfg, log: log}
}

// Cfg 暴露生效配置（验收与测试读口径用）。
func (s *Service) Cfg() Config { return s.cfg }

// AppliedIncome 支付入账时实际记入的分成金额（验收核对用；未入账为 0）。
type AppliedIncome struct {
	Applied bool  `json:"applied"`
	Amount  int64 `json:"amount"` // 分（创作者所得）
}

// ApplyPaidHook 支付成功后的充电分成发放（**在 pay 的入账事务内执行**）。
//
// 实现 pay.PaidHook 的签名，与 VIP-01 的 ApplyPaidHook 并列由 router 组合注入
// （pay 只有单个 PaidHook 槽位，充电与会员各自只处理自己的订单、互不干扰）。
//
// 非充电订单直接放行：pay 的钩子对所有支付都触发，不能因为「不是充电订单」报错。
// 幂等由两层保证：pay_order 状态机（RowsAffected）与 charge_log.status 条件 UPDATE。
func (s *Service) ApplyPaidHook(tx *gorm.DB, orderNo string, _ int64) error {
	applied, amount, err := s.repo.CreditIncomeByOrder(tx, orderNo)
	if err != nil {
		return err
	}
	if applied {
		s.log.Info("充电分成已入账", zap.String("order_no", orderNo), zap.Int64("amount", amount))
	}
	return nil
}

// ApplyPaidHookResult 与 ApplyPaidHook 同逻辑，但返回是否入账（集成测试与运维脚本用）。
func (s *Service) ApplyPaidHookResult(tx *gorm.DB, orderNo string, userID int64) (*AppliedIncome, error) {
	applied, amount, err := s.repo.CreditIncomeByOrder(tx, orderNo)
	if err != nil {
		return nil, err
	}
	return &AppliedIncome{Applied: applied, Amount: amount}, nil
}

// ---- 档位 ----

// Tiers 上架档位列表（spec CHG-01：5/10/50/自定义元）。
func (s *Service) Tiers() ([]TierView, error) {
	rows, err := s.repo.TierList()
	if err != nil {
		return nil, err
	}
	out := make([]TierView, 0, len(rows))
	for _, t := range rows {
		out = append(out, TierView{
			Code: t.Code, Name: t.Name, Amount: t.Amount,
			MinAmount: t.MinAmount, MaxAmount: t.MaxAmount,
			Custom: t.Code == TierCustomCode,
		})
	}
	return out, nil
}

// ---- 充电下单 ----

// CreateChargeReq 充电下单请求。
//
// **金额由服务端算**：客户端只传档位 code（自定义档才传 amount 且必须落在 min/max 内），
// 与 VIP-01 的下单口径一致——若信客户端金额，用户可以用 1 分钱充出 100 元的收益。
type CreateChargeReq struct {
	UserID    int64
	AnchorID  int64
	VideoID   int64
	TierCode  string
	Amount    int64 // 仅自定义档使用（分）
	Anonymous bool
	Message   string
	ClientIP  string
	Channel   string
}

// CreateCharge 充电下单：校验 → 服务端算价 → 复用 PAY-01 支付链路下单 → 落充电记录。
//
// 时序保证（plan §2H.3）：充电记录在**把支付凭据返回给客户端之前**落库。
// 客户端在拿到 pay_params 之前既不知道订单号、也无法完成支付，而 mock 渠道不存在
// 「未收款先回调」；因此「已支付但无充电记录」在结构上不可达，等价于同事务的效果。
// 代价是失败时可能留下一条待支付订单，由支付域既有的惰性关单（超时 15 分钟）回收——
// 这是可接受的残留（无资金占用、无收益错记），故不为它引入跨域分布式事务。
func (s *Service) CreateCharge(ctx context.Context, req *CreateChargeReq) (*pay.OrderView, error) {
	if req.Channel == "" {
		req.Channel = pay.ChannelMock
	}
	anchor, err := s.anchorByID(req.AnchorID)
	if err != nil {
		return nil, err
	}
	// 自充无意义且可被用来「左手倒右手」刷分成与提现，直接拒绝。
	if req.UserID == anchor.ID {
		return nil, errcode.ErrChargeSelf
	}
	if len([]rune(req.Message)) > 200 {
		return nil, errcode.ErrChargeMessageTooLong
	}
	amount, err := s.resolveAmount(req.TierCode, req.Amount)
	if err != nil {
		return nil, err
	}
	// 指定稿件时必须归属该 UP 主，否则等于拿别人的稿件给 A 充电（收益记错人）。
	if req.VideoID > 0 {
		if err := s.checkVideoOwner(req.VideoID, req.AnchorID); err != nil {
			return nil, err
		}
	}
	order, err := s.pay.CreateOrder(ctx, &pay.CreateOrderReq{
		UserID:  req.UserID,
		Amount:  amount,
		Channel: req.Channel,
		// sku 留空：支付域的单槽 skuHook 被会员商品占用，且充电的商品契约
		// （档位价、限购）已在本服务内校验；回调侧按 order_no 反查 charge_log 即可识别，
		// 详见 plan §2H.4「为什么充电不复用 sku 台账」。
		Sku:      "",
		Subject:  "充电给 " + anchor.Nickname,
		ClientIP: req.ClientIP,
	})
	if err != nil {
		return nil, err
	}
	c := &ChargeLog{
		ChargeNo: genChargeNo("C"), OrderNo: order.OrderNo,
		UserID: req.UserID, AnchorID: req.AnchorID, VideoID: req.VideoID,
		Amount: amount, ShareRate: int(ShareRateCreator),
		Anonymous: boolToInt8(req.Anonymous), Message: req.Message,
		Status: ChargeUnpaid,
	}
	if err := s.repo.CreateCharge(c); err != nil {
		// 记录写失败：订单已在（待支付），既不能收款也不会有回调。
		// 不删除订单（用户可见的订单流水中断更糟），由超时关单回收；此处必须报错，
		// 绝不能假装下单成功——否则用户会拿着一个永远不会产生充电记录的订单号去付款。
		s.log.Error("充电记录落库失败（订单保留待超时关闭）",
			zap.String("order_no", order.OrderNo), zap.Error(err))
		return nil, errcode.ErrInternal
	}
	return order, nil
}

// resolveAmount 档位算价（服务端权威）。
func (s *Service) resolveAmount(tierCode string, custom int64) (int64, error) {
	tier, err := s.repo.TierByCode(tierCode)
	if err != nil {
		return 0, err
	}
	if tier.Status != TierOn {
		return 0, errcode.ErrChargeTierNotFound
	}
	if tier.Code == TierCustomCode {
		if custom < tier.MinAmount || custom > tier.MaxAmount {
			return 0, errcode.ErrChargeAmountInvalid.WithMsg(
				fmt.Sprintf("自定义金额需在 %d~%d 分之间", tier.MinAmount, tier.MaxAmount))
		}
		return custom, nil
	}
	if tier.Amount <= 0 {
		return 0, errcode.ErrChargeAmountInvalid
	}
	return tier.Amount, nil
}

// checkVideoOwner 校验稿件归属（稿件必须属于被充电的 UP 主）。
func (s *Service) checkVideoOwner(videoID, anchorID int64) error {
	row := struct{ UserID int64 }{}
	if err := s.repo.DB().Table("video").Select("user_id").
		Where("id = ?", videoID).Scan(&row).Error; err != nil {
		return err
	}
	if row.UserID == 0 {
		return errcode.ErrChargeAnchorNotFound.WithMsg("稿件不存在")
	}
	if row.UserID != anchorID {
		return errcode.ErrInvalidParams.WithMsg("稿件不属于该 UP 主")
	}
	return nil
}

// anchorByID 校验被充电的 UP 主存在。
func (s *Service) anchorByID(id int64) (*UserBrief, error) {
	if id <= 0 {
		return nil, errcode.ErrChargeAnchorNotFound
	}
	row := struct {
		ID       int64
		Nickname string
	}{}
	if err := s.repo.DB().Table("user").Select("id, nickname").
		Where("id = ?", id).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, errcode.ErrChargeAnchorNotFound
	}
	return &UserBrief{ID: row.ID, Nickname: row.Nickname}, nil
}

// ---- 充电记录与鸣谢名单 ----

// UserBrief 展示用最小用户信息（只读模型）。
type UserBrief struct {
	ID       int64
	Nickname string
	Avatar   string
}

// userBriefs 批量取昵称/头像（展示用途的只读查询，失败降级为无昵称）。
func (s *Service) userBriefs(ids []int64) map[int64]UserBrief {
	out := map[int64]UserBrief{}
	if len(ids) == 0 {
		return out
	}
	rows := make([]UserBrief, 0, len(ids))
	if err := s.repo.DB().Table("user").Select("id, nickname, avatar").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

// MyCharges 我发出的充电记录。
func (s *Service) MyCharges(userID int64, page, size int) (map[string]any, error) {
	page, size = normPage(page, size)
	list, total, err := s.repo.ListCharges(ChargeFilter{UserID: userID, OnlyPaid: true, Page: page, Size: size})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"list": s.toChargeViews(list), "total": total, "page": page, "size": size,
	}, nil
}

// toChargeViews 充电记录对外视图；匿名条目不下发昵称。
func (s *Service) toChargeViews(list []ChargeLog) []ChargeView {
	ids := make([]int64, 0, len(list))
	for _, c := range list {
		if c.Anonymous == 0 {
			ids = append(ids, c.UserID)
		}
	}
	briefs := s.userBriefs(ids)
	out := make([]ChargeView, 0, len(list))
	for _, c := range list {
		v := ChargeView{
			ChargeNo: c.ChargeNo, OrderNo: c.OrderNo,
			UserID: itoa(c.UserID), AnchorID: itoa(c.AnchorID),
			Amount: c.Amount, Anonymous: c.Anonymous == 1,
			Message: c.Message, CreatedAt: fmtTime(c.CreatedAt),
		}
		if c.VideoID > 0 {
			v.VideoID = itoa(c.VideoID)
		}
		if c.PaidAt != nil {
			v.PaidAt = fmtTime(*c.PaidAt)
		}
		if c.Anonymous == 0 {
			v.Nickname = briefs[c.UserID].Nickname
		}
		out = append(out, v)
	}
	return out
}

// ThanksList 鸣谢名单（spec CHG-03：充电者名单展示，可匿名）。
//
// 只列**已支付**的充电（未支付不算打赏）；匿名条目只暴露金额与时间，
// 不返回 user_id 与昵称——「可匿名」若在接口层泄露身份，等于没有匿名。
func (s *Service) ThanksList(anchorID, videoID int64, limit int) ([]ThanksItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	list, _, err := s.repo.ListCharges(ChargeFilter{
		AnchorID: anchorID, VideoID: videoID, OnlyPaid: true, Page: 1, Size: limit,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(list))
	for _, c := range list {
		if c.Anonymous == 0 {
			ids = append(ids, c.UserID)
		}
	}
	briefs := s.userBriefs(ids)
	out := make([]ThanksItem, 0, len(list))
	for _, c := range list {
		it := ThanksItem{
			Amount: c.Amount, Message: c.Message,
			Anonymous: c.Anonymous == 1, CreatedAt: fmtTime(c.CreatedAt),
		}
		if c.Anonymous == 0 {
			b := briefs[c.UserID]
			it.Nickname, it.Avatar = b.Nickname, b.Avatar
		} else {
			it.Nickname = "匿名用户"
		}
		out = append(out, it)
	}
	return out, nil
}

// ---- 创作者收益与提现 ----

// Balance 创作者账户（现金负债口径）。
func (s *Service) Balance(userID int64) (*CreatorBalanceView, error) {
	return s.repo.Balance(userID)
}

// ApplyWithdraw 提现申请：校验门槛与频次 → 事务内行锁冻结（防超提）→ 落单。
func (s *Service) ApplyWithdraw(userID int64, amount int64, account, accountName string) (*WithdrawView, error) {
	if amount < s.cfg.MinWithdrawAmount {
		return nil, errcode.ErrWithdrawAmountInvalid.WithMsg(
			fmt.Sprintf("单笔提现不得低于 %d 分", s.cfg.MinWithdrawAmount))
	}
	if strings.TrimSpace(account) == "" || strings.TrimSpace(accountName) == "" {
		return nil, errcode.ErrWithdrawAccountInvalid
	}
	n, err := s.repo.CountRecentWithdraws(userID)
	if err != nil {
		return nil, err
	}
	// 事务外先做一次快速拒绝：省掉一次无谓事务（真正的判定在 ApplyWithdrawTx 内，
	// 因为只有持锁判定才能挡住并发绕过）。
	if n >= int64(s.cfg.MaxWithdrawPerDay) {
		return nil, errcode.ErrWithdrawTooFrequent.WithMsg(
			fmt.Sprintf("24 小时内最多发起 %d 次提现", s.cfg.MaxWithdrawPerDay))
	}
	w := &WithdrawOrder{
		WithdrawNo: genChargeNo("W"), UserID: userID, Amount: amount,
		Channel: pay.ChannelMock, Account: maskAccount(account), AccountName: accountName,
		Status: WithdrawAuditing,
	}
	// 频次与余额校验都在 ApplyWithdrawTx 的**事务内**复核（同一把行锁），
	// 故并发请求既不会绕过日限，也不会超提。
	if err := s.repo.ApplyWithdrawTx(w, s.cfg.MaxWithdrawPerDay); err != nil {
		return nil, err
	}
	s.log.Info("提现申请已受理", zap.Int64("uid", userID),
		zap.String("withdraw_no", w.WithdrawNo), zap.Int64("amount", amount))
	return s.withdrawView(w), nil
}

// MyWithdraws 我的提现记录。
func (s *Service) MyWithdraws(userID int64, page, size int) (map[string]any, error) {
	page, size = normPage(page, size)
	list, total, err := s.repo.ListWithdraws(WithdrawFilter{UserID: userID, Page: page, Size: size})
	if err != nil {
		return nil, err
	}
	views := make([]*WithdrawView, 0, len(list))
	for i := range list {
		views = append(views, s.withdrawView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": page, "size": size}, nil
}

// Settlements 我的结算单。
func (s *Service) Settlements(userID int64, month string, page, size int) (map[string]any, error) {
	return s.settlementPage(userID, month, page, size)
}

// ---- 管理端 ----

// AdminCharges 充电记录查询（管理端）。
func (s *Service) AdminCharges(f ChargeFilter) (map[string]any, error) {
	f.Page, f.Size = normPage(f.Page, f.Size)
	list, total, err := s.repo.ListCharges(f)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"list": s.toChargeViews(list), "total": total, "page": f.Page, "size": f.Size,
	}, nil
}

// AdminWithdraws 提现审核台列表（管理端）。
func (s *Service) AdminWithdraws(f WithdrawFilter) (map[string]any, error) {
	f.Page, f.Size = normPage(f.Page, f.Size)
	list, total, err := s.repo.ListWithdraws(f)
	if err != nil {
		return nil, err
	}
	views := make([]*WithdrawView, 0, len(list))
	for i := range list {
		views = append(views, s.withdrawView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": f.Page, "size": f.Size}, nil
}

// AdminAuditWithdraw 审核（通过/驳回）。
func (s *Service) AdminAuditWithdraw(withdrawNo string, approve bool, adminID int64, note string) (*WithdrawView, error) {
	w, err := s.repo.AuditWithdrawTx(withdrawNo, approve, adminID, note)
	if err != nil {
		return nil, err
	}
	return s.withdrawView(w), nil
}

// AdminPayout 打款（mock 通道；重复打款被双闸拦下 → 92303）。
func (s *Service) AdminPayout(withdrawNo string, fail bool, remark string) (*WithdrawView, error) {
	tradeNo := ""
	if !fail {
		tradeNo = "MOCKPAY" + withdrawNo
	}
	w, err := s.repo.PayoutWithdrawTx(withdrawNo, fail, tradeNo, remark)
	if err != nil {
		return nil, err
	}
	return s.withdrawView(w), nil
}

// AdminSettleMonth 手动触发月结（管理端补跑；与 Worker 同一实现）。
func (s *Service) AdminSettleMonth(month string) (*SettleResult, error) {
	if month == "" {
		month = lastMonth(time.Now())
	}
	return s.repo.SettleMonth(month)
}

// AdminSettlements 结算单查询（管理端）。
func (s *Service) AdminSettlements(userID int64, month string, page, size int) (map[string]any, error) {
	return s.settlementPage(userID, month, page, size)
}

// ---- 内部 ----

func (s *Service) settlementPage(userID int64, month string, page, size int) (map[string]any, error) {
	page, size = normPage(page, size)
	list, total, err := s.repo.SettlementList(userID, month, page, size)
	if err != nil {
		return nil, err
	}
	views := make([]SettlementView, 0, len(list))
	for _, x := range list {
		views = append(views, SettlementView{
			SettleMonth: x.SettleMonth, UserID: itoa(x.UserID),
			ChargeCount: int64(x.ChargeCount), Amount: x.Amount,
			PlatformAmount: x.PlatformAmount, CreatedAt: fmtTime(x.CreatedAt),
		})
	}
	return map[string]any{"list": views, "total": total, "page": page, "size": size}, nil
}

func (s *Service) withdrawView(w *WithdrawOrder) *WithdrawView {
	return &WithdrawView{
		WithdrawNo: w.WithdrawNo, UserID: itoa(w.UserID),
		Amount: w.Amount, Fee: w.Fee, Channel: w.Channel,
		Account: w.Account, AccountName: w.AccountName,
		Status: w.Status, StatusText: withdrawStatusText(w.Status),
		AuditNote: w.AuditNote, ChannelTradeNo: w.ChannelTradeNo, FailReason: w.FailReason,
		CreatedAt: fmtTime(w.CreatedAt),
		AuditAt:   fmtTimePtr(w.AuditAt), PaidAt: fmtTimePtr(w.PaidAt),
	}
}

// maskAccount 收款账户脱敏（只留首尾，避免接口/后台明文外泄完整账号）。
func maskAccount(a string) string {
	a = strings.TrimSpace(a)
	r := []rune(a)
	if len(r) <= 4 {
		return a
	}
	return string(r[:2]) + strings.Repeat("*", len(r)-4) + string(r[len(r)-2:])
}

// lastMonth 上一自然月（YYYY-MM）。
func lastMonth(now time.Time) string { return now.AddDate(0, -1, 0).Format("2006-01") }

// normPage 分页归一（与 pay 同口径）。
func normPage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}

// genChargeNo 业务单号：前缀 + 时间 + 随机（充电 C / 提现 W）。
func genChargeNo(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s%s%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(b))
}

func boolToInt8(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
