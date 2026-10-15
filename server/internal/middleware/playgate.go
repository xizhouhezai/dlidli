package middleware

import (
	"context"
	"sync/atomic"

	"github.com/dlidli/server/internal/pkg/qualitytier"
)

// VIPCheck 会员判定（服务端权威口径）。由装配方注入，middleware 不 import 会员域
// （与 interaction/live 的 VipColors 注入同一思路）。
type VIPCheck func(ctx context.Context, uid int64) (bool, error)

// gateState 门禁依赖的一份不可变快照（读侧一次原子载入即拿到自洽的一组依赖）。
type gateState struct {
	tiers *qualitytier.Store
	vip   VIPCheck
}

// PlayGate 播放清晰度门禁的装配期依赖：档位配置 + 会员判定。
//
// 为什么需要这个持有者：`/static` 静态资源路由在 `router.New` 里注册得**早于**业务模块装配
// （本地存储驱动即注册，且 DB 未就绪时也要能服务静态资源），而门禁依赖的档位配置来自 video 域、
// 会员判定来自 vip 域——两者都晚于它。把静态服务挪进「DB 就绪」分支会丢掉降级语义，
// 故改为「先注册、后填充」：启动期单线程写入，服务期只读，用原子快照避免竞态。
//
// 未填充时的行为是**明确降级**：档位快照为空 → 不按 member_only 二次拦截（签名仍然照校），
// 即退化为升级前口径，而不是把整个静态资源服务打挂。
type PlayGate struct{ p atomic.Pointer[gateState] }

// NewPlayGate 构建空门禁（两侧依赖随后由装配方注入）。
func NewPlayGate() *PlayGate { return &PlayGate{} }

// SetTiers 注入档位配置快照的**共享持有者**（管理端改配置后由 video 域 Replace，
// 门禁侧立即可见——两侧读的是同一个 *qualitytier.Store，不是各自的副本）。
func (g *PlayGate) SetTiers(tiers *qualitytier.Store) {
	if g == nil || tiers == nil {
		return
	}
	g.update(func(s *gateState) { s.tiers = tiers })
}

// SetVIP 注入会员判定（vip 域的最小只读接口）。
func (g *PlayGate) SetVIP(check VIPCheck) {
	if g == nil || check == nil {
		return
	}
	g.update(func(s *gateState) { s.vip = check })
}

func (g *PlayGate) update(f func(*gateState)) {
	for {
		cur := g.p.Load()
		next := gateState{}
		if cur != nil {
			next = *cur
		}
		f(&next)
		if g.p.CompareAndSwap(cur, &next) {
			return
		}
	}
}

// tiers 当前档位快照（未注入返回 nil，判定侧对 nil 一律放行）。
func (g *PlayGate) tiersSet() *qualitytier.Set {
	if g == nil {
		return nil
	}
	st := g.p.Load()
	if st == nil || st.tiers == nil {
		return nil
	}
	return st.tiers.Set()
}

// IsVip 门禁用的会员判定。**未注入或判定出错一律返回 false**（fail closed）：
// 这是会员限定档位的守门判定，宁可把付费档位拦住，不可放行。
func (g *PlayGate) IsVip(ctx context.Context, uid int64) bool {
	if g == nil || uid <= 0 {
		return false
	}
	st := g.p.Load()
	if st == nil || st.vip == nil {
		return false
	}
	ok, err := st.vip(ctx, uid)
	if err != nil {
		return false
	}
	return ok
}
