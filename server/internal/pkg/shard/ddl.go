package shard

import (
	"fmt"
	"strings"
)

// ShadowDDL 生成指定互动表的影子分片建表语句。
// 语句只使用内部白名单表名，执行前仍需在迁移窗口审阅与备份；本函数不连接数据库。
func ShadowDDL(base string) ([]string, error) {
	if _, ok := allowedBases[base]; !ok {
		return nil, fmt.Errorf("unsupported shard table %q", base)
	}
	statements := make([]string, 0, Count)
	for i := uint64(0); i < Count; i++ {
		name := fmt.Sprintf("%s_%02d", base, i)
		statements = append(statements,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` LIKE `%s`;", name, base))
	}
	return statements, nil
}

// ShadowDDLAll 按固定顺序生成三类互动表的影子 DDL。
func ShadowDDLAll() (string, error) {
	var all []string
	for _, base := range []string{BaseUserAction, BaseComment, BaseDanmaku} {
		statements, err := ShadowDDL(base)
		if err != nil {
			return "", err
		}
		all = append(all, statements...)
	}
	return strings.Join(all, "\n") + "\n", nil
}
