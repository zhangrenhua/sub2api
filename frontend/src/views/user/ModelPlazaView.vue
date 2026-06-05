<template>
  <AppLayout>
    <div class="space-y-5">
      <!-- 标题 + 刷新 -->
      <div class="flex items-end justify-between gap-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('modelPlaza.modelsTitle') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('modelPlaza.modelsDesc') }}
          </p>
        </div>
        <button
          class="btn btn-secondary flex-shrink-0"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="load"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- 搜索 + 平台筛选 -->
      <div class="flex flex-wrap items-center gap-3">
        <div class="relative w-full sm:w-80">
          <Icon
            name="search"
            size="md"
            class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
          />
          <input
            v-model="search"
            type="text"
            :placeholder="t('modelPlaza.searchPlaceholder')"
            class="input pl-10"
          />
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            class="pfilter"
            :class="activePlatform === '' ? activeCls : idleCls"
            @click="activePlatform = ''"
          >
            {{ t('modelPlaza.allPlatforms') }}
          </button>
          <button
            v-for="p in platforms"
            :key="p"
            class="pfilter"
            :class="activePlatform === p ? activeCls : idleCls"
            @click="activePlatform = p"
          >
            <PlatformIcon :platform="p as GroupPlatform" size="xs" />
            {{ p }}
          </button>
        </div>
      </div>

      <!-- 状态 / 网格 -->
      <div v-if="loading" class="flex justify-center py-16">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>
      <div
        v-else-if="filteredModels.length === 0"
        class="flex flex-col items-center justify-center gap-2 py-16 text-gray-400"
      >
        <Icon name="inbox" size="xl" class="h-12 w-12" />
        <p class="text-sm">{{ t('modelPlaza.empty') }}</p>
      </div>
      <div
        v-else
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        <article
          v-for="m in filteredModels"
          :key="m.key"
          class="rounded-2xl border border-gray-200 bg-white shadow-sm transition hover:shadow-md dark:border-dark-700 dark:bg-dark-800"
        >
          <!-- 卡片主体：有分组时整块可点开 -->
          <div
            class="p-4"
            :class="m.groups.length ? 'cursor-pointer select-none' : ''"
            @click="m.groups.length && toggle(m.key)"
          >
            <div class="flex items-center gap-3">
              <span
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-900"
              >
                <PlatformIcon :platform="m.platform as GroupPlatform" size="sm" />
              </span>
              <div class="min-w-0 flex-1">
                <b class="block truncate text-sm font-semibold text-gray-900 dark:text-white">
                  {{ m.name }}
                </b>
                <span class="text-[11px] uppercase tracking-wide text-gray-400 dark:text-gray-500">
                  {{ m.platform }}
                </span>
              </div>
              <span
                class="flex-shrink-0 rounded-full bg-primary-50 px-2 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
              >
                {{ billingLabel(m) }}
              </span>
            </div>

            <!-- 基准单价 -->
            <div class="mt-3 border-t border-dashed border-gray-200 pt-3 dark:border-dark-700">
              <template v-if="m.pricing && m.pricing.billing_mode === 'token'">
                <div class="flex items-baseline justify-between py-0.5 text-[13px] text-gray-500 dark:text-gray-400">
                  <span>{{ t('modelPlaza.priceInput') }}</span>
                  <b class="font-mono text-sm text-gray-900 dark:text-white">{{ price(m.pricing.input_price, 1_000_000) }}</b>
                </div>
                <div class="flex items-baseline justify-between py-0.5 text-[13px] text-gray-500 dark:text-gray-400">
                  <span>{{ t('modelPlaza.priceOutput') }}</span>
                  <b class="font-mono text-sm text-gray-900 dark:text-white">{{ price(m.pricing.output_price, 1_000_000) }}</b>
                </div>
                <div class="mt-1.5 text-[11px] text-gray-400 dark:text-gray-500">{{ t('modelPlaza.perMillion') }}</div>
              </template>
              <template v-else-if="m.pricing && m.pricing.billing_mode === 'per_request'">
                <div class="flex items-baseline gap-2">
                  <b class="font-mono text-xl text-gray-900 dark:text-white">{{ price(m.pricing.per_request_price, 1) }}</b>
                  <span class="text-[13px] text-gray-500 dark:text-gray-400">{{ t('modelPlaza.perRequest') }}</span>
                </div>
              </template>
              <template v-else-if="m.pricing && m.pricing.billing_mode === 'image'">
                <div class="flex items-baseline gap-2">
                  <b class="font-mono text-xl text-gray-900 dark:text-white">{{ price(m.pricing.image_output_price, 1) }}</b>
                  <span class="text-[13px] text-gray-500 dark:text-gray-400">{{ t('modelPlaza.perImage') }}</span>
                </div>
              </template>
              <div v-else class="text-[12px] italic text-gray-400 dark:text-gray-500">
                {{ t('modelPlaza.noPrice') }}
              </div>
            </div>

            <!-- 展开提示 -->
            <button
              v-if="m.groups.length"
              type="button"
              class="mt-2.5 flex w-full items-center justify-between text-[12px] font-medium text-primary-600 dark:text-primary-400"
              @click.stop="toggle(m.key)"
            >
              <span>{{ t('modelPlaza.viewGroups', { n: m.groups.length }) }}</span>
              <Icon
                name="chevronDown"
                size="sm"
                class="transition-transform"
                :class="{ 'rotate-180': isOpen(m.key) }"
              />
            </button>
          </div>

          <!-- 分组倍率明细 -->
          <div
            v-if="m.groups.length && isOpen(m.key)"
            class="space-y-2 border-t border-gray-100 bg-gray-50/60 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/40"
          >
            <div
              v-for="g in m.groups"
              :key="g.id"
              class="rounded-lg bg-white px-3 py-2 dark:bg-dark-800"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="truncate text-[13px] font-medium text-gray-800 dark:text-gray-200">
                  {{ g.name }}
                </span>
                <span class="flex-shrink-0 font-mono text-[13px] font-semibold text-primary-700 dark:text-primary-300">
                  ×{{ formatRate(rateOf(g)) }}
                </span>
              </div>
              <!-- 折算后实际单价 -->
              <div
                v-if="m.pricing && m.pricing.billing_mode === 'token'"
                class="mt-1 font-mono text-[11.5px] text-gray-500 dark:text-gray-400"
              >
                {{ t('modelPlaza.priceInput') }} {{ eff(m.pricing.input_price, rateOf(g), 1_000_000) }}
                · {{ t('modelPlaza.priceOutput') }} {{ eff(m.pricing.output_price, rateOf(g), 1_000_000) }} /1M
              </div>
              <div
                v-else-if="m.pricing && m.pricing.billing_mode === 'per_request'"
                class="mt-1 font-mono text-[11.5px] text-gray-500 dark:text-gray-400"
              >
                {{ eff(m.pricing.per_request_price, rateOf(g), 1) }} {{ t('modelPlaza.perRequest') }}
              </div>
              <div
                v-else-if="m.pricing && m.pricing.billing_mode === 'image'"
                class="mt-1 font-mono text-[11.5px] text-gray-500 dark:text-gray-400"
              >
                {{ eff(m.pricing.image_output_price, rateOf(g), 1) }} {{ t('modelPlaza.perImage') }}
              </div>
            </div>
          </div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import userChannelsAPI, { type UserAvailableChannel, type UserAvailableGroup, type UserSupportedModelPricing } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import type { GroupPlatform } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const search = ref('')
const activePlatform = ref('')
const openKeys = ref<Set<string>>(new Set())

const activeCls = 'bg-primary-600 text-white'
const idleCls =
  'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'

interface PlazaModel {
  key: string
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
  groups: UserAvailableGroup[]
}

// 跨渠道聚合：模型按 平台::名称 去重，保留首个定价，并合并所有可达分组（按 id 去重）。
const allModels = computed<PlazaModel[]>(() => {
  const map = new Map<string, PlazaModel & { groupMap: Map<number, UserAvailableGroup> }>()
  for (const ch of channels.value) {
    for (const sec of ch.platforms) {
      for (const m of sec.supported_models) {
        const platform = m.platform || sec.platform
        const key = `${platform}::${m.name}`
        let entry = map.get(key)
        if (!entry) {
          entry = { key, name: m.name, platform, pricing: m.pricing, groups: [], groupMap: new Map() }
          map.set(key, entry)
        }
        for (const g of sec.groups) {
          if (!entry.groupMap.has(g.id)) entry.groupMap.set(g.id, g)
        }
      }
    }
  }
  // 分组按折算倍率升序（便宜在前）
  return Array.from(map.values()).map((e) => ({
    key: e.key,
    name: e.name,
    platform: e.platform,
    pricing: e.pricing,
    groups: Array.from(e.groupMap.values()).sort((a, b) => rateOf(a) - rateOf(b) || a.name.localeCompare(b.name)),
  }))
})

const platforms = computed<string[]>(() => {
  const set = new Set<string>()
  for (const m of allModels.value) set.add(m.platform)
  return Array.from(set).sort()
})

const filteredModels = computed<PlazaModel[]>(() => {
  const q = search.value.trim().toLowerCase()
  return allModels.value
    .filter((m) => (activePlatform.value ? m.platform === activePlatform.value : true))
    .filter((m) => (q ? m.name.toLowerCase().includes(q) : true))
    .sort((a, b) => a.name.localeCompare(b.name))
})

function rateOf(g: UserAvailableGroup): number {
  return userGroupRates.value[g.id] ?? g.rate_multiplier
}

function formatRate(v: number): string {
  return Number(v.toFixed(4)).toString()
}

function price(value: number | null, scale: number): string {
  return formatScaled(value, scale)
}

// 折算后实际单价 = 基准单价 × 分组倍率
function eff(value: number | null, mult: number, scale: number): string {
  return formatScaled(value == null ? null : value * mult, scale)
}

function isOpen(key: string): boolean {
  return openKeys.value.has(key)
}

function toggle(key: string) {
  const next = new Set(openKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  openKeys.value = next
}

function billingLabel(m: PlazaModel): string {
  const mode = m.pricing?.billing_mode
  if (mode === 'token') return t('modelPlaza.billingToken')
  if (mode === 'per_request') return t('modelPlaza.billingPerRequest')
  if (mode === 'image') return t('modelPlaza.billingImage')
  return t('modelPlaza.billingUnknown')
}

async function load() {
  loading.value = true
  try {
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch(() => ({}) as Record<number, number>),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.pfilter {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 8px 14px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  transition: all 0.15s;
}
</style>
