<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { formatCount, formatDuration, type User } from '@dlidli/shared'
import { ApiError, type VideoCard, type CollectionItem } from '@dlidli/api-client'
import { api, hasLogin, saveLogin, clearLogin } from '@/api'

const DEFAULT_AVATAR = '/static/default-avatar.png'
const DEFAULT_COVER = '/static/default-cover.svg'

// —— 登录态 ——
const loggedIn = ref(false)
const profile = ref<User | null>(null)
const loginLoading = ref(false)
const loginForm = ref({ phone: '', code: '' })
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

// —— 我的数据 ——
const mineList = ref<VideoCard[]>([])
const collections = ref<CollectionItem[]>([])
const listLoading = ref(false)

async function loginSuccess(pair: { access_token: string; refresh_token: string }) {
  saveLogin(pair.access_token, pair.refresh_token)
  loggedIn.value = true
  loginForm.value = { phone: '', code: '' }
  await loadMine()
}

async function doLogin() {
  if (!/^1\d{10}$/.test(loginForm.value.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  if (!loginForm.value.code) {
    uni.showToast({ title: '请输入验证码', icon: 'none' })
    return
  }
  loginLoading.value = true
  try {
    const pair = await api.auth.loginBySms(loginForm.value.phone, loginForm.value.code)
    await loginSuccess(pair)
  } catch (e) {
    uni.showToast({
      title: e instanceof ApiError ? e.message : '登录失败',
      icon: 'none',
    })
  } finally {
    loginLoading.value = false
  }
}

async function sendCode() {
  if (!/^1\d{10}$/.test(loginForm.value.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  try {
    const res = await api.auth.sendSmsCode(loginForm.value.phone)
    if (res.debug_code) loginForm.value.code = res.debug_code // dev 自动填充
    uni.showToast({ title: '验证码已发送', icon: 'none' })
    countdown.value = 60
    timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0 && timer) clearInterval(timer)
    }, 1000)
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '发送失败', icon: 'none' })
  }
}

function doLogout() {
  clearLogin()
  loggedIn.value = false
  profile.value = null
  mineList.value = []
  collections.value = []
}

async function loadMine() {
  listLoading.value = true
  try {
    const [me, mine, cols] = await Promise.all([
      api.auth.me(),
      api.video.mine(1, 20),
      api.interaction.listCollections().catch(() => [] as CollectionItem[]),
    ])
    profile.value = me
    mineList.value = mine.list
    collections.value = cols
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    listLoading.value = false
  }
}

function goVideo(bvid: string) {
  uni.navigateTo({ url: '/pages/video/video?bvid=' + bvid })
}

function goMessages() {
  uni.navigateTo({ url: '/pages/messages/messages' })
}

function goCollection() {
  uni.navigateTo({ url: '/pages/collection/collection' })
}

function goHistory() {
  uni.navigateTo({ url: '/pages/history/history' })
}

onShow(() => {
  loggedIn.value = hasLogin()
  if (loggedIn.value) loadMine()
})
</script>

<template>
  <view class="profile">
    <!-- 未登录：短信登录面板 -->
    <view v-if="!loggedIn" class="login-panel">
      <view class="login-panel__brand">
        <text class="login-panel__logo">DliDli</text>
        <text class="login-panel__slogan">登录后管理你的投稿与收藏</text>
      </view>
      <view class="login-panel__form">
        <input
          v-model="loginForm.phone"
          class="login-panel__input"
          type="number"
          maxlength="11"
          placeholder="手机号（未注册将自动注册）"
        />
        <view class="login-panel__code">
          <input
            v-model="loginForm.code"
            class="login-panel__input login-panel__input--code"
            type="number"
            maxlength="6"
            placeholder="6 位验证码"
          />
          <button class="login-panel__send" :disabled="countdown > 0" @tap="sendCode">
            {{ countdown > 0 ? countdown + 's' : '获取验证码' }}
          </button>
        </view>
        <button class="login-panel__btn" :loading="loginLoading" @tap="doLogin">登录</button>
      </view>
    </view>

    <!-- 已登录 -->
    <view v-else class="mine">
      <view class="mine__user">
        <image class="mine__avatar" :src="profile?.avatar || DEFAULT_AVATAR" mode="aspectFill" />
        <view class="mine__info">
          <text class="mine__name">{{ profile?.nickname }}</text>
          <text class="mine__meta">Lv{{ profile?.level }} · {{ profile?.coin }} 硬币</text>
        </view>
        <text class="mine__logout" @tap="doLogout">退出</text>
      </view>

      <!-- 快捷入口 -->
      <view class="quick">
        <view class="quick__item" @tap="goMessages">
          <text class="quick__icon i-mingcute-mail-line" />
          <text class="quick__label">我的消息</text>
        </view>
        <view class="quick__item" @tap="goCollection">
          <text class="quick__icon i-mingcute-star-line" />
          <text class="quick__label">我的收藏</text>
        </view>
        <view class="quick__item" @tap="goHistory">
          <text class="quick__icon i-mingcute-history-line" />
          <text class="quick__label">观看历史</text>
        </view>
      </view>

      <!-- 我的投稿 -->
      <view class="section">
        <text class="section__title">我的投稿</text>
        <view v-if="listLoading" class="tip">加载中…</view>
        <view v-else-if="mineList.length === 0" class="tip">还没有投稿</view>
        <view v-for="v in mineList" :key="v.bvid" class="row" @tap="goVideo(v.bvid)">
          <view class="row__cover">
            <image class="row__img" :src="v.cover || DEFAULT_COVER" mode="aspectFill" />
            <text v-if="v.duration > 0" class="row__dur">{{ formatDuration(v.duration) }}</text>
          </view>
          <view class="row__info">
            <text class="row__title">{{ v.title }}</text>
            <text class="row__meta"
              >{{ formatCount(v.stat.view) }}观看 · {{ v.status === 4 ? '已发布' : '未发布' }}</text
            >
          </view>
        </view>
      </view>

      <!-- 我的收藏 -->
      <view class="section">
        <text class="section__title">我的收藏夹</text>
        <view v-if="collections.length === 0" class="tip">还没有收藏夹</view>
        <view v-for="c in collections" :key="c.id" class="row row--static">
          <text class="row__title">{{ c.name }}</text>
          <text class="row__meta">{{ c.is_default === 1 ? '默认' : '' }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.profile {
  min-height: 100vh;
  background: v.$bg;
  padding: 24rpx;
}

.login-panel {
  margin-top: 120rpx;
  background: v.$surface;
  border-radius: 16rpx;
  padding: 48rpx 40rpx;
  box-shadow: 0 4rpx 16rpx rgba(0, 0, 0, 0.06);
}

.login-panel__brand {
  text-align: center;
  margin-bottom: 48rpx;
}

.login-panel__logo {
  display: block;
  font-size: 56rpx;
  font-weight: 700;
  color: v.$primary;
}

.login-panel__slogan {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: v.$text-3;
}

.login-panel__form {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.login-panel__input {
  height: 88rpx;
  padding: 0 24rpx;
  border-radius: 12rpx;
  background: v.$bg;
  font-size: 28rpx;
}

.login-panel__code {
  display: flex;
  gap: 16rpx;
}

.login-panel__input--code {
  flex: 1;
}

.login-panel__send {
  height: 88rpx;
  line-height: 88rpx;
  padding: 0 24rpx;
  font-size: 26rpx;
  color: v.$primary;
  background: v.$primary-light;
  border-radius: 12rpx;
  white-space: nowrap;
}

.login-panel__btn {
  margin-top: 12rpx;
  height: 88rpx;
  line-height: 88rpx;
  border-radius: 12rpx;
  background: v.$primary;
  color: #fff;
  font-size: 30rpx;
}

.mine__user {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 24rpx;
  background: v.$surface;
  border-radius: 16rpx;
}

.mine__avatar {
  width: 96rpx;
  height: 96rpx;
  border-radius: 50%;
  background: v.$primary;
}

.mine__info {
  flex: 1;
}

.mine__name {
  display: block;
  font-size: 32rpx;
  font-weight: 600;
}

.mine__meta {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: v.$text-3;
}

.mine__logout {
  font-size: 26rpx;
  color: v.$text-3;
}

.quick {
  display: flex;
  background: v.$surface;
  border-radius: 16rpx;
  margin-top: 24rpx;
  padding: 24rpx;
  gap: 24rpx;
}

.quick__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8rpx;
  padding: 16rpx 32rpx;
  border-radius: 12rpx;
  background: v.$bg;
}

.quick__icon {
  font-size: 48rpx;
  color: v.$primary;
}

.quick__label {
  font-size: 22rpx;
  color: v.$text-2;
}

.section {
  margin-top: 24rpx;
  background: v.$surface;
  border-radius: 16rpx;
  padding: 24rpx;
}

.section__title {
  display: block;
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.row {
  display: flex;
  gap: 16rpx;
  align-items: center;
  padding: 16rpx 0;
  border-bottom: 1rpx solid v.$border;
}

.row--static {
  border-bottom: none;
}

.row__cover {
  position: relative;
  width: 200rpx;
  height: 120rpx;
  border-radius: 8rpx;
  overflow: hidden;
  flex-shrink: 0;
}

.row__img {
  width: 100%;
  height: 100%;
}

.row__dur {
  position: absolute;
  right: 8rpx;
  bottom: 8rpx;
  padding: 2rpx 8rpx;
  border-radius: 4rpx;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 20rpx;
}

.row__info {
  flex: 1;
  min-width: 0;
}

.row__title {
  display: block;
  font-size: 28rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__meta {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.tip {
  text-align: center;
  font-size: 26rpx;
  color: v.$text-3;
  padding: 32rpx 0;
}
</style>
