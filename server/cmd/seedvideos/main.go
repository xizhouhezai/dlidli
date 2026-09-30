// cmd/seedvideos: 测试视频种子脚本——从公开源爬取视频并走完整投稿链路入库。
// 流程：下载 → 分片上传 → 投稿 → 等转码 → 审核发布。
// 用法：go run ./cmd/seedvideos [-n 视频数] [-phone 种子用户手机号]
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const base = "http://127.0.0.1:8000/api/v1"

// Source 视频源。
type Source struct {
	URL       string
	Title     string
	Desc      string
	Tags      []string
	CatID     int
	Target    string // 目标文件名
	LocalPath string // 本地素材绝对路径；非空则跳过网络下载
	License   string // 素材许可（写入稿件简介，便于追溯）
	Credit    string // 出处/作者
	Hash      string // 文件 SHA-256（本地素材预计算，用于幂等跳过）
}

// filterExisting 跳过「已生成过稿件」的素材，保证脚本可重复运行而不产生重复数据。
//
// 判据是 video_stream.play_path（原画流）是否已指向该文件的内容寻址 key
// （store_key 形如 videos/source/<sha256><ext>），而不是 upload_file.file_hash：
//   - 0033 迁移后 upload_file 唯一键为 (user_id, file_hash)，同一文件**允许多个用户各登记一条**，
//     故「哈希已存在」不再等价于「已入库」，用它做幂等判据会误跳过（新用户其实可以正常投稿）。
//   - 稿件一旦生成，其原画流 key 必然含该哈希，是最贴近「这份素材已经用过了」的信号。
func filterExisting(sources []Source, dsn string) (kept []Source, skipped []string, err error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	for _, s := range sources {
		if s.Hash == "" && s.LocalPath != "" {
			s.Hash = sha256File(s.LocalPath)
		}
		var n int64
		if err := db.Table("video_stream").
			Where("play_path LIKE ?", "videos/source/"+s.Hash+"%").
			Count(&n).Error; err != nil {
			return nil, nil, err
		}
		if n > 0 {
			skipped = append(skipped, s.Target)
			continue
		}
		kept = append(kept, s)
	}
	return kept, skipped, nil
}

// buildLocalSources 从本地素材目录构建源列表（配合 .dev-logs/seed-assets 的下载产物）。
// 清单文件 manifest.json 由下载器产出，含许可与出处；无清单时按文件名兜底。
func buildLocalSources(dir string) ([]Source, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取素材目录失败: %w", err)
	}
	// 可选：读 manifest 补充许可与出处
	type mEntry struct {
		Out     string  `json:"out"`
		Title   string  `json:"title"`
		License string  `json:"license"`
		Credit  string  `json:"credit"`
		Width   int     `json:"width"`
		Height  int     `json:"height"`
		Duration float64 `json:"duration"`
		Error   string  `json:"error"`
	}
	meta := map[string]mEntry{}
	if b, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
		var list []mEntry
		if json.Unmarshal(b, &list) == nil {
			for _, m := range list {
				if m.Error == "" {
					meta[m.Out] = m
				}
			}
		}
	}

	var list []Source
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		// 后端投稿白名单：mp4/mov/mkv/flv/avi
		switch ext {
		case ".mp4", ".mov", ".mkv", ".flv", ".avi":
		default:
			continue
		}
		m := meta[name]
		title := m.Title
		if title == "" {
			title = strings.TrimSuffix(name, ext)
		}
		// 按分辨率归类到合适分区（对齐 Web 端分区语义，避免全挤在一个分区）
		cat := catForVideo(m.Width, m.Height)
		desc := "测试数据（开源素材）"
		if m.License != "" {
			desc = fmt.Sprintf("测试数据｜素材许可：%s｜出处：%s", m.License, m.Credit)
		}
		list = append(list, Source{
			Title:     title,
			Desc:      desc,
			Tags:      tagsForVideo(m.Width, m.Height, m.Duration),
			CatID:     cat,
			Target:    name,
			LocalPath: filepath.Join(dir, name),
			License:   m.License,
			Credit:    m.Credit,
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Target < list[j].Target })
	return list, nil
}

// catForVideo 按画幅把素材分到不同分区，让种子数据的首页分布更自然。
func catForVideo(w, h int) int {
	switch {
	case w == 0 || h == 0:
		return 11 // 时尚（未知）
	case h > w:
		return 8 // 舞蹈（竖屏）
	case w >= 3000:
		return 3 // 科技数码（4K）
	case w >= 1800:
		return 1 // 动画（1080p）
	case w >= 1200:
		return 4 // 知识（720p）
	case w >= 600:
		return 5 // 生活
	default:
		return 9 // 影视（低分辨率测试序列）
	}
}

// tagsForVideo 生成与画幅/时长相关的标签，便于按标签检索测试。
func tagsForVideo(w, h int, dur float64) []string {
	tags := []string{"测试数据", "开源素材"}
	if w > 0 && h > 0 {
		tags = append(tags, fmt.Sprintf("%dp", h))
	}
	if h > w {
		tags = append(tags, "竖屏")
	}
	if dur > 60 {
		tags = append(tags, "长视频")
	}
	return tags
}


// buildSources 程序化生成 50 个测试视频源（test-videos.co.uk 组合 + filesamples 多格式 + 其他）。
// 所有 URL 均已验证可达（2026-08-05）；下载失败自动跳过不影响其余。
func buildSources() []Source {
	var list []Source
	// test-videos.co.uk：Big Buck Bunny / Sintel / Jellyfish × 360/720/1080 × 多档体积
	movies := []struct {
		Name  string // 目录名
		Title string
		Tags  []string
		Cat   int
	}{
		{"bigbuckbunny", "Big Buck Bunny 动画片段", []string{"动画", "开源电影"}, 1},
		{"sintel", "Sintel 奇幻短片", []string{"动画", "奇幻"}, 1},
		{"jellyfish", "水母生态演示", []string{"自然", "演示"}, 9},
	}
	resSizes := map[string][]string{
		"360":  {"1MB", "5MB", "10MB"},
		"720":  {"1MB", "5MB", "10MB", "30MB"},
		"1080": {"1MB", "5MB", "10MB"},
	}
	title := map[string]string{
		"bigbuckbunny": "Big_Buck_Bunny", "sintel": "Sintel", "jellyfish": "Jellyfish",
	}
	for _, m := range movies {
		for res, sizes := range resSizes {
			for _, s := range sizes {
				// 30MB 仅 720 存在（已探测），跳过其余 30MB 组合
				if s == "30MB" && res != "720" {
					continue
				}
				file := fmt.Sprintf("%s_%s_10s_%s.mp4", title[m.Name], res, s)
				list = append(list, Source{
					URL:   fmt.Sprintf("https://test-videos.co.uk/vids/%s/mp4/h264/%s/%s", m.Name, res, file),
					Title: fmt.Sprintf("%s（%sp）", m.Title, res),
					Desc:  "开源测试视频（种子数据）",
					Tags:  m.Tags, CatID: m.Cat, Target: file,
				})
			}
		}
	}
	// filesamples.com：多格式样本
	fs := []struct {
		File string
		Ext  string
	}{
		{"sample_640x360.mp4", "mp4"}, {"sample_960x540.mp4", "mp4"},
		{"sample_1280x720.mp4", "mp4"}, {"sample_1920x1080.mp4", "mp4"},
		{"sample_640x360.mkv", "mkv"}, {"sample_960x540.mkv", "mkv"}, {"sample_1280x720.mkv", "mkv"}, {"sample_1920x1080.mkv", "mkv"},
		{"sample_640x360.mov", "mov"}, {"sample_960x540.mov", "mov"}, {"sample_1280x720.mov", "mov"},
		{"sample_640x360.avi", "avi"}, {"sample_1280x720.avi", "avi"}, {"sample_1920x1080.avi", "avi"},
		{"sample_640x360.flv", "flv"}, {"sample_1280x720.flv", "flv"}, {"sample_1920x1080.flv", "flv"},
		{"sample_1920x1080.mov", "mov"},
	}
	for _, f := range fs {
		list = append(list, Source{
			URL:   "https://filesamples.com/samples/video/" + f.Ext + "/" + f.File,
			Title: "格式样本 " + f.File,
			Desc:  "多格式测试视频样本（种子数据）",
			Tags:  []string{"样本", strings.ToUpper(f.Ext)}, CatID: 11, Target: f.File,
		})
	}
	// 其他固定源
	list = append(list,
		Source{URL: "https://www.learningcontainer.com/wp-content/uploads/2020/05/sample-mp4-file.mp4",
			Title: "通用视频样本", Desc: "learningcontainer 示例（种子数据）", Tags: []string{"样本"}, CatID: 11, Target: "sample-mp4-file.mp4"},
		Source{URL: "https://www.w3schools.com/html/mov_bbb.mp4",
			Title: "Big Buck Bunny 示例短片", Desc: "HTML5 播放示例 BBB 片段（种子数据）", Tags: []string{"动画"}, CatID: 1, Target: "mov_bbb.mp4"},
	)
	return list
}

func main() {
	n := flag.Int("n", 0, "爬取视频数量（默认全部 50）")
	phone := flag.String("phone", "13900000116", "种子用户手机号（自动注册）")
	localDir := flag.String("local", "", "本地素材目录（如 .dev-logs/seed-assets/mp4）；指定后跳过下载")
	users := flag.Int("users", 1, "把素材均摊到 N 个测试用户（每个用户独立注册并登录）")
	phoneBase := flag.String("phone-base", "139000001", "多用户手机号前缀（后 2 位为序号，如 13900000101）")
	dryRun := flag.Bool("dry-run", false, "只打印分配计划，不上传")
	dsn := flag.String("dsn", "", "数据库 DSN（用于幂等跳过已入库素材，如 root:dlidli123@tcp(127.0.0.1:3306)/dlidli?charset=utf8mb4&parseTime=True&loc=Local）")
	flag.Parse()

	// 1. 构建源列表
	var sources []Source
	if *localDir != "" {
		var err error
		sources, err = buildLocalSources(*localDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("本地素材目录 %s：%d 个可用视频\n", *localDir, len(sources))
	} else {
		sources = buildSources()
		fmt.Printf("源列表共 %d 个视频\n", len(sources))
	}
	if len(sources) == 0 {
		log.Fatal("没有可用素材")
	}
	count := *n
	if count <= 0 || count > len(sources) {
		count = len(sources)
	}
	sources = sources[:count]

	// 1.5 幂等过滤：跳过 file_hash 已入库的素材（重复投稿同一文件会因 VID-24 归属校验失败）
	if *dsn != "" {
		kept, skipped, err := filterExisting(sources, *dsn)
		if err != nil {
			log.Fatalf("幂等过滤失败: %v", err)
		}
		if len(skipped) > 0 {
			fmt.Printf("跳过 %d 个已入库素材（哈希已存在）：%s\n", len(skipped), strings.Join(skipped, ", "))
		}
		sources = kept
		if len(sources) == 0 {
			fmt.Println("所有素材均已入库，无需操作")
			return
		}
	}

	// 2. 准备 N 个测试用户
	type account struct {
		phone string
		token string
		uid   string
	}
	accs := make([]account, 0, *users)
	for i := 0; i < *users; i++ {
		p := *phone
		if *users > 1 {
			p = fmt.Sprintf("%s%02d", *phoneBase, i+1)
		}
		token, uid := login(p)
		fmt.Printf("用户[%d] phone=%s uid=%s 登录成功\n", i+1, p, uid)
		accs = append(accs, account{phone: p, token: token, uid: uid})
	}

	// 3. 轮转分配：第 i 条素材分给第 i%users 个用户（均摊）
	fmt.Printf("\n分配计划：%d 个视频 → %d 个用户（每人约 %d 条）\n", len(sources), len(accs), len(sources)/len(accs))
	for i, s := range sources {
		if i < 10 || i >= len(sources)-3 {
			fmt.Printf("  [%2d] %-42s → 用户[%d]\n", i+1, truncate(s.Title, 42), i%len(accs)+1)
		} else if i == 10 {
			fmt.Printf("  ...\n")
		}
	}
	if *dryRun {
		fmt.Println("\n[--dry-run] 仅打印计划，未执行上传")
		return
	}

	adminToken := adminLogin()

	// 4. 逐条走完整链路
	ok, fail := 0, 0
	perUser := make([]int, len(accs))
	for i, src := range sources {
		acc := accs[i%len(accs)]
		fmt.Printf("\n[%d/%d] %s（%s）→ 用户[%d]\n", i+1, len(sources), src.Title, src.Target, i%len(accs)+1)
		bvid, err := seedOne(acc.token, adminToken, src)
		if err != nil {
			fmt.Printf("  ✗ 失败：%v\n", err)
			fail++
			continue
		}
		fmt.Printf("  ✓ 已发布：/video/%s\n", bvid)
		perUser[i%len(accs)]++
		ok++
	}

	fmt.Printf("\n完成：成功 %d，失败 %d\n", ok, fail)
	for i, c := range perUser {
		fmt.Printf("  用户[%d] %s：%d 条\n", i+1, accs[i].phone, c)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// seedOne 单个视频完整链路：下载（或用本地素材）→ 上传 → 投稿 → 转码 → 审核发布。
func seedOne(token, adminToken string, src Source) (string, error) {
	// 1. 取素材：本地优先，否则下载到临时文件
	var path string
	if src.LocalPath != "" {
		path = src.LocalPath
		size := mustSize(path)
		fmt.Printf("  使用本地素材 %d KB\n", size/1024)
	} else {
		tmp, err := os.CreateTemp("", "seed_*.mp4")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		if err := download(src.URL, tmp); err != nil {
			return "", fmt.Errorf("下载失败: %w", err)
		}
		path = tmp.Name()
		size := mustSize(path)
		fmt.Printf("  下载完成 %d KB\n", size/1024)
	}

	// 2. 上传
	fileID, err := upload(path, token)
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	// 3. 投稿
	bvid, err := submit(token, fileID, src)
	if err != nil {
		return "", fmt.Errorf("投稿失败: %w", err)
	}
	fmt.Printf("  投稿成功 bvid=%s\n", bvid)

	// 4. 等转码完成（转码中列表中不再含该 bvid 即完成；公开详情仅返回已发布不可用）
	if err := waitTranscode(adminToken, bvid, 600*time.Second); err != nil {
		return "", err
	}
	// 5. 审核通过（dev autoApprove=false）
	if err := approve(adminToken, bvid); err != nil {
		return "", fmt.Errorf("审核失败: %w", err)
	}
	return bvid, nil
}

func download(url string, dst *os.File) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (dlidli seed script)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

func upload(path, token string) (string, error) {
	hash := sha256File(path)
	fi := mustSize(path)
	initBody, _ := json.Marshal(map[string]any{
		"file_name": filepath.Base(path), "file_size": fi, "file_hash": hash,
	})
	var initResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Fast       bool   `json:"fast"`
			FileID     string `json:"file_id"`
			UploadID   string `json:"upload_id"`
			ChunkSize  int    `json:"chunk_size"`
			ChunkCount int    `json:"chunk_count"`
		} `json:"data"`
	}
	if err := doJSON("POST", "/upload/init", token, initBody, &initResp); err != nil {
		return "", err
	}
	// 必须校验业务码，否则失败时会拿空 upload_id 继续跑，错误被推迟到投稿阶段才暴露
	if initResp.Code != 0 {
		return "", fmt.Errorf("upload/init 返回错误 %d: %s", initResp.Code, initResp.Message)
	}
	if initResp.Data.Fast {
		return initResp.Data.FileID, nil
	}
	if initResp.Data.UploadID == "" || initResp.Data.ChunkCount == 0 {
		return "", fmt.Errorf("upload/init 未返回有效会话（upload_id=%q chunk_count=%d）", initResp.Data.UploadID, initResp.Data.ChunkCount)
	}
	data, _ := os.ReadFile(path)
	for i := 0; i < initResp.Data.ChunkCount; i++ {
		start := i * initResp.Data.ChunkSize
		end := start + initResp.Data.ChunkSize
		if end > len(data) {
			end = len(data)
		}
		req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/upload/%s/parts/%d", base, initResp.Data.UploadID, i), bytes.NewReader(data[start:end]))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
	}
	var comp struct {
		Code int `json:"code"`
		Data struct {
			FileID string `json:"file_id"`
		}
	}
	if err := doJSON("POST", "/upload/"+initResp.Data.UploadID+"/complete", token, nil, &comp); err != nil {
		return "", err
	}
	return comp.Data.FileID, nil
}

func submit(token, fileID string, src Source) (string, error) {
	if fileID == "" {
		return "", fmt.Errorf("file_id 为空，上传未成功")
	}
	body, _ := json.Marshal(map[string]any{
		"file_id": fileID, "title": src.Title, "description": src.Desc,
		"category_id": src.CatID, "tags": src.Tags, "copyright": 2, // 转载（开源素材）
	})
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Bvid   string `json:"bvid"`
			Status int8   `json:"status"`
		} `json:"data"`
	}
	if err := doJSON("POST", "/videos", token, body, &resp); err != nil {
		return "", err
	}
	// 必须校验业务码：接口报错时若不检查，会返回空 bvid 且无 error，形成「假成功」
	if resp.Code != 0 {
		return "", fmt.Errorf("投稿接口返回错误 %d: %s", resp.Code, resp.Message)
	}
	if resp.Data.Bvid == "" {
		return "", fmt.Errorf("投稿成功但未返回 bvid")
	}
	return resp.Data.Bvid, nil
}

// waitTranscode 轮询 admin 转码中列表：bvid 不再出现即转码完成（进入待审/发布）。
func waitTranscode(adminToken, bvid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var resp struct {
			Data struct {
				List []struct {
					Bvid string `json:"bvid"`
				} `json:"list"`
			}
		}
		if err := doJSON("GET", "/admin/videos?status=2&page_size=50", adminToken, nil, &resp); err != nil {
			return err
		}
		found := false
		for _, v := range resp.Data.List {
			if v.Bvid == bvid {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("转码等待超时（可能转码失败，稿件停留在转码中）")
}

func approve(adminToken, bvid string) error {
	body, _ := json.Marshal(map[string]any{"approve": true})
	var resp struct {
		Code int `json:"code"`
	}
	return doJSON("POST", "/admin/videos/"+bvid+"/review", adminToken, body, &resp)
}

func login(phone string) (token, uid string) {
	// 短信验证码有 60s 冷却（smsSendCooldown），多用户连续登录与重复运行都会撞上，
	// 故对「发送过于频繁」做退避重试，而不是直接退出。
	var smsResp struct {
		Code int `json:"code"`
		Data struct {
			DebugCode string `json:"debug_code"`
		}
	}
	const maxAttempts = 8
	for attempt := 1; ; attempt++ {
		sms, _ := json.Marshal(map[string]string{"phone": phone})
		smsResp = struct {
			Code int `json:"code"`
			Data struct {
				DebugCode string `json:"debug_code"`
			}
		}{}
		if err := doJSON("POST", "/auth/sms-code", "", sms, &smsResp); err != nil {
			log.Fatal("sms-code 失败: ", err)
		}
		if smsResp.Code == 0 {
			break
		}
		// 20001 = 发送过于频繁
		if smsResp.Code == 20001 && attempt < maxAttempts {
			fmt.Printf("  验证码限流，等待 62s 后重试（第 %d 次）...\n", attempt)
			time.Sleep(62 * time.Second)
			continue
		}
		log.Fatalf("sms-code 返回错误: %d", smsResp.Code)
	}
	loginBody, _ := json.Marshal(map[string]string{"phone": phone, "code": smsResp.Data.DebugCode})
	var lr struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
			User        struct {
				UID string `json:"id"`
			} `json:"user"`
		}
	}
	if err := doJSON("POST", "/auth/login/sms", "", loginBody, &lr); err != nil {
		log.Fatal("登录失败: ", err)
	}
	if lr.Code != 0 {
		log.Fatal("登录返回错误: ", lr.Code)
	}
	return lr.Data.AccessToken, lr.Data.User.UID
}

func adminLogin() string {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	var lr struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		}
	}
	if err := doJSON("POST", "/admin/login", "", body, &lr); err != nil {
		log.Fatal("admin 登录失败: ", err)
	}
	return lr.Data.Token
}

func doJSON(method, path, token string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func sha256File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func mustSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
	return fi.Size()
}
