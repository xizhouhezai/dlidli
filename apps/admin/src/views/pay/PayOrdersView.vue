<script setup lang="ts">
// 支付与对账（M4-PAY-01，spec §6.7）：订单查询 / 对账日清单 / 异常订单处理台。
//
// 权限：页面 pay:view、手动补单 pay:order、对账与异常处理 pay:recon。
// **金额一律经 utils/money.ts 换算**：后端所有金额字段单位为分，
// 页面展示与输入在元，二者之间的转换只允许走那一个出入口（含边界用例）。
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { AdminPayOrder, AdminPayReconDaily, AdminPayReconIssue } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import { usePagedList } from '@/composables/usePagedList'
import { fenToYuan, yuanToFen, yuanParseErrorText } from '@/utils/money'
import PageHead from '@/components/PageHead.vue'

const activeTab = ref('orders')
const { run } = useApiAction()

const STATUS_META: Record<
  number,
  { label: string; type: 'success' | 'warning' | 'danger' | 'info' }
> = {
  0: { label: '待支付', type: 'warning' },
  1: { label: '已支付', type: 'success' },
  2: { label: '已关闭', type: 'info' },
  3: { label: '已退款', type: 'danger' },
}

const CHANNELS = [
  { value: 'mock', label: 'mock（联调渠道）' },
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信支付' },
]

// ---- Tab1：订单 ----
const oOrderNo = ref('')
const oUserId = ref<number | undefined>(undefined)
const oChannel = ref('')
const oStatus = ref<number | undefined>(undefined)
// 金额筛选：页面填**元**，提交前转分。用字符串承接输入，避免 v-model.number 的精度损失。
const oMinYuan = ref('')
const oMaxYuan = ref('')
const oRange = ref<[string, string] | undefined>(undefined)

const {
  list: orders,
  total: oTotal,
  loading: oLoading,
  page: oPage,
  pageSize: oPageSize,
  load: loadOrders,
  search: searchOrders,
  onPageChange: onOPageChange,
} = usePagedList<AdminPayOrder>(
  (page, size) => adminApi.adminPay.orders(buildOrderQuery(page, size)),
  { errorText: '订单加载失败' },
)

/** 把元输入转成分；非法输入返回 undefined 并提示（静默忽略会让运营以为筛选生效了）。 */
function amountParam(yuan: string): number | undefined {
  if (yuan.trim() === '') return undefined
  const parsed = yuanToFen(yuan)
  if (!parsed.ok) {
    ElMessage.warning(`金额「${yuan}」无效：${yuanParseErrorText(parsed.reason)}`)
    return undefined
  }
  return parsed.fen
}

function buildOrderQuery(page: number, size: number) {
  return {
    order_no: oOrderNo.value.trim() || undefined,
    user_id: oUserId.value || undefined,
    channel: oChannel.value || undefined,
    status: oStatus.value,
    min_amount: amountParam(oMinYuan.value),
    max_amount: amountParam(oMaxYuan.value),
    start_date: oRange.value?.[0] || undefined,
    end_date: oRange.value?.[1] || undefined,
    page,
    size,
  }
}

function resetOrderFilter() {
  oOrderNo.value = ''
  oUserId.value = undefined
  oChannel.value = ''
  oStatus.value = undefined
  oMinYuan.value = ''
  oMaxYuan.value = ''
  oRange.value = undefined
  searchOrders()
}

/** 手动补单：二次确认。仅「待支付/已关闭」需要此操作（已支付/已退款后端会拒绝）。 */
async function markPaid(row: AdminPayOrder) {
  try {
    await ElMessageBox.confirm(
      `确认为订单 ${row.order_no} 手动补单？\n金额 ${fenToYuan(row.amount)} 元，用户 ${row.user_id}。\n` +
        '该操作会在人工确认收款后入账并开通对应权益，操作人会记入现金流水备注。',
      '手动补单',
      { type: 'warning', confirmButtonText: '确认补单' },
    )
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminPay.markPaid(row.order_no), {
    success: '补单成功',
    fallback: '补单失败',
  })
  if (ok) loadOrders()
}

/** 手动关单（运维用）。 */
async function closeExpired() {
  try {
    await ElMessageBox.confirm(
      '确定执行超时关单吗？将把所有已过期的待支付订单置为已关闭。',
      '超时关单',
      {
        type: 'warning',
      },
    )
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminPay.closeExpired(), {
    success: '已执行关单',
    fallback: '关单失败',
  })
  if (ok) loadOrders()
}

// ---- Tab2：对账日清单 ----
const rDate = ref('')
const rChannel = ref('')
const {
  list: dailies,
  total: dTotal,
  loading: dLoading,
  page: dPage,
  pageSize: dPageSize,
  search: searchDaily,
  onPageChange: onDPageChange,
} = usePagedList<AdminPayReconDaily>(
  (page, size) =>
    adminApi.adminPay.reconDaily({
      date: rDate.value || undefined,
      channel: rChannel.value || undefined,
      page,
      size,
    }),
  { immediate: false, errorText: '对账单加载失败' },
)

/** 执行对账（重跑幂等）：日期必填，否则后端返回 92011。 */
async function runRecon() {
  if (!rDate.value) {
    ElMessage.warning('请先选择对账日期')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定对 ${rDate.value}${rChannel.value ? ' 的 ' + rChannel.value + ' 渠道' : ' 的全部渠道'} 执行对账吗？\n重复执行是幂等的：日对账单按（日期,渠道）覆盖，异常按（日期,订单,类型）覆盖。`,
      '执行对账',
      { type: 'warning' },
    )
  } catch {
    return
  }
  const ok = await run(
    () => adminApi.adminPay.reconRun({ date: rDate.value, channel: rChannel.value || undefined }),
    {
      success: '对账完成',
      fallback: '对账失败',
    },
  )
  if (ok) searchDaily()
}

// ---- Tab3：异常订单处理台 ----
const iDate = ref('')
const iChannel = ref('')
const iStatus = ref<number | undefined>(undefined)
const iType = ref<number | undefined>(undefined)
const ISSUE_TYPES = [
  { value: 1, label: '金额不符' },
  { value: 2, label: '长期挂起' },
  { value: 3, label: '流水缺失' },
  { value: 4, label: '重复回调' },
]
const {
  list: issues,
  total: iTotal,
  loading: iLoading,
  page: iPage,
  pageSize: iPageSize,
  load: loadIssues,
  search: searchIssues,
  onPageChange: onIPageChange,
} = usePagedList<AdminPayReconIssue>(
  (page, size) =>
    adminApi.adminPay.reconIssues({
      date: iDate.value || undefined,
      channel: iChannel.value || undefined,
      status: iStatus.value,
      issue_type: iType.value,
      page,
      size,
    }),
  { immediate: false, errorText: '异常清单加载失败' },
)

/** 处理异常：二次确认 + 填写处理备注（备注与处理人一起落库，是复核依据）。 */
async function handleIssue(row: AdminPayReconIssue) {
  let note = ''
  try {
    const res = await ElMessageBox.prompt(
      `标记异常「${row.type_text}」（订单 ${row.order_no}）为已处理。\n${row.detail}\n请填写处理说明（会与处理人一并落库）：`,
      '处理异常',
      {
        type: 'warning',
        confirmButtonText: '确认处理',
        inputPlaceholder: '如：已人工核对渠道账单，差异为手续费',
      },
    )
    note = res.value ?? ''
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminPay.handleIssue(row.id, note), {
    success: '已处理',
    fallback: '处理失败',
  })
  if (ok) loadIssues()
}

function openDaily() {
  activeTab.value = 'daily'
  searchDaily()
}
function openIssues() {
  activeTab.value = 'issues'
  searchIssues()
}
</script>

<template>
  <div>
    <PageHead
      title="支付与对账"
      sub="订单查询 · 日对账清单 · 异常订单处理台（金额单位：元，后端按分存储）"
    />

    <div class="page-card">
      <el-tabs
        v-model="activeTab"
        @tab-change="
          (n: string) => (n === 'daily' ? openDaily() : n === 'issues' ? openIssues() : undefined)
        "
      >
        <!-- Tab1 订单查询 -->
        <el-tab-pane label="订单查询" name="orders">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-input
              v-model="oOrderNo"
              class="w-220px"
              placeholder="订单号"
              clearable
              @keyup.enter="searchOrders"
            />
            <el-input v-model.number="oUserId" class="w-160px" placeholder="用户 ID" clearable />
            <el-select v-model="oChannel" class="w-160px" placeholder="全部渠道" clearable>
              <el-option v-for="c in CHANNELS" :key="c.value" :value="c.value" :label="c.label" />
            </el-select>
            <el-select v-model="oStatus" class="w-130px" placeholder="全部状态" clearable>
              <el-option :value="0" label="待支付" />
              <el-option :value="1" label="已支付" />
              <el-option :value="2" label="已关闭" />
              <el-option :value="3" label="已退款" />
            </el-select>
            <el-input v-model="oMinYuan" class="w-120px" placeholder="金额下限(元)" clearable />
            <span class="text-text-3">—</span>
            <el-input v-model="oMaxYuan" class="w-120px" placeholder="金额上限(元)" clearable />
            <el-date-picker
              v-model="oRange"
              type="daterange"
              value-format="YYYY-MM-DD"
              start-placeholder="创建起"
              end-placeholder="创建止"
              class="w-260px"
            />
            <el-button class="pink-btn" @click="searchOrders"> 查询 </el-button>
            <el-button @click="resetOrderFilter"> 重置 </el-button>
            <el-button v-perm="'pay:order'" @click="closeExpired"> 超时关单 </el-button>
          </div>

          <el-table v-loading="oLoading" :data="orders" stripe>
            <el-table-column prop="order_no" label="订单号" min-width="230" />
            <el-table-column prop="user_id" label="用户" width="120" />
            <el-table-column label="金额" width="110">
              <template #default="{ row }">
                <span class="font-600">¥{{ fenToYuan(row.amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="渠道" width="100">
              <template #default="{ row }">{{ row.channel }}</template>
            </el-table-column>
            <el-table-column prop="subject" label="商品" min-width="140" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="STATUS_META[row.status]?.type" size="small">
                  {{ row.status_text || STATUS_META[row.status]?.label }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="退款" width="100">
              <template #default="{ row }">
                <span v-if="row.refund_amount">¥{{ fenToYuan(row.refund_amount) }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" min-width="160" />
            <el-table-column label="操作" width="110" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.status === 0 || row.status === 2"
                  v-perm="'pay:order'"
                  size="small"
                  type="warning"
                  @click="markPaid(row)"
                >
                  补单
                </el-button>
                <span v-else class="text-text-3 text-3">—</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="oTotal"
              :page-size="oPageSize"
              :current-page="oPage"
              @current-change="onOPageChange"
            />
          </div>
        </el-tab-pane>

        <!-- Tab2 对账日清单 -->
        <el-tab-pane label="对账日清单" name="daily">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-date-picker
              v-model="rDate"
              type="date"
              value-format="YYYY-MM-DD"
              placeholder="对账日期"
            />
            <el-select v-model="rChannel" class="w-170px" placeholder="全部渠道" clearable>
              <el-option v-for="c in CHANNELS" :key="c.value" :value="c.value" :label="c.label" />
            </el-select>
            <el-button class="pink-btn" @click="searchDaily"> 查询 </el-button>
            <el-button v-perm="'pay:recon'" type="warning" @click="runRecon"> 执行对账 </el-button>
          </div>

          <el-table v-loading="dLoading" :data="dailies" stripe>
            <el-table-column prop="recon_date" label="对账日期" width="120" />
            <el-table-column prop="channel" label="渠道" width="100" />
            <el-table-column prop="order_count" label="订单数" width="90" />
            <el-table-column prop="paid_count" label="已支付" width="90" />
            <el-table-column label="支付金额" width="120">
              <template #default="{ row }">¥{{ fenToYuan(row.paid_amount) }}</template>
            </el-table-column>
            <el-table-column prop="refund_count" label="退款笔数" width="100" />
            <el-table-column label="退款金额" width="120">
              <template #default="{ row }">¥{{ fenToYuan(row.refund_amount) }}</template>
            </el-table-column>
            <el-table-column label="流水净额" width="120">
              <template #default="{ row }">¥{{ fenToYuan(row.cash_log_amount) }}</template>
            </el-table-column>
            <el-table-column label="差异数" width="90">
              <template #default="{ row }">
                <span :class="row.diff_count > 0 ? 'text-red-500 font-600' : ''">
                  {{ row.diff_count }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.status === 0 ? 'success' : 'danger'" size="small">
                  {{ row.status_text }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="dTotal"
              :page-size="dPageSize"
              :current-page="dPage"
              @current-change="onDPageChange"
            />
          </div>
        </el-tab-pane>

        <!-- Tab3 异常订单处理台 -->
        <el-tab-pane label="异常订单处理台" name="issues">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-date-picker
              v-model="iDate"
              type="date"
              value-format="YYYY-MM-DD"
              placeholder="对账日期"
            />
            <el-select v-model="iChannel" class="w-170px" placeholder="全部渠道" clearable>
              <el-option v-for="c in CHANNELS" :key="c.value" :value="c.value" :label="c.label" />
            </el-select>
            <el-select v-model="iType" class="w-140px" placeholder="全部类型" clearable>
              <el-option
                v-for="t in ISSUE_TYPES"
                :key="t.value"
                :value="t.value"
                :label="t.label"
              />
            </el-select>
            <el-select v-model="iStatus" class="w-140px" placeholder="全部状态" clearable>
              <el-option :value="0" label="待处理" />
              <el-option :value="1" label="已处理" />
            </el-select>
            <el-button class="pink-btn" @click="searchIssues"> 查询 </el-button>
          </div>

          <el-table v-loading="iLoading" :data="issues" stripe>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="recon_date" label="对账日期" width="110" />
            <el-table-column prop="channel" label="渠道" width="90" />
            <el-table-column prop="order_no" label="订单号" min-width="220" />
            <el-table-column label="异常类型" width="110">
              <template #default="{ row }">
                <el-tag type="danger" size="small">{{ row.type_text }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" label="详情" min-width="240" show-overflow-tooltip />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.status === 0 ? 'warning' : 'success'" size="small">
                  {{ row.status === 0 ? '待处理' : '已处理' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处理人/时间" min-width="160">
              <template #default="{ row }">
                <span v-if="row.handled_at" class="text-3 text-text-2">
                  {{ row.handled_by }} · {{ row.handled_at }}
                </span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column label="处理备注" min-width="160">
              <template #default="{ row }">
                <span v-if="row.handle_note">{{ row.handle_note }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="110" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.status === 0"
                  v-perm="'pay:recon'"
                  size="small"
                  type="primary"
                  @click="handleIssue(row)"
                >
                  处理
                </el-button>
                <span v-else class="text-text-3 text-3">已处理</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="iTotal"
              :page-size="iPageSize"
              :current-page="iPage"
              @current-change="onIPageChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>
