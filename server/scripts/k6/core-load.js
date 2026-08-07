// DliDli 核心链路压测（M1-REL-01，PRD 压测需求）
// 场景：首页推荐 → 视频详情 → 弹幕列表 → 搜索（匿名大流量）；登录链路单独小流量（sms 限频保护）
// 运行：.\scripts\k6\k6.exe run scripts/k6/core-load.js（工作目录 server/）
import http from 'k6/http'
import { check, sleep } from 'k6'
import { SharedArray } from 'k6/data'

// ---- 配置 ----
const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8000/api/v1'
// 已发布的种子视频（真实存在，50 视频数据规模中取样）
const BVIDS = new SharedArray('bvids', () => [
  'DV2U14DNjuFQ8', 'DV2U14AhKi2fA', 'DV2U147VXPy7s', 'DV2U145PG6Sm0', 'DV2U143zJI84e',
])

export const options = {
  scenarios: {
    // 匿名核心链路：大流量
    anonymous: {
      executor: 'ramping-vus',
      exec: 'anonymous',
      startVUs: 5,
      stages: [
        { duration: '20s', target: 20 },
        { duration: '30s', target: 20 },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    // 登录链路：小流量（dev sms 限频 60s，单次迭代即可）
    login: {
      executor: 'constant-vus',
      exec: 'login',
      vus: 1,
      duration: '5s',
      startTime: '5s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
}

function pickBvid() {
  return BVIDS[Math.floor(Math.random() * BVIDS.length)]
}

// ---- 匿名核心链路 ----
export function anonymous() {
  // 1. 首页推荐
  const feed = http.get(`${BASE}/recommend/videos?page=1&page_size=10`)
  check(feed, { 'feed 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })

  // 2. 视频详情
  const bvid = pickBvid()
  const detail = http.get(`${BASE}/videos/${bvid}`)
  check(detail, { 'detail 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })

  // 3. 弹幕列表
  const dm = http.get(`${BASE}/videos/${bvid}/danmaku/list?page=1&page_size=50`)
  check(dm, { 'danmaku 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })

  // 4. 搜索
  const search = http.get(`${BASE}/search?keyword=${encodeURIComponent('动画')}&page=1&page_size=20`)
  check(search, { 'search 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })

  sleep(1)
}

// ---- 登录链路（sms-code + login，dev debug_code） ----
export function login() {
  const phone = '13900000116'
  const sms = http.post(`${BASE}/auth/sms-code`, JSON.stringify({ phone }), {
    headers: { 'Content-Type': 'application/json' },
  })
  const code = sms.json('data.debug_code')
  if (!code) {
    // dev sms 限频（60s+ 窗口）：失败时放慢重试，避免重试风暴
    check(sms, { 'sms-code 返回 debug_code': (r) => !!r.json('data.debug_code') })
    sleep(5)
    return
  }
  const lr = http.post(`${BASE}/auth/login/sms`, JSON.stringify({ phone, code }), {
    headers: { 'Content-Type': 'application/json' },
  })
  check(lr, { 'login 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })
  if (lr.json('code') === 0) {
    // 登录态请求：个人资料
    const token = lr.json('data.access_token')
    const me = http.get(`${BASE}/users/me`, { headers: { Authorization: `Bearer ${token}` } })
    check(me, { 'me 200 + code=0': (r) => r.status === 200 && r.json('code') === 0 })
  }
  sleep(2) // 让出限频窗口
}
