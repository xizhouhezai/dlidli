package pay

import (
	"strings"
	"testing"
)

// 管理端订单筛选（金额 / 日期）用例：**不需 DB、不需 DLIDLI_TEST_DSN**。
//
// 判据是「筛选条件 → WHERE 片段」的映射本身，故直接断言 orderConds 的纯函数输出。
// 之所以不走 GORM 的 DryRun 会话：mysql 方言在 Initialize 阶段就会拨号建连接，
// 于是「无库也跑」这个前提根本不成立 —— 这里选纯函数，让该字段的语义在任何环境都可验。
func TestOrderConds(t *testing.T) {
	table := []struct {
		name     string
		filter   OrderFilter
		wantSQL  []string
		wantArgv [][]any
		wantOmit []string
	}{
		{
			name:     "零值不产生任何筛选条件（页面不填即全量）",
			filter:   OrderFilter{},
			wantOmit: []string{"amount >=", "amount <=", "created_at >=", "created_at <"},
		},
		{
			name:     "金额区间按分过滤（后端只收分，换算在前端）",
			filter:   OrderFilter{MinAmount: 1500, MaxAmount: 5000},
			wantSQL:  []string{"amount >= ?", "amount <= ?"},
			wantArgv: [][]any{{int64(1500)}, {int64(5000)}},
		},
		{
			name:     "仅金额下限",
			filter:   OrderFilter{MinAmount: 1500},
			wantSQL:  []string{"amount >= ?"},
			wantArgv: [][]any{{int64(1500)}},
		},
		{
			name:     "日期为半开区间（不套 DATE()，末日 +1 天由 MySQL 算）",
			filter:   OrderFilter{StartDate: "2026-10-01", EndDate: "2026-10-15"},
			wantSQL:  []string{"created_at >= ?", "created_at < DATE_ADD(?, INTERVAL 1 DAY)"},
			wantArgv: [][]any{{"2026-10-01 00:00:00"}, {"2026-10-15 00:00:00"}},
		},
		{
			name:     "订单号/用户/渠道/状态各自成条",
			filter:   OrderFilter{OrderNo: "PAY1", UserID: 92600001, Channel: "mock", Status: statusPtr(1)},
			wantSQL:  []string{"order_no = ?", "user_id = ?", "channel = ?", "status = ?"},
			wantArgv: [][]any{{"PAY1"}, {int64(92600001)}, {"mock"}, {int8(1)}},
		},
		{
			name:     "状态 0 是有效筛选（待支付），不得被当作零值丢弃",
			filter:   OrderFilter{Status: statusPtr(0)},
			wantSQL:  []string{"status = ?"},
			wantArgv: [][]any{{int8(0)}},
		},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			conds := orderConds(tc.filter)
			sqls := make([]string, 0, len(conds))
			for _, c := range conds {
				sqls = append(sqls, c.SQL)
			}
			joined := strings.Join(sqls, " | ")
			for _, want := range tc.wantSQL {
				if !containsSQL(sqls, want) {
					t.Fatalf("缺少条件 %q\n实际: %s", want, joined)
				}
			}
			for _, omit := range tc.wantOmit {
				if strings.Contains(joined, omit) {
					t.Fatalf("不应包含 %q\n实际: %s", omit, joined)
				}
			}
			// 参数逐个核对：SQL 片段对但参数错位（如金额与用户互换）同样是缺陷。
			for i, want := range tc.wantArgv {
				if i >= len(conds) {
					t.Fatalf("条件数量不足：想核对第 %d 条，实际 %d 条", i, len(conds))
				}
				got := conds[i].Args
				if len(got) != len(want) || got[0] != want[0] {
					t.Fatalf("第 %d 条参数不符：want %v, got %v", i, want, got)
				}
			}
			if len(tc.wantArgv) > 0 && len(conds) != len(tc.wantArgv) {
				t.Fatalf("条件数量不符：want %d, got %d (%s)", len(tc.wantArgv), len(conds), joined)
			}
		})
	}
}

func containsSQL(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func statusPtr(v int8) *int8 { return &v }
