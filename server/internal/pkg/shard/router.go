// Package shard 提供互动明细逻辑分表的稳定路由基础（M3-ENG-03）。
// 当前仅负责纯函数路由与安全物理表名生成，不改变现有单表读写。
package shard

import (
	"fmt"
	"hash/fnv"
)

const (
	// Count 第一阶段固定 16 个分片；改变分片数会改变历史路由，必须走迁移方案。
	Count uint64 = 16

	BaseComment    = "comment"
	BaseDanmaku    = "danmaku"
	BaseUserAction = "user_action"
)

var allowedBases = map[string]struct{}{
	BaseComment:    {},
	BaseDanmaku:    {},
	BaseUserAction: {},
}

// HashString 返回稳定的 FNV-1a 64 位哈希。它不依赖进程随机种子，适合持久化路由。
func HashString(value string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return h.Sum64()
}

// IndexByUint64 根据无符号业务键计算分片编号。
func IndexByUint64(key uint64) uint64 { return key & (Count - 1) }

// IndexByString 根据字符串业务键计算分片编号。
func IndexByString(key string) uint64 { return IndexByUint64(HashString(key)) }

// TableFor 根据白名单基础表名和业务键生成物理分片表名。
// 物理表名不能由调用方直接拼接，避免把用户输入带入 SQL identifier。
func TableFor(base string, key uint64) (string, error) {
	if _, ok := allowedBases[base]; !ok {
		return "", fmt.Errorf("unsupported shard table %q", base)
	}
	return fmt.Sprintf("%s_%02d", base, IndexByUint64(key)), nil
}

// TableForString 是 TableFor 的字符串键版本，适用于 openid 等非数值键。
func TableForString(base, key string) (string, error) {
	if _, ok := allowedBases[base]; !ok {
		return "", fmt.Errorf("unsupported shard table %q", base)
	}
	return fmt.Sprintf("%s_%02d", base, IndexByString(key)), nil
}

// Tables 返回指定基础表的全部物理表名，顺序稳定，可用于跨分片聚合。
func Tables(base string) ([]string, error) {
	if _, ok := allowedBases[base]; !ok {
		return nil, fmt.Errorf("unsupported shard table %q", base)
	}
	tables := make([]string, Count)
	for i := uint64(0); i < Count; i++ {
		tables[i] = fmt.Sprintf("%s_%02d", base, i)
	}
	return tables, nil
}
