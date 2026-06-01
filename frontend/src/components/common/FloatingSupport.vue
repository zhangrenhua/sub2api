<template>
  <a
    v-if="supportUrl"
    :href="supportUrl"
    target="_blank"
    rel="noopener noreferrer"
    :title="t('common.contactSupport')"
    :aria-label="t('common.contactSupport')"
    class="fixed bottom-6 right-6 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-[#229ED9] text-white shadow-lg ring-1 ring-black/5 transition-transform hover:scale-110 hover:bg-[#1c8dc4]"
  >
    <!-- Telegram glyph -->
    <svg viewBox="0 0 24 24" class="h-7 w-7" fill="currentColor" aria-hidden="true">
      <path d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.31-.07-.46-.52-.19L7.74 13.3 3.64 12c-.88-.25-.89-.86.2-1.3l15.97-6.16c.73-.33 1.43.18 1.15 1.3l-2.72 12.81c-.19.91-.74 1.13-1.5.71L12.6 16.3l-1.99 1.93c-.23.23-.42.42-.83.42z" />
    </svg>
  </a>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

// Online customer-support contact (Telegram), configured via system settings.
// Empty string hides the floating widget entirely.
const supportUrl = computed(() => appStore.supportUrl)
</script>
