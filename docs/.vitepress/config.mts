import { defineConfig } from 'vitepress'
import { defineTeekConfig } from 'vitepress-theme-teek/config'

// Teek 主题配置：保留 VitePress 默认 hero 首页，启用文档页美化与主题增强
const teekConfig = defineTeekConfig({
  teekHome: false, // 不用博客风首页
  vpHome: true, // 保留现有 hero + features 首页
  author: { name: 'DliDli Team' },
  // 文章页增强
  breadcrumb: { enabled: true, showCurrentName: true },
  codeBlock: { collapseHeight: 700 },
  articleUpdate: { enabled: true, limit: 5 },
  // 右下角主题增强面板（布局/主题色/聚光灯）
  themeEnhance: {
    layoutSwitch: { defaultMode: 'original' },
    themeColor: { defaultColorName: 'vp-default', append: [] },
  },
  vitePlugins: {
    sidebar: false, // 保留手写 sidebar，不自动生成
    permalink: false,
    mdH1: false, // 不自动插入 H1，文档已自带标题
    docAnalysis: false,
  },
})

export default defineConfig({
  extends: teekConfig,
  lang: 'zh-CN',
  title: 'DliDli 视频社区',
  description: 'DliDli 视频社区项目文档：功能规格（SDD）、技术架构、开发进度管理',
  lastUpdated: true,

  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: '功能规格（SDD）', link: '/specs/' },
      { text: '产品文档', link: '/product/overview' },
      { text: '技术架构', link: '/architecture/overview' },
      { text: '项目管理', link: '/project/roadmap' }
    ],

    sidebar: {
      '/specs/': [
        {
          text: '规范',
          items: [
            { text: 'SDD 总纲', link: '/specs/' },
            { text: '模板：spec（需求）', link: '/specs/templates/spec' },
            { text: '模板：plan（方案）', link: '/specs/templates/plan' },
            { text: '模板：tasks（任务）', link: '/specs/templates/tasks' }
          ]
        },
        {
          text: '账号体系',
          items: [
            { text: 'spec 需求', link: '/specs/account/spec' },
            { text: 'plan 方案', link: '/specs/account/plan' },
            { text: 'tasks 任务', link: '/specs/account/tasks' }
          ]
        },
        {
          text: '视频投稿与播放',
          items: [
            { text: 'spec 需求', link: '/specs/video/spec' },
            { text: 'plan 方案', link: '/specs/video/plan' },
            { text: 'tasks 任务', link: '/specs/video/tasks' }
          ]
        },
        {
          text: '弹幕系统',
          items: [
            { text: 'spec 需求', link: '/specs/danmaku/spec' },
            { text: 'plan 方案', link: '/specs/danmaku/plan' },
            { text: 'tasks 任务', link: '/specs/danmaku/tasks' }
          ]
        },
        {
          text: '互动体系',
          items: [
            { text: 'spec 需求', link: '/specs/interaction/spec' },
            { text: 'plan 方案', link: '/specs/interaction/plan' },
            { text: 'tasks 任务', link: '/specs/interaction/tasks' }
          ]
        },
        {
          text: '社区动态与关注',
          items: [
            { text: 'spec 需求', link: '/specs/community/spec' },
            { text: 'plan 方案', link: '/specs/community/plan' },
            { text: 'tasks 任务', link: '/specs/community/tasks' }
          ]
        },
        {
          text: '消息通知与私信',
          items: [
            { text: 'spec 需求', link: '/specs/notification/spec' },
            { text: 'plan 方案', link: '/specs/notification/plan' },
            { text: 'tasks 任务', link: '/specs/notification/tasks' }
          ]
        },
        {
          text: '搜索与推荐',
          items: [
            { text: 'spec 需求', link: '/specs/search-recommend/spec' },
            { text: 'plan 方案', link: '/specs/search-recommend/plan' },
            { text: 'tasks 任务', link: '/specs/search-recommend/tasks' }
          ]
        },
        {
          text: '创作者中心',
          items: [
            { text: 'spec 需求', link: '/specs/creator/spec' },
            { text: 'plan 方案', link: '/specs/creator/plan' },
            { text: 'tasks 任务', link: '/specs/creator/tasks' }
          ]
        },
        {
          text: '内容审核与管理后台',
          items: [
            { text: 'spec 需求', link: '/specs/admin/spec' },
            { text: 'plan 方案', link: '/specs/admin/plan' },
            { text: 'tasks 任务', link: '/specs/admin/tasks' }
          ]
        },
        {
          text: '会员与商业化',
          items: [
            { text: 'spec 需求', link: '/specs/monetization/spec' },
            { text: 'plan 方案', link: '/specs/monetization/plan' },
            { text: 'tasks 任务', link: '/specs/monetization/tasks' }
          ]
        },
        {
          text: '直播（V3 预研）',
          items: [
            { text: 'spec 需求', link: '/specs/live/spec' },
            { text: 'plan 方案', link: '/specs/live/plan' },
            { text: 'tasks 任务', link: '/specs/live/tasks' }
          ]
        },
        {
          text: '工程与端侧（横切）',
          items: [
            { text: 'tasks 任务', link: '/specs/engineering/tasks' }
          ]
        }
      ],

      '/product/': [
        {
          text: '产品总览',
          items: [
            { text: '产品概述', link: '/product/overview' },
            { text: '用户画像与场景', link: '/product/personas' },
            { text: '版本规划（MVP → V3）', link: '/product/versions' }
          ]
        },
        {
          text: '非功能需求',
          items: [
            { text: '性能 / 安全 / 合规', link: '/product/nfr' }
          ]
        },
        {
          text: '功能规格（SDD）',
          items: [
            { text: '前往 specs 规格库', link: '/specs/' }
          ]
        }
      ],

      '/architecture/': [
        {
          text: '技术架构',
          items: [
            { text: '总体架构', link: '/architecture/overview' },
            { text: '后端架构（Go）', link: '/architecture/backend' },
            { text: '前端架构（Web/H5/小程序）', link: '/architecture/frontend' },
            { text: '数据模型设计', link: '/architecture/data-model' },
            { text: '视频处理流水线', link: '/architecture/video-pipeline' }
          ]
        }
      ],

      '/project/': [
        {
          text: '项目管理',
          items: [
            { text: '路线图（Roadmap）', link: '/project/roadmap' },
            { text: '开发进度管理', link: '/project/progress' },
            { text: '协作规范', link: '/project/conventions' },
            { text: '部署与运行指南', link: '/project/deployment' }
          ]
        }
      ]
    },

    outline: { level: [2, 3], label: '本页目录' },
    docFooter: { prev: '上一页', next: '下一页' },
    lastUpdatedText: '最后更新',
    search: { provider: 'local' },
    footer: {
      message: 'DliDli 视频社区项目文档',
      copyright: '© 2026 DliDli Team'
    }
  }
})
