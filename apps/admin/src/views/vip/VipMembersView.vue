<script setup lang="ts">
// 会员与套餐（M4-VIP-01，spec §2.7）：套餐配置（改价/上下架）+ 会员查询 + 手工补发硬币。
//
// 权限：页面 vip:view、套餐配置 vip:plan、补发硬币 vip:grant。
// **改价与补发是独立权限点**：能看会员不等于能改价、更不等于能发币 —— 故按钮各自挂 v-perm。
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiErrorMessage, type AdminVipMember, type AdminVipPlan } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import { usePagedList } from '@/composables/usePagedList'
import { fenToYuan, yuanToFen, yuanParseErrorText } from '@/utils/money'
import PageHead from '@/components/PageHead.vue'

const activeTab = ref('plans')
const { run } = useApiAction()

// 套餐上下架状态：与后端 vip.PlanOn / vip.PlanOff 逐字对齐（1 上架 / 0 下架）。
// 后端对非法值会静默改判为下架，故这里用常量而非字面量，避免「点了上架仍显示下架」。
const PLAN_ON = 1
const PLAN_OFF = 0

// ---- Tab1：套餐配置 ----
// 后端返回 list（上架）与 all（全部）：**「是否上架」由两者之差判定**，
// PlanView 本身没有 status 字段（它只描述套餐内容），这个区别不写清楚会误判上下架。
const plans = ref<AdminVipPlan[]>([])
const plansLoading = ref(false)

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminApi.adminVip.plans()
    const onShelf = new Set(res.list.map((p) => p.code))
    plans.value = res.all.map(
      (p) => ({ ...p, _onShelf: onShelf.has(p.code) }) as AdminVipPlan & { _onShelf: boolean },
    )
  } catch (err) {
    ElMessage.error(apiErrorMessage(err, '套餐加载失败'))
  } finally {
    plansLoading.value = false
  }
}

function isOnShelf(p: AdminVipPlan): boolean {
  return (p as AdminVipPlan & { _onShelf?: boolean })._onShelf === true
}

// 编辑弹窗：金额字段用**字符串**承接元输入，提交时统一走 yuanToFen 转分。
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(false)
const form = reactive({
  code: '',
  name: '',
  months: 1,
  priceYuan: '',
  originalYuan: '',
  firstMonthYuan: '',
  autoRenew: false,
  sort: 0,
  status: 1,
})

function openCreate() {
  editing.value = false
  form.code = ''
  form.name = ''
  form.months = 1
  form.priceYuan = ''
  form.originalYuan = ''
  form.firstMonthYuan = ''
  form.autoRenew = false
  form.sort = 0
  form.status = PLAN_ON
  dialogVisible.value = true
}

function openEdit(p: AdminVipPlan) {
  editing.value = true
  form.code = p.code
  form.name = p.name
  form.months = p.months
  form.priceYuan = fenToYuan(p.price)
  form.originalYuan = p.original_price ? fenToYuan(p.original_price) : ''
  form.firstMonthYuan = p.first_month_price ? fenToYuan(p.first_month_price) : ''
  form.autoRenew = p.auto_renew
  form.sort = p.sort
  form.status = isOnShelf(p) ? PLAN_ON : PLAN_OFF
  dialogVisible.value = true
}

/** 可空金额：空串按 0 分处理（原价/首月价 0 表示不展示/无优惠）。 */
function optionalFen(yuan: string): number | null {
  if (yuan.trim() === '') return 0
  const parsed = yuanToFen(yuan)
  if (!parsed.ok) {
    ElMessage.warning(`金额「${yuan}」无效：${yuanParseErrorText(parsed.reason)}`)
    return null
  }
  return parsed.fen
}

async function save() {
  if (!form.code.trim() || !form.name.trim()) {
    ElMessage.warning('套餐编码与名称必填')
    return
  }
  if (form.months <= 0) {
    ElMessage.warning('月数必须为正整数')
    return
  }
  const price = yuanToFen(form.priceYuan)
  if (!price.ok) {
    ElMessage.warning(`售价无效：${yuanParseErrorText(price.reason)}`)
    return
  }
  const original = optionalFen(form.originalYuan)
  const firstMonth = optionalFen(form.firstMonthYuan)
  if (original === null || firstMonth === null) return

  saving.value = true
  try {
    await adminApi.adminVip.upsertPlan({
      code: form.code.trim(),
      name: form.name.trim(),
      months: form.months,
      price: price.fen,
      original_price: original,
      first_month_price: firstMonth,
      auto_renew: form.autoRenew,
      sort: form.sort,
      status: form.status,
    })
    dialogVisible.value = false
    ElMessage.success('已保存')
    await loadPlans()
  } catch (err) {
    ElMessage.error(apiErrorMessage(err, '保存失败'))
  } finally {
    saving.value = false
  }
}

/** 上下架：改价/上下架是同一接口（status 1 上架 / 2 下架），二次确认后提交。 */
async function toggleShelf(p: AdminVipPlan) {
  const toOn = !isOnShelf(p)
  try {
    await ElMessageBox.confirm(
      `确定${toOn ? '上架' : '下架'}套餐「${p.name}」吗？\n售价 ¥${fenToYuan(p.price)} / ${p.months} 个月。` +
        (toOn ? '' : '\n下架后新用户无法购买，已购买用户的权益不受影响。'),
      toOn ? '上架套餐' : '下架套餐',
      { type: 'warning' },
    )
  } catch {
    return
  }
  const ok = await run(
    () =>
      adminApi.adminVip.upsertPlan({
        code: p.code,
        name: p.name,
        months: p.months,
        price: p.price,
        original_price: p.original_price,
        first_month_price: p.first_month_price,
        auto_renew: p.auto_renew,
        sort: p.sort,
        status: toOn ? PLAN_ON : PLAN_OFF,
      }),
    { success: toOn ? '已上架' : '已下架', fallback: '操作失败' },
  )
  if (ok) loadPlans()
}

// ---- Tab2：会员查询 ----
const mUserId = ref<number | undefined>(undefined)
const mOnlyVip = ref(false)
const {
  list: members,
  total: mTotal,
  loading: mLoading,
  page: mPage,
  pageSize: mPageSize,
  load: loadMembers,
  search: searchMembers,
  onPageChange: onMPageChange,
} = usePagedList<AdminVipMember>(
  (page, size) =>
    adminApi.adminVip.members({
      user_id: mUserId.value || undefined,
      only_vip: mOnlyVip.value ? 1 : undefined,
      page,
      size,
    }),
  { immediate: false, errorText: '会员列表加载失败' },
)

/** 手工补发本月硬币：二次确认（幂等：同一用户同一月只会发放一次）。 */
async function grantCoins(row: AdminVipMember) {
  try {
    await ElMessageBox.confirm(
      `确定为 UID ${row.user_id} 补发本月会员硬币吗？\n该操作是幂等的：同一用户同一自然月重复点击只会发放一次。`,
      '手工补发硬币',
      { type: 'warning', confirmButtonText: '确认补发' },
    )
  } catch {
    return
  }
  const ok = await run(() => adminApi.adminVip.grantCoins(row.user_id), {
    success: '已补发',
    fallback: '补发失败',
  })
  if (ok) loadMembers()
}

function openMembers() {
  activeTab.value = 'members'
  searchMembers()
}
</script>

<template>
  <div>
    <PageHead
      title="会员与套餐"
      sub="套餐配置（改价/上下架） · 会员查询 · 手工补发硬币（金额单位：元）"
    />

    <div class="page-card">
      <el-tabs v-model="activeTab" @tab-change="(n: string) => n === 'members' && openMembers()">
        <!-- Tab1 套餐配置 -->
        <el-tab-pane label="套餐配置" name="plans">
          <div class="flex items-center justify-between mb-4">
            <span class="text-3 text-text-2">共 {{ plans.length }} 个套餐（含已下架）</span>
            <el-button v-perm="'vip:plan'" type="primary" class="pink-btn" @click="openCreate">
              新建套餐
            </el-button>
          </div>

          <el-table v-loading="plansLoading" :data="plans" stripe>
            <el-table-column label="套餐" min-width="180">
              <template #default="{ row }">
                <div class="text-3.5 font-600">{{ row.name }}</div>
                <div class="text-3 text-text-2">{{ row.code }} · {{ row.months }} 个月</div>
              </template>
            </el-table-column>
            <el-table-column label="售价" width="110">
              <template #default="{ row }">
                <span class="font-600">¥{{ fenToYuan(row.price) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="原价" width="110">
              <template #default="{ row }">
                <span v-if="row.original_price">¥{{ fenToYuan(row.original_price) }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column label="首月优惠" width="110">
              <template #default="{ row }">
                <span v-if="row.first_month_price">¥{{ fenToYuan(row.first_month_price) }}</span>
                <span v-else class="text-text-3">—</span>
              </template>
            </el-table-column>
            <el-table-column label="连续包月" width="100">
              <template #default="{ row }">
                <el-tag :type="row.auto_renew ? 'success' : 'info'" size="small">
                  {{ row.auto_renew ? '支持' : '不支持' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="sort" label="排序" width="80" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="isOnShelf(row) ? 'success' : 'info'" size="small">
                  {{ isOnShelf(row) ? '已上架' : '已下架' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <el-button v-perm="'vip:plan'" link type="primary" @click="openEdit(row)">
                  编辑
                </el-button>
                <el-button v-perm="'vip:plan'" link @click="toggleShelf(row)">
                  {{ isOnShelf(row) ? '下架' : '上架' }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- Tab2 会员查询 -->
        <el-tab-pane label="会员查询" name="members">
          <div class="flex items-center gap-2 mb-4">
            <el-input
              v-model.number="mUserId"
              class="w-200px"
              placeholder="按用户 ID 查询"
              clearable
              @keyup.enter="searchMembers"
            />
            <el-checkbox v-model="mOnlyVip" @change="searchMembers"> 只看生效中 </el-checkbox>
            <el-button class="pink-btn" @click="searchMembers"> 查询 </el-button>
            <span class="text-3 text-text-2">共 {{ mTotal }} 条</span>
          </div>

          <el-table v-loading="mLoading" :data="members" stripe>
            <el-table-column prop="user_id" label="用户 ID" min-width="160" />
            <el-table-column label="会员状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.is_vip ? 'success' : 'info'" size="small">
                  {{ row.is_vip ? '生效中' : '已过期' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="level" label="等级" width="80" />
            <el-table-column prop="expire_at" label="到期时间" min-width="160" />
            <el-table-column prop="started_at" label="开通时间" min-width="160" />
            <el-table-column prop="total_months" label="累计月数" width="100" />
            <el-table-column label="连续包月" width="100">
              <template #default="{ row }">
                <el-tag :type="row.auto_renew ? 'success' : 'info'" size="small">
                  {{ row.auto_renew ? '已签约' : '未签约' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-perm="'vip:grant'"
                  size="small"
                  type="warning"
                  @click="grantCoins(row)"
                >
                  补发硬币
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              layout="total, prev, pager, next"
              :total="mTotal"
              :page-size="mPageSize"
              :current-page="mPage"
              @current-change="onMPageChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 套餐编辑弹窗：金额以元填写，提交前转分 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑套餐' : '新建套餐'" width="520px">
      <el-form label-width="110px" class="max-w-480px">
        <el-form-item label="套餐编码">
          <el-input
            v-model="form.code"
            :disabled="editing"
            placeholder="如 vip_month（保存后不可改）"
          />
        </el-form-item>
        <el-form-item label="套餐名称">
          <el-input v-model="form.name" placeholder="如 大会员 月度" />
        </el-form-item>
        <el-form-item label="月数">
          <el-input-number v-model="form.months" :min="1" :max="120" />
        </el-form-item>
        <el-form-item label="售价（元）">
          <el-input v-model="form.priceYuan" placeholder="如 15.00" />
        </el-form-item>
        <el-form-item label="原价（元）">
          <el-input v-model="form.originalYuan" placeholder="0 或不填 = 不展示原价" />
        </el-form-item>
        <el-form-item label="首月价（元）">
          <el-input v-model="form.firstMonthYuan" placeholder="0 或不填 = 无首月优惠" />
        </el-form-item>
        <el-form-item label="连续包月">
          <el-switch v-model="form.autoRenew" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="PLAN_ON"> 上架 </el-radio>
            <el-radio :value="PLAN_OFF"> 下架 </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false"> 取消 </el-button>
        <el-button type="primary" class="pink-btn" :loading="saving" @click="save">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
