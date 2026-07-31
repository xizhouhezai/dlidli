// dlidli-migrate 数据库迁移工具。
//
// 用法：
//
//	go run ./cmd/migrate                # 应用全部未执行的迁移
//	go run ./cmd/migrate -down          # 回滚一步
//	go run ./cmd/migrate -dsn "..."     # 指定数据库（默认本地 dev）
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql" // 同时完成 database/sql 的 mysql 驱动注册
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		dir  = flag.String("dir", "scripts/migrations", "迁移文件目录")
		dsn  = flag.String("dsn", "mysql://root:root@tcp(127.0.0.1:3307)/dlidli?multiStatements=true", "数据库 DSN")
		down = flag.Bool("down", false, "回滚一步")
	)
	flag.Parse()

	// 环境变量优先（CI/生产不落盘密码）
	if env := os.Getenv("DLIDLI_MIGRATE_DSN"); env != "" {
		*dsn = env
	}

	if err := ensureDatabase(*dsn); err != nil {
		fmt.Fprintf(os.Stderr, "检查/创建数据库失败: %v\n", err)
		os.Exit(1)
	}

	m, err := migrate.New("file://"+*dir, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化迁移失败: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	if *down {
		err = m.Steps(-1)
	} else {
		err = m.Up()
	}

	switch {
	case err == nil:
		fmt.Println("迁移完成")
	case errors.Is(err, migrate.ErrNoChange):
		fmt.Println("无需变更")
	default:
		fmt.Fprintf(os.Stderr, "迁移失败: %v\n", err)
		os.Exit(1)
	}
}

// ensureDatabase 若目标库不存在则自动创建（开发环境便利，生产库应预先建好）。
func ensureDatabase(dsn string) error {
	raw := strings.TrimPrefix(dsn, "mysql://")
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[:i]
	}
	slash := strings.LastIndex(raw, "/")
	if slash < 0 || slash == len(raw)-1 {
		return nil // 未指定库名，交给 migrate 自行报错
	}
	dbName := raw[slash+1:]

	db, err := sql.Open("mysql", raw[:slash+1])
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	return err
}
