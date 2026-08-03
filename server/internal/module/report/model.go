// Package report 举报域（M2-AUD-03）：全对象类型举报 + 后台处理队列。
package report

import "time"

// 举报对象类型
const (
	TargetVideo   int8 = 1 // 视频
	TargetComment int8 = 2 // 评论
	TargetDanmaku int8 = 3 // 弹幕
	TargetDynamic int8 = 4 // 动态
	TargetUser    int8 = 5 // 用户
)

// 举报类型（字典：与 admin 后台 SYS-03 数据字典对齐）
const (
	ReasonIllegal int8 = 1 // 违法违规
	ReasonPorn    int8 = 2 // 色情低俗
	ReasonAbuse   int8 = 3 // 人身攻击
	ReasonSpam    int8 = 4 // 垃圾广告
	ReasonSpoiler int8 = 5 // 剧透
	ReasonOther   int8 = 6 // 其他
)

// 处理状态
const (
	StatusPending int8 = 0 // 待处理
	StatusHandled int8 = 1 // 已处理（删除/处罚）
	StatusIgnored int8 = 2 // 已忽略
)

// ReasonNames 举报类型文案。
var ReasonNames = map[int8]string{
	ReasonIllegal: "违法违规", ReasonPorn: "色情低俗", ReasonAbuse: "人身攻击",
	ReasonSpam: "垃圾广告", ReasonSpoiler: "剧透", ReasonOther: "其他",
}

// TargetNames 对象类型文案。
var TargetNames = map[int8]string{
	TargetVideo: "视频", TargetComment: "评论", TargetDanmaku: "弹幕",
	TargetDynamic: "动态", TargetUser: "用户",
}

// Report 对应 report 表。
type Report struct {
	ID           int64 `gorm:"primaryKey"`
	ReporterID   int64
	TargetType   int8
	TargetID     int64
	ReasonType   int8
	Reason       string
	Status       int8
	HandlerID    int64
	HandleResult string
	HandledAt    *time.Time
	CreatedAt    time.Time
}

func (Report) TableName() string { return "report" }
