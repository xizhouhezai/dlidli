package video

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/searchindex"
	"github.com/dlidli/server/internal/pkg/bvid"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
)

// 直播回放稿件的内部生成路径（M4-LIV-14，spec LIV-14 / plan liv14 D3）。
//
// 为什么不在 live 模块里直接建稿：`video.Service.Submit` 是**用户投稿**的入口，
// 它以 file_id 取 `upload_file`（用户上传记录）为源，而回放的源是 SRS DVR 落在
// 服务端的录制文件——两者在「源从哪来」这一步上不可能共用。故这里新增一个
// **内部路径**：由 live 侧把已落盘的源文件登记进 upload_file 语义的存储 key，
// 再调用本方法建稿；转码排程/状态机/搜索索引全部复用 video 域既有实现。
//
// 与 Submit 的三处刻意差异：
//  1. 初始状态是 **StatusDraft（草稿）**，不是 StatusTranscoding：草稿态对他人不可见，
//     而转码产物由后台 Worker 补齐 —— 转码中并不妨碍「主播可选发布」这个语义，
//     但**可见性必须在建稿瞬间就是正确的**（转码完成后 Worker 不会自动推状态，见 transcoder.go）。
//  2. 不做 EnsureCanPublish / 机审风险分级：源是主播自己的直播流，且直播本身已经过
//     截帧机审（LIV-05）；此处再拦一次只会让「直播被误判过」的主播连回放都生成不了。
//  3. 允许自定义标题/简介/标签（回放标题由 live 侧按「直播回放 · 标题 · 日期」生成）。

// CreateReplayDraftReq 内部建稿请求（仅供 live 模块调用，不走 HTTP）。
type CreateReplayDraftReq struct {
	UserID     int64  // 稿件归属（= 主播）
	Title      string // 自动生成的标题
	Desc       string // 简介（含来源房间，便于合规追溯）
	SourceKey  string // 转码源存储 key（SRS DVR 录制文件已登记为 upload_file 的 store_key）
	SourceSize int64  // 源文件字节数
	CategoryID int    // 分区（0 = 取第一个启用分区）
	Cover      string // 封面 URL（可空，转码时自动抽帧补齐）
}

// CreateReplayDraft 由直播回放素材建**草稿**稿件：登记稿件 + 原画流 + 转码任务，不推进状态。
//
// 返回稿件详情（含 bvid）与稿件 ID，供 live 侧回填 live_replay.video_id。
func (s *Service) CreateReplayDraft(ctx context.Context, req *CreateReplayDraftReq) (*Detail, int64, error) {
	if strings.TrimSpace(req.SourceKey) == "" {
		return nil, 0, errcode.ErrInvalidParams.WithMsg("回放源文件不能为空")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "直播回放"
	}
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80])
	}

	categoryID := req.CategoryID
	if categoryID <= 0 {
		// 未指定分区时取第一个启用分区：稿件必须有分区（video.category_id NOT NULL），
		// 而回放是**自动**产生的，不能因为「主播没选分区」就整体失败。
		cats, err := s.repo.Categories()
		if err != nil {
			return nil, 0, err
		}
		if len(cats) == 0 {
			return nil, 0, errcode.ErrInvalidParams.WithMsg("无可用分区，无法生成回放稿件")
		}
		categoryID = cats[0].ID
	} else if ok, err := s.repo.CategoryExists(categoryID); err != nil {
		return nil, 0, err
	} else if !ok {
		return nil, 0, errcode.ErrInvalidParams.WithMsg("分区不存在")
	}

	tagsJSON, _ := json.Marshal([]string{"直播回放"})
	id := snowflake.NextID()
	v := &Video{
		ID:          id,
		Bvid:        bvid.Encode(id),
		UserID:      req.UserID,
		Title:       title,
		Cover:       req.Cover,
		Description: req.Desc,
		CategoryID:  categoryID,
		Tags:        string(tagsJSON),
		Copyright:   1, // 自制：直播是主播自制内容
		Status:      StatusDraft,
		RiskLevel:   RiskLow,
	}

	// 原画流指向 DVR 录制源（quality=0=源文件直出，与投稿同口径）。
	stream := &Stream{
		Quality:  0,
		Format:   "flv", // SRS DVR 录制格式（plan liv14 D1）
		PlayPath: req.SourceKey,
		FileSize: req.SourceSize,
	}

	// 转码档位：与投稿同一份配置（transcode_enabled=1 的档位）。
	// 转码关闭时不排任务——此时原画流即为唯一产物，草稿仍可发布（与投稿口径一致）。
	var jobs []int16
	if s.cfg.Transcode.Enabled {
		jobs = s.tiers.Set().TranscodeQualities()
		// 转码关闭时若仍排任务，Worker 不存在 → 任务永远 pending；
		// 而状态是草稿，不影响可见性，故这里只需不排任务，无需改状态。
	}

	if err := s.repo.CreateWithStat(v, stream, jobs); err != nil {
		return nil, 0, err
	}
	// 草稿不计入检索索引（未发布不可搜）；发布时由 PublishReplay 触发 upsert。
	detail, err := s.detail(ctx, v, true)
	if err != nil {
		return nil, 0, err
	}
	return detail, v.ID, nil
}

// PublishReplayOwner 校验稿件归属并返回（供 live 侧发布/删除回放前做归属判定）。
func (s *Service) PublishReplayOwner(videoID, uid int64) (*Video, error) {
	v, err := s.repo.FindVideoByID(videoID)
	if err != nil {
		return nil, err
	}
	if v == nil || v.Status == StatusDeleted {
		return nil, errcode.ErrNotFound
	}
	if v.UserID != uid {
		return nil, errcode.ErrForbidden
	}
	return v, nil
}

// PublishDraft 把**草稿**稿件置为已发布（M4-LIV-14：主播发布回放）。
//
// 只接受 StatusDraft → StatusPublished，避免它被当成通用的状态改写口：
// 待审/驳回稿件的发布必须走审核链（Review），否则等于绕开审核。
func (s *Service) PublishDraft(ctx context.Context, videoID, uid int64) error {
	v, err := s.PublishReplayOwner(videoID, uid)
	if err != nil {
		return err
	}
	if v.Status != StatusDraft {
		return errcode.ErrInvalidParams.WithMsg("稿件不是草稿，无法发布")
	}
	now := time.Now()
	if err := s.repo.UpdateVideoFields(v.ID, map[string]any{
		"status": StatusPublished, "published_at": now,
	}); err != nil {
		return err
	}
	// 发布旁路（动态生成、+经验、检索 upsert）与投稿发布共用同一条链。
	s.firePublish(ctx, v.ID, uid)
	return nil
}

// DeleteReplay 删除回放稿件（软删，仅作者；live 侧调用前已做归属校验）。
func (s *Service) DeleteReplay(ctx context.Context, videoID, uid int64) error {
	v, err := s.PublishReplayOwner(videoID, uid)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(v); err != nil {
		return err
	}
	s.fireSearchChange(ctx, v.ID, searchindex.ActionDelete)
	return nil
}

// ReplayBrief 回放列表要用的稿件摘要（live 侧拼装「我的回放」用，避免 live 依赖 video 读模型）。
type ReplayBrief struct {
	Bvid   string `json:"bvid"`
	Title  string `json:"title"`
	Status int8   `json:"status"`
}

// ReplayBriefs 按稿件 ID 批量取摘要（含草稿——这是回放列表的核心用例）。
//
// 不能用 CardMapByIDs：那个读模型只取 StatusPublished，而回放列表恰恰**必须**包含草稿，
// 否则主播看不到自己未发布的回放，也就无从发布。
func (s *Service) ReplayBriefs(ids []int64) (map[int64]ReplayBrief, error) {
	out := make(map[int64]ReplayBrief, len(ids))
	for _, id := range ids {
		v, err := s.repo.FindVideoByID(id)
		if err != nil {
			return nil, err
		}
		if v == nil || v.Status == StatusDeleted {
			continue
		}
		out[id] = ReplayBrief{Bvid: v.Bvid, Title: v.Title, Status: v.Status}
	}
	return out, nil
}
