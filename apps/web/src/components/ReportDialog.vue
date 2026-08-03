<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, type ReportReasonType } from '@dlidli/api-client'
import { api } from '@/api'

const props = defineProps<{ targetType: 1 | 2 | 3 | 4 | 5; targetId: string; title?: string }>()

const visible = ref(false)
const reasonType = ref<ReportReasonType>(1)
const reason = ref('')
const submitting = ref(false)

const REASONS: { value: ReportReasonType; label: string }[] = [
  { value: 1, label: '违法违规' },
  { value: 2, label: '色情低俗' },
  { value: 3, label: '人身攻击' },
  { value: 4, label: '垃圾广告' },
  { value: 5, label: '剧透' },
  { value: 6, label: '其他' },
]

function open() {
  reasonType.value = 1
  reason.value = ''
  visible.value = true
}

async function submit() {
  submitting.value = true
  try {
    await api.report.submit({
      target_type: props.targetType,
      target_id: props.targetId,
      reason_type: reasonType.value,
      reason: reason.value.trim(),
    })
    ElMessage.success('举报已提交，感谢你的反馈')
    visible.value = false
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '提交失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({ open })
</script>

<template>
  <el-dialog
    v-model="visible"
    title="举报"
    width="420px"
  >
    <div class="report-dialog">
      <p
        v-if="title"
        class="report-dialog__target"
      >
        {{ title }}
      </p>
      <el-radio-group v-model="reasonType">
        <el-radio
          v-for="r in REASONS"
          :key="r.value"
          :value="r.value"
        >
          {{ r.label }}
        </el-radio>
      </el-radio-group>
      <el-input
        v-model="reason"
        type="textarea"
        :rows="3"
        maxlength="500"
        show-word-limit
        placeholder="补充说明（选填）"
      />
    </div>
    <template #footer>
      <el-button @click="visible = false">
        取消
      </el-button>
      <el-button
        type="primary"
        :loading="submitting"
        @click="submit"
      >
        提交举报
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.report-dialog__target {
  margin: 0 0 12px;
  font-size: 13px;
  color: v.$text-2;
  @include v.ellipsis(1);
}

.report-dialog :deep(.el-radio-group) {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px 4px;
  margin-bottom: 12px;
}
</style>
