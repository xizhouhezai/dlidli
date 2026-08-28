<script setup lang="ts">
// 播放页评论区（M1 版）：两级评论、热度/最新排序、点赞、回复、删除、加载更多
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatPubdate } from '@dlidli/shared'
import { ApiError, type CommentItem } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import defaultAvatar from '@/assets/default-avatar.png'
import ReportDialog from '@/components/ReportDialog.vue'

const props = defineProps<{ bvid: string }>()

const router = useRouter()
const userStore = useUserStore()

const PAGE_SIZE = 20

const sort = ref<'hot' | 'new'>('hot')
const comments = ref<CommentItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

const input = ref('')
const posting = ref(false)

// 回复态：正在回复的评论 id（一级或楼中楼）
const replyTo = reactive({ rootId: '', parentId: '', nickname: '' })
const replyInput = ref('')

// 已点赞集合（会话级乐观状态）
const likedSet = ref(new Set<string>())

// 举报：待举报评论（一级或楼中楼）
const reportDialog = ref<InstanceType<typeof ReportDialog> | null>(null)
const reportComment = ref<CommentItem | null>(null)

function openReport(c: CommentItem) {
  if (!requireLogin()) return
  reportComment.value = c
  reportDialog.value?.open()
}

// 加载更多评论（多语句逻辑收敛为具名方法，避免内联多语句 handler）
function loadMore() {
  page.value++
  load()
}

async function load(reset = false) {
  if (reset) {
    page.value = 1
    comments.value = []
  }
  loading.value = true
  try {
    const res = await api.interaction.comments(props.bvid, {
      sort: sort.value,
      page: page.value,
      page_size: PAGE_SIZE,
    })
    comments.value = reset ? res.list : [...comments.value, ...res.list]
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(() => load(true))

function switchSort(s: 'hot' | 'new') {
  if (sort.value === s) return
  sort.value = s
  load(true)
}

function requireLogin(): boolean {
  if (!userStore.token) {
    router.push('/login')
    return false
  }
  return true
}

async function postRoot() {
  if (!requireLogin()) return
  const content = input.value.trim()
  if (!content) return
  posting.value = true
  try {
    const item = await api.interaction.addComment(props.bvid, { content })
    comments.value.unshift(item)
    total.value++
    input.value = ''
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '发布失败，请重试')
  } finally {
    posting.value = false
  }
}

function openReply(root: CommentItem, target?: CommentItem) {
  if (!requireLogin()) return
  replyTo.rootId = root.id
  replyTo.parentId = target?.id ?? root.id
  replyTo.nickname = (target ?? root).user.nickname
  replyInput.value = ''
}

function closeReply() {
  replyTo.rootId = ''
  replyTo.parentId = ''
}

async function postReply(root: CommentItem) {
  const content = replyInput.value.trim()
  if (!content) return
  posting.value = true
  try {
    const item = await api.interaction.addComment(props.bvid, {
      content,
      root_id: replyTo.rootId,
      parent_id: replyTo.parentId,
    })
    root.replies = [...(root.replies ?? []), item]
    root.reply_cnt++
    closeReply()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '回复失败，请重试')
  } finally {
    posting.value = false
  }
}

async function toggleLike(c: CommentItem) {
  if (!requireLogin()) return
  try {
    const { liked } = await api.interaction.likeComment(c.id)
    c.like_cnt += liked ? 1 : -1
    if (liked) likedSet.value.add(c.id)
    else likedSet.value.delete(c.id)
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  }
}

async function remove(c: CommentItem, root?: CommentItem) {
  try {
    await ElMessageBox.confirm('确定删除这条评论吗？', '删除评论', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.interaction.deleteComment(c.id)
    if (root) {
      root.replies = root.replies?.filter((r) => r.id !== c.id)
      root.reply_cnt = Math.max(0, root.reply_cnt - 1)
    } else {
      comments.value = comments.value.filter((r) => r.id !== c.id)
      total.value = Math.max(0, total.value - 1)
    }
    ElMessage.success('已删除')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '删除失败')
  }
}

async function expandReplies(root: CommentItem) {
  const loaded = root.replies?.length ?? 0
  const res = await api.interaction.replies(root.id, Math.floor(loaded / 10) + 1, 10)
  const exist = new Set((root.replies ?? []).map((r) => r.id))
  root.replies = [...(root.replies ?? []), ...res.list.filter((r) => !exist.has(r.id))]
}
</script>

<template>
  <div class="cmt">
    <div class="cmt__head">
      <span class="cmt__count">{{ total }} 条评论</span>
      <span class="cmt__sort" :class="{ 'is-active': sort === 'hot' }" @click="switchSort('hot')"
        >最热</span
      >
      <span class="cmt__divider">|</span>
      <span class="cmt__sort" :class="{ 'is-active': sort === 'new' }" @click="switchSort('new')"
        >最新</span
      >
    </div>

    <!-- 发布框 -->
    <div class="cmt__post">
      <el-avatar :size="36" :src="userStore.profile?.avatar || defaultAvatar" class="cmt__avatar">
        {{ userStore.profile?.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <el-input
        v-model="input"
        :placeholder="userStore.token ? '发一条友善的评论' : '登录后发表评论'"
        :disabled="!userStore.token"
        maxlength="1000"
        @keyup.enter="postRoot"
      />
      <el-button type="primary" class="cmt__send" :loading="posting" @click="postRoot">
        发布
      </el-button>
    </div>

    <el-empty
      v-if="!loading && comments.length === 0"
      description="还没有评论，来抢沙发～"
      :image-size="72"
    />

    <!-- 评论列表 -->
    <div v-for="c in comments" :key="c.id" class="cmt-item">
      <el-avatar :size="36" :src="c.user.avatar || defaultAvatar" class="cmt__avatar">
        {{ c.user.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <div class="cmt-item__body">
        <p class="cmt-item__user">
          {{ c.user.nickname }}
          <el-tag v-if="c.is_top" size="small" type="warning"> 置顶 </el-tag>
        </p>
        <p class="cmt-item__content">
          {{ c.content }}
        </p>
        <p class="cmt-item__meta">
          <span>{{ formatPubdate(c.created_at) }}</span>
          <span
            class="cmt-item__op"
            :class="{ 'is-liked': likedSet.has(c.id) }"
            @click="toggleLike(c)"
            ><span
              class="align-middle mr-1"
              :class="
                likedSet.has(c.id) ? 'i-mingcute-thumb-up-2-fill' : 'i-mingcute-thumb-up-2-line'
              "
            />{{ c.like_cnt || '' }}</span
          >
          <span class="cmt-item__op" @click="openReply(c)">回复</span>
          <span v-if="c.is_self" class="cmt-item__op" @click="remove(c)">删除</span>
          <span class="cmt-item__op" @click="openReport(c)">举报</span>
        </p>

        <!-- 回复输入 -->
        <div v-if="replyTo.rootId === c.id" class="cmt-reply-box">
          <el-input
            v-model="replyInput"
            :placeholder="`回复 @${replyTo.nickname}`"
            maxlength="1000"
            @keyup.enter="postReply(c)"
          />
          <el-button type="primary" size="small" :loading="posting" @click="postReply(c)">
            回复
          </el-button>
          <el-button size="small" @click="closeReply"> 取消 </el-button>
        </div>

        <!-- 楼中楼 -->
        <div v-if="c.replies?.length" class="cmt-replies">
          <div v-for="r in c.replies" :key="r.id" class="cmt-reply">
            <span class="cmt-reply__user">{{ r.user.nickname }}：</span>
            <span>{{ r.content }}</span>
            <span class="cmt-item__meta cmt-reply__meta">
              <span>{{ formatPubdate(r.created_at) }}</span>
              <span
                class="cmt-item__op"
                :class="{ 'is-liked': likedSet.has(r.id) }"
                @click="toggleLike(r)"
                ><span
                  class="align-middle mr-1"
                  :class="
                    likedSet.has(r.id) ? 'i-mingcute-thumb-up-2-fill' : 'i-mingcute-thumb-up-2-line'
                  "
                />{{ r.like_cnt || '' }}</span
              >
              <span class="cmt-item__op" @click="openReply(c, r)">回复</span>
              <span v-if="r.is_self" class="cmt-item__op" @click="remove(r, c)">删除</span>
              <span class="cmt-item__op" @click="openReport(r)">举报</span>
            </span>
          </div>
          <span
            v-if="(c.replies?.length ?? 0) < c.reply_cnt"
            class="cmt-item__op cmt-replies__more"
            @click="expandReplies(c)"
          >
            展开更多回复（共 {{ c.reply_cnt }} 条）
          </span>
        </div>
      </div>
    </div>

    <div v-if="comments.length < total" class="cmt__more">
      <el-button link :loading="loading" @click="loadMore"> 加载更多评论 </el-button>
    </div>
  </div>

  <!-- 举报弹层 -->
  <ReportDialog
    ref="reportDialog"
    :target-type="2"
    :target-id="reportComment?.id ?? ''"
    :title="reportComment ? `评论：${reportComment.content}` : ''"
  />
</template>

<style scoped>
.cmt__head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.cmt__count {
  font-size: 15px;
  font-weight: 600;
}

.cmt__sort {
  font-size: 13px;
  color: var(--dli-text-2);
  cursor: pointer;
}

.cmt__sort.is-active {
  color: var(--dli-primary);
  font-weight: 600;
}

.cmt__divider {
  color: #e3e5e7;
}

.cmt__post {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.cmt__avatar {
  background: var(--dli-primary);
  color: #fff;
  font-weight: 600;
  flex-shrink: 0;
}

.cmt__send {
  --el-button-bg-color: var(--dli-primary);
  --el-button-border-color: var(--dli-primary);
  --el-button-hover-bg-color: #fc8bab;
  --el-button-hover-border-color: #fc8bab;
}

.cmt-item {
  display: flex;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid #f1f2f3;
}

.cmt-item__body {
  flex: 1;
  min-width: 0;
}

.cmt-item__user {
  margin: 0;
  font-size: 13px;
  color: var(--dli-text-2);
  font-weight: 600;
}

.cmt-item__content {
  margin: 6px 0;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.cmt-item__meta {
  margin: 0;
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--dli-text-2);
}

.cmt-item__op {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
}

.cmt-item__op:hover,
.cmt-item__op.is-liked {
  color: var(--dli-primary);
}

.cmt-reply-box {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}

.cmt-replies {
  margin-top: 10px;
  padding: 10px 12px;
  background: #f6f7f8;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cmt-reply {
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;
}

.cmt-reply__user {
  color: var(--dli-text-2);
  font-weight: 600;
}

.cmt-reply__meta {
  margin-top: 2px;
}

.cmt-replies__more {
  font-size: 12px;
  color: var(--dli-text-2);
}

.cmt__more {
  text-align: center;
  padding: 12px 0;
}
</style>
