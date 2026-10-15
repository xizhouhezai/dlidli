package pay

import (
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// ReconService 对账服务（spec §6.5）：
//
// **口径（必须写清）**：本卡做的是「本地订单 ↔ 本地现金流水」的一致性核对 ——
// 从 pay_order 聚合出订单口径，从 cash_log 聚合出资金口径，两条**独立写入路径**相互印证；
// 差异按订单逐条登记到异常清单，由人工处理台处置。
//
// **边界（不在本卡）**：真实渠道对账文件（支付宝/微信账单下载与解析）是显式换入点，
// 见 alipay.go / wechat.go 的接入步骤 9。**不做自动冲正**（plan §2 决策：风险高）。
type ReconService struct {
	repo *repo
	cfg  Config
	log  *zap.Logger
}

// NewReconService 构造。
func NewReconService(r *repo, cfg Config, log *zap.Logger) *ReconService {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg.StuckHours <= 0 {
		cfg.StuckHours = DefaultConfig().StuckHours
	}
	return &ReconService{repo: r, cfg: cfg, log: log}
}

// ReconResult 对账结果（重跑返回同一结论，spec §6.5）。
type ReconResult struct {
	ReconDate string        `json:"recon_date"`
	Channels  []*DailyView  `json:"channels"`
	Issues    []*IssueView  `json:"issues"`
	IssueNum  int           `json:"issue_num"`
}

// DailyView 日对账单对外视图。
//
// 独立 DTO 而非直接序列化 PayReconDaily：ORM 模型字段名是 Go 命名（ID/ReconDate/
// DiffCount），直接返回会让接口出现 PascalCase 键，与 OrderView/IssueView 的
// snake_case 口径不一致，前端要维护两套取字段方式（实测首轮 E2E 正是这么被绊住的）。
type DailyView struct {
	ReconDate     string `json:"recon_date"`
	Channel       string `json:"channel"`
	OrderCount    int64  `json:"order_count"`
	PaidCount     int64  `json:"paid_count"`
	PaidAmount    int64  `json:"paid_amount"`
	RefundCount   int64  `json:"refund_count"`
	RefundAmount  int64  `json:"refund_amount"`
	CashLogAmount int64  `json:"cash_log_amount"`
	DiffCount     int64  `json:"diff_count"`
	Status        int8   `json:"status"`
	StatusText    string `json:"status_text"`
	UpdatedAt     string `json:"updated_at"`
}

func toDailyView(d *PayReconDaily) *DailyView {
	txt := "正常"
	if d.Status == ReconHasIssue {
		txt = "有异常"
	}
	return &DailyView{
		ReconDate: dateOnly(d.ReconDate), Channel: d.Channel,
		OrderCount: d.OrderCount, PaidCount: d.PaidCount, PaidAmount: d.PaidAmount,
		RefundCount: d.RefundCount, RefundAmount: d.RefundAmount,
		CashLogAmount: d.CashLogAmount, DiffCount: d.DiffCount,
		Status: d.Status, StatusText: txt,
		UpdatedAt: d.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// IssueView 异常项对外视图。
type IssueView struct {
	ID         int64  `json:"id"`
	ReconDate  string `json:"recon_date"`
	Channel    string `json:"channel"`
	OrderNo    string `json:"order_no"`
	IssueType  int8   `json:"issue_type"`
	TypeText   string `json:"type_text"`
	Detail     string `json:"detail"`
	Status     int8   `json:"status"`
	HandledBy  string `json:"handled_by,omitempty"`
	HandledAt  string `json:"handled_at,omitempty"`
	HandleNote string `json:"handle_note,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func toIssueView(i *PayReconIssue) *IssueView {
	v := &IssueView{
		ID: i.ID, ReconDate: dateOnly(i.ReconDate), Channel: i.Channel, OrderNo: i.OrderNo,
		IssueType: i.IssueType, TypeText: issueTypeText(i.IssueType), Detail: i.Detail,
		Status: i.Status, HandleNote: i.HandleNote,
		CreatedAt: i.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if i.HandledBy > 0 {
		v.HandledBy = itoa(i.HandledBy)
	}
	if i.HandledAt != nil {
		v.HandledAt = i.HandledAt.Format("2006-01-02 15:04:05")
	}
	return v
}

// Run 执行某日某渠道（channel 为空则全部已注册渠道）的对账并落库。
//
// 幂等：日对账单按 (日期,渠道) UPSERT，异常按 (日期,订单,类型) UPSERT，
// 故可反复执行而不膨胀清单——这是人工复核的前提（spec §6.5 / plan §2E.6）。
func (s *ReconService) Run(date, channel string) (*ReconResult, error) {
	if err := validateDate(date); err != nil {
		return nil, err
	}
	channels := []string{ChannelMock, ChannelAlipay, ChannelWeChat}
	if channel != "" {
		if !IsRegistered(channel) {
			return nil, errcode.ErrPayChannelUnknown
		}
		channels = []string{channel}
	}
	res := &ReconResult{ReconDate: date}
	for _, ch := range channels {
		agg, err := s.repo.ReconAggregate(ch, date)
		if err != nil {
			return nil, err
		}
		issues, err := s.repo.DetectIssues(ch, date, s.cfg.StuckHours)
		if err != nil {
			return nil, err
		}
		// 先登记异常再统计 diff_count：清单是对账产出的一部分，不是事后补充。
		for i := range issues {
			if err := s.repo.UpsertIssue(&issues[i]); err != nil {
				return nil, err
			}
		}
		daily := &PayReconDaily{
			ReconDate:     parseDate(date),
			Channel:       ch,
			OrderCount:    agg.OrderCount,
			PaidCount:     agg.PaidCount,
			PaidAmount:    agg.PaidAmount,
			RefundCount:   agg.RefundCount,
			RefundAmount:  agg.RefundAmount,
			CashLogAmount: agg.CashLogPaid + agg.CashLogRefnd, // 退款流水为负值，相加即净额
			DiffCount:     int64(len(issues)),
			Status:        ReconOK,
		}
		if len(issues) > 0 {
			daily.Status = ReconHasIssue
		}
		// 流水口径与订单口径不符时，即使四类异常都没命中也要标异常——否则「两条口径
		// 对不上但没有任何一条判据命中」会成为对账的盲区。
		expectCash := agg.PaidAmount - agg.RefundAmount
		if expectCash != daily.CashLogAmount {
			daily.Status = ReconHasIssue
			if daily.DiffCount == 0 {
				daily.DiffCount = 1
			}
		}
		if err := s.repo.UpsertReconDaily(daily); err != nil {
			return nil, err
		}
		res.Channels = append(res.Channels, toDailyView(daily))
		s.log.Info("对账完成",
			zap.String("date", date), zap.String("channel", ch),
			zap.Int64("order_count", agg.OrderCount), zap.Int64("paid_count", agg.PaidCount),
			zap.Int64("paid_amount", agg.PaidAmount), zap.Int64("cash_log_amount", daily.CashLogAmount),
			zap.Int("issues", len(issues)))

		// 全部渠道模式：异常清单只按「本渠道 + 本日」取，避免跨渠道串味。
		views, err := s.issueViews(date, ch)
		if err != nil {
			return nil, err
		}
		res.Issues = append(res.Issues, views...)
	}
	res.IssueNum = len(res.Issues)
	return res, nil
}

// issueViews 取某日某渠道的异常清单视图（含历史已处理项，便于人工复核连续性）。
func (s *ReconService) issueViews(date, channel string) ([]*IssueView, error) {
	rows, _, err := s.repo.ListIssues(IssueFilter{Date: date, Channel: channel, Page: 1, Size: 200})
	if err != nil {
		return nil, err
	}
	out := make([]*IssueView, 0, len(rows))
	for i := range rows {
		out = append(out, toIssueView(&rows[i]))
	}
	return out, nil
}

// Daily 日对账单查询（管理端）。
func (s *ReconService) Daily(date, channel string, page, size int) (map[string]any, error) {
	page, size = normPage(page, size)
	list, total, err := s.repo.ListDaily(date, channel, page, size)
	if err != nil {
		return nil, err
	}
	views := make([]*DailyView, 0, len(list))
	for i := range list {
		views = append(views, toDailyView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": page, "size": size}, nil
}

// Issues 异常订单清单（管理端）。
func (s *ReconService) Issues(f IssueFilter) (map[string]any, error) {
	f.Page, f.Size = normPage(f.Page, f.Size)
	list, total, err := s.repo.ListIssues(f)
	if err != nil {
		return nil, err
	}
	views := make([]*IssueView, 0, len(list))
	for i := range list {
		views = append(views, toIssueView(&list[i]))
	}
	return map[string]any{"list": views, "total": total, "page": f.Page, "size": f.Size}, nil
}

// Handle 异常处理（人工处理台）。
func (s *ReconService) Handle(id, adminID int64, note string) (*IssueView, error) {
	if id <= 0 {
		return nil, errcode.ErrInvalidParams
	}
	issue, err := s.repo.HandleIssue(id, adminID, note)
	if err != nil {
		return nil, err
	}
	return toIssueView(issue), nil
}

// validateDate 校验对账日期（YYYY-MM-DD 且不得为未来日期；spec §6.8 的 92011）。
func validateDate(date string) error {
	_, err := parseDateChecked(date)
	return err
}

// parseDate 把已验证的 YYYY-MM-DD 转为本地时区当日 0 点（validateDate 已先行校验）。
func parseDate(date string) time.Time {
	d, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	return d
}

// parseDateChecked 校验并解析日期。
func parseDateChecked(date string) (time.Time, error) {
	if date == "" {
		return time.Time{}, errcode.ErrPayReconDateInvalid
	}
	d, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, errcode.ErrPayReconDateInvalid
	}
	today := time.Now().Truncate(24 * time.Hour)
	if d.After(today) {
		return time.Time{}, errcode.ErrPayReconDateInvalid
	}
	return d, nil
}
