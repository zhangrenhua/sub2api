<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl">
      <!-- Header -->
      <div class="mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('modelMarketplace.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ t('modelMarketplace.description') }}
        </p>
      </div>

      <!-- Toolbar: search + platform filters -->
      <div class="mb-6 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div class="relative w-full sm:w-80">
          <Icon
            name="search"
            size="md"
            class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
          />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('modelMarketplace.searchPlaceholder')"
            class="input pl-10"
          />
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="p in platformFilters"
            :key="p"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors"
            :class="
              activePlatform === p
                ? 'border-primary-500 bg-primary-500 text-white'
                : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700'
            "
            @click="activePlatform = p"
          >
            <PlatformIcon v-if="p !== 'all'" :platform="p as GroupPlatform" size="xs" />
            {{ p === 'all' ? t('common.all', 'All') : platformLabel(p) }}
          </button>
          <button
            :disabled="loading"
            class="btn btn-secondary"
            :title="t('common.refresh', 'Refresh')"
            @click="load"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div
          v-for="n in 8"
          :key="n"
          class="h-36 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-800"
        ></div>
      </div>

      <!-- Empty -->
      <div
        v-else-if="filteredBuckets.length === 0"
        class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-gray-300 py-20 text-center dark:border-dark-600"
      >
        <Icon name="grid" size="lg" class="mb-3 text-gray-300 dark:text-dark-500" />
        <p class="text-sm font-medium text-gray-600 dark:text-dark-300">{{ t('modelMarketplace.empty') }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('modelMarketplace.emptyHint') }}</p>
      </div>

      <!-- Platform sections -->
      <div v-else class="space-y-8">
        <section v-for="bucket in filteredBuckets" :key="bucket.platform">
          <div class="mb-3 flex items-center gap-2">
            <span
              class="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-sm font-semibold"
              :class="platformBadgeClass(bucket.platform)"
            >
              <PlatformIcon :platform="bucket.platform as GroupPlatform" size="sm" />
              {{ platformLabel(bucket.platform) }}
            </span>
            <span class="text-xs text-gray-400 dark:text-dark-500">
              {{ t('modelMarketplace.modelCount', { count: bucket.models.length }) }}
            </span>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            <button
              v-for="m in bucket.models"
              :key="m.platform + '::' + m.name"
              type="button"
              class="group flex flex-col rounded-2xl border bg-white p-4 text-left shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md dark:bg-dark-800"
              :class="platformBorderClass(m.platform)"
              @click="openModel(m)"
            >
              <!-- name -->
              <div class="flex items-start justify-between gap-2">
                <span class="break-all font-mono text-sm font-semibold text-gray-900 dark:text-white">
                  {{ m.name }}
                </span>
                <PlatformIcon
                  :platform="m.platform as GroupPlatform"
                  size="md"
                  :class="platformTextClass(m.platform)"
                />
              </div>

              <!-- official pricing summary -->
              <div class="mt-3 flex-1 space-y-1 text-xs">
                <div class="mb-1 text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">
                  {{ t('modelMarketplace.officialPricing') }}
                </div>
                <template v-if="m.pricing">
                  <template v-if="m.pricing.billing_mode === BILLING_MODE_TOKEN">
                    <div class="flex justify-between text-gray-600 dark:text-dark-300">
                      <span>{{ t('availableChannels.pricing.inputPrice') }}</span>
                      <span class="font-mono">{{ formatScaled(m.pricing.input_price, 1e6) }}</span>
                    </div>
                    <div class="flex justify-between text-gray-600 dark:text-dark-300">
                      <span>{{ t('availableChannels.pricing.outputPrice') }}</span>
                      <span class="font-mono">{{ formatScaled(m.pricing.output_price, 1e6) }}</span>
                    </div>
                    <div class="text-right text-[10px] text-gray-400 dark:text-dark-500">
                      {{ t('availableChannels.pricing.unitPerMillion') }}
                    </div>
                  </template>
                  <div
                    v-else-if="m.pricing.billing_mode === BILLING_MODE_PER_REQUEST"
                    class="flex justify-between text-gray-600 dark:text-dark-300"
                  >
                    <span>{{ t('availableChannels.pricing.perRequestPrice') }}</span>
                    <span class="font-mono">
                      {{ formatScaled(m.pricing.per_request_price, 1) }}
                      {{ t('availableChannels.pricing.unitPerRequest') }}
                    </span>
                  </div>
                  <div
                    v-else
                    class="flex justify-between text-gray-600 dark:text-dark-300"
                  >
                    <span>{{ t('availableChannels.pricing.imageOutputPrice') }}</span>
                    <span class="font-mono">
                      {{ formatScaled(m.pricing.image_output_price, 1) }}
                      {{ t('availableChannels.pricing.unitPerRequest') }}
                    </span>
                  </div>
                </template>
                <div v-else class="text-gray-400 dark:text-dark-500">
                  {{ t('availableChannels.noPricing') }}
                </div>
              </div>

              <!-- footer: groups + lowest rate -->
              <div class="mt-3 flex items-center justify-between border-t border-gray-100 pt-2 text-[11px] dark:border-dark-700">
                <span class="text-gray-400 dark:text-dark-500">
                  {{ t('modelMarketplace.groupCount', { count: m.groups.length }) }}
                </span>
                <span
                  v-if="m.lowestRate != null"
                  class="rounded-full bg-primary-50 px-2 py-0.5 font-medium text-primary-600 dark:bg-primary-500/10 dark:text-primary-400"
                >
                  {{ t('modelMarketplace.lowestRate', { rate: formatRate(m.lowestRate) }) }}
                </span>
              </div>
            </button>
          </div>
        </section>
      </div>
    </div>

    <!-- Drawer: per-group pricing -->
    <Teleport to="body">
      <div v-if="selected" class="fixed inset-0 z-[9998]" @click="selected = null">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"></div>
      </div>
      <transition name="drawer-slide">
        <aside
          v-if="selected"
          class="fixed right-0 top-0 z-[9999] flex h-full w-full max-w-md flex-col bg-white shadow-2xl dark:bg-dark-900 sm:w-[28rem]"
        >
          <!-- header -->
          <div
            class="flex items-start justify-between gap-3 border-b px-5 py-4"
            :class="platformBorderClass(selected.platform)"
          >
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <PlatformIcon
                  :platform="selected.platform as GroupPlatform"
                  size="md"
                  :class="platformTextClass(selected.platform)"
                />
                <span class="break-all font-mono text-base font-semibold text-gray-900 dark:text-white">
                  {{ selected.name }}
                </span>
              </div>
              <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
                {{ t('modelMarketplace.drawer.subtitle') }}
              </p>
            </div>
            <button
              type="button"
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
              :title="t('modelMarketplace.drawer.close')"
              @click="selected = null"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- body -->
          <div class="flex-1 space-y-3 overflow-y-auto px-5 py-4">
            <!-- official block -->
            <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/60">
              <div class="mb-2 flex items-center justify-between">
                <span class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">
                  {{ t('modelMarketplace.drawer.official') }}
                </span>
                <span class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300">
                  ×1.0
                </span>
              </div>
              <PricingBlock :pricing="selected.pricing" />
            </div>

            <!-- per-group blocks -->
            <div
              v-for="(g, idx) in sortedGroups"
              :key="g.group.id"
              class="rounded-xl border p-3"
              :class="
                idx === 0
                  ? 'border-primary-300 bg-primary-50/50 dark:border-primary-500/40 dark:bg-primary-500/5'
                  : 'border-gray-200 dark:border-dark-700'
              "
            >
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <div class="flex items-center gap-1.5">
                  <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ g.group.name }}</span>
                  <span
                    v-if="idx === 0"
                    class="rounded-full bg-primary-500 px-1.5 py-0.5 text-[10px] font-semibold text-white"
                  >
                    {{ t('modelMarketplace.drawer.best') }}
                  </span>
                  <span
                    v-if="g.group.subscription_type === 'subscription'"
                    class="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
                  >
                    {{ t('modelMarketplace.drawer.subscription') }}
                  </span>
                </div>
                <span
                  class="rounded-full px-2 py-0.5 text-[11px] font-semibold"
                  :class="
                    g.isCustomRate
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                      : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
                  "
                  :title="g.isCustomRate ? t('modelMarketplace.drawer.yourRate') : t('modelMarketplace.drawer.rate')"
                >
                  ×{{ formatRate(g.rate) }}
                </span>
              </div>
              <PricingBlock :pricing="g.effective" />
            </div>

            <div
              v-if="sortedGroups.length === 0"
              class="py-8 text-center text-sm text-gray-400 dark:text-dark-500"
            >
              {{ t('modelMarketplace.drawer.noGroups') }}
            </div>
          </div>
        </aside>
      </transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, type VNode } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PricingRow from '@/components/channels/PricingRow.vue'
import userChannelsAPI, {
  type UserAvailableChannel,
  type UserAvailableGroup,
  type UserSupportedModelPricing
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import { platformBadgeClass, platformBorderClass, platformTextClass, platformLabel } from '@/utils/platformColors'
import { BILLING_MODE_TOKEN, BILLING_MODE_PER_REQUEST, BILLING_MODE_IMAGE } from '@/constants/channel'
import type { GroupPlatform } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

interface MarketGroup {
  group: UserAvailableGroup
  rate: number
  isCustomRate: boolean
  effective: UserSupportedModelPricing | null
}
interface MarketModel {
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
  groups: UserAvailableGroup[]
  lowestRate: number | null
}
interface PlatformBucket {
  platform: string
  models: MarketModel[]
}

const loading = ref(false)
const searchQuery = ref('')
const activePlatform = ref<string>('all')
const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const selected = ref<MarketModel | null>(null)

const perMillionScale = 1_000_000

/** Effective multiplier for a group: user-specific override takes priority over the group default. */
function rateFor(g: UserAvailableGroup): { rate: number; custom: boolean } {
  const override = userGroupRates.value[g.id]
  if (override != null && override !== g.rate_multiplier) return { rate: override, custom: true }
  return { rate: override ?? g.rate_multiplier, custom: false }
}

function formatRate(rate: number): string {
  return Number(rate.toFixed(4)).toString()
}

/** Scale every price field in a pricing object by `m` (official × group multiplier). */
function scalePricing(p: UserSupportedModelPricing | null, m: number): UserSupportedModelPricing | null {
  if (!p) return null
  const s = (v: number | null) => (v == null ? null : v * m)
  return {
    ...p,
    input_price: s(p.input_price),
    output_price: s(p.output_price),
    cache_write_price: s(p.cache_write_price),
    cache_read_price: s(p.cache_read_price),
    image_output_price: s(p.image_output_price),
    per_request_price: s(p.per_request_price),
    intervals: (p.intervals || []).map((iv) => ({
      ...iv,
      input_price: s(iv.input_price),
      output_price: s(iv.output_price),
      cache_write_price: s(iv.cache_write_price),
      cache_read_price: s(iv.cache_read_price),
      per_request_price: s(iv.per_request_price)
    }))
  }
}

/** Flatten channels → per-platform model buckets, deduping models and merging accessible groups. */
const buckets = computed<PlatformBucket[]>(() => {
  const map = new Map<string, MarketModel>()
  for (const ch of channels.value) {
    for (const section of ch.platforms) {
      for (const model of section.supported_models) {
        const key = `${section.platform}::${model.name}`
        let entry = map.get(key)
        if (!entry) {
          entry = {
            name: model.name,
            platform: section.platform,
            pricing: model.pricing,
            groups: [],
            lowestRate: null
          }
          map.set(key, entry)
        } else if (!entry.pricing && model.pricing) {
          entry.pricing = model.pricing
        }
        // 模型广场只展示公开分组（非专属）。专属分组不在此呈现。
        const seen = new Set(entry.groups.map((g) => g.id))
        for (const g of section.groups) {
          if (g.is_exclusive) continue
          if (!seen.has(g.id)) {
            entry.groups.push(g)
            seen.add(g.id)
          }
        }
      }
    }
  }

  // compute lowest effective rate per model
  for (const m of map.values()) {
    let lowest: number | null = null
    for (const g of m.groups) {
      const { rate } = rateFor(g)
      if (lowest == null || rate < lowest) lowest = rate
    }
    m.lowestRate = lowest
  }

  // bucket by platform, sort models by name
  const byPlatform = new Map<string, MarketModel[]>()
  for (const m of map.values()) {
    // 仅保留至少有一个公开分组的模型。
    if (m.groups.length === 0) continue
    if (!byPlatform.has(m.platform)) byPlatform.set(m.platform, [])
    byPlatform.get(m.platform)!.push(m)
  }
  const order = ['anthropic', 'openai', 'gemini', 'antigravity']
  return [...byPlatform.entries()]
    .sort((a, b) => {
      const ia = order.indexOf(a[0])
      const ib = order.indexOf(b[0])
      return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib) || a[0].localeCompare(b[0])
    })
    .map(([platform, models]) => ({
      platform,
      models: models.sort((a, b) => a.name.localeCompare(b.name))
    }))
})

const platformFilters = computed<string[]>(() => ['all', ...buckets.value.map((b) => b.platform)])

const filteredBuckets = computed<PlatformBucket[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return buckets.value
    .filter((b) => activePlatform.value === 'all' || b.platform === activePlatform.value)
    .map((b) => {
      if (!q) return b
      const models = b.models.filter(
        (m) => m.name.toLowerCase().includes(q) || b.platform.toLowerCase().includes(q)
      )
      return { ...b, models }
    })
    .filter((b) => b.models.length > 0)
})

/** Groups for the selected model, sorted cheapest-first, with effective pricing precomputed. */
const sortedGroups = computed<MarketGroup[]>(() => {
  const m = selected.value
  if (!m) return []
  return m.groups
    .map((g) => {
      const { rate, custom } = rateFor(g)
      return { group: g, rate, isCustomRate: custom, effective: scalePricing(m.pricing, rate) }
    })
    .sort((a, b) => a.rate - b.rate)
})

function openModel(m: MarketModel) {
  selected.value = m
}

async function load() {
  loading.value = true
  try {
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      })
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

/**
 * Small render-function component that renders the price rows for a pricing object,
 * reusing PricingRow. Keeps the template tidy since it's used for both the official
 * block and every group block.
 */
const PricingBlock = (props: { pricing: UserSupportedModelPricing | null }): VNode => {
  const p = props.pricing
  if (!p) {
    return h('div', { class: 'text-xs text-gray-400 dark:text-dark-500' }, t('availableChannels.noPricing'))
  }
  const rows: VNode[] = []
  const row = (label: string, value: number | null, unit: string, scale: number) =>
    h(PricingRow, { label, value, unit, scale })

  if (p.billing_mode === BILLING_MODE_TOKEN) {
    rows.push(row(t('availableChannels.pricing.inputPrice'), p.input_price, t('availableChannels.pricing.unitPerMillion'), perMillionScale))
    rows.push(row(t('availableChannels.pricing.outputPrice'), p.output_price, t('availableChannels.pricing.unitPerMillion'), perMillionScale))
    if (p.cache_write_price != null)
      rows.push(row(t('availableChannels.pricing.cacheWritePrice'), p.cache_write_price, t('availableChannels.pricing.unitPerMillion'), perMillionScale))
    if (p.cache_read_price != null)
      rows.push(row(t('availableChannels.pricing.cacheReadPrice'), p.cache_read_price, t('availableChannels.pricing.unitPerMillion'), perMillionScale))
    if (p.image_output_price != null && p.image_output_price > 0)
      rows.push(row(t('availableChannels.pricing.imageOutputPrice'), p.image_output_price, t('availableChannels.pricing.unitPerMillion'), perMillionScale))
  } else if (p.billing_mode === BILLING_MODE_PER_REQUEST) {
    rows.push(row(t('availableChannels.pricing.perRequestPrice'), p.per_request_price, t('availableChannels.pricing.unitPerRequest'), 1))
  } else if (p.billing_mode === BILLING_MODE_IMAGE) {
    rows.push(row(t('availableChannels.pricing.imageOutputPrice'), p.image_output_price, t('availableChannels.pricing.unitPerRequest'), 1))
  }
  return h('div', { class: 'space-y-1 text-xs' }, rows)
}
</script>

<style scoped>
.drawer-slide-enter-active,
.drawer-slide-leave-active {
  transition: transform 0.25s ease;
}
.drawer-slide-enter-from,
.drawer-slide-leave-to {
  transform: translateX(100%);
}
</style>
