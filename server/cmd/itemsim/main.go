// cmd/itemsim: ItemCF 相似视频离线计算（M3-REC-03）。
// 输入：user_behavior action=3（有效播放视为"看过"）；输出：video_similar（每稿 top 10 相似，余弦相似度）。
// 用法：go run ./cmd/itemsim
package main

import (
	"fmt"
	"log"
	"math"
	"sort"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const topN = 10      // 每稿保留相似数量
const minScore = 0.1 // 相似度阈值

func main() {
	dsn := "root:root@tcp(127.0.0.1:3307)/dlidli?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败: ", err)
	}

	// 1. 用户 → 看过视频集合（action=3 有效播放）
	type row struct {
		UserID  int64
		VideoID int64
	}
	var rows []row
	if err := db.Raw("SELECT DISTINCT user_id, video_id FROM user_behavior WHERE action = 3").Scan(&rows).Error; err != nil {
		log.Fatal("读取行为日志失败: ", err)
	}
	userVideos := map[int64]map[int64]bool{}
	for _, r := range rows {
		if userVideos[r.UserID] == nil {
			userVideos[r.UserID] = map[int64]bool{}
		}
		userVideos[r.UserID][r.VideoID] = true
	}

	// 2. 视频 → 看过用户集合 + 共现矩阵
	videoUsers := map[int64]map[int64]bool{}
	coCount := map[[2]int64]int{}
	for uid, vids := range userVideos {
		for vid := range vids {
			if videoUsers[vid] == nil {
				videoUsers[vid] = map[int64]bool{}
			}
			videoUsers[vid][uid] = true
		}
		vs := make([]int64, 0, len(vids))
		for vid := range vids {
			vs = append(vs, vid)
		}
		for i := 0; i < len(vs); i++ {
			for j := i + 1; j < len(vs); j++ {
				a, b := vs[i], vs[j]
				if a > b {
					a, b = b, a
				}
				coCount[[2]int64{a, b}]++
			}
		}
	}

	// 3. 余弦相似度 = 共现数 / sqrt(|U_a| * |U_b|)，取每稿 top N
	similar := map[int64][]struct {
		Other int64
		Score float64
	}{}
	for pair, cnt := range coCount {
		ua := float64(len(videoUsers[pair[0]]))
		ub := float64(len(videoUsers[pair[1]]))
		if ua == 0 || ub == 0 {
			continue
		}
		score := float64(cnt) / math.Sqrt(ua*ub)
		if score < minScore {
			continue
		}
		similar[pair[0]] = append(similar[pair[0]], struct {
			Other int64
			Score float64
		}{pair[1], score})
		similar[pair[1]] = append(similar[pair[1]], struct {
			Other int64
			Score float64
		}{pair[0], score})
	}

	// 4. upsert 入库
	type simRow struct {
		VideoID        int64
		SimilarVideoID int64
		Score          float64
	}
	var toSave []simRow
	for vid, list := range similar {
		sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
		if len(list) > topN {
			list = list[:topN]
		}
		for _, it := range list {
			toSave = append(toSave, simRow{vid, it.Other, it.Score})
		}
	}
	if len(toSave) > 0 {
		err = db.Table("video_similar").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "video_id"}, {Name: "similar_video_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"score"}),
		}).Create(&toSave).Error
		if err != nil {
			log.Fatal("写入相似视频失败: ", err)
		}
	}
	fmt.Printf("ItemCF 完成：行为用户 %d，视频 %d，相似关系 %d 条（每稿 top %d，阈值 %v）\n",
		len(userVideos), len(videoUsers), len(toSave), topN, minScore)
}
