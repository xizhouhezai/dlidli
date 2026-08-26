<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import defaultAvatar from '@/assets/default-avatar.png'
import ReportDialog from '@/components/ReportDialog.vue'
import { useBlockActions } from '@/composables/message/useBlockActions'
import { useConversations } from '@/composables/message/useConversations'
import { useMessagesWs } from '@/composables/message/useMessagesWs'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

// —— 拆分出的子模块（M3-ENG-11）：会话消息 / 拉黑 / WS 实时 ——
const convApi = useConversations(route, router)
const blockApi = useBlockActions(() => convApi.activePeer.value)
const wsApi = useMessagesWs({
  getPeer: () => convApi.activePeer.value,
  pushMessage: (m) => convApi.messages.value.push(m),
  scrollBottom: convApi.scrollBottom,
  onIncomingOther: () => void convApi.loadConvs(),
  markCurrentRead: () => void convApi.markCurrentRead(),
})

const {
  convs,
  activePeer,
  activeConv,
  messages,
  input,
  sending,
  listBox,
  loadConvs,
  openPeer,
  send,
  formatTime,
} = convApi
const { blockedMe, iBlocked, toggleBlock, loadBlockStatus, reset: resetBlock } = blockApi

// 举报会话对象（target_type=5：举报用户，即私信对方）
const reportDialog = ref<InstanceType<typeof ReportDialog>>()
function openReport() {
  reportDialog.value?.open()
}

// 模板 ref 绑定：vue-tsc 对解构变量不识别模板引用，此处显式登记避免误报未使用
void listBox

/** 打开会话：加载拉黑状态（消息与 URL 同步由 conversations 处理）。 */
async function selectPeer(peer: string) {
  await openPeer(peer)
  resetBlock()
  void loadBlockStatus()
}

onMounted(() => {
  void loadConvs()
  wsApi.connectWs()
  const peer = route.query.peer as string | undefined
  if (peer) {
    void selectPeer(peer)
  }
})

// 路由 peer 参数变化时切换会话
watch(
  () => route.query.peer,
  (p) => {
    if (p && String(p) !== activePeer.value) void selectPeer(String(p))
  },
)

onBeforeUnmount(() => {
  wsApi.closeWs()
})
</script>

<template>
  <div class="msg-wrap">
    <!-- 会话列表 -->
    <div class="msg-convs">
      <div class="msg-convs__head">
        私信
      </div>
      <el-empty
        v-if="convs.length === 0"
        description="暂无会话"
        :image-size="60"
      />
      <div
        v-for="c in convs"
        :key="c.peer_id"
        class="msg-conv"
        :class="{ 'is-active': activePeer === c.peer_id }"
        @click="openPeer(c.peer_id)"
      >
        <img
          :src="c.avatar || defaultAvatar"
          alt=""
          class="msg-conv__avatar"
        >
        <div class="msg-conv__main">
          <div class="msg-conv__top">
            <span class="msg-conv__name">{{ c.nickname }}</span>
            <span class="msg-conv__time">{{ formatTime(c.last_at) }}</span>
          </div>
          <div class="msg-conv__bottom">
            <span class="msg-conv__preview">{{ c.last_content }}</span>
            <span
              v-if="c.unread > 0"
              class="msg-conv__badge"
            >{{ c.unread > 99 ? '99+' : c.unread }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 聊天窗 -->
    <div class="msg-chat">
      <div
        v-if="!activePeer"
        class="msg-chat__empty"
      >
        选择会话开始聊天
      </div>
      <template v-else>
        <div class="msg-chat__head">
          <span
            class="msg-chat__name"
            @click="router.push(`/space/${activePeer}`)"
          >{{ activeConv?.nickname ?? '对方' }}</span>
          <span class="msg-chat__actions">
            <span
              class="msg-chat__action"
              :title="iBlocked ? '取消拉黑' : '拉黑对方'"
              @click="toggleBlock"
            >
              <span class="i-mingcute-user-remove-line" />
              {{ iBlocked ? '已拉黑' : '拉黑' }}
            </span>
            <span
              class="msg-chat__action"
              title="举报会话"
              @click="openReport"
            >
              <span class="i-mingcute-warning-line" />
              举报
            </span>
          </span>
        </div>
        <div
          ref="listBox"
          class="msg-chat__list"
        >
          <div
            v-for="m in messages"
            :key="m.id"
            class="msg-bubble"
            :class="m.mine ? 'is-mine' : 'is-peer'"
          >
            <img
              :src="m.mine ? (userStore.profile?.avatar || defaultAvatar) : (activeConv?.avatar || defaultAvatar)"
              alt=""
              class="msg-bubble__avatar"
            >
            <div class="msg-bubble__body">
              <p
                v-if="m.content_type === 2"
                class="msg-bubble__img"
              >
                <img
                  :src="m.content"
                  alt="图片消息"
                >
              </p>
              <p
                v-else
                class="msg-bubble__text"
              >
                {{ m.content }}
              </p>
              <span class="msg-bubble__time">{{ formatTime(m.created_at) }}</span>
            </div>
          </div>
        </div>
        <div class="msg-chat__input">
          <el-alert
            v-if="blockedMe"
            type="warning"
            :closable="false"
            title="对方已将你拉黑，无法发送私信"
            class="mb-2"
          />
          <el-input
            v-model="input"
            type="textarea"
            :rows="2"
            maxlength="500"
            :disabled="blockedMe"
            :placeholder="blockedMe ? '对方已将你拉黑，无法发送消息' : '输入消息（≤500 字，敏感内容将被拦截）'"
            @keydown.enter.exact.prevent="send"
          />
          <el-button
            type="primary"
            :loading="sending"
            :disabled="blockedMe"
            @click="send"
          >
            发送
          </el-button>
        </div>
      </template>
    </div>
  </div>

  <!-- 举报会话对象（target_type=5：举报用户，即私信对方） -->
  <ReportDialog
    ref="reportDialog"
    :target-type="5"
    :target-id="activePeer"
    :title="activeConv ? `私信会话：${activeConv.nickname}` : '私信会话'"
  />
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.msg-wrap {
  display: flex;
  height: calc(100vh - 120px);
  gap: 12px;
  max-width: 1100px;
  margin: 0 auto;
}

.msg-convs {
  width: 280px;
  flex-shrink: 0;
  background: v.$surface;
  border-radius: v.$radius-lg;
  overflow-y: auto;
  padding: 12px;
}

.msg-convs__head {
  font-size: 16px;
  font-weight: 700;
  padding: 4px 8px 12px;
}

.msg-conv {
  display: flex;
  gap: 10px;
  padding: 10px 8px;
  border-radius: v.$radius-md;
  cursor: pointer;
  transition: background 0.15s;

  &:hover {
    background: #f6f7f8;
  }

  &.is-active {
    background: #{v.$primary}14;
  }
}

.msg-conv__avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  flex-shrink: 0;
}

.msg-conv__main {
  min-width: 0;
  flex: 1;
}

.msg-conv__top {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.msg-conv__name {
  font-size: 13.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.msg-conv__time {
  font-size: 11px;
  color: v.$text-3;
  flex-shrink: 0;
}

.msg-conv__bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
}

.msg-conv__preview {
  font-size: 12px;
  color: v.$text-2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.msg-conv__badge {
  background: v.$primary;
  color: #fff;
  font-size: 11px;
  min-width: 16px;
  height: 16px;
  line-height: 16px;
  border-radius: 8px;
  text-align: center;
  padding: 0 4px;
  flex-shrink: 0;
}

.msg-chat {
  flex: 1;
  background: v.$surface;
  border-radius: v.$radius-lg;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.msg-chat__empty {
  margin: auto;
  color: v.$text-3;
  font-size: 14px;
}

.msg-chat__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid v.$border;
}

.msg-chat__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.msg-chat__action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12.5px;
  color: v.$text-2;
  cursor: pointer;
  transition: color 0.15s;

  &:hover {
    color: v.$primary;
  }
}

.msg-chat__name {
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;

  &:hover {
    color: v.$primary;
  }
}

.msg-chat__list {
  flex: 1;
  overflow-y: auto;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.msg-bubble {
  display: flex;
  gap: 8px;
  max-width: 70%;

  &.is-mine {
    align-self: flex-end;
    flex-direction: row-reverse;
  }
}

.msg-bubble__avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  flex-shrink: 0;
}

.msg-bubble__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.msg-bubble__text {
  margin: 0;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 13.5px;
  line-height: 1.5;
  background: #f6f7f8;
  word-break: break-all;
}

.is-mine .msg-bubble__text {
  background: v.$primary;
  color: #fff;
}

.msg-bubble__img img {
  max-width: 180px;
  border-radius: 10px;
  display: block;
}

.msg-bubble__time {
  font-size: 11px;
  color: v.$text-3;
}

.is-mine .msg-bubble__time {
  text-align: right;
}

.msg-chat__input {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  padding: 12px 18px;
  border-top: 1px solid v.$border;
}
</style>
