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
  description: 'DliDli 视频社区项目文档：产品需求、技术架构、开发进度管理',
  lastUpdated: true,

  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: '产品文档', link: '/product/overview' },
      { text: '技术架构', link: '/architecture/overview' },
      { text: '项目管理', link: '/project/roadmap' }
    ],

    sidebar: {
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
          text: '功能需求（PRD）',
          items: [
            { text: '用户账号体系', link: '/product/prd/account' },
            { text: '视频投稿与播放', link: '/product/prd/video' },
            { text: '弹幕系统', link: '/product/prd/danmaku' },
            { text: '互动体系（点赞/投币/收藏/评论）', link: '/product/prd/interaction' },
            { text: '搜索与推荐', link: '/product/prd/search-recommend' },
            { text: '社区动态与关注', link: '/product/prd/community' },
            { text: '消息通知', link: '/product/prd/notification' },
            { text: '创作者中心', link: '/product/prd/creator' },
            { text: '直播（远期）', link: '/product/prd/live' },
            { text: '会员与商业化', link: '/product/prd/monetization' },
            { text: '内容审核与管理后台', link: '/product/prd/admin' }
          ]
        },
        {
          text: '非功能需求',
          items: [
            { text: '性能 / 安全 / 合规', link: '/product/nfr' }
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
            { text: '开发清单（Checklist）', link: '/project/checklist' },
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
