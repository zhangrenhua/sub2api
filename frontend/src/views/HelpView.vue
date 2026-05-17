<template>
  <div class="relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-white to-primary-50/30 dark:from-dark-950 dark:via-dark-900 dark:to-dark-900">
    <div class="pointer-events-none absolute inset-0 z-0 overflow-hidden">
      <div class="absolute -left-32 top-32 h-72 w-72 rounded-full bg-primary-300/20 blur-3xl dark:bg-primary-900/20"></div>
      <div class="absolute -right-32 top-1/3 h-72 w-72 rounded-full bg-pink-300/20 blur-3xl dark:bg-pink-900/20"></div>
    </div>

    <header class="relative z-20 border-b border-gray-200/60 bg-white/70 backdrop-blur-md dark:border-dark-800/60 dark:bg-dark-900/70">
      <div class="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
        <router-link to="/home" class="flex items-center gap-3">
          <div class="h-9 w-9 overflow-hidden rounded-xl shadow-md">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="hidden text-base font-semibold text-gray-900 dark:text-white sm:inline">{{ siteName }}</span>
          <span class="text-sm text-gray-400 dark:text-dark-500">/</span>
          <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ content.chrome.title }}</span>
        </router-link>

        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <button
            @click="toggleMobileToc"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white lg:hidden"
            :title="content.chrome.toc"
          >
            <Icon name="menu" size="md" />
          </button>
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            to="/home"
            class="hidden items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white sm:inline-flex"
          >
            <Icon name="arrowLeft" size="sm" />
            <span>{{ content.chrome.backHome }}</span>
          </router-link>
        </div>
      </div>
    </header>

    <div class="relative z-10 mx-auto flex max-w-7xl gap-8 px-6 py-10">
      <!-- Sidebar TOC (desktop) -->
      <aside class="sticky top-24 hidden h-[calc(100vh-7rem)] w-64 shrink-0 overflow-y-auto lg:block">
        <nav class="space-y-1 pr-2">
          <p class="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
            {{ content.chrome.toc }}
          </p>
          <a
            v-for="section in content.sections"
            :key="section.id"
            :href="`#${section.id}`"
            @click.prevent="scrollTo(section.id)"
            class="block rounded-lg px-3 py-1.5 text-sm transition-colors"
            :class="activeId === section.id
              ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
              : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white'"
          >
            {{ section.title }}
          </a>
        </nav>
      </aside>

      <!-- Mobile TOC drawer -->
      <Transition
        enter-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-200"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="mobileTocOpen"
          @click="mobileTocOpen = false"
          class="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm lg:hidden"
        ></div>
      </Transition>
      <Transition
        enter-active-class="transition-transform duration-200"
        enter-from-class="-translate-x-full"
        enter-to-class="translate-x-0"
        leave-active-class="transition-transform duration-200"
        leave-from-class="translate-x-0"
        leave-to-class="-translate-x-full"
      >
        <aside
          v-if="mobileTocOpen"
          class="fixed inset-y-0 left-0 z-40 w-72 overflow-y-auto bg-white p-6 shadow-xl dark:bg-dark-900 lg:hidden"
        >
          <div class="mb-4 flex items-center justify-between">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ content.chrome.toc }}</p>
            <button @click="mobileTocOpen = false" class="rounded p-1 text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-800">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <nav class="space-y-1">
            <a
              v-for="section in content.sections"
              :key="section.id"
              :href="`#${section.id}`"
              @click.prevent="scrollTo(section.id); mobileTocOpen = false"
              class="block rounded-lg px-3 py-2 text-sm transition-colors"
              :class="activeId === section.id
                ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                : 'text-gray-600 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-800'"
            >
              {{ section.title }}
            </a>
          </nav>
        </aside>
      </Transition>

      <!-- Article -->
      <article class="help-prose min-w-0 flex-1">
        <!-- Hero -->
        <div class="mb-10 rounded-2xl border border-primary-100 bg-gradient-to-br from-primary-50 to-white p-8 dark:border-primary-900/40 dark:from-primary-950/40 dark:to-dark-900">
          <div class="mb-3 inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-primary-700 shadow-sm dark:bg-dark-800/70 dark:text-primary-300">
            <Icon name="book" size="xs" />
            <span>{{ content.chrome.tagline }}</span>
          </div>
          <h1 class="mb-3 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ content.chrome.title }}</h1>
          <p class="text-base text-gray-600 dark:text-dark-300">{{ content.chrome.intro }}</p>
        </div>

        <!-- Sections -->
        <section
          v-for="section in content.sections"
          :id="section.id"
          :key="section.id"
          class="help-section"
        >
          <h2>{{ section.title }}</h2>
          <HelpBlock
            v-for="(block, i) in section.blocks"
            :key="i"
            :block="block"
            :copy-label="content.chrome.copy"
            :copied-label="content.chrome.copied"
          />
        </section>

        <!-- Back to top -->
        <div class="my-12 flex justify-center">
          <button
            @click="scrollTo(content.sections[0].id)"
            class="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white/60 px-5 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-white hover:text-gray-900 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-200 dark:hover:bg-dark-800 dark:hover:text-white"
          >
            <Icon name="arrowUp" size="sm" />
            {{ content.chrome.backToTop }}
          </button>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HelpBlock from './help/HelpBlock.vue'
import { zh, en, type HelpFactory, type HelpContent } from './help/content'

const { t, locale } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')

const apiBase = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || ''
  const raw = (configured || (typeof window !== 'undefined' ? window.location.origin : '')).trim()
  return raw.replace(/\/+$/, '')
})
const apiHost = computed(() => apiBase.value.replace(/^https?:\/\//, ''))

const factoryFor = (lang: string): HelpFactory => (lang === 'zh' ? zh : en)
const content = computed<HelpContent>(() => factoryFor(locale.value)(apiBase.value, apiHost.value))

const isDark = ref(document.documentElement.classList.contains('dark'))
const toggleTheme = () => {
  const root = document.documentElement
  if (root.classList.contains('dark')) {
    root.classList.remove('dark')
    localStorage.setItem('theme', 'light')
    isDark.value = false
  } else {
    root.classList.add('dark')
    localStorage.setItem('theme', 'dark')
    isDark.value = true
  }
}

const mobileTocOpen = ref(false)
const toggleMobileToc = () => { mobileTocOpen.value = !mobileTocOpen.value }

const activeId = ref<string>(content.value.sections[0]?.id ?? '')

const scrollTo = (id: string) => {
  const el = document.getElementById(id)
  if (!el) return
  const headerOffset = 80
  const top = el.getBoundingClientRect().top + window.pageYOffset - headerOffset
  window.scrollTo({ top, behavior: 'smooth' })
}

let observer: IntersectionObserver | null = null
const observeSections = () => {
  observer?.disconnect()
  observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter(e => e.isIntersecting)
        .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (visible.length > 0) {
        activeId.value = visible[0].target.id
      }
    },
    { rootMargin: '-100px 0px -60% 0px', threshold: 0 }
  )
  content.value.sections.forEach(section => {
    const el = document.getElementById(section.id)
    if (el) observer!.observe(el)
  })
}

onMounted(() => observeSections())
onBeforeUnmount(() => observer?.disconnect())

// Re-observe when locale (and therefore section IDs) might change.
watch(locale, async () => {
  await nextTick()
  observeSections()
})
</script>

<style scoped>
.help-prose {
  color: rgb(31 41 55);
  line-height: 1.7;
}
:global(.dark) .help-prose {
  color: rgb(229 231 235);
}

.help-section {
  scroll-margin-top: 5rem;
  margin-bottom: 3rem;
}

.help-prose :deep(h2) {
  font-size: 1.625rem;
  font-weight: 700;
  margin-top: 2rem;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid rgb(229 231 235);
  color: rgb(17 24 39);
}
:global(.dark) .help-prose :deep(h2) {
  color: rgb(255 255 255);
  border-color: rgb(38 38 47);
}

.help-prose :deep(h3) {
  font-size: 1.25rem;
  font-weight: 600;
  margin-top: 1.75rem;
  margin-bottom: 0.75rem;
  color: rgb(17 24 39);
}
:global(.dark) .help-prose :deep(h3) {
  color: rgb(243 244 246);
}

.help-prose :deep(h4) {
  font-size: 1.0625rem;
  font-weight: 600;
  margin-top: 1.25rem;
  margin-bottom: 0.5rem;
  color: rgb(55 65 81);
}
:global(.dark) .help-prose :deep(h4) {
  color: rgb(209 213 219);
}

.help-prose :deep(p) {
  margin-top: 0.75rem;
  margin-bottom: 0.75rem;
}

.help-prose :deep(a) {
  color: rgb(37 99 235);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.help-prose :deep(a:hover) {
  color: rgb(29 78 216);
}
:global(.dark) .help-prose :deep(a) {
  color: rgb(96 165 250);
}

.help-prose :deep(ul),
.help-prose :deep(ol) {
  margin: 0.75rem 0;
  padding-left: 1.5rem;
}
.help-prose :deep(ul) { list-style: disc; }
.help-prose :deep(ol) { list-style: decimal; }
.help-prose :deep(li) { margin: 0.35rem 0; }
.help-prose :deep(li > ul),
.help-prose :deep(li > ol) { margin: 0.25rem 0; }

.help-prose :deep(code) {
  background: rgb(243 244 246);
  color: rgb(220 38 38);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.85em;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
:global(.dark) .help-prose :deep(code) {
  background: rgba(255, 255, 255, 0.08);
  color: rgb(252 165 165);
}

.help-prose :deep(kbd) {
  background: rgb(255 255 255);
  border: 1px solid rgb(209 213 219);
  border-bottom-width: 2px;
  border-radius: 0.25rem;
  padding: 0.05rem 0.35rem;
  font-size: 0.8em;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: rgb(55 65 81);
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.05);
}
:global(.dark) .help-prose :deep(kbd) {
  background: rgb(38 38 47);
  color: rgb(229 231 235);
  border-color: rgb(64 64 80);
}

.help-prose :deep(.help-steps) {
  counter-reset: step;
  list-style: none !important;
  padding-left: 0 !important;
}
.help-prose :deep(.help-steps li) {
  position: relative;
  padding-left: 2.5rem;
  margin: 0.6rem 0 !important;
}
.help-prose :deep(.help-steps li::before) {
  counter-increment: step;
  content: counter(step);
  position: absolute;
  left: 0;
  top: 0.1rem;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 9999px;
  background: linear-gradient(135deg, rgb(99 102 241), rgb(168 85 247));
  color: white;
  font-weight: 600;
  font-size: 0.875rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.help-prose :deep(.help-callout) {
  margin: 1rem 0;
  padding: 1rem 1.25rem;
  border-radius: 0.75rem;
  border-left-width: 4px;
}
.help-prose :deep(.help-callout-info) {
  background: rgb(239 246 255);
  border-color: rgb(59 130 246);
  color: rgb(30 64 175);
}
.help-prose :deep(.help-callout-warning) {
  background: rgb(254 252 232);
  border-color: rgb(234 179 8);
  color: rgb(133 77 14);
}
.help-prose :deep(.help-callout-tip) {
  background: rgb(240 253 244);
  border-color: rgb(34 197 94);
  color: rgb(22 101 52);
}
:global(.dark) .help-prose :deep(.help-callout-info) {
  background: rgba(59, 130, 246, 0.1);
  color: rgb(147 197 253);
}
:global(.dark) .help-prose :deep(.help-callout-warning) {
  background: rgba(234, 179, 8, 0.1);
  color: rgb(253 224 71);
}
:global(.dark) .help-prose :deep(.help-callout-tip) {
  background: rgba(34, 197, 94, 0.1);
  color: rgb(134 239 172);
}
.help-prose :deep(.help-callout code) {
  background: rgba(0, 0, 0, 0.06);
  color: inherit;
}
:global(.dark) .help-prose :deep(.help-callout code) {
  background: rgba(255, 255, 255, 0.1);
}

.help-prose :deep(.help-table-wrap) {
  overflow-x: auto;
  margin: 1rem 0;
  border-radius: 0.75rem;
  border: 1px solid rgb(229 231 235);
}
:global(.dark) .help-prose :deep(.help-table-wrap) {
  border-color: rgb(38 38 47);
}
.help-prose :deep(.help-table-wrap table) {
  width: 100%;
  border-collapse: collapse;
}
.help-prose :deep(.help-table-wrap th),
.help-prose :deep(.help-table-wrap td) {
  padding: 0.75rem 1rem;
  text-align: left;
  border-bottom: 1px solid rgb(229 231 235);
}
:global(.dark) .help-prose :deep(.help-table-wrap th),
:global(.dark) .help-prose :deep(.help-table-wrap td) {
  border-color: rgb(38 38 47);
}
.help-prose :deep(.help-table-wrap th) {
  background: rgb(249 250 251);
  font-weight: 600;
  font-size: 0.875rem;
  color: rgb(55 65 81);
}
:global(.dark) .help-prose :deep(.help-table-wrap th) {
  background: rgba(255, 255, 255, 0.04);
  color: rgb(209 213 219);
}
.help-prose :deep(.help-table-wrap tr:last-child td) {
  border-bottom: 0;
}

.help-prose :deep(.help-faq) {
  margin: 0.75rem 0;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.75rem;
  background: rgba(255, 255, 255, 0.5);
  overflow: hidden;
}
:global(.dark) .help-prose :deep(.help-faq) {
  border-color: rgb(38 38 47);
  background: rgba(255, 255, 255, 0.02);
}
.help-prose :deep(.help-faq summary) {
  cursor: pointer;
  padding: 0.875rem 1.125rem;
  font-weight: 500;
  list-style: none;
  position: relative;
  padding-right: 2.5rem;
  color: rgb(31 41 55);
}
:global(.dark) .help-prose :deep(.help-faq summary) {
  color: rgb(229 231 235);
}
.help-prose :deep(.help-faq summary::-webkit-details-marker) { display: none; }
.help-prose :deep(.help-faq summary::after) {
  content: '+';
  position: absolute;
  right: 1.125rem;
  top: 50%;
  transform: translateY(-50%);
  font-size: 1.25rem;
  color: rgb(107 114 128);
  transition: transform 0.2s;
}
.help-prose :deep(.help-faq[open] summary::after) {
  content: '−';
}
.help-prose :deep(.help-faq summary:hover) {
  background: rgba(0, 0, 0, 0.02);
}
:global(.dark) .help-prose :deep(.help-faq summary:hover) {
  background: rgba(255, 255, 255, 0.04);
}
.help-prose :deep(.help-faq > *:not(summary)) {
  padding: 0 1.125rem;
}
.help-prose :deep(.help-faq > *:last-child:not(summary)) {
  padding-bottom: 1rem;
}
.help-prose :deep(.help-faq[open] summary) {
  border-bottom: 1px solid rgb(229 231 235);
  margin-bottom: 0.75rem;
}
:global(.dark) .help-prose :deep(.help-faq[open] summary) {
  border-color: rgb(38 38 47);
}
</style>
