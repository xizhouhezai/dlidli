<script setup lang="ts">
// 弹幕设置面板（不透明度 / 字号 / 显示区域 / 滚动速度 / 同屏密度）。
// dm 为父级 useDanmakuController 的返回对象，解构 ref 成员保持响应性。
import type { useDanmakuController } from '@/composables/video/useDanmakuController'

type DmCtl = ReturnType<typeof useDanmakuController>

const props = defineProps<{ dm: DmCtl }>()

const { dmSettings, dmSettingsVisible, AREA_OPTIONS, SPEED_OPTIONS, DENSITY_OPTIONS } = props.dm
</script>

<template>
  <el-dialog v-model="dmSettingsVisible" title="弹幕设置" width="420px" top="18vh">
    <div class="dm-settings">
      <div class="dm-settings__row">
        <span class="dm-settings__label">不透明度</span>
        <el-slider v-model="dmSettings.opacity" :min="0.2" :max="1" :step="0.05" class="flex-1" />
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">字号</span>
        <el-slider
          v-model="dmSettings.fontScale"
          :min="0.8"
          :max="1.5"
          :step="0.05"
          class="flex-1"
        />
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">显示区域</span>
        <el-radio-group v-model="dmSettings.area">
          <el-radio-button v-for="o in AREA_OPTIONS" :key="o.value" :value="o.value">
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">滚动速度</span>
        <el-radio-group v-model="dmSettings.speed">
          <el-radio-button v-for="o in SPEED_OPTIONS" :key="o.value" :value="o.value">
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">同屏密度</span>
        <el-radio-group v-model="dmSettings.density">
          <el-radio-button v-for="o in DENSITY_OPTIONS" :key="o.value" :value="o.value">
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

/* 弹幕设置面板 */
.dm-settings {
  display: grid;
  gap: 16px;
}

.dm-settings__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.dm-settings__label {
  flex-shrink: 0;
  width: 64px;
  font-size: 13.5px;
  color: v.$text-1;
}
</style>
