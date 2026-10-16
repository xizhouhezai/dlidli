<script setup lang="ts">
// 充电与结算（M4-PAY-02，spec CHG-01/02/03 + CRT-32）：
// 充电流水 / 结算单查询 / 提现审核台（审核 + 打款 + 月结补跑）。
//
// 权限：页面 charge:view；审核 charge:audit、打款 charge:payout、月结 charge:settle
// 各自独立 —— **能看记录不等于能动钱**，故三个按钮分别挂 v-perm，后端逐个路由挂 RequirePerm。
// 金额一律经 utils/money.ts 在「分（后端）」与「元（页面）」之间换算。
import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import type { AdminChargeItem, AdminSettlementItem, AdminWithdrawItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import { usePagedList } from '@/composables/usePagedList'
import { fenToYuan } from '@/utils/money'
import PageHead from '@/components/PageHead.vue'

const activeTab = ref('withdraws')
const { run } = useApiAction()

/** 提现状态：0 审核中 / 2 已通过 / 3 已拒绝 / 4 已打款 / 5 打款失败。 */
const WITHDRAW_STATUS: Record<
  number,
  { label: string; type: 'success' | 'warning' | 'danger' | 'info' }
> = {
  0: { label: '审核中', type: 'warning' },
  2: { label: '已通过', type: 'success' },
  3: { label: '已拒绝', type: 'info' },
  4: { label: '已打款', type: 'success' },
  5: { label: '打款失败', type: 'danger' },
}

// ---- Tab1：提现审核台 ----
const wUserId = ref<number | undefined>(undefined)
const wStatus = ref<number | undefined>(undefined)
const {
  list: withdraws,
  total: wTotal,
  loading: wLoading,
  page: wPage,
  pageSize: wPageSize,
  load: loadWithdraws,
  search: searchWithdraws,
  onPageChange: onWPageChange,
} = usePagedList<AdminWithdrawItem>(
  (page, size) =>
    adminApi.adminCharge.withdraws({
      user_id: wUserId.value || undefined,
      status: wStatus.value,
      page,
      size,
    }),
  { errorText: '提现单加载失败' },
)

/** 审核：二次确认 + 备注（备注与审核人、审核时间一并落库）。 */
async function audit(row: AdminWithdrawItem, approve: boolean) {
  let note = ''
  try {
    const res = await ElMessageBox.prompt(
      `${approve ? '通过' : '驳回'}提现单 ${row.withdraw_no}。\n创作者 ${row.user_id} · 金额 ¥${fenToYuan(row.amount)} · ${row.account_name}（${row.account}）\n` +
        (approve ? '通过后即可打款。' : '驳回将解冻并返还该笔金额。') +
        '\n请填写审核备注（会与审核人一并落库）：',
      approve ? '通过提现' : '驳回提现',
      {
        type: 'warning',
        confirmButtonText: approve ? '确认通过' : '确认驳回',
        inputPlaceholder: approve ? '如：资料齐全，同意出款' : '如：账户信息不符，请重新提交',
      },
    )
    note = res.value ?? ''
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminCharge.auditWithdraw(row.withdraw_no, approve, note), {
    success: approve ? '已通过' : '已驳回',
    fallback: '审核失败',
  })
  if (ok) loadWithdraws()
}

/** 打款：二次确认 + 备注。真实出款通道为换入点，当前走 mock，但状态机与幂等按真实契约。 */
async function payout(row: AdminWithdrawItem, fail = false) {
  let remark = ''
  try {
    const res = await ElMessageBox.prompt(
      (fail
        ? `标记提现单 ${row.withdraw_no} 打款失败？\n金额 ¥${fenToYuan(row.amount)} 将解冻返还给创作者。`
        : `确认对提现单 ${row.withdraw_no} 发起打款？\n创作者 ${row.user_id} · ¥${fenToYuan(row.amount)} → ${row.account_name}（${row.account}）\n` +
          '同一单重复打款会被状态机与唯一键双重拦下。') + '\n请填写备注：',
      fail ? '打款失败' : '确认打款',
      {
        type: 'warning',
        confirmButtonText: fail ? '确认失败' : '确认打款',
        inputPlaceholder: fail ? '如：渠道返回账户异常' : '如：已核对账户信息',
      },
    )
    remark = res.value ?? ''
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminCharge.payout(row.withdraw_no, fail, remark), {
    success: fail ? '已标记失败' : '打款完成',
    fallback: '打款失败',
  })
  if (ok) loadWithdraws()
}

// ---- Tab2：充电流水 ----
const cUserId = ref<number | undefined>(undefined)
const cAnchorId = ref<number | undefined>(undefined)
const {
  list: charges,
  total: cTotal,
  loading: cLoading,
  page: cPage,
  pageSize: cPageSize,
  search: searchCharges,
  onPageChange: onCPageChange,
} = usePagedList<AdminChargeItem>(
  (page, size) =>
    adminApi.adminCharge.charges({
      user_id: cUserId.value || undefined,
      anchor_id: cAnchorId.value || undefined,
      page,
      size,
    }),
  { immediate: false, errorText: '充电记录加载失败' },
)

// ---- Tab3：结算单 ----
const sUserId = ref<number | undefined>(undefined)
const sMonth = ref('')
const {
  list: settlements,
  total: sTotal,
  loading: sLoading,
  page: sPage,
  pageSize: sPageSize,
  search: searchSettlements,
  onPageChange: onSPageChange,
} = usePagedList<AdminSettlementItem>(
  (page, size) =>
    adminApi.adminCharge.settlements({
      user_id: sUserId.value || undefined,
      month: sMonth.value || undefined,
      page,
      size,
    }),
  { immediate: false, errorText: '结算单加载失败' },
)

/** 手动补跑月结（与月度 Worker 同一实现；幂等可重复跑）。 */
async function settleRun() {
  try {
    await ElMessageBox.confirm(
      `确定执行${sMonth.value ? ' ' + sMonth.value : '（默认上一自然月）'} 的月度结算吗？\n` +
        '与定时任务同一实现：按充电流水的支付时间归属自然月，重复执行不会重复计额。',
      '手动月结',
      { type: 'warning' },
    )
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminCharge.settleRun(sMonth.value), {
    success: '月结完成',
    fallback: '月结失败',
  })
  if (ok) searchSettlements()
}

function openCharges() {
  activeTab.value = 'charges'
  searchCharges()
}
function openSettlements() {
  activeTab.value = 'settlements'
  searchSettlements()
}
</script>

<template>
  <div>
    <PageHead
      title="充电与结算"
      sub="提现审核台 · 充电流水 · 结算单（金额单位：元，后端按分存储）"
    />

    <div class="page-card">
      <el-tabs
        v-model="activeTab"
        @tab-change="
          (n: string) =>
            n === 'charges' ? openCharges() : n === 'settlements' ? openSettlements() : undefined
        "
      >
        <!-- Tab1 提现审核台 -->
        <el-tab-pane label="提现审核台" name="withdraws">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-input
              v-model.number="wUserId"
              class="w-200px"
              placeholder="按创作者 ID 查询"
              clearable
              @keyup.enter="searchWithdraws"
            />
            <el-select v-model="wStatus" class="w-150px" placeholder="全部状态" clearable>
              <el-option :value="0" label="审核中" />
              <el-option :value="2" label="已通过" />
              <el-option :value="3" label="已拒绝" />
              <el-option :value="4" label="已打款" />
              <el-option :value="5" label="打款失败" />
            </el-select>
            <el-button class="pink-btn" @click="searchWithdraws"> 查询 </el-button>
            <span class="text-3 text-text-2">共 {{ wTotal }} 笔</span>
          </div>

          <el-table v-loading="wLoading" :data="withdraws" stripe>
            <el-table-column prop="withdraw_no" label="提现单号" min-width="210" />
            <el-table-column prop="user_id" label="创作者" width="120" />
            <el-table-column label="金额" width="110">
              <template #default="{ row }">
                <span class="font-600">¥{{ fenToYuan(row.amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="手续费" width="100">
              <template #default="{ row }">
                <span v-if="row.fee">¥{{ fenToYuan(row.fee) }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column label="收款账户" min-width="180">
              <template #default="{ row }">
                <div>{{ row.account_name || '—' }}</div>
                <div class="text-3 text-text-2">{{ row.account }}</div>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="WITHDRAW_STATUS[row.status]?.type" size="small">
                  {{ row.status_text || WITHDRAW_STATUS[row.status]?.label }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="审核 / 打款" min-width="180">
              <template #default="{ row }">
                <div v-if="row.audit_at" class="text-3 text-text-2">审核 {{ row.audit_at }}</div>
                <div v-if="row.paid_at" class="text-3 text-text-2">打款 {{ row.paid_at }}</div>
                <div v-if="row.audit_note" class="text-3 text-text-3">
                  备注：{{ row.audit_note }}
                </div>
                <div v-if="row.fail_reason" class="text-3 text-red-500">
                  失败原因：{{ row.fail_reason }}
                </div>
                <span v-if="!row.audit_at && !row.paid_at" class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="申请时间" min-width="160" />
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <template v-if="row.status === 0">
                  <el-button
                    v-perm="'charge:audit'"
                    size="small"
                    type="success"
                    @click="audit(row, true)"
                  >
                    通过
                  </el-button>
                  <el-button
                    v-perm="'charge:audit'"
                    size="small"
                    type="danger"
                    @click="audit(row, false)"
                  >
                    驳回
                  </el-button>
                </template>
                <template v-else-if="row.status === 2">
                  <el-button
                    v-perm="'charge:payout'"
                    size="small"
                    type="primary"
                    @click="payout(row, false)"
                  >
                    打款
                  </el-button>
                  <el-button v-perm="'charge:payout'" size="small" @click="payout(row, true)">
                    标记失败
                  </el-button>
                </template>
                <span v-else class="text-text-3 text-3">—</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="wTotal"
              :page-size="wPageSize"
              :current-page="wPage"
              @current-change="onWPageChange"
            />
          </div>
        </el-tab-pane>

        <!-- Tab2 充电流水 -->
        <el-tab-pane label="充电流水" name="charges">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-input
              v-model.number="cUserId"
              class="w-200px"
              placeholder="按充电者 ID 查询"
              clearable
              @keyup.enter="searchCharges"
            />
            <el-input
              v-model.number="cAnchorId"
              class="w-200px"
              placeholder="按 UP 主 ID 查询"
              clearable
              @keyup.enter="searchCharges"
            />
            <el-button class="pink-btn" @click="searchCharges"> 查询 </el-button>
            <span class="text-3 text-text-2">共 {{ cTotal }} 笔</span>
          </div>

          <el-table v-loading="cLoading" :data="charges" stripe>
            <el-table-column prop="charge_no" label="充电单号" min-width="200" />
            <el-table-column prop="order_no" label="订单号" min-width="220" />
            <el-table-column label="充电者" min-width="140">
              <template #default="{ row }">
                <span v-if="row.anonymous" class="text-text-3">匿名</span>
                <span v-else>{{ row.nickname || row.user_id }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="anchor_id" label="UP 主" width="120" />
            <el-table-column label="金额" width="110">
              <template #default="{ row }">
                <span class="font-600">¥{{ fenToYuan(row.amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="留言" min-width="160">
              <template #default="{ row }">
                <span v-if="row.message">{{ row.message }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column prop="paid_at" label="支付时间" min-width="160" />
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="cTotal"
              :page-size="cPageSize"
              :current-page="cPage"
              @current-change="onCPageChange"
            />
          </div>
        </el-tab-pane>

        <!-- Tab3 结算单 -->
        <el-tab-pane label="结算单" name="settlements">
          <div class="flex flex-wrap items-center gap-2 mb-4">
            <el-input
              v-model.number="sUserId"
              class="w-200px"
              placeholder="按创作者 ID 查询"
              clearable
              @keyup.enter="searchSettlements"
            />
            <el-date-picker
              v-model="sMonth"
              type="month"
              value-format="YYYY-MM"
              placeholder="结算月"
              class="w-160px"
            />
            <el-button class="pink-btn" @click="searchSettlements"> 查询 </el-button>
            <el-button v-perm="'charge:settle'" type="warning" @click="settleRun">
              手动月结
            </el-button>
            <span class="text-3 text-text-2">共 {{ sTotal }} 条</span>
          </div>

          <el-table v-loading="sLoading" :data="settlements" stripe>
            <el-table-column prop="settle_month" label="结算月" width="110" />
            <el-table-column prop="user_id" label="创作者" min-width="160" />
            <el-table-column prop="charge_count" label="充电笔数" width="110" />
            <el-table-column label="创作者分成" width="130">
              <template #default="{ row }">
                <span class="font-600">¥{{ fenToYuan(row.amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="平台分成" width="130">
              <template #default="{ row }">¥{{ fenToYuan(row.platform_amount) }}</template>
            </el-table-column>
            <el-table-column label="合计" width="130">
              <template #default="{ row }">
                ¥{{ fenToYuan(row.amount + row.platform_amount) }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="生成时间" min-width="160" />
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="sTotal"
              :page-size="sPageSize"
              :current-page="sPage"
              @current-change="onSPageChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>
