import { ref, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError, type DanmakuItem, type VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import DanmakuLayer from '@/components/DanmakuLayer.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import { useDanmakuSettings, type DanmakuSettings } from '@/composables/useDanmakuSettings'

/**
 * 弹幕控制器（VideoView 拆分，M3-ENG-06）：
 * 发送器（模式/颜色/级别门槛）、列表面板（加载/定位/颜色可读映射）、设置面板、弹幕举报。
 */
export function useDanmakuController(
  detail: Ref<VideoDetail | null>,
  videoEl: Ref<HTMLVideoElement | undefined>,
) {
  const router = useRouter()
  const userStore = useUserStore()

  const dmEnabled = ref(true)
  const dmInput = ref('')
  const dmSending = ref(false)
  const dmLayer = ref<InstanceType<typeof DanmakuLayer>>()
  const dmSettings = useDanmakuSettings().settings

  // 发送工具条（M2-DM-01）：模式 + 颜色（非白/顶底需 Lv3）
  const dmMode = ref<1 | 2 | 3>(1)
  const dmColor = ref(0xffffff)
  const DM_COLORS: Array<{ name: string; value: number }> = [
    { name: '白', value: 0xffffff },
    { name: '红', value: 0xff0000 },
    { name: '橙', value: 0xff7f00 },
    { name: '黄', value: 0xffff00 },
    { name: '绿', value: 0x00ff00 },
    { name: '蓝', value: 0x00bfff },
    { name: '紫', value: 0xff00ff },
    { name: '粉', value: 0xff69b4 },
  ]
  const isDmLevel3 = () => (userStore.profile?.level ?? 0) >= 3

  // 弹幕列表面板（右侧内嵌，M3-DM-01 列表）
  const dmList = ref<DanmakuItem[]>([])
  const dmListTotal = ref(0)
  const dmListPage = ref(1)
  const dmListLoading = ref(false)

  async function loadDmList(reset = false) {
    if (reset) {
      dmListPage.value = 1
      dmList.value = []
    }
    if (!detail.value) return
    dmListLoading.value = true
    try {
      const res = await api.danmaku.listAll(detail.value.bvid, dmListPage.value, 50)
      dmList.value = reset ? res.list : [...dmList.value, ...res.list]
      dmListTotal.value = res.total
    } catch {
      ElMessage.error('弹幕列表加载失败')
    } finally {
      dmListLoading.value = false
    }
  }

  function dmSeekTo(timeMs: number) {
    const video = videoEl.value
    if (!video) return
    video.currentTime = timeMs / 1000
  }

  /** 弹幕文字颜色：浅色底上白/近白弹幕映射为深灰，保证可读（彩色保留）。 */
  function dmTextColor(color: number): string {
    const hex = (color || 0xffffff).toString(16).padStart(6, '0')
    const r = parseInt(hex.slice(0, 2), 16)
    const g = parseInt(hex.slice(2, 4), 16)
    const b = parseInt(hex.slice(4, 6), 16)
    const lum = 0.299 * r + 0.587 * g + 0.114 * b
    return lum > 200 ? '#333' : '#' + hex
  }

  // 弹幕设置面板
  const dmSettingsVisible = ref(false)
  const AREA_OPTIONS: Array<{ label: string; value: DanmakuSettings['area'] }> = [
    { label: '1/4 屏', value: 'quarter' },
    { label: '半屏', value: 'half' },
    { label: '全屏', value: 'full' },
  ]
  const SPEED_OPTIONS: Array<{ label: string; value: DanmakuSettings['speed'] }> = [
    { label: '慢', value: 'slow' },
    { label: '标准', value: 'normal' },
    { label: '快', value: 'fast' },
  ]
  const DENSITY_OPTIONS: Array<{ label: string; value: DanmakuSettings['density'] }> = [
    { label: '低', value: 'low' },
    { label: '标准', value: 'normal' },
    { label: '高', value: 'high' },
  ]

  // 弹幕举报（复用 ReportDialog，target_type=3）
  const dmReportDialog = ref<InstanceType<typeof ReportDialog> | null>(null)
  const dmReportItem = ref<DanmakuItem | null>(null)
  function onDmReport(item: DanmakuItem) {
    dmReportItem.value = item
    dmReportDialog.value?.open()
  }

  async function sendDanmaku() {
    if (!userStore.token) {
      router.push('/login')
      return
    }
    const content = dmInput.value.trim()
    if (!content || !detail.value) return
    dmSending.value = true
    try {
      const item = await api.danmaku.send(detail.value.bvid, {
        content,
        time_ms: Math.floor((videoEl.value?.currentTime ?? 0) * 1000),
        mode: dmMode.value,
        color: dmColor.value,
      })
      dmLayer.value?.inject(item) // 乐观上屏
      dmInput.value = ''
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '发送失败，请重试')
    } finally {
      dmSending.value = false
    }
  }

  return {
    dmEnabled,
    dmInput,
    dmSending,
    dmLayer,
    dmSettings,
    dmMode,
    dmColor,
    DM_COLORS,
    isDmLevel3,
    dmList,
    dmListTotal,
    dmListPage,
    dmListLoading,
    loadDmList,
    dmSeekTo,
    dmTextColor,
    dmSettingsVisible,
    AREA_OPTIONS,
    SPEED_OPTIONS,
    DENSITY_OPTIONS,
    dmReportDialog,
    dmReportItem,
    onDmReport,
    sendDanmaku,
  }
}
