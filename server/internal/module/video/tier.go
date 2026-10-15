package video

import (
	"errors"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/qualitytier"
	"gorm.io/gorm"
)

// qualityTierRow 对应 video_quality_tier 表（迁移 0040）。
type qualityTierRow struct {
	ID               int64 `gorm:"primaryKey"`
	Quality          int16
	Label            string
	Height           int
	VBits            string
	MaxRate          string
	BufSize          string
	ABits            string
	MemberOnly       int8
	TranscodeEnabled int8
	Sort             int
}

func (qualityTierRow) TableName() string { return "video_quality_tier" }

func (r qualityTierRow) toTier() qualitytier.Tier {
	return qualitytier.Tier{
		Quality:          r.Quality,
		Label:            r.Label,
		Height:           r.Height,
		VBits:            r.VBits,
		MaxRate:          r.MaxRate,
		BufSize:          r.BufSize,
		ABits:            r.ABits,
		MemberOnly:       r.MemberOnly == 1,
		TranscodeEnabled: r.TranscodeEnabled == 1,
		Sort:             r.Sort,
	}
}

// ListTiers 全档位（sort 升序）。
func (r *Repo) ListTiers() ([]qualitytier.Tier, error) {
	var rows []qualityTierRow
	if err := r.db.Order("sort, quality").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]qualitytier.Tier, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toTier())
	}
	return out, nil
}

// GetTierRow 按档位号取配置行（管理端更新用）。
func (r *Repo) GetTierRow(quality int16) (*qualityTierRow, error) {
	var row qualityTierRow
	err := r.db.Where("quality = ?", quality).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateTierFields 按字段更新档位配置（只改运营可改的列；quality 是主键语义，不可改）。
func (r *Repo) UpdateTierFields(quality int16, fields map[string]any) error {
	return r.db.Model(&qualityTierRow{}).Where("quality = ?", quality).Updates(fields).Error
}

// ---- 档位配置服务（挂在 video 模块上：档位是转码与播放流的能力，权限走 video:*）----

// TierView 管理端档位视图（snake_case：与既有后台 DTO 口径一致，不直接序列化 ORM 模型）。
type TierView struct {
	Quality          int16  `json:"quality"`
	Label            string `json:"label"`
	Height           int    `json:"height"`
	VBits            string `json:"v_bits"`
	MaxRate          string `json:"max_rate"`
	BufSize          string `json:"buf_size"`
	ABits            string `json:"a_bits"`
	MemberOnly       bool   `json:"member_only"`
	TranscodeEnabled bool   `json:"transcode_enabled"`
	Sort             int    `json:"sort"`
}

// TierUpdate 管理端档位变更（指针字段：未传的列不动）。
type TierUpdate struct {
	Label            *string `json:"label"`
	Height           *int    `json:"height"`
	VBits            *string `json:"v_bits"`
	MaxRate          *string `json:"max_rate"`
	BufSize          *string `json:"buf_size"`
	ABits            *string `json:"a_bits"`
	MemberOnly       *bool   `json:"member_only"`
	TranscodeEnabled *bool   `json:"transcode_enabled"`
	Sort             *int    `json:"sort"`
}

// LoadTiers 读全量档位配置。
func (s *Service) LoadTiers() (*qualitytier.Set, error) {
	tiers, err := s.repo.ListTiers()
	if err != nil {
		return nil, err
	}
	return qualitytier.NewSet(tiers), nil
}

// SetTierSet 装配期装载档位快照（启动时调用一次）。
func (s *Service) SetTierSet(set *qualitytier.Set) { s.tiers.Replace(set) }

// TierStore 档位配置持有者（供 /static 门禁共享同一份快照：
// 管理端改配置后两侧同时可见，不是各持副本）。
func (s *Service) TierStore() *qualitytier.Store { return s.tiers }

// ReloadTiers 重新读取并替换档位快照（配置改动后调用，使下发的判定立即生效）。
func (s *Service) ReloadTiers() error {
	set, err := s.LoadTiers()
	if err != nil {
		return err
	}
	s.tiers.Replace(set)
	return nil
}

// AdminTiers 管理端：全档位配置（含未启用转码的档位，便于运营看到全貌）。
func (s *Service) AdminTiers() []TierView {
	tiers := s.tiers.Set().All()
	out := make([]TierView, 0, len(tiers))
	for _, t := range tiers {
		out = append(out, TierView{
			Quality: t.Quality, Label: t.Label, Height: t.Height,
			VBits: t.VBits, MaxRate: t.MaxRate, BufSize: t.BufSize, ABits: t.ABits,
			MemberOnly: t.MemberOnly, TranscodeEnabled: t.TranscodeEnabled, Sort: t.Sort,
		})
	}
	return out
}

// AdminUpdateTier 管理端：改档位配置（开关 / 码率 / 会员限定 / 排序）。
//
// 两道自校验（防把手改到站不住的状态）：
//   - 关掉 0 档（原画）的不可变性：原画档不是转码目标，不允许开启排程；
//   - 不允许把**所有**档位都设为会员限定：那等于没有免费可播档位，属配置事故。
func (s *Service) AdminUpdateTier(quality int16, req *TierUpdate, adminID int64) (*TierView, error) {
	row, err := s.repo.GetTierRow(quality)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errcode.ErrInvalidParams.WithMsg("清晰度档位不存在")
	}
	fields := map[string]any{}
	if req.Label != nil {
		fields["label"] = *req.Label
	}
	if req.Height != nil {
		if *req.Height < 0 {
			return nil, errcode.ErrInvalidParams.WithMsg("目标高度不能为负")
		}
		fields["height"] = *req.Height
	}
	if req.VBits != nil {
		fields["v_bits"] = *req.VBits
	}
	if req.MaxRate != nil {
		fields["max_rate"] = *req.MaxRate
	}
	if req.BufSize != nil {
		fields["buf_size"] = *req.BufSize
	}
	if req.ABits != nil {
		fields["a_bits"] = *req.ABits
	}
	if req.MemberOnly != nil {
		if quality == qualitytier.Origin && *req.MemberOnly {
			return nil, errcode.ErrInvalidParams.WithMsg("原画档不能设为会员限定")
		}
		if *req.MemberOnly {
			free := 0
			for _, t := range s.tiers.Set().All() {
				if t.Quality != quality && !t.MemberOnly {
					free++
				}
			}
			if free == 0 {
				return nil, errcode.ErrInvalidParams.WithMsg("至少需保留一个非会员限定档位")
			}
		}
		v := int8(0)
		if *req.MemberOnly {
			v = 1
		}
		fields["member_only"] = v
	}
	if req.TranscodeEnabled != nil {
		if quality == qualitytier.Origin && *req.TranscodeEnabled {
			return nil, errcode.ErrInvalidParams.WithMsg("原画档不参与转码排程")
		}
		v := int8(0)
		if *req.TranscodeEnabled {
			v = 1
		}
		fields["transcode_enabled"] = v
	}
	if req.Sort != nil {
		fields["sort"] = *req.Sort
	}
	if len(fields) > 0 {
		if err := s.repo.UpdateTierFields(quality, fields); err != nil {
			return nil, err
		}
		if err := s.ReloadTiers(); err != nil {
			return nil, err
		}
	}
	updated, ok := s.tiers.Set().Get(quality)
	if !ok {
		return nil, errcode.ErrInternal
	}
	_ = adminID // 审计由 admin 中间件链路上的既有审计机制承担（与 pay/vip 管理端同口径）
	view := TierView{
		Quality: updated.Quality, Label: updated.Label, Height: updated.Height,
		VBits: updated.VBits, MaxRate: updated.MaxRate, BufSize: updated.BufSize, ABits: updated.ABits,
		MemberOnly: updated.MemberOnly, TranscodeEnabled: updated.TranscodeEnabled, Sort: updated.Sort,
	}
	return &view, nil
}
