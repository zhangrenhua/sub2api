<template>
  <div class="min-h-full bg-gray-50 dark:bg-dark-900">
    <div class="mx-auto max-w-4xl px-4 py-5">
      <!-- 顶部：返回 + 标题 + 刷新 -->
      <div class="mb-5 flex items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <router-link
            to="/image-workbench"
            class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
          >
            ← {{ t('imageWorkbench.backToStudio') }}
          </router-link>
          <div>
            <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.tasksTitle') }}</h1>
            <p class="text-xs text-gray-400">{{ t('imageWorkbench.tasksDescription') }}</p>
          </div>
        </div>
        <button
          class="inline-flex items-center gap-1 rounded-full bg-gray-900 px-3 py-2 text-xs font-medium text-white transition hover:bg-gray-700 disabled:opacity-50 dark:bg-dark-700 dark:hover:bg-dark-600"
          :disabled="loading"
          @click="reload"
        >
          <Icon name="clock" size="sm" />
          {{ t('imageWorkbench.refresh') }}
        </button>
      </div>

      <!-- 状态筛选 -->
      <div class="mb-4 flex flex-wrap gap-1.5">
        <button
          v-for="f in filters"
          :key="f.value"
          class="rounded-full px-3 py-1 text-xs font-medium transition"
          :class="filter === f.value
            ? 'bg-primary-600 text-white'
            : 'border border-gray-200 bg-white text-gray-500 hover:border-primary-400 dark:border-dark-600 dark:bg-dark-800'"
          @click="setFilter(f.value)"
        >
          {{ f.label }}
        </button>
      </div>

      <!-- 列表 -->
      <div v-if="!loading && tasks.length === 0" class="rounded-2xl border border-dashed border-gray-200 py-16 text-center text-sm text-gray-400 dark:border-dark-600">
        {{ t('imageWorkbench.noTasks') }}
      </div>

      <div v-else class="space-y-2.5">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex items-start gap-3">
            <span class="mt-0.5 inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" :class="statusClass(task.status)">
              <span class="h-1.5 w-1.5 rounded-full" :class="dotClass(task.status)"></span>
              {{ statusLabel(task.status) }}
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm text-gray-800 dark:text-gray-100" :title="task.prompt">{{ task.prompt }}</p>
              <div class="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[11px] text-gray-400">
                <span>#{{ task.id }}</span>
                <span v-if="task.size">{{ task.size }}</span>
                <span>x{{ task.n }}</span>
                <span>{{ formatTime(task.created_at) }}</span>
                <span v-if="task.status === 'running'" class="tabular-nums font-medium text-primary-500">{{ formatElapsed(elapsedOf(task.updated_at)) }}</span>
              </div>
              <p v-if="task.status === 'error' && task.error" class="mt-1 rounded-md bg-red-50 px-2 py-1 text-xs text-red-500 dark:bg-red-900/20">{{ task.error }}</p>
            </div>
          </div>

          <!-- 结果缩略图 -->
          <div v-if="task.images && task.images.length" class="mt-2.5 grid grid-cols-3 gap-2 sm:grid-cols-5">
            <div v-for="img in task.images" :key="img.id" class="group relative overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
              <img :src="img.url" :alt="img.prompt" class="aspect-square w-full object-cover" loading="lazy" />
              <a
                :href="img.url"
                :download="'image-' + img.id + '.png'"
                class="absolute bottom-1 right-1 hidden rounded bg-black/50 px-1.5 py-0.5 text-[10px] text-white group-hover:block"
              >
                {{ t('imageWorkbench.download') }}
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import imageWorkbenchAPI, { type WorkbenchTask, type WorkbenchTaskStatus } from '@/api/imageWorkbench'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

type FilterValue = '' | WorkbenchTaskStatus
const filter = ref<FilterValue>('')
const tasks = ref<WorkbenchTask[]>([])
const loading = ref(false)
const nowTick = ref(Date.now())

const filters = computed<{ value: FilterValue; label: string }[]>(() => [
  { value: '', label: t('imageWorkbench.filterAll') },
  { value: 'queued', label: t('imageWorkbench.statusQueued') },
  { value: 'running', label: t('imageWorkbench.filterRunning') },
  { value: 'done', label: t('imageWorkbench.filterDone') },
  { value: 'error', label: t('imageWorkbench.filterError') }
])

function statusLabel(s: WorkbenchTaskStatus): string {
  return t(
    {
      queued: 'imageWorkbench.statusQueued',
      running: 'imageWorkbench.filterRunning',
      done: 'imageWorkbench.filterDone',
      error: 'imageWorkbench.filterError'
    }[s]
  )
}
function statusClass(s: WorkbenchTaskStatus): string {
  return {
    queued: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300',
    running: 'bg-primary-50 text-primary-600 dark:bg-primary-900/30',
    done: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30',
    error: 'bg-red-50 text-red-500 dark:bg-red-900/20'
  }[s]
}
function dotClass(s: WorkbenchTaskStatus): string {
  return {
    queued: 'bg-gray-400',
    running: 'bg-primary-500 animate-pulse',
    done: 'bg-emerald-500',
    error: 'bg-red-500'
  }[s]
}
function elapsedOf(ts: string): number {
  return Math.max(0, Math.floor((nowTick.value - new Date(ts).getTime()) / 1000))
}
function formatElapsed(s: number): string {
  if (s < 60) return `${s}s`
  return `${Math.floor(s / 60)}m${String(s % 60).padStart(2, '0')}s`
}
function formatTime(ts: string): string {
  return new Date(ts).toLocaleString()
}

let pollHandle: number | undefined

async function refresh() {
  try {
    tasks.value = await imageWorkbenchAPI.listTasks(filter.value, 100, 0)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } }; message?: string }
    appStore.showError(err?.response?.data?.message || err?.message || 'failed')
  }
}

async function pollLoop() {
  await refresh()
  const active = tasks.value.some((tk) => tk.status === 'queued' || tk.status === 'running')
  pollHandle = window.setTimeout(pollLoop, active ? 2500 : 9000)
}

async function reload() {
  loading.value = true
  if (pollHandle) window.clearTimeout(pollHandle)
  await refresh()
  loading.value = false
  pollHandle = window.setTimeout(pollLoop, 2500)
}

function setFilter(v: FilterValue) {
  if (filter.value === v) return
  filter.value = v
  reload()
}

let tickHandle: number | undefined

onMounted(async () => {
  loading.value = true
  await pollLoop()
  loading.value = false
  tickHandle = window.setInterval(() => {
    nowTick.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (pollHandle) window.clearTimeout(pollHandle)
  if (tickHandle) window.clearInterval(tickHandle)
})
</script>
