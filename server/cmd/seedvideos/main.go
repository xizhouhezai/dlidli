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
	"strings"
	"time"
)

const base = "http://127.0.0.1:8000/api/v1"

// Source 视频源。
type Source struct {
	URL    string
	Title  string
	Desc   string
	Tags   []string
	CatID  int
	Target string // 目标文件名
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
		Source{"https://www.learningcontainer.com/wp-content/uploads/2020/05/sample-mp4-file.mp4",
			"通用视频样本", "learningcontainer 示例（种子数据）", []string{"样本"}, 11, "sample-mp4-file.mp4"},
		Source{"https://www.w3schools.com/html/mov_bbb.mp4",
			"Big Buck Bunny 示例短片", "HTML5 播放示例 BBB 片段（种子数据）", []string{"动画"}, 1, "mov_bbb.mp4"},
	)
	return list
}

func main() {
	n := flag.Int("n", 0, "爬取视频数量（默认全部 50）")
	phone := flag.String("phone", "13900000116", "种子用户手机号（自动注册）")
	flag.Parse()

	sources := buildSources()
	fmt.Printf("源列表共 %d 个视频\n", len(sources))
	token, uid := login(*phone)
	fmt.Printf("种子用户 uid=%s 登录成功\n", uid)
	adminToken := adminLogin()

	count := *n
	if count <= 0 || count > len(sources) {
		count = len(sources)
	}
	ok, fail := 0, 0
	for i, src := range sources[:count] {
		fmt.Printf("\n[%d/%d] %s（%s）\n", i+1, count, src.Title, src.Target)
		bvid, err := seedOne(token, adminToken, src)
		if err != nil {
			fmt.Printf("  ✗ 失败：%v\n", err)
			fail++
			continue
		}
		fmt.Printf("  ✓ 已发布：/video/%s\n", bvid)
		ok++
	}
	fmt.Printf("\n完成：成功 %d，失败 %d\n", ok, fail)
}

// seedOne 单个视频完整链路：下载 → 上传 → 投稿 → 转码 → 审核发布。
func seedOne(token, adminToken string, src Source) (string, error) {
	// 1. 下载
	tmp, err := os.CreateTemp("", "seed_*.mp4")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := download(src.URL, tmp); err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	size := mustSize(tmp.Name())
	fmt.Printf("  下载完成 %d KB\n", size/1024)

	// 2. 上传
	fileID, err := upload(tmp.Name(), token)
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
	if err := waitTranscode(adminToken, bvid, 180*time.Second); err != nil {
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
		Code int `json:"code"`
		Data struct {
			Fast       bool   `json:"fast"`
			FileID     string `json:"file_id"`
			UploadID   string `json:"upload_id"`
			ChunkSize  int    `json:"chunk_size"`
			ChunkCount int    `json:"chunk_count"`
		}
	}
	if err := doJSON("POST", "/upload/init", token, initBody, &initResp); err != nil {
		return "", err
	}
	if initResp.Data.Fast {
		return initResp.Data.FileID, nil
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
	body, _ := json.Marshal(map[string]any{
		"file_id": fileID, "title": src.Title, "description": src.Desc,
		"category_id": src.CatID, "tags": src.Tags, "copyright": 2, // 转载（开源素材）
	})
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Bvid   string `json:"bvid"`
			Status int8   `json:"status"`
		}
	}
	if err := doJSON("POST", "/videos", token, body, &resp); err != nil {
		return "", err
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
	sms, _ := json.Marshal(map[string]string{"phone": phone})
	var smsResp struct {
		Code int `json:"code"`
		Data struct {
			DebugCode string `json:"debug_code"`
		}
	}
	if err := doJSON("POST", "/auth/sms-code", "", sms, &smsResp); err != nil {
		log.Fatal("sms-code 失败: ", err)
	}
	if smsResp.Code != 0 {
		log.Fatal("sms-code 返回错误: ", smsResp.Code)
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

var _ = strings.TrimSpace // 保留 strings 引用（后续扩展源列表解析用）
