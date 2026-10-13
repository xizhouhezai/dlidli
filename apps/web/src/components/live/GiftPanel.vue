<script setup lang="ts">
// 礼物面板（M4-LIV-04，LIV-13）：礼物列表 + 金瓜子余额 + 连击赠送。
// 余额不足（91021）提示引导；dev 直充提示为临时口径（PAY-01 接入后改为真实充值入口）。
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, type LiveGiftItem } from '@dlidli/api-client'
import { api } from '@/api'
import { readToken } from '@/utils/token'

const props = defineProps<{ roomId: string }>()
const emit = defineEmits<{ sent: [gift: LiveGiftItem, count: number] }>()

const gifts = ref<LiveGiftItem[]>([])
const balance = ref<number | null>(null)
const selected = ref<LiveGiftItem | null>(null)
const count = ref(1)
const sending = ref(false)

const loggedIn = computed(() => !!readToken())

onMounted(async () => {
  try {
    gifts.value = await api.live.gifts()
  } catch {
    /* 礼物列表加载失败静默，面板显示空态 */
  }
  if (loggedIn.value) {
    void refreshBalance()
  }
})

async function refreshBalance() {
  try {
    const res = await api.live.melonBalance()
    balance.value = res.balance
  } catch {
    /* silent */
  }
}

async function send() {
  if (!selected.value || sending.value) return
  sending.value = true
  try {
    const res = await api.live.sendGift(props.roomId, selected.value.id, count.value)
    balance.value = res.balance
    ElMessage.success(`送出 ${selected.value.name} ×${count.value}`)
    emit('sent', selected.value, count.value)
  } catch (err) {
    if (err instanceof ApiError && err.code === 91021) {
      ElMessage.warning('金瓜子不足，请联系管理员直充（开发期）')
    } else if (err instanceof ApiError && err.code === 91020) {
      ElMessage.warning('房间未在直播，无法赠送')
    } else {
      ElMessage.error('赠送失败，请重试')
    }
  } finally {
    sending.value = false
  }
}

defineExpose({ refreshBalance })
</script>

<template>
  <div class="gift-panel">
    <div class="gift-panel__head">
      <span class="i-mingcute-gift-line mr-1" />礼物
      <span v-if="balance !== null" class="gift-panel__balance">
        <span class="i-mingcute-coin-2-line mr-0.5" />{{ balance }}
      </span>
      <span v-else-if="!loggedIn" class="gift-panel__balance">登录后可送礼</span>
    </div>

    <div class="gift-panel__grid">
      <button
        v-for="g in gifts"
        :key="g.id"
        class="gift-panel__item"
        :class="{ 'gift-panel__item--on': selected?.id === g.id }"
        @click="selected = g"
      >
        <span class="gift-panel__icon" :data-level="g.level">{{ g.name.slice(0, 2) }}</span>
        <span class="gift-panel__name">{{ g.name }}</span>
        <span class="gift-panel__price">
          <span class="i-mingcute-coin-2-line" />{{ g.price }}
        </span>
      </button>
    </div>

    <div class="gift-panel__foot">
      <el-input-number v-model="count" :min="1" :max="999" size="small" />
      <el-button
        type="primary"
        size="small"
        :disabled="!selected || !loggedIn || sending"
        @click="send"
      >
        赠送
      </el-button>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.gift-panel {
  border-top: 1px solid v.$border;
  padding: 8px 10px;
  background: #fafafa;
}

.gift-panel__head {
  display: flex;
  align-items: center;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}

.gift-panel__balance {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  font-size: 12px;
  font-weight: 500;
  color: #d98b00;
}

.gift-panel__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
  max-height: 168px;
  overflow-y: auto;
}

.gift-panel__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 8px 4px;
  border: 1px solid v.$border;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
  transition: border-color 0.15s;

  &:hover {
    border-color: v.$primary;
  }

  &--on {
    border-color: v.$primary;
    box-shadow: 0 0 0 1px v.$primary;
  }
}

.gift-panel__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #f9a7b9, #f06d92);

  &[data-level='2'] {
    background: linear-gradient(135deg, #ffd66b, #f59e0b);
  }
}

.gift-panel__name {
  font-size: 12px;
  color: v.$text-1;
}

.gift-panel__price {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-size: 11px;
  color: #d98b00;
}

.gift-panel__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
}
</style>
