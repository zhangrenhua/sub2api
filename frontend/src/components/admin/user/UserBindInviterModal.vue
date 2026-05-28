<template>
  <BaseDialog :show="show" :title="t('admin.users.bindInviter.title')" width="narrow" @close="$emit('close')">
    <div v-if="user" class="space-y-5">
      <!-- 用户信息 -->
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700/40">
        <div class="flex h-11 w-11 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-lg font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.bindInviter.hint') }}</p>
        </div>
      </div>

      <!-- 邀请码输入 -->
      <div>
        <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.users.bindInviter.codeLabel') }}
        </label>
        <input
          v-model="code"
          type="text"
          autocomplete="off"
          :placeholder="t('admin.users.bindInviter.codePlaceholder')"
          class="input w-full uppercase"
          @keyup.enter="handleBind"
        />
        <p class="mt-2 text-xs text-gray-400">{{ t('admin.users.bindInviter.note') }}</p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary px-5">{{ t('common.cancel') }}</button>
        <button @click="handleBind" :disabled="submitting || !code.trim()" class="btn btn-primary px-6">
          <svg v-if="submitting" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ submitting ? t('admin.users.bindInviter.binding') : t('admin.users.bindInviter.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const code = ref('')
const submitting = ref(false)

watch(
  () => props.show,
  (v) => {
    if (v) code.value = ''
  },
)

const handleBind = async () => {
  if (!props.user || submitting.value) return
  const trimmed = code.value.trim()
  if (!trimmed) return
  submitting.value = true
  try {
    await adminAPI.affiliates.bindUserInviter(props.user.id, trimmed)
    appStore.showSuccess(t('admin.users.bindInviter.success'))
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.bindInviter.failed'))
  } finally {
    submitting.value = false
  }
}
</script>
