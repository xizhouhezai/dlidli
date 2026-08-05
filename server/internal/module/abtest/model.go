// Package abtest A/B 实验分流框架（M3-OPS-03）：实验配置 + 按用户哈希分流。
// 应用侧通过 Variant(uid, target) 获取用户所属策略，无实验时返回空串（默认策略）。
package abtest

import "time"

// Experiment 对应 experiment 表。
type Experiment struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	Name      string    `json:"name"`
	Target    string    `json:"target"`   // 应用场景：recommend 等
	VariantA  string    `json:"variant_a"` // A 组策略标识
	VariantB  string    `json:"variant_b"` // B 组策略标识
	Ratio     int8      `json:"ratio"`    // B 组流量占比 0-100
	Status    int8      `json:"status"`   // 0启用 1停用
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Experiment) TableName() string { return "experiment" }
