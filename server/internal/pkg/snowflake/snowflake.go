// Package snowflake 提供全局唯一 ID 生成（时间有序）。
package snowflake

import (
	sf "github.com/bwmarrin/snowflake"
)

var node *sf.Node

// Init 初始化生成器；nodeID 取值 0-1023，多实例部署时需保证唯一。
func Init(nodeID int64) error {
	n, err := sf.NewNode(nodeID)
	if err != nil {
		return err
	}
	node = n
	return nil
}

// NextID 生成下一个 ID。
func NextID() int64 {
	return node.Generate().Int64()
}
