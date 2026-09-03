# DliDli 视频社区

一个类哔哩哔哩的视频社区项目（Monorepo）。后端使用 Go，前端覆盖 Web / H5 / 小程序，后续扩展移动端 App。

## 仓库结构

```
dlidli/
├── docs/          # VitePress 文档系统（产品需求、技术架构、进度管理）
├── apps/          # 前端应用
│   ├── web/           # Web 端（PC 浏览器）
│   ├── admin/         # 管理后台（独立应用：审核工作台等内部系统）
│   ├── h5/            # H5 移动端网页
│   └── miniprogram/   # 微信小程序
├── packages/      # 前端共享包（API SDK、UI 组件、工具库）
├── server/        # Go 后端服务
└── pnpm-workspace.yaml
```

## 快速开始

```bash
# 安装依赖
pnpm install

# 一键启动所有服务（自动迁移 + 后端 API :8000 + Web :5173 + Admin :5175 + H5 :5176）
pnpm dev:all
# 附加开关：--docs 加文档站 | --skip-migrate 跳过迁移 | --no-h5 不启 H5 | --check-only 仅检查依赖
# 依赖：本机 MySQL(3307)/Redis(6379) 或 docker compose -f server/deploy/docker-compose.yaml up -d mysql redis

# 或按需分别启动
pnpm web:dev    # Web 前端
pnpm admin:dev  # 管理后台
pnpm h5:dev     # H5 移动端
pnpm docs:dev   # 文档站（产品需求 / 架构 / 进度都在这里）

# 单独启动后端（server 目录）
cd server && go run ./cmd/migrate && go run ./cmd/api
```

## 文档导航

- 产品文档：`docs/product/`（产品概述、功能需求、非功能需求）
- 技术架构：`docs/architecture/`（后端架构、前端架构、数据模型）
- 项目管理：`docs/project/`（路线图、开发进度、开发清单）

## 技术选型（概要）

| 层 | 技术 |
| --- | --- |
| 后端 | Go（Gin）、MySQL、Redis、Kafka、Elasticsearch、FFmpeg、对象存储 |
| Web / H5 | Vue 3 + Vite + TypeScript |
| 小程序 | uni-app（与 H5 共享代码） |
| 文档 | VitePress |
| 包管理 | pnpm workspace |
