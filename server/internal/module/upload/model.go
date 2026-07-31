// Package upload 上传域：分片上传、断点续传、秒传。
//
// 本地开发（无 Docker/MinIO）：分片直传业务服务器，合并后落本地磁盘 Storage；
// 生产切 MinIO 后 init 返回预签名 URL、客户端直传对象存储，接口协议保持兼容。
package upload

import "time"

// UploadFile 对应 upload_file 表（已合并完成的文件）。
type UploadFile struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64
	FileName  string
	FileHash  string
	FileSize  int64
	StoreKey  string
	CreatedAt time.Time
}

func (UploadFile) TableName() string { return "upload_file" }

// InitReq 初始化上传请求。
type InitReq struct {
	FileName string `json:"file_name" binding:"required"`
	FileSize int64  `json:"file_size" binding:"required,gt=0"`
	FileHash string `json:"file_hash" binding:"required,len=64"` // SHA-256 hex
}

// InitResp 初始化上传响应。
type InitResp struct {
	// Fast 为 true 表示秒传命中，FileID/StoreKey 直接可用
	Fast     bool   `json:"fast"`
	FileID   string `json:"file_id,omitempty"`
	StoreKey string `json:"store_key,omitempty"`

	UploadID   string `json:"upload_id,omitempty"`
	ChunkSize  int64  `json:"chunk_size,omitempty"`
	ChunkCount int    `json:"chunk_count,omitempty"`
	// Uploaded 已上传分片下标（断点续传）
	Uploaded []int `json:"uploaded,omitempty"`
}

// CompleteResp 合并完成响应。
type CompleteResp struct {
	FileID   string `json:"file_id"`
	StoreKey string `json:"store_key"`
}
