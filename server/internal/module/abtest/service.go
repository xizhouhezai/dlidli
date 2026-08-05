package abtest

import (
	"context"
	"hash/fnv"
	"strconv"
)

// Service A/B 实验：分流判定 + admin CRUD。
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Variant 用户分流：命中启用实验时按 uid 哈希稳定分配 A/B 组（无实验返回空串=默认策略）。
// 分流算法：FNV-1a(uid) % 100 < ratio → B 组，否则 A 组（稳定、无需用户表额外字段）。
func (s *Service) Variant(ctx context.Context, uid int64, target string) (string, error) {
	e, err := s.repo.EnabledOf(target)
	if err != nil || e == nil {
		return "", err
	}
	if uid <= 0 {
		return e.VariantA, nil
	}
	h := fnv.New32a()
	// 按 target+uid 哈希，同一用户在多次请求间稳定
	_, _ = h.Write([]byte(target))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(strconv.FormatInt(uid, 10)))
	if int(h.Sum32()%100) < int(e.Ratio) {
		return e.VariantB, nil
	}
	return e.VariantA, nil
}

// SaveReq 新建/编辑实验请求。
type SaveReq struct {
	Name     string `json:"name" binding:"required,max=64"`
	Target   string `json:"target" binding:"required,max=32"`
	VariantA string `json:"variant_a" binding:"required,max=64"`
	VariantB string `json:"variant_b" binding:"required,max=64"`
	Ratio    int8   `json:"ratio" binding:"min=0,max=100"`
	Status   int8   `json:"status" binding:"oneof=0 1"`
	Remark   string `json:"remark" binding:"max=200"`
}

// AdminList 全部实验。
func (s *Service) AdminList(_ context.Context) ([]Experiment, error) {
	return s.repo.ListAll()
}

// AdminCreate 新建实验。
func (s *Service) AdminCreate(_ context.Context, req *SaveReq) error {
	return s.repo.Create(&Experiment{
		Name: req.Name, Target: req.Target, VariantA: req.VariantA, VariantB: req.VariantB,
		Ratio: req.Ratio, Status: req.Status, Remark: req.Remark,
	})
}

// AdminUpdate 编辑实验。
func (s *Service) AdminUpdate(_ context.Context, id int64, req *SaveReq) error {
	return s.repo.Update(id, map[string]any{
		"name": req.Name, "target": req.Target, "variant_a": req.VariantA, "variant_b": req.VariantB,
		"ratio": req.Ratio, "status": req.Status, "remark": req.Remark,
	})
}

// AdminDelete 删除实验。
func (s *Service) AdminDelete(_ context.Context, id int64) error {
	return s.repo.Delete(id)
}
