package pay

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 管理端订单查询的参数解析用例：**不需 DB、不需 DLIDLI_TEST_DSN**。
//
// 覆盖 order_filter_test.go 之外的一段：HTTP query → OrderFilter 的绑定。
// 新增的 min_amount/max_amount/start_date/end_date 若只测「条件 → SQL」的生成，
// 一个「忘了在 handler 里透传」的缺陷仍会全绿 —— 而漏透传正是这类改动的典型失效模式。
//
// 文件名说明：`dryrun_test.go` 是首版方案的遗留（当时打算用 GORM 的 DryRun 会话
// 读出 SQL 再断言）。实测发现 mysql 方言在 Initialize 阶段就会拨号建连接，
// 「无库也能跑」的前提不成立，故改用纯函数断言（见 order_filter_test.go）+
// 本文件的绑定断言。文件改名与删除在本会话受沙箱限制（见 .dev-logs/admin-fill/README.md），
// 内容已与此处职责一致，宿主可在发布时按需改名。
func TestAdminOrdersQueryParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	table := []struct {
		name  string
		query string
		want  OrderFilter
	}{
		{
			name:  "空 query 得到零值筛选（全量）",
			query: "",
			want:  OrderFilter{},
		},
		{
			name:  "金额与日期被透传",
			query: "?min_amount=1500&max_amount=5000&start_date=2026-10-01&end_date=2026-10-15",
			want: OrderFilter{
				MinAmount: 1500, MaxAmount: 5000,
				StartDate: "2026-10-01", EndDate: "2026-10-15",
			},
		},
		{
			name:  "非法金额退化为「不筛选」而非 0 元条件",
			query: "?min_amount=abc",
			want:  OrderFilter{},
		},
		{
			name:  "金额为 0 视为不筛选（0 分不是有意义的订单查询条件）",
			query: "?min_amount=0",
			want:  OrderFilter{},
		},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/api/v1/admin/pay/orders"+tc.query, nil)
			got := bindOrderFilter(c)
			if got.MinAmount != tc.want.MinAmount || got.MaxAmount != tc.want.MaxAmount ||
				got.StartDate != tc.want.StartDate || got.EndDate != tc.want.EndDate {
				t.Fatalf("筛选项不符：want %+v, got %+v", tc.want, got)
			}
		})
	}
}
