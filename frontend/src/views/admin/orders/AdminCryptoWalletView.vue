<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.title') }}</h2>
        <button @click="refreshAll" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div v-if="loading && !overview" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="overview">
        <!-- Not initialized: setup panel -->
        <div v-if="!overview.initialized" class="card space-y-4 p-5">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.initTitle') }}</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.crypto.initHint') }}</p>
          <textarea
            v-model="importMnemonic"
            rows="2"
            class="input w-full font-mono text-sm"
            :placeholder="t('payment.crypto.importPlaceholder')"
          />
          <div class="flex items-center gap-3">
            <input v-model="totpCode" maxlength="6" class="input w-32" :placeholder="t('payment.crypto.totpCode')" />
            <button @click="handleInit" :disabled="busy" class="btn btn-primary">{{ t('payment.crypto.initButton') }}</button>
          </div>
        </div>

        <!-- Generated mnemonic: show once for backup -->
        <div v-if="generatedMnemonic" class="card space-y-3 border-amber-300 p-5 dark:border-amber-700">
          <h3 class="text-sm font-semibold text-amber-700 dark:text-amber-400">{{ t('payment.crypto.backupTitle') }}</h3>
          <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('payment.crypto.backupWarning') }}</p>
          <code class="block break-all rounded bg-gray-100 p-3 font-mono text-sm dark:bg-dark-700">{{ generatedMnemonic }}</code>
          <div class="flex gap-2">
            <button @click="copy(generatedMnemonic)" class="btn btn-secondary btn-sm">{{ t('common.copy') }}</button>
            <button @click="generatedMnemonic = ''" class="btn btn-primary btn-sm">{{ t('payment.crypto.backupDone') }}</button>
          </div>
        </div>

        <!-- Balance overview cards -->
        <div v-if="overview.initialized" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.depositTotal') }}</p>
            <p class="mt-1 text-xl font-bold tabular-nums text-gray-900 dark:text-white">{{ fmtUsdt(overview.deposit_total_usdt) }}</p>
            <p class="text-xs text-gray-400">{{ overview.deposit_addresses }} {{ t('payment.crypto.addresses') }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.collectionBalance') }}</p>
            <p class="mt-1 text-xl font-bold tabular-nums text-gray-900 dark:text-white">{{ fmtUsdt(overview.collection_balance) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.feeTrx') }}</p>
            <p class="mt-1 text-xl font-bold tabular-nums" :class="overview.fee_trx_balance < 100 ? 'text-red-500' : 'text-gray-900 dark:text-white'">{{ overview.fee_trx_balance.toFixed(2) }} TRX</p>
            <p class="text-xs text-gray-400 break-all">{{ overview.fee_address }}</p>
          </div>
          <div class="card flex flex-col justify-between p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.actions') }}</p>
            <div class="mt-2 flex flex-col gap-2">
              <button @click="handleRefreshBalances" :disabled="busy" class="btn btn-secondary btn-sm">{{ t('payment.crypto.refreshBalances') }}</button>
              <button @click="showSweep = true" :disabled="busy" class="btn btn-primary btn-sm">{{ t('payment.crypto.sweepButton') }}</button>
            </div>
          </div>
        </div>

        <!-- Collection address -->
        <div v-if="overview.initialized" class="card space-y-3 p-5">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.collectionAddress') }}</h3>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model="collectionAddress" class="input min-w-0 flex-1 font-mono text-sm" :placeholder="t('payment.crypto.collectionPlaceholder')" />
            <input v-model="totpCode" maxlength="6" class="input w-32" :placeholder="t('payment.crypto.totpCode')" />
            <button @click="handleSetCollection" :disabled="busy" class="btn btn-primary">{{ t('common.save') }}</button>
          </div>
          <p class="text-xs text-gray-400">{{ t('payment.crypto.collectionHint') }}</p>
        </div>

        <!-- Deposit addresses table -->
        <div v-if="overview.initialized" class="card p-5">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.depositAddresses') }}</h3>
          <div v-if="!addresses.length" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.admin.noData') }}</div>
          <table v-else class="w-full text-left text-sm">
            <thead class="text-xs text-gray-500 dark:text-gray-400">
              <tr>
                <th class="py-2">{{ t('payment.crypto.userId') }}</th>
                <th class="py-2">{{ t('payment.crypto.address') }}</th>
                <th class="py-2 text-right">{{ t('payment.crypto.balance') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="a in addresses" :key="a.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="py-2">{{ a.user_id }}</td>
                <td class="py-2 font-mono text-xs">{{ a.address }}</td>
                <td class="py-2 text-right tabular-nums">{{ fmtUsdt(a.last_balance) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Recent sweep jobs -->
        <div v-if="overview.initialized && sweepJobs.length" class="card p-5">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.sweepHistory') }}</h3>
          <table class="w-full text-left text-sm">
            <thead class="text-xs text-gray-500 dark:text-gray-400">
              <tr>
                <th class="py-2">#</th>
                <th class="py-2">{{ t('payment.crypto.status') }}</th>
                <th class="py-2 text-right">{{ t('payment.crypto.tasks') }}</th>
                <th class="py-2 text-right">{{ t('payment.crypto.swept') }}</th>
                <th class="py-2">{{ t('payment.crypto.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="j in sweepJobs" :key="j.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="py-2">{{ j.id }}</td>
                <td class="py-2"><span :class="jobStatusClass(j.status)">{{ j.status }}</span></td>
                <td class="py-2 text-right tabular-nums">{{ j.completed_tasks }}/{{ j.total_tasks }}</td>
                <td class="py-2 text-right tabular-nums">{{ fmtUsdt(j.total_swept) }}</td>
                <td class="py-2 text-xs text-gray-400">{{ formatTime(j.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <!-- Sweep confirmation -->
      <ConfirmDialog
        :show="showSweep"
        :title="t('payment.crypto.sweepConfirmTitle')"
        :message="t('payment.crypto.sweepConfirmMessage')"
        :confirm-text="t('payment.crypto.sweepButton')"
        @confirm="handleSweep"
        @cancel="showSweep = false"
      >
        <template #default>
          <input v-model="totpCode" maxlength="6" class="input mt-3 w-full" :placeholder="t('payment.crypto.totpCode')" />
        </template>
      </ConfirmDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  adminCryptoWalletAPI,
  type CryptoWalletOverview,
  type CryptoDepositAddress,
  type SweepJob
} from '@/api/admin/cryptoWallet'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const busy = ref(false)
const overview = ref<CryptoWalletOverview | null>(null)
const addresses = ref<CryptoDepositAddress[]>([])
const sweepJobs = ref<SweepJob[]>([])

const importMnemonic = ref('')
const generatedMnemonic = ref('')
const collectionAddress = ref('')
const totpCode = ref('')
const showSweep = ref(false)

function fmtUsdt(v: number): string {
  return `${(v ?? 0).toFixed(2)} USDT`
}
function formatTime(s: string): string {
  return s ? new Date(s).toLocaleString() : '-'
}
function jobStatusClass(status: string): string {
  if (status === 'completed') return 'text-green-600 dark:text-green-400'
  if (status === 'failed') return 'text-red-500'
  return 'text-amber-600 dark:text-amber-400'
}
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(t('common.copied'))
  } catch { /* ignore */ }
}
function fail(err: unknown) {
  appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
}

async function loadOverview() {
  const res = await adminCryptoWalletAPI.getOverview()
  overview.value = res
  collectionAddress.value = res.collection_address || ''
}
async function loadAddresses() {
  const res = await adminCryptoWalletAPI.listAddresses({ page: 1, page_size: 100 })
  addresses.value = res.items || []
}
async function loadJobs() {
  const res = await adminCryptoWalletAPI.listSweepJobs(10)
  sweepJobs.value = res.items || []
}

async function refreshAll() {
  loading.value = true
  try {
    await loadOverview()
    if (overview.value?.initialized) {
      await Promise.all([loadAddresses(), loadJobs()])
    }
  } catch (err) { fail(err) } finally { loading.value = false }
}

async function handleInit() {
  busy.value = true
  try {
    const res = await adminCryptoWalletAPI.initWallet({
      mnemonic: importMnemonic.value.trim() || undefined,
      totp_code: totpCode.value
    })
    if (res.mnemonic) generatedMnemonic.value = res.mnemonic
    importMnemonic.value = ''
    totpCode.value = ''
    appStore.showSuccess(t('payment.crypto.initSuccess'))
    await refreshAll()
  } catch (err) { fail(err) } finally { busy.value = false }
}

async function handleSetCollection() {
  busy.value = true
  try {
    await adminCryptoWalletAPI.setCollectionAddress({ address: collectionAddress.value.trim(), totp_code: totpCode.value })
    totpCode.value = ''
    appStore.showSuccess(t('common.saved'))
    await loadOverview()
  } catch (err) { fail(err) } finally { busy.value = false }
}

async function handleRefreshBalances() {
  busy.value = true
  try {
    const res = await adminCryptoWalletAPI.refreshBalances()
    appStore.showSuccess(t('payment.crypto.refreshed', { n: res.refreshed }))
    await Promise.all([loadOverview(), loadAddresses()])
  } catch (err) { fail(err) } finally { busy.value = false }
}

async function handleSweep() {
  busy.value = true
  try {
    await adminCryptoWalletAPI.startSweep({ totp_code: totpCode.value })
    totpCode.value = ''
    showSweep.value = false
    appStore.showSuccess(t('payment.crypto.sweepStarted'))
    await loadJobs()
  } catch (err) { fail(err) } finally { busy.value = false }
}

onMounted(refreshAll)
</script>
