<script setup lang="ts">
// 直播巡查（M4-LIV-05，spec §5E.4）：直播中房间列表 + 手动切断/恢复 + 截帧机审与违规记录。
//
// 页面权限 live:audit（菜单），按钮权限 live:cut（切断/恢复）。
// 两个 Tab 而非两个页面：巡查是一个连续动作——看到异常房间就地切断，
// 再切到「违规记录」核对处置结果；拆成两个页面会把这条动线切断。
import { onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiErrorMessage, type AdminLiveRoom, type AdminLiveViolation } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import { usePagedList } from '@/composables/usePagedList'
import PageHead from '@/components/PageHead.vue'

const activeTab = ref('rooms')
const { run } = useApiAction()

// ---- Tab1：直播中房间 ----
const rooms = ref<AdminLiveRoom[]>([])
const roomsLoading = ref(false)
let roomsTimer: number | undefined

async function loadRooms() {
  roomsLoading.value = true
  try {
    rooms.value = (await adminApi.adminLive.rooms()).list
  } catch (err) {
    ElMessage.error(apiErrorMessage(err, '巡查列表加载失败，请稍后再试'))
  } finally {
    roomsLoading.value = false
  }
}

/** 开播时长：由 started_at 推算，未开播显示 —。 */
function liveDuration(startedAt?: string): string {
  if (!startedAt) return '—'
  // 后端格式 'YYYY-MM-DD HH:mm:ss'：Safari 不认这种带空格的写法，替换成 T 再解析。
  const start = new Date(startedAt.replace(' ', 'T')).getTime()
  if (Number.isNaN(start)) return '—'
  const mins = Math.max(0, Math.floor((Date.now() - start) / 60000))
  if (mins < 60) return `${mins} 分钟`
  return `${Math.floor(mins / 60)} 小时 ${mins % 60} 分钟`
}

/** 最近截帧判定标签：通过 / 命中 / 机审不可用三种必须可区分。 */
function auditTag(row: AdminLiveRoom): {
  text: string
  type: 'success' | 'danger' | 'warning' | 'info'
} {
  const a = row.last_audit
  if (!a) return { text: '无截帧记录', type: 'info' }
  if (a.degraded) return { text: '机审不可用（未切断）', type: 'warning' }
  return a.pass
    ? { text: '判定通过', type: 'success' }
    : { text: `命中：${a.hit_words || '—'}`, type: 'danger' }
}

/** 手动切断：二次确认 + 填写原因（原因会落违规流水，追责时是唯一现场说明）。 */
async function cutRoom(row: AdminLiveRoom) {
  let reason = ''
  try {
    const res = await ElMessageBox.prompt(
      `将切断「${row.title || '房间 ' + row.id}」（主播 ${row.anchor_id}）的直播并封禁该房间。\n该操作立即生效，请填写处理原因以便追溯：`,
      '切断直播',
      { type: 'warning', confirmButtonText: '确认切断', inputPlaceholder: '如：画面涉嫌违规' },
    )
    reason = res.value ?? ''
  } catch {
    return // 用户取消
  }
  const ok = await run(() => adminApi.adminLive.cut(row.id, reason), {
    success: '已切断直播',
    fallback: '切断失败',
  })
  if (ok) loadRooms()
}

/** 恢复直播：解封房间。 */
async function restoreRoom(row: AdminLiveRoom) {
  try {
    await ElMessageBox.confirm(
      `确定恢复「${row.title || '房间 ' + row.id}」吗？主播将可重新推流。`,
      '恢复直播',
      { type: 'warning', confirmButtonText: '确认恢复' },
    )
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminLive.restore(row.id), {
    success: '已恢复',
    fallback: '恢复失败',
  })
  if (ok) loadRooms()
}

// ---- Tab2：截帧机审 + 违规记录 ----
// 违规记录**不分房间**：巡台要回答的是「最近谁被切了、谁切的」，
// 按房间过滤留给 room_id 参数（页面上以「只看某房间」输入框暴露）。
const roomFilter = ref<number | undefined>(undefined)
const {
  list: violations,
  total: vTotal,
  loading: vLoading,
  page: vPage,
  pageSize: vPageSize,
  search: searchViolations,
  onPageChange: onVPageChange,
} = usePagedList<AdminLiveViolation>(
  (page, size) => adminApi.adminLive.violations({ room_id: roomFilter.value, page, size }),
  { immediate: false, errorText: '违规记录加载失败' },
)

/** 机审开关态：关闭时后端返回 91027，此处按「未启用」正常展示而非报错。 */
const auditEnabled = ref<boolean | null>(null)
const auditDetail = ref('')
async function loadAuditStatus() {
  try {
    const st = await adminApi.adminLive.auditStatus()
    auditEnabled.value = st.enabled
    auditDetail.value = `截帧间隔 ${st.frame_interval_seconds}s · 黑屏阈值 ${st.black_yavg_max} · 纯色阈值 ${st.flat_range_max}`
  } catch {
    // 91027 = 截帧机审未启用：这是合法状态（dev 默认关闭），不是错误
    auditEnabled.value = false
    auditDetail.value =
      '截帧机审未启用（live.audit.enabled=false）：截图不会产生，违规只能靠人工巡查发现'
  }
}

function openRecords() {
  activeTab.value = 'records'
  loadAuditStatus()
  searchViolations()
}

function resetRoomFilter() {
  roomFilter.value = undefined
  searchViolations()
}

/** 每 15s 轮询直播中列表（rooms 不带分页参数，每次取全量直播中房间）。 */
function startPolling() {
  stopPolling()
  roomsTimer = window.setInterval(() => {
    if (activeTab.value === 'rooms') loadRooms()
  }, 15000)
}
function stopPolling() {
  if (roomsTimer !== undefined) {
    window.clearInterval(roomsTimer)
    roomsTimer = undefined
  }
}

loadRooms()
loadAuditStatus()
startPolling()
// 组件卸载即停轮询：否则切走后定时器仍在打接口（资源泄漏且污染其它页面的观测）。
onUnmounted(stopPolling)
</script>

<template>
  <div>
    <PageHead title="直播巡查" sub="直播中房间巡查 · 手动切断/恢复 · 截帧机审与违规记录" />

    <div class="page-card">
      <el-tabs v-model="activeTab" @tab-change="(n: string) => n === 'records' && openRecords()">
        <!-- Tab1 直播中房间 -->
        <el-tab-pane label="直播中房间" name="rooms">
          <div class="flex items-center justify-between mb-3">
            <span class="text-3 text-text-2">
              每 15 秒自动刷新 · 共 {{ rooms.length }} 个直播中房间
            </span>
            <el-button class="pink-btn" @click="loadRooms"> 刷新 </el-button>
          </div>

          <el-table v-loading="roomsLoading" :data="rooms" stripe>
            <el-table-column label="房间" min-width="200">
              <template #default="{ row }">
                <div class="text-3.5 font-600">{{ row.title || '（未命名）' }}</div>
                <div class="text-3 text-text-2">房间 #{{ row.id }} · 主播 {{ row.anchor_id }}</div>
              </template>
            </el-table-column>
            <el-table-column label="在线数" width="90">
              <template #default="{ row }">
                <span class="text-3.5">{{ row.online }}</span>
              </template>
            </el-table-column>
            <el-table-column label="开播时长" min-width="130">
              <template #default="{ row }">
                <span class="text-3 text-text-2">{{ liveDuration(row.started_at) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="最近截帧判定" min-width="200">
              <template #default="{ row }">
                <el-tag :type="auditTag(row).type" size="small">{{ auditTag(row).text }}</el-tag>
                <div v-if="row.last_audit" class="text-3 text-text-3 mt-1">
                  {{ row.last_audit.created_at }}
                  <span v-if="row.last_audit.image_key"> · 图 {{ row.last_audit.image_key }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag v-if="row.ban_status === 1" type="danger" size="small">已切断</el-tag>
                <el-tag v-else type="success" size="small">直播中</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.ban_status !== 1"
                  v-perm="'live:cut'"
                  size="small"
                  type="danger"
                  @click="cutRoom(row)"
                >
                  切断
                </el-button>
                <el-button v-else v-perm="'live:cut'" size="small" @click="restoreRoom(row)">
                  恢复
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- Tab2 截帧机审与违规记录 -->
        <el-tab-pane label="截帧机审与违规记录" name="records">
          <el-alert
            v-if="auditEnabled === false"
            class="mb-3"
            type="warning"
            :closable="false"
            title="截帧机审未启用"
            :description="auditDetail"
          />
          <el-alert
            v-else-if="auditEnabled"
            class="mb-3"
            type="success"
            :closable="false"
            title="截帧机审运行中"
            :description="auditDetail"
          />

          <div class="flex items-center gap-2 mb-3">
            <el-input
              v-model.number="roomFilter"
              class="w-200px"
              placeholder="按房间 ID 过滤（选填）"
              clearable
              @keyup.enter="searchViolations"
            />
            <el-button class="pink-btn" @click="searchViolations"> 查询 </el-button>
            <el-button @click="resetRoomFilter"> 重置 </el-button>
            <span class="text-3 text-text-2">共 {{ vTotal }} 条违规记录</span>
          </div>

          <el-table v-loading="vLoading" :data="violations" stripe>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column label="房间 / 主播" min-width="160">
              <template #default="{ row }">
                <div>房间 #{{ row.room_id }}</div>
                <div class="text-3 text-text-2">主播 {{ row.anchor_id }}</div>
              </template>
            </el-table-column>
            <el-table-column label="来源" width="100">
              <template #default="{ row }">
                <el-tag :type="row.source === 2 ? 'warning' : 'info'" size="small">
                  {{ row.source_text }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="原因" min-width="160">
              <template #default="{ row }">
                <span :class="row.reason ? '' : 'text-text-3'">{{ row.reason || '—' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="切断方式" min-width="150">
              <template #default="{ row }">
                <el-tag :type="row.cut_mode === 1 ? 'success' : 'warning'" size="small">
                  {{ row.cut_mode_text }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作人" min-width="120">
              <template #default="{ row }">
                <span v-if="row.operator">{{ row.operator }}</span>
                <span v-else class="text-text-3">系统（机审）</span>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" min-width="160" />
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="prev, pager, next"
              :total="vTotal"
              :page-size="vPageSize"
              :current-page="vPage"
              @current-change="onVPageChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>
