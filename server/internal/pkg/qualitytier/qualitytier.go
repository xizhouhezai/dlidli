// Package qualitytier 清晰度档位配置（M4-VIP-02）。
//
// 为什么单独成一个 pkg：档位配置同时被**两个方向**使用——
//   - video 模块：转码参数与投稿排程、下发播放地址时的过滤；
//   - middleware.PlaySignGuard：/static 入口的签名校验（会员限定档位二次拦截）。
//
// 而 video 模块已经 import middleware（取 CtxUserID），middleware 反向 import video 会成环，
// 故把「纯数据 + 纯判定」下沉到本包，两侧都只依赖它。
//
// 判定口径（spec §2.1 / §8.2）：
//   - `member_only=1` 的档位仅大会员可播放；
//   - 免费用户的**分辨率上限** = 非会员限定档位中最高的 height（初始数据即 1080）；
//   - `quality=0`（原画）不适用档位上限，但适用**源分辨率上限**：
//     原画直出的是上传的源文件，4K 源的原画行本身就是 4K，故按该行的 height 判定；
//   - `height=0` 表示**未探测**（既有历史行都是 0）：只按 member_only 判定，
//     不套用分辨率上限——保证升级前既有 0/360/720 数据的可播性零变化。
package qualitytier

import (
	"sort"
	"sync/atomic"
)

// Tier 单个清晰度档位配置（对应 video_quality_tier 一行）。
type Tier struct {
	Quality          int16
	Label            string
	Height           int // 转码目标高度；0=不改分辨率（原画档）
	VBits            string
	MaxRate          string
	BufSize          string
	ABits            string
	MemberOnly       bool
	TranscodeEnabled bool
	Sort             int
}

// 原画档编号（源文件直出，不是转码目标）。
const Origin = int16(0)

// Set 档位集合的**不可变快照**。
//
// 管理端改配置后整体替换（atomic.Pointer），读侧无锁：
// 下发过滤与签名校验每个播放请求都要查档位，加锁读不划算；
// 配置改动是低频运营动作，整体重建一个 map 的成本可忽略。
type Set struct {
	byQuality   map[int16]Tier
	maxFreeH    int   // 免费分辨率上限 = 非会员限定档位中的最大 height
	transcodeOn []int16 // 需要排转码任务的档位（升序）
	all         []Tier  // 全档位（sort 升序），供下发与管理端展示
}

// NewSet 由档位列表构建快照。
func NewSet(tiers []Tier) *Set {
	s := &Set{byQuality: make(map[int16]Tier, len(tiers))}
	for _, t := range tiers {
		s.byQuality[t.Quality] = t
		if !t.MemberOnly && t.Height > s.maxFreeH {
			s.maxFreeH = t.Height
		}
	}
	s.all = append(s.all, tiers...)
	sort.Slice(s.all, func(i, j int) bool {
		if s.all[i].Sort != s.all[j].Sort {
			return s.all[i].Sort < s.all[j].Sort
		}
		return s.all[i].Quality < s.all[j].Quality
	})
	for _, t := range s.all {
		if t.Quality != Origin && t.TranscodeEnabled {
			s.transcodeOn = append(s.transcodeOn, t.Quality)
		}
	}
	return s
}

// Get 取档位；不存在返回 (零值, false)。
func (s *Set) Get(quality int16) (Tier, bool) {
	if s == nil {
		return Tier{}, false
	}
	t, ok := s.byQuality[quality]
	return t, ok
}

// All 全档位（sort 升序，副本）。
func (s *Set) All() []Tier {
	if s == nil {
		return nil
	}
	return append([]Tier(nil), s.all...)
}

// TranscodeQualities 投稿时需要排队转码的档位（升序）。
func (s *Set) TranscodeQualities() []int16 {
	if s == nil {
		return nil
	}
	return append([]int16(nil), s.transcodeOn...)
}

// MaxFreeHeight 免费用户的通关分辨率上限（非会员限定档位中的最大 height）。
func (s *Set) MaxFreeHeight() int {
	if s == nil {
		return 0
	}
	return s.maxFreeH
}

// Allowance 一次「该清晰度能否下发给该身份」的判定结果。
type Allowance struct {
	Allowed bool
	Reason  string // 拒绝原因（内部日志与验收可读）
}

// Decide 判定某档位（含该流的实际高度）能否下发给给定身份。
//
// height 传该流的实际画面高度（0=未知，见包注释）。vip 为服务端权威会员判定结果。
// 快照为空（配置未加载）时一律放行：这是**升级前的行为**，配置缺失不应让全站不可播。
func (s *Set) Decide(quality int16, height int, vip bool) Allowance {
	if s == nil {
		return Allowance{Allowed: true}
	}
	if vip {
		return Allowance{Allowed: true}
	}
	t, ok := s.Get(quality)
	if ok && t.MemberOnly {
		return Allowance{Reason: "该清晰度为大会员专属档位"}
	}
	// 原画（或任何未探测高度的行）按源分辨率判定：4K 源的原画行不得下发给免费用户。
	// height=0 视为未知 → 不套用上限（既有历史行的行为不变）。
	if height > 0 && s.maxFreeH > 0 && height > s.maxFreeH {
		return Allowance{Reason: "超出免费用户分辨率上限"}
	}
	return Allowance{Allowed: true}
}

// Store 档位配置的常驻持有者（管理端改配置后由其整体替换快照）。
type Store struct{ p atomic.Pointer[Set] }

// NewStore 构建持有者。
func NewStore(s *Set) *Store {
	st := &Store{}
	if s != nil {
		st.p.Store(s)
	}
	return st
}

// Set 取当前快照（可能为 nil：配置未加载）。
func (st *Store) Set() *Set {
	if st == nil {
		return nil
	}
	return st.p.Load()
}

// Replace 整体替换快照（管理端改配置后调用）。
func (st *Store) Replace(s *Set) {
	if st == nil || s == nil {
		return
	}
	st.p.Store(s)
}
