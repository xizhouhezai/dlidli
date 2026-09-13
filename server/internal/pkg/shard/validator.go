package shard

import "fmt"

// Assignment 描述一行历史数据在迁移回填中的目标路由。
// SourceTable 必须是逻辑基础表名，Key 是 ADR 规定的业务路由键，TargetTable 是导出工具记录的目标表。
type Assignment struct {
	SourceTable string
	Key         uint64
	TargetTable string
}

// ValidateAssignments 校验回填/双写工具输出的目标分片是否符合当前路由契约。
// 返回每类基础表的行数统计；不连接数据库，也不执行 DDL/DML。
func ValidateAssignments(rows []Assignment) (map[string]int, error) {
	counts := make(map[string]int, len(allowedBases))
	for i, row := range rows {
		if _, ok := allowedBases[row.SourceTable]; !ok {
			return nil, fmt.Errorf("row %d: unsupported source table %q", i, row.SourceTable)
		}
		want, err := TableFor(row.SourceTable, row.Key)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		if row.TargetTable != want {
			return nil, fmt.Errorf("row %d: route mismatch for %s key %d: got %q want %q", i, row.SourceTable, row.Key, row.TargetTable, want)
		}
		counts[row.SourceTable]++
	}
	return counts, nil
}
