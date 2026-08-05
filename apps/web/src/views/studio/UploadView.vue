<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError, type CategoryItem, type VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'
import { uploadVideoFile, type UploadProgress } from '@/utils/uploader'
import { captureVideoPoster } from '@/utils/poster'

const router = useRouter()

// 上传状态
const file = ref<File | null>(null)
const fileId = ref('')
const progress = ref<UploadProgress | null>(null)
const uploading = ref(false)
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement>()

// 多P投稿（PRD VID-05）：分P列表（标题 + 已上传 fileId）
interface PartDraft {
  title: string
  fileId: string
  uploading: boolean
  progress: UploadProgress | null
}
const parts = ref<PartDraft[]>([])

function addPart() {
  if (parts.value.length >= 10) {
    ElMessage.warning('最多 10 个分P')
    return
  }
  parts.value.push({ title: '', fileId: '', uploading: false, progress: null })
}

function removePart(i: number) {
  parts.value.splice(i, 1)
}

async function uploadPart(i: number, f: File) {
  const ext = f.name.slice(f.name.lastIndexOf('.')).toLowerCase()
  if (!ACCEPT_EXTS.includes(ext)) {
    ElMessage.warning(`仅支持 ${ACCEPT_EXTS.join(' / ')} 格式`)
    return
  }
  const p = parts.value[i]
  p.uploading = true
  p.progress = null
  try {
    const res = await uploadVideoFile(f, (pr) => (p.progress = pr))
    p.fileId = res.fileId
    if (!p.title) p.title = f.name.slice(0, f.name.lastIndexOf('.')).slice(0, 80)
    ElMessage.success(`分P${i + 1} 上传完成`)
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '上传失败，请重试')
  } finally {
    p.uploading = false
  }
}

// 封面：优先视频首帧 poster → 用户上传 → 默认封面（展示时兜底）
const posterBlob = ref<Blob | null>(null)
const posterUrl = ref('')
const coverFile = ref<File | null>(null)
const coverUrl = ref('')
const selectedCover = ref<'poster' | 'custom' | ''>('')
const coverInput = ref<HTMLInputElement>()

// 表单
const categories = ref<CategoryItem[]>([])
const form = reactive({
  title: '',
  description: '',
  categoryId: undefined as number | undefined,
  tags: [] as string[],
  copyright: 1 as 1 | 2,
})
const tagInput = ref('')
const submitting = ref(false)
const published = ref<VideoDetail | null>(null)

const ACCEPT_EXTS = ['.mp4', '.mov', '.mkv', '.flv', '.avi']

onMounted(async () => {
  try {
    categories.value = (await api.video.categories()).filter((c) => c.parent_id === 0)
  } catch {
    ElMessage.error('分区加载失败')
  }
})

function pickFile() {
  fileInput.value?.click()
}

function onFileChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) startUpload(f)
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) startUpload(f)
}

async function startUpload(f: File) {
  const ext = f.name.slice(f.name.lastIndexOf('.')).toLowerCase()
  if (!ACCEPT_EXTS.includes(ext)) {
    ElMessage.warning(`仅支持 ${ACCEPT_EXTS.join(' / ')} 格式`)
    return
  }
  file.value = f
  fileId.value = ''
  uploading.value = true
  // 标题默认取文件名（去扩展名）
  if (!form.title) form.title = f.name.slice(0, f.name.lastIndexOf('.')).slice(0, 80)

  // 并行截取首帧作为封面首选（mkv/flv 等浏览器不支持的格式会失败，静默回退）
  captureVideoPoster(f).then((blob) => {
    if (blob) {
      posterBlob.value = blob
      posterUrl.value = URL.createObjectURL(blob)
      if (!selectedCover.value) selectedCover.value = 'poster'
    }
  })

  try {
    const res = await uploadVideoFile(f, (p) => (progress.value = p))
    fileId.value = res.fileId
    ElMessage.success('视频上传完成')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '上传失败，请重试')
    file.value = null
    progress.value = null
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

function pickCover() {
  coverInput.value?.click()
}

function onCoverChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  if (f.size > 5 * 1024 * 1024) {
    ElMessage.warning('封面大小须在 5MB 以内')
    return
  }
  coverFile.value = f
  coverUrl.value = URL.createObjectURL(f)
  selectedCover.value = 'custom'
  if (coverInput.value) coverInput.value.value = ''
}

/** 投稿前解析封面 URL：poster 优先 → 上传封面 → 空（展示端用默认封面） */
async function resolveCover(): Promise<string> {
  if (selectedCover.value === 'poster' && posterBlob.value) {
    return (await api.video.uploadCover(posterBlob.value, 'poster.jpg')).cover
  }
  if (selectedCover.value === 'custom' && coverFile.value) {
    return (await api.video.uploadCover(coverFile.value)).cover
  }
  return ''
}

function resetAll() {
  published.value = null
  file.value = null
  fileId.value = ''
  progress.value = null
  parts.value = []
  posterBlob.value = null
  posterUrl.value = ''
  coverFile.value = null
  coverUrl.value = ''
  selectedCover.value = ''
}

function addTag() {
  const t = tagInput.value.trim()
  if (!t) return
  if (form.tags.includes(t)) {
    tagInput.value = ''
    return
  }
  if (form.tags.length >= 10) {
    ElMessage.warning('最多 10 个标签')
    return
  }
  form.tags.push(t)
  tagInput.value = ''
}

async function onSubmit() {
  const validParts = parts.value.filter((p) => p.fileId)
  if (!fileId.value && validParts.length === 0) {
    ElMessage.warning('请先上传视频文件')
    return
  }
  if (!form.title.trim() || !form.categoryId || form.tags.length === 0) {
    ElMessage.warning('请填写标题、选择分区并至少添加 1 个标签')
    return
  }
  submitting.value = true
  try {
    const cover = await resolveCover()
    published.value = await api.video.submit({
      file_id: fileId.value,
      title: form.title.trim(),
      description: form.description,
      category_id: form.categoryId,
      tags: form.tags,
      copyright: form.copyright,
      cover,
      // 多P：有已上传分P时提交 parts（后端按分P建 video_part + 各自流）
      parts: validParts.map((p) => ({ file_id: p.fileId, title: p.title })),
    })
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '投稿失败，请重试')
  } finally {
    submitting.value = false
  }
}

const stageText: Record<UploadProgress['stage'], string> = {
  hash: '文件校验中',
  upload: '上传中',
  merge: '合并处理中',
}
</script>

<template>
  <div class="upload-wrap">
    <!-- 投稿成功 -->
    <el-result
      v-if="published"
      icon="success"
      title="投稿成功！"
      :sub-title="`稿件号 ${published.bvid} · ${published.status === 4 ? '已发布' : '审核中，通过后自动发布'}`"
    >
      <template #extra>
        <el-button
          type="primary"
          @click="router.push('/')"
        >
          回首页看看
        </el-button>
        <el-button @click="resetAll">
          再投一稿
        </el-button>
      </template>
    </el-result>

    <el-card
      v-else
      shadow="never"
    >
      <template #header>
        <span>视频投稿</span>
      </template>

      <!-- 第一步：上传文件 -->
      <div
        v-if="!file"
        class="drop-zone"
        :class="{ 'is-over': dragOver }"
        @click="pickFile"
        @dragover.prevent="dragOver = true"
        @dragleave="dragOver = false"
        @drop.prevent="onDrop"
      >
        <p class="drop-zone__icon">
          📤
        </p>
        <p class="drop-zone__text">
          点击或拖拽视频到此处上传
        </p>
        <p class="drop-zone__tip">
          支持 mp4 / mov / mkv / flv / avi，单文件 ≤ 8GB，支持断点续传与秒传
        </p>
        <input
          ref="fileInput"
          type="file"
          :accept="ACCEPT_EXTS.join(',')"
          hidden
          @change="onFileChange"
        >
      </div>

      <!-- 上传进度 -->
      <div
        v-else
        class="file-status"
      >
        <span class="file-status__name">🎬 {{ file.name }}</span>
        <template v-if="fileId">
          <el-tag type="success">
            上传完成
          </el-tag>
        </template>
        <template v-else-if="progress">
          <el-progress
            class="file-status__bar"
            :percentage="progress.percent"
            :status="progress.stage === 'merge' ? 'warning' : undefined"
          />
          <span class="file-status__stage">{{ stageText[progress.stage] }}</span>
        </template>
      </div>

      <el-divider />

      <!-- 分P管理（多P投稿 PRD VID-05） -->
      <div class="parts-block">
        <div class="parts-block__head">
          <span class="parts-block__title">分P管理</span>
          <span class="parts-block__tip">可选：添加多个视频组成合集式稿件（最多 10 P）</span>
          <el-button
            link
            type="primary"
            class="ml-auto"
            @click="addPart"
          >
            + 添加分P
          </el-button>
        </div>
        <div
          v-for="(p, i) in parts"
          :key="i"
          class="part-row"
        >
          <el-input
            v-model="p.title"
            :placeholder="`分P${i + 1} 标题（默认取文件名）`"
            maxlength="80"
            class="part-row__title"
          />
          <span
            v-if="p.fileId"
            class="part-row__done"
          >已上传 ✓</span>
          <span
            v-else-if="p.progress"
            class="part-row__progress"
          >{{ p.progress.percent }}% {{ stageText[p.progress.stage] }}</span>
          <label
            v-else
            class="part-row__pick"
          >
            选择视频
            <input
              type="file"
              :accept="ACCEPT_EXTS.join(',')"
              hidden
              @change="(e) => uploadPart(i, (e.target as HTMLInputElement).files?.[0] as File)"
            >
          </label>
          <el-button
            link
            type="danger"
            @click="removePart(i)"
          >
            删除
          </el-button>
        </div>
      </div>

      <el-divider />

      <!-- 第二步：稿件信息 -->
      <el-form
        label-width="80px"
        class="submit-form"
        :disabled="uploading"
      >
        <el-form-item
          label="标题"
          required
        >
          <el-input
            v-model="form.title"
            maxlength="80"
            show-word-limit
            placeholder="给视频起个好标题"
          />
        </el-form-item>

        <el-form-item
          label="分区"
          required
        >
          <el-select
            v-model="form.categoryId"
            placeholder="选择分区"
            style="width: 240px"
          >
            <el-option
              v-for="c in categories"
              :key="c.id"
              :label="c.name"
              :value="c.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item
          label="标签"
          required
        >
          <div class="tags-row">
            <el-tag
              v-for="t in form.tags"
              :key="t"
              closable
              @close="form.tags.splice(form.tags.indexOf(t), 1)"
            >
              {{ t }}
            </el-tag>
            <el-input
              v-model="tagInput"
              class="tag-input"
              placeholder="回车添加标签（1-10 个）"
              @keyup.enter="addTag"
            />
          </div>
        </el-form-item>

        <el-form-item label="封面">
          <div class="cover-row">
            <!-- 首帧 poster 候选 -->
            <div
              v-if="posterUrl"
              class="cover-option"
              :class="{ 'is-selected': selectedCover === 'poster' }"
              @click="selectedCover = 'poster'"
            >
              <img
                :src="posterUrl"
                alt="视频首帧"
              >
              <span class="cover-option__label">视频首帧</span>
            </div>

            <!-- 自定义上传候选 -->
            <div
              v-if="coverUrl"
              class="cover-option"
              :class="{ 'is-selected': selectedCover === 'custom' }"
              @click="selectedCover = 'custom'"
            >
              <img
                :src="coverUrl"
                alt="自定义封面"
              >
              <span class="cover-option__label">自定义</span>
            </div>

            <!-- 上传入口 -->
            <div
              class="cover-option cover-option--add"
              @click="pickCover"
            >
              <span>＋</span>
              <span class="cover-option__label">上传封面</span>
            </div>
            <input
              ref="coverInput"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              hidden
              @change="onCoverChange"
            >
          </div>
          <p class="cover-tip">
            选填：默认优先使用视频首帧，可上传自定义封面替换；都没有时展示 DliDli 默认封面
          </p>
        </el-form-item>

        <el-form-item label="类型">
          <el-radio-group v-model="form.copyright">
            <el-radio :value="1">
              自制
            </el-radio>
            <el-radio :value="2">
              转载
            </el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="简介">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            maxlength="2000"
            show-word-limit
            placeholder="介绍一下这个视频吧（选填）"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            class="submit-btn"
            :loading="submitting"
            :disabled="!fileId"
            @click="onSubmit"
          >
            {{ fileId ? '立即投稿' : '等待视频上传完成' }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
@use '@/styles/variables' as v;

.upload-wrap {
  max-width: 860px;
  margin: 0 auto;
}

/* 分P管理（多P投稿） */
.parts-block {
  margin-bottom: 4px;
}

.parts-block__head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.parts-block__title {
  font-size: 14px;
  font-weight: 600;
}

.parts-block__tip {
  font-size: 12px;
  color: v.$text-3;
}

.part-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.part-row__title {
  flex: 1;
}

.part-row__done {
  color: #10b981;
  font-size: 13px;
  flex-shrink: 0;
}

.part-row__progress {
  color: v.$text-2;
  font-size: 12px;
  flex-shrink: 0;
}

.part-row__pick {
  color: v.$primary;
  font-size: 13px;
  cursor: pointer;
  flex-shrink: 0;
}

.drop-zone {
  border: 2px dashed #d0d4d9;
  border-radius: 12px;
  padding: 48px 24px;
  text-align: center;
  cursor: pointer;
  transition: all 0.15s;
}

.drop-zone:hover,
.drop-zone.is-over {
  border-color: var(--dli-primary);
  background: #fff7fa;
}

.drop-zone__icon {
  font-size: 40px;
  margin: 0;
}

.drop-zone__text {
  font-size: 16px;
  font-weight: 600;
  margin: 12px 0 4px;
}

.drop-zone__tip {
  font-size: 12px;
  color: var(--dli-text-2);
  margin: 0;
}

.file-status {
  display: flex;
  align-items: center;
  gap: 16px;
}

.file-status__name {
  font-size: 14px;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-status__bar {
  flex: 1;
}

.file-status__stage {
  font-size: 12px;
  color: var(--dli-text-2);
  flex-shrink: 0;
}

.tags-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  width: 100%;
}

.tag-input {
  width: 200px;
}

/* 封面候选 */
.cover-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.cover-option {
  position: relative;
  width: 160px;
  aspect-ratio: 16 / 10;
  border-radius: 8px;
  overflow: hidden;
  border: 2px solid transparent;
  cursor: pointer;
  transition: border-color 0.15s;
}

.cover-option img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.cover-option.is-selected {
  border-color: var(--dli-primary);
}

.cover-option__label {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 2px 0;
  text-align: center;
  font-size: 12px;
  color: #fff;
  background: rgba(0, 0, 0, 0.45);
}

.cover-option--add {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 2px dashed #d0d4d9;
  color: var(--dli-text-2);
  font-size: 24px;
}

.cover-option--add:hover {
  border-color: var(--dli-primary);
  color: var(--dli-primary);
}

.cover-option--add .cover-option__label {
  position: static;
  background: none;
  color: inherit;
  font-size: 12px;
}

.cover-tip {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
  width: 100%;
}

.submit-btn {
  min-width: 160px;
  --el-button-bg-color: var(--dli-primary);
  --el-button-border-color: var(--dli-primary);
  --el-button-hover-bg-color: #fc8bab;
  --el-button-hover-border-color: #fc8bab;
}
</style>
