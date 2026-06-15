<template>
  <div class="min-h-full bg-gray-50 dark:bg-dark-900">
    <div class="mx-auto max-w-5xl px-4 py-5">
      <!-- 顶部栏：logo（→ /usage） / tabs / 秘钥 -->
      <div class="mb-6 flex items-center justify-between gap-3">
        <div class="flex shrink-0 items-center gap-3">
          <router-link to="/usage" class="flex items-center gap-2">
            <img :src="siteLogo || '/logo.png'" alt="logo" class="h-8 w-8 rounded-lg object-contain" />
            <span class="hidden text-sm font-semibold text-gray-700 dark:text-gray-200 md:inline">{{ siteName }}</span>
          </router-link>
          <button
            class="inline-flex items-center gap-1 rounded-full bg-gray-900 px-3 py-2 text-xs font-medium text-white transition hover:bg-gray-700 dark:bg-dark-700 dark:hover:bg-dark-600"
            @click="newSession"
          >
            + {{ t('imageWorkbench.newSession') }}
          </button>
          <router-link
            to="/image-workbench/tasks"
            class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
          >
            <Icon name="clock" size="sm" />
            <span class="hidden sm:inline">{{ t('imageWorkbench.viewTasks') }}</span>
            <span v-if="queueTasks.length" class="rounded-full bg-primary-100 px-1.5 text-[10px] text-primary-600 dark:bg-primary-900/40">{{ queueTasks.length }}</span>
          </router-link>
        </div>

        <div class="flex rounded-full bg-gray-100 p-1 text-sm dark:bg-dark-800">
          <button
            class="rounded-full px-4 py-1.5 font-medium transition"
            :class="tab === 'studio' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500'"
            @click="tab = 'studio'"
          >
            {{ t('imageWorkbench.studio') }}
          </button>
          <button
            class="rounded-full px-4 py-1.5 font-medium transition"
            :class="tab === 'gallery' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500'"
            @click="tab = 'gallery'"
          >
            {{ t('imageWorkbench.gallery') }}
          </button>
        </div>

        <div v-if="imageKeys.length" class="flex shrink-0 items-center gap-1.5">
          <span class="hidden text-xs text-gray-400 sm:inline">{{ t('imageWorkbench.apiKey') }}</span>
          <select
            v-model="selectedKeyId"
            class="input h-10 max-w-[150px] text-sm"
            :title="t('imageWorkbench.apiKeyHint')"
          >
            <option v-for="k in imageKeys" :key="k.id" :value="k.id">
              {{ k.name || ('#' + k.id) }}
            </option>
          </select>
        </div>
        <span v-else class="w-8 shrink-0"></span>
      </div>

      <!-- 无可用秘钥引导 -->
      <div
        v-if="!loadingKeys && imageKeys.length === 0"
        class="rounded-2xl border border-dashed border-gray-300 bg-white p-12 text-center dark:border-dark-600 dark:bg-dark-800"
      >
        <p class="text-gray-700 dark:text-gray-200">{{ t('imageWorkbench.noKeyTitle') }}</p>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.noKeyHint') }}</p>
        <router-link
          to="/keys"
          class="mt-4 inline-flex items-center rounded-full bg-primary-600 px-5 py-2 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ t('imageWorkbench.createKey') }}
        </router-link>
      </div>

      <template v-else>
        <!-- ===== 创作台 ===== -->
        <div v-show="tab === 'studio'">
          <!-- Hero + 模板（空态） -->
          <div v-if="chatMessages.length === 0" class="pb-4 pt-2 text-center">
            <h1 class="text-3xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-4xl">
              {{ t('imageWorkbench.heroTitle') }}
            </h1>
            <p class="mx-auto mt-3 max-w-xl text-sm text-gray-500 dark:text-gray-400">
              {{ t('imageWorkbench.heroSubtitle') }}
            </p>
            <div class="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <button
                v-for="p in presets"
                :key="p.id"
                class="group overflow-hidden rounded-2xl border border-gray-200 bg-white text-left transition hover:-translate-y-0.5 hover:shadow-md dark:border-dark-600 dark:bg-dark-800"
                @click="usePreset(p)"
              >
                <div class="relative aspect-[4/3] overflow-hidden bg-gray-100 dark:bg-dark-700">
                  <img :src="p.image" :alt="p.title" loading="lazy" class="h-full w-full object-cover transition duration-300 group-hover:scale-105" />
                  <span class="absolute right-1.5 top-1.5 rounded bg-black/55 px-1.5 py-0.5 text-[10px] text-white">{{ p.size }}</span>
                </div>
                <div class="p-2.5">
                  <div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ p.title }}</div>
                  <div class="mt-0.5 truncate text-xs text-gray-400">{{ p.hint }}</div>
                  <div class="mt-1.5 text-xs font-medium text-primary-600">{{ t('imageWorkbench.useTemplate') }} →</div>
                </div>
              </button>
            </div>
          </div>

          <!-- 对话结果 -->
          <div v-else ref="chatRef" class="mb-4 max-h-[55vh] space-y-5 overflow-y-auto">
            <div v-for="(msg, idx) in chatMessages" :key="msg.id ?? idx">
              <div class="mb-1.5 flex items-end justify-end gap-2">
                <img v-if="msg.inputPreview" :src="msg.inputPreview" class="h-10 w-10 rounded-lg object-cover" />
                <span class="inline-block max-w-[80%] rounded-2xl bg-primary-600 px-3.5 py-2 text-sm text-white">{{ msg.prompt }}</span>
              </div>
              <div v-if="msg.status === 'queued'" class="flex items-center gap-2 text-sm text-gray-400">
                <span class="h-2 w-2 rounded-full bg-gray-300"></span>
                <span>{{ t('imageWorkbench.statusQueued') }}</span>
              </div>
              <div v-else-if="msg.status === 'running'" class="flex items-center gap-2 text-sm text-gray-400">
                <span class="h-2 w-2 animate-pulse rounded-full bg-primary-500"></span>
                <span>{{ t('imageWorkbench.generating') }}</span>
                <span class="tabular-nums font-medium text-primary-500">{{ formatElapsed(msg.elapsed) }}</span>
              </div>
              <div v-else-if="msg.status === 'error'" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-500 dark:bg-red-900/20">
                {{ msg.error }}
                <button class="ml-2 font-medium underline hover:text-red-600" @click="retry(msg)">{{ t('imageWorkbench.retry') }}</button>
              </div>
              <div v-else class="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
                <div
                  v-for="img in msg.images"
                  :key="img.id"
                  class="group relative overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600"
                >
                  <img :src="img.url" :alt="img.prompt" class="aspect-square w-full object-cover" loading="lazy" />
                  <div class="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-gradient-to-t from-black/60 to-transparent p-1.5 opacity-0 transition group-hover:opacity-100">
                    <span class="text-[10px] text-white">{{ remainingLabel(img.expires_at) }}</span>
                    <span class="flex gap-1">
                      <button class="rounded bg-white/25 px-1.5 py-0.5 text-[10px] text-white hover:bg-white/45" @click="editFrom(img)">{{ t('imageWorkbench.editThis') }}</button>
                      <a :href="img.url" :download="'image-' + img.id + '.png'" class="rounded bg-white/25 px-1.5 py-0.5 text-[10px] text-white hover:bg-white/45">{{ t('imageWorkbench.download') }}</a>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 任务队列：排队中 / 执行中（服务端异步执行，刷新不丢） -->
          <div v-if="queueTasks.length" class="mb-2 rounded-xl border border-gray-200 bg-white p-2.5 dark:border-dark-600 dark:bg-dark-800">
            <div class="mb-1.5 flex items-center gap-1.5 text-xs font-semibold text-gray-600 dark:text-gray-300">
              <Icon name="clock" size="sm" />
              <span>{{ t('imageWorkbench.taskQueue') }}</span>
              <span class="rounded-full bg-primary-100 px-1.5 text-[10px] text-primary-600 dark:bg-primary-900/40">{{ queueTasks.length }}</span>
              <router-link to="/image-workbench/tasks" class="ml-auto text-[11px] font-normal text-primary-600 hover:underline">{{ t('imageWorkbench.viewAll') }} →</router-link>
            </div>
            <div class="space-y-1">
              <div
                v-for="task in queueTasks"
                :key="task.id"
                class="flex items-center gap-2 rounded-lg border border-gray-100 px-2 py-1 text-xs dark:border-dark-700"
              >
                <span v-if="task.status === 'running'" class="h-2 w-2 shrink-0 animate-pulse rounded-full bg-primary-500"></span>
                <span v-else class="h-2 w-2 shrink-0 rounded-full bg-gray-300"></span>
                <span class="flex-1 truncate text-gray-600 dark:text-gray-300">{{ task.prompt }}</span>
                <span v-if="task.status === 'running'" class="tabular-nums text-primary-500">{{ formatElapsed(task.elapsed) }}</span>
                <span v-else class="text-gray-400">{{ t('imageWorkbench.statusQueued') }}</span>
              </div>
            </div>
          </div>

          <!-- 风格快捷 -->
          <div class="mb-2 flex flex-wrap gap-1.5">
            <button
              v-for="st in styles"
              :key="st"
              class="rounded-full border border-gray-200 bg-white px-2.5 py-0.5 text-xs text-gray-500 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800"
              @click="appendStyle(st)"
            >
              {{ st }}
            </button>
          </div>

          <!-- 编辑底图提示（上传可多张，最多 4）-->
          <div v-if="isEditing" class="mb-2 flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300">
            <div class="flex flex-wrap gap-1.5">
              <div v-for="(p, i) in editPreviews" :key="i" class="relative">
                <img :src="p" class="h-9 w-9 rounded-lg object-cover" />
                <button
                  v-if="uploadedImages.length"
                  class="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[10px] leading-none text-white"
                  @click="removeUploaded(i)"
                >
                  ×
                </button>
              </div>
            </div>
            <span>{{ t('imageWorkbench.editingBase') }}</span>
            <button class="ml-auto underline" @click="clearBase">{{ t('imageWorkbench.cancelEdit') }}</button>
          </div>

          <!-- 输入框（突出圆角卡片 + 内置工具条）-->
          <div class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-600 dark:bg-dark-800">
            <textarea
              v-model="prompt"
              rows="2"
              :maxlength="PROMPT_MAX"
              class="w-full resize-none border-0 bg-transparent px-1 text-sm text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
              :placeholder="t('imageWorkbench.promptPlaceholder')"
              @keydown.enter.exact.prevent="send"
            ></textarea>
            <div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-3">
              <button
                class="flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 text-gray-500 hover:border-primary-400 hover:text-primary-600 dark:border-dark-600"
                :title="t('imageWorkbench.upload')"
                @click="pickFile"
              >
                <Icon name="upload" size="sm" />
              </button>
              <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onFile" />
              <span class="rounded-lg bg-gray-100 px-2.5 py-2.5 text-xs text-gray-500 dark:bg-dark-700">{{ MODEL }}</span>
              <label class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                <span class="whitespace-nowrap">{{ t('imageWorkbench.size') }}</span>
                <select v-model="size" class="input h-10 text-sm">
                  <optgroup v-for="g in sizeGroups" :key="g.label" :label="g.label">
                    <option v-for="s in g.sizes" :key="s" :value="s">{{ s }}</option>
                  </optgroup>
                </select>
              </label>
              <label class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                <span class="whitespace-nowrap">{{ t('imageWorkbench.count') }}</span>
                <select v-model.number="count" class="input h-10 w-16 text-sm">
                  <option v-for="n in [1, 2, 3, 4]" :key="n" :value="n">x{{ n }}</option>
                </select>
              </label>
              <span
                class="ml-auto tabular-nums text-xs"
                :class="prompt.length >= PROMPT_MAX ? 'text-red-500' : 'text-gray-400'"
              >
                {{ prompt.length }}/{{ PROMPT_MAX }}
              </span>
              <button
                class="flex h-11 w-11 items-center justify-center rounded-full bg-primary-600 text-white transition hover:bg-primary-700 disabled:opacity-40"
                :disabled="!prompt.trim() || !selectedKeyId || submitting"
                :title="isEditing ? t('imageWorkbench.edit') : t('imageWorkbench.generate')"
                @click="send"
              >
                <Icon name="arrowUp" size="sm" />
              </button>
            </div>
          </div>
        </div>

        <!-- ===== 图片库 ===== -->
        <div v-show="tab === 'gallery'">
          <div v-if="historyImages.length === 0" class="rounded-2xl border border-dashed border-gray-200 py-16 text-center text-sm text-gray-400 dark:border-dark-600">
            {{ t('imageWorkbench.noHistory') }}
          </div>
          <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-4 md:grid-cols-5">
            <div v-for="img in historyImages" :key="img.id" class="group relative overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
              <img :src="img.url" :alt="img.prompt" class="aspect-square w-full cursor-pointer object-cover" loading="lazy" @click="editFromGallery(img)" />
              <button class="absolute right-1 top-1 hidden rounded-full bg-black/50 px-1.5 text-xs text-white group-hover:block" @click="removeImage(img)">×</button>
              <div class="absolute inset-x-0 bottom-0 hidden items-center justify-between gap-1 bg-gradient-to-t from-black/60 to-transparent px-1.5 py-1 group-hover:flex">
                <span class="text-[10px] text-white">{{ remainingLabel(img.expires_at) }}</span>
                <a
                  :href="img.url"
                  :download="'image-' + img.id + '.png'"
                  class="rounded bg-white/25 px-1.5 py-0.5 text-[10px] text-white hover:bg-white/45"
                  @click.stop
                >
                  {{ t('imageWorkbench.download') }}
                </a>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import keysAPI from '@/api/keys'
import userChannelsAPI from '@/api/channels'
import imageWorkbenchAPI, { type WorkbenchImage, type WorkbenchTask, type GenerateParams } from '@/api/imageWorkbench'
import { useAppStore } from '@/stores/app'
import type { ApiKey } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const MODEL = 'gpt-image-2'
const PROMPT_MAX = 10000 // 图片描述字符上限
const MAX_ACTIVE_TASKS = 20 // 每用户队列(排队+执行中)上限（服务端同样限制）
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo)

interface PromptPreset {
  id: string
  title: string
  hint: string
  image: string // 样例图（自托管于 public/presets）
  size: string // 宽高比，如 9:16
  prompt: string
}

function formatElapsed(s?: number): string {
  const v = s || 0
  if (v < 60) return `${v}s`
  return `${Math.floor(v / 60)}m${String(v % 60).padStart(2, '0')}s`
}

const tab = ref<'studio' | 'gallery'>('studio')
const loadingKeys = ref(true)
const keys = ref<ApiKey[]>([])
const selectedKeyId = ref<number | null>(null)
const size = ref('1024x1024')
const count = ref(1)

const sizeGroups = [
  { label: '1:1 正方形', sizes: ['1024x1024', '1536x1536', '2048x2048'] },
  { label: '3:2 横向', sizes: ['1536x1024', '1920x1280'] },
  { label: '2:3 竖向', sizes: ['1024x1536'] },
  { label: '16:9 横屏', sizes: ['1280x720', '1920x1080', '2560x1440', '3840x2160'] },
  { label: '9:16 竖屏', sizes: ['1080x1920', '1440x2560', '2160x3840'] },
  { label: '4:3', sizes: ['1600x1200', '2048x1536'] }
]

const prompt = ref('')
const submitting = ref(false)
const tasks = ref<WorkbenchTask[]>([]) // 服务端任务（desc：最新在前）
const historyImages = ref<WorkbenchImage[]>([])
const baseImage = ref<WorkbenchImage | null>(null)
const uploadedImages = ref<Array<{ b64: string; preview: string }>>([])
const fileInput = ref<HTMLInputElement | null>(null)
const chatRef = ref<HTMLElement | null>(null)
const sessionId = ref(Math.random().toString(36).slice(2, 14))
const previewMap = ref<Record<number, string>>({}) // 本会话提交任务的输入图预览（仅前端展示）
const hideBeforeMs = ref(0) // 「新建会话」后隐藏此前的任务，仅清空创作台视图
const nowTick = ref(Date.now())

const imageKeys = computed(() => keys.value.filter((k) => k.group?.platform === 'openai'))
const editPreviews = computed(() =>
  uploadedImages.value.length
    ? uploadedImages.value.map((u) => u.preview)
    : baseImage.value
      ? [baseImage.value.url]
      : []
)
const isEditing = computed(() => uploadedImages.value.length > 0 || !!baseImage.value)

function isActive(t: WorkbenchTask): boolean {
  return t.status === 'queued' || t.status === 'running'
}
function elapsedOf(ts: string): number {
  return Math.max(0, Math.floor((nowTick.value - new Date(ts).getTime()) / 1000))
}

// 创作台对话流：升序（旧→新），「新建会话」之后的任务才显示
const chatMessages = computed(() =>
  [...tasks.value]
    .filter((t) => new Date(t.created_at).getTime() >= hideBeforeMs.value)
    .reverse()
    .map((t) => ({
      id: t.id,
      prompt: t.prompt,
      status: t.status,
      error: t.error,
      images: t.images,
      size: t.size,
      n: t.n,
      inputPreview: previewMap.value[t.id],
      elapsed: t.status === 'running' ? elapsedOf(t.updated_at) : 0
    }))
)
type ChatMessage = (typeof chatMessages.value)[number]

// 任务队列（排队中 + 执行中），与「新建会话」无关，始终展示全部活跃任务
const queueTasks = computed(() =>
  [...tasks.value]
    .filter(isActive)
    .reverse()
    .map((t) => ({
      id: t.id,
      prompt: t.prompt,
      status: t.status,
      elapsed: t.status === 'running' ? elapsedOf(t.updated_at) : 0
    }))
)

const styles = ['写实摄影', '油画', '水彩', '3D 渲染', '动漫', '扁平插画', '赛博朋克', '像素风', '线稿', '低多边形', '国风工笔']
// 预设示例：样例图 + 完整提示词，点击即「套用」到下方输入框（可再自由编辑）。
const presets: PromptPreset[] = [
  {
    id: 'stellar-poster',
    title: '轮廓宇宙海报',
    hint: '高审美叙事海报、角色宇宙主题视觉、收藏版概念海报。',
    image: '/presets/stellar-poster.webp',
    size: '9:16',
    prompt: `请根据【主题：在此填写你的主题，例如某个世界观或角色】自动生成一张高审美的“轮廓宇宙 / 收藏版叙事海报”风格作品。不要将画面局限于固定器物或常见容器，不要优先默认瓶子、沙漏、玻璃罩、怀表之类的常规载体，而是由 AI 根据主题自行判断并选择一个最契合、最有象征意义、轮廓最强、最适合承载完整叙事世界的主轮廓载体。这个主轮廓可以是器物、建筑、门、塔、拱门、穹顶、楼梯井、长廊、雕像、侧脸、眼睛、手掌、头骨、羽翼、面具、镜面、王座、圆环、裂缝、光幕、阴影、几何结构、空间切面、舞台框景、抽象符号或其他更有创意与主题代表性的视觉轮廓，要求合理布局。优先选择最能放大主题气质、最能形成强烈视觉记忆点、最能体现史诗感、神秘感、诗意感或设计感的轮廓，而不是最安全、最普通、最常见的容器。画面的核心不是简单把世界装进某个物体里，而是让完整的主题世界自然生长在这个主轮廓之中、之内、之上、之边界里或与其结构融为一体，形成一种“主题宇宙依附于一个象征性轮廓展开”的高级叙事效果。主轮廓必须清晰、优雅、有辨识度，并在整体构图中占据核心地位。轮廓内部或边界中需要自动生成与主题强绑定的完整叙事世界，内容应当丰富、饱满、层次清晰，包括最能代表主题的标志性场景、核心建筑或空间结构、象征符号与隐喻元素、角色关系或文明痕迹、远景中景近景的空间递进、具有命运感和情绪张力的氛围层次，以及门、台阶、桥梁、水面、烟雾、路径、光源、遗迹、机械结构、自然景观、抽象形态、生物或道具等叙事细节。所有元素必须统一、自然、有主次、有层级地融合，像一个完整世界真实孕育在这个轮廓结构之中，而不是简单拼贴、裁切填充、素材堆叠或模板化背景。整体构图需要具有强烈的收藏版海报气质与高级设计感，大结构稳定，主轮廓强烈明确，内部世界具有纵深、秩序和呼吸感，细节丰富但不拥挤，内容丰满但不杂乱，可以适度加入小比例人物剪影、远处建筑、光柱、门洞、桥、阶梯、回廊、倒影、天光或远景结构来增强尺度感、故事感与史诗感。整体画面要安静、宏大、凝练、富有余味，不要平均铺满，不要廉价热闹，不要无重点堆砌。风格融合收藏版电影海报构图、高级叙事型视觉设计、梦幻水彩质感与纸张印刷品气质，强调纸张颗粒感、边缘飞白、水彩刷痕、轻微晕染、空气透视、柔和雾化、局部体积光、光雾穿透、大面积留白与克制版式。色彩由 AI 根据主题自动判断并匹配最合适的高级配色方案，但必须保持统一、克制、耐看、低饱和、高级。最终要求：第一眼有强烈的主题识别度和轮廓记忆点，第二眼有完整丰富的叙事世界，第三眼仍有细节和余味。`
  },
  {
    id: 'qinghua-museum-infographic',
    title: '青花瓷博物馆图鉴',
    hint: '文博专题、器物拆解、中文信息图和展板式视觉。',
    image: '/presets/qinghua-museum-infographic.webp',
    size: '4:3',
    prompt: `请根据“青花瓷”自动生成一张“博物馆图鉴式中文拆解信息图”。要求整张图兼具真实写实主视觉、结构拆解、中文标注、材质说明、纹样寓意、色彩含义和核心特征总结。你需要根据主题自动判断最合适的主体对象、服饰体系、器物结构、时代风格、关键部件、材质工艺、颜色方案与版式结构，用户无需再提供其他信息。整体风格应为：国家博物馆展板、历史服饰图鉴、文博专题信息图，而不是普通海报、古风写真、电商详情页或动漫插画。背景采用米白、绢纸白、浅茶色等纸张质感，整体高级、克制、专业、可收藏。版式固定为：顶部：中文主标题 + 副标题 + 导语；左侧：结构拆解区，中文引线标注关键部件，并配局部特写；右上：材质 / 工艺 / 质感区，展示真实纹理小样并附说明；右中：纹样 / 色彩 / 寓意区，展示主色板、纹样样本和文化解释；底部：穿着顺序 / 构成流程图 + 核心特征总结。所有文字必须为简体中文，清晰、规整、可读，不要乱码、错字、英文或拼音。重点突出真实结构、材质差异、文化说明与图鉴气质。避免：海报感、影楼感、电商感、动漫感、cosplay感、乱标注、错结构、糊字、假材质、过度装饰。`
  },
  {
    id: 'editorial-fashion',
    title: '古风联动宣传图',
    hint: '古风角色联动、游戏活动主视觉、电影感人物宣传图。',
    image: '/presets/editorial-fashion.webp',
    size: '9:16',
    prompt: `古风人物联动活动宣传图，人物占画面 80% 以上，角色立于古城城墙之上，优雅侧身回眸姿态，突出古典身姿曲线，穿着融合古风元素的联动服饰，整体造型唯美古典。高品质真人级 3D 古风游戏截图风格，电影级光影，人物清丽绝俗、长发微散，眼神柔美回眸，轻纱飘逸。背景为夜晚古城墙，青砖城垛、灯笼照明、月光洒落，古建筑灯火点点，氛围梦幻唯美。高细节，8K 品质，精致渲染，电影级构图，光影细腻，古典武侠风。`
  },
  {
    id: 'forza-horizon-shenzhen',
    title: '开放世界赛车实机图',
    hint: '游戏主视觉、次世代赛车截图、城市宣传感概念图。',
    image: '/presets/forza-horizon-shenzhen.webp',
    size: '16:9',
    prompt: `创作一张开放世界赛车游戏的实机截图，背景城市为深圳，时间设定为近未来。画面需要体现真实次世代开放世界赛车游戏的实机演出效果，包含具有深圳辨识度的城市天际线、现代高楼、道路环境、灯光氛围与速度感。构图中在合适位置放置游戏 logo 与宣传文案，整体像官方概念宣传截图而不是普通海报。要求 8K 超高清，电影级光影，真实车辆材质、反射、路面细节与空气透视，画面高级、震撼、写实。`
  }
]

// 宽高比 → 工具栏里实际可选的尺寸
const aspectSizeMap: Record<string, string> = {
  '1:1': '1024x1024',
  '3:2': '1536x1024',
  '2:3': '1024x1536',
  '16:9': '1920x1080',
  '9:16': '1080x1920',
  '4:3': '1600x1200'
}

function appendStyle(st: string) {
  prompt.value = prompt.value.trim() ? `${prompt.value.trim()}, ${st}` : st
}

function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result || ''))
    r.onerror = () => reject(r.error)
    r.readAsDataURL(blob)
  })
}

async function usePreset(p: PromptPreset) {
  // 套用预设：填入提示词 + 切到对应尺寸，并把样例图作为参考图（图生图）
  prompt.value = p.prompt
  if (aspectSizeMap[p.size]) size.value = aspectSizeMap[p.size]
  baseImage.value = null
  uploadedImages.value = []
  try {
    const resp = await fetch(p.image)
    if (!resp.ok) throw new Error('load preset image failed')
    const dataUrl = await blobToDataUrl(await resp.blob())
    const b64 = dataUrl.includes(',') ? dataUrl.slice(dataUrl.indexOf(',') + 1) : dataUrl
    uploadedImages.value = [{ b64, preview: dataUrl }]
  } catch {
    // 参考图加载失败则退化为纯文生图，不阻断套用
  }
}
function editFrom(img: WorkbenchImage) {
  uploadedImages.value = []
  baseImage.value = img
}
function editFromGallery(img: WorkbenchImage) {
  editFrom(img)
  tab.value = 'studio'
}
function clearBase() {
  uploadedImages.value = []
  baseImage.value = null
}
function removeUploaded(idx: number) {
  uploadedImages.value.splice(idx, 1)
}
function newSession() {
  // 仅清空创作台视图（任务仍在服务端运行/可在任务队列页查询），回到工作台首页
  hideBeforeMs.value = Date.now()
  uploadedImages.value = []
  baseImage.value = null
  prompt.value = ''
  sessionId.value = Math.random().toString(36).slice(2, 14)
  tab.value = 'studio'
}
function pickFile() {
  fileInput.value?.click()
}
function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!files.length) return
  const remaining = 4 - uploadedImages.value.length
  if (remaining <= 0) {
    appStore.showError(t('imageWorkbench.maxImages'))
    return
  }
  baseImage.value = null // 上传与「改这张」互斥
  for (const f of files.slice(0, remaining)) {
    if (f.size > 20 * 1024 * 1024) {
      appStore.showError(t('imageWorkbench.imageTooLarge'))
      continue
    }
    const reader = new FileReader()
    reader.onload = () => {
      const dataUrl = String(reader.result || '')
      const b64 = dataUrl.includes(',') ? dataUrl.slice(dataUrl.indexOf(',') + 1) : dataUrl
      uploadedImages.value.push({ b64, preview: dataUrl })
    }
    reader.readAsDataURL(f)
  }
  if (files.length > remaining) appStore.showError(t('imageWorkbench.maxImages'))
}
function remainingLabel(expiresAt: string): string {
  const ms = new Date(expiresAt).getTime() - Date.now()
  if (ms <= 0) return t('imageWorkbench.expired')
  const days = Math.floor(ms / 86400000)
  if (days >= 1) return t('imageWorkbench.daysLeft', { n: days })
  const hours = Math.max(1, Math.floor(ms / 3600000))
  return t('imageWorkbench.hoursLeft', { n: hours })
}

// 提交：建一个服务端异步任务（不阻塞，可连续提交；worker 在后台执行，刷新不丢）
async function send() {
  if (!prompt.value.trim() || !selectedKeyId.value || submitting.value) return
  if (queueTasks.value.length >= MAX_ACTIVE_TASKS) {
    appStore.showError(t('imageWorkbench.queueFull', { n: MAX_ACTIVE_TASKS }))
    return
  }
  const params: GenerateParams = {
    api_key_id: selectedKeyId.value,
    prompt: prompt.value.trim(),
    model: MODEL,
    size: size.value,
    n: count.value,
    session_id: sessionId.value
  }
  if (uploadedImages.value.length) params.base_images_b64 = uploadedImages.value.map((u) => u.b64)
  else if (baseImage.value) params.base_image_id = baseImage.value.id
  const preview = uploadedImages.value[0]?.preview || baseImage.value?.url

  submitting.value = true
  try {
    const task = await imageWorkbenchAPI.generate(params)
    if (task) {
      if (preview) previewMap.value[task.id] = preview
      tasks.value = [task, ...tasks.value]
    }
    prompt.value = ''
    uploadedImages.value = []
    baseImage.value = null
    await scrollToBottom()
    restartPoll() // 立即进入快速轮询
  } catch (e: unknown) {
    appStore.showError(errMessage(e))
  } finally {
    submitting.value = false
  }
}

// 重试失败任务：用其 prompt/size/n 以当前所选 key 重新提交（上传底图不可恢复，按文生图重试）
async function retry(msg: ChatMessage) {
  if (!selectedKeyId.value) {
    appStore.showError(t('imageWorkbench.failed'))
    return
  }
  if (queueTasks.value.length >= MAX_ACTIVE_TASKS) {
    appStore.showError(t('imageWorkbench.queueFull', { n: MAX_ACTIVE_TASKS }))
    return
  }
  submitting.value = true
  try {
    const task = await imageWorkbenchAPI.generate({
      api_key_id: selectedKeyId.value,
      prompt: msg.prompt,
      model: MODEL,
      size: msg.size,
      n: msg.n,
      session_id: sessionId.value
    })
    if (task) tasks.value = [task, ...tasks.value]
    await scrollToBottom()
    restartPoll()
  } catch (e: unknown) {
    appStore.showError(errMessage(e))
  } finally {
    submitting.value = false
  }
}

// ---- 轮询：服务端任务状态 ----
let pollHandle: number | undefined
const knownDone = new Set<number>()
let seeded = false

async function refreshTasks() {
  try {
    const list = await imageWorkbenchAPI.listTasks('', 50, 0)
    tasks.value = list
    let newlyDone = false
    for (const tk of list) {
      if (tk.status === 'done' && !knownDone.has(tk.id)) {
        knownDone.add(tk.id)
        if (seeded) newlyDone = true
      }
    }
    seeded = true
    if (newlyDone) await loadHistory() // 有任务刚完成 → 刷新图片库
  } catch {
    /* 轮询失败忽略，下次重试 */
  }
}

async function pollLoop() {
  await refreshTasks()
  const active = tasks.value.some(isActive)
  pollHandle = window.setTimeout(pollLoop, active ? 2500 : 9000)
}
function restartPoll() {
  if (pollHandle) window.clearTimeout(pollHandle)
  pollHandle = window.setTimeout(pollLoop, 600)
}

async function loadHistory() {
  try {
    historyImages.value = await imageWorkbenchAPI.history(60, 0)
  } catch {
    /* ignore */
  }
}

async function removeImage(img: WorkbenchImage) {
  try {
    await imageWorkbenchAPI.remove(img.id)
    historyImages.value = historyImages.value.filter((x) => x.id !== img.id)
  } catch (e: unknown) {
    appStore.showError(errMessage(e))
  }
}

function errMessage(e: unknown): string {
  const err = e as { response?: { data?: { message?: string } }; message?: string }
  return err?.response?.data?.message || err?.message || t('imageWorkbench.failed')
}

async function scrollToBottom() {
  await nextTick()
  if (chatRef.value) chatRef.value.scrollTop = chatRef.value.scrollHeight
}

let tickHandle: number | undefined

onMounted(async () => {
  try {
    const keyRes = await keysAPI.list(1, 100)
    keys.value = keyRes.items || []
    if (imageKeys.value.length > 0) selectedKeyId.value = imageKeys.value[0].id
    await userChannelsAPI.getAvailable().catch(() => [])
  } catch (e: unknown) {
    appStore.showError(errMessage(e))
  } finally {
    loadingKeys.value = false
  }
  await loadHistory()
  await pollLoop() // 加载服务端任务（含刷新前提交的）并开始轮询
  tickHandle = window.setInterval(() => {
    nowTick.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (pollHandle) window.clearTimeout(pollHandle)
  if (tickHandle) window.clearInterval(tickHandle)
})
</script>
