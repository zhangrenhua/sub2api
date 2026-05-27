/**
 * Admin Crypto (TRC20) HD Wallet API endpoints.
 *
 * Backs the self-custodied USDT collection wallet: balance overview, per-user
 * deposit addresses, and the TOTP-gated sensitive operations (initialization,
 * collection address, one-click sweep).
 */

import { apiClient } from '../client'

export interface CryptoWalletOverview {
  initialized: boolean
  fee_address: string
  fee_trx_balance: number
  collection_address: string
  collection_balance: number
  deposit_addresses: number
  deposit_total_usdt: number
  // ERC20 (Ethereum)
  eth_fee_address: string
  eth_fee_balance: number
  eth_collection_address: string
  eth_collection_balance: number
  erc20_deposit_addresses: number
  erc20_deposit_total_usdt: number
  balances_as_of: string
}

export interface CryptoDepositAddress {
  id: number
  user_id: number
  network: string
  address: string
  derivation_index: number
  last_balance: number
  last_balance_at: string | null
  created_at: string
}

export interface InitWalletResult {
  /** Present ONLY when freshly generated — show once for offline backup. */
  mnemonic?: string
  fee_address: string
  generated: boolean
  initialized: boolean
}

export type SweepTaskStatus =
  | 'pending'
  | 'gas_funding'
  | 'gas_confirmed'
  | 'sweeping'
  | 'confirmed'
  | 'failed'

export interface SweepJob {
  id: number
  status: 'pending' | 'running' | 'completed' | 'failed'
  created_by: string
  total_tasks: number
  completed_tasks: number
  total_swept: number
  collection_address: string
  error: string
  created_at: string
  finished_at: string | null
}

export interface SweepTask {
  id: number
  job_id: number
  user_id: number
  address: string
  amount: number
  status: SweepTaskStatus
  gas_fund_tx: string
  sweep_tx: string
  error: string
}

// The shared apiClient response interceptor unwraps the business payload into
// response.data, so each method returns that .data directly.
export const adminCryptoWalletAPI = {
  /** Wallet balance overview for the dashboard. */
  async getOverview(): Promise<CryptoWalletOverview> {
    return (await apiClient.get<CryptoWalletOverview>('/admin/payment/crypto/overview')).data
  },

  /** Paged per-user deposit addresses with cached balances. network: ''|TRC20|ERC20. */
  async listAddresses(params?: { page?: number; page_size?: number; network?: string }): Promise<{ items: CryptoDepositAddress[]; total: number }> {
    return (await apiClient.get<{ items: CryptoDepositAddress[]; total: number }>(
      '/admin/payment/crypto/addresses',
      { params }
    )).data
  },

  /** Refresh cached on-chain balances. */
  async refreshBalances(): Promise<{ refreshed: number }> {
    return (await apiClient.post<{ refreshed: number }>('/admin/payment/crypto/refresh-balances')).data
  },

  /**
   * Initialize/import the master mnemonic (one-time).
   * Leave mnemonic empty to generate a fresh one (returned once for backup).
   */
  async initWallet(data: { mnemonic?: string }): Promise<InitWalletResult> {
    return (await apiClient.post<InitWalletResult>('/admin/payment/crypto/wallet/init', data)).data
  },

  /** Set the TRC20 sweep destination (cold) address. */
  async setCollectionAddress(data: { address: string }): Promise<void> {
    await apiClient.put('/admin/payment/crypto/wallet/collection-address', data)
  },

  /** Set the ERC20 (ETH) sweep destination (cold) address. */
  async setEthCollectionAddress(data: { address: string }): Promise<void> {
    await apiClient.put('/admin/payment/crypto/wallet/eth-collection-address', data)
  },

  /** Trigger a one-click TRC20 consolidation. */
  async startSweep(): Promise<SweepJob> {
    return (await apiClient.post<SweepJob>('/admin/payment/crypto/sweep')).data
  },

  /** Trigger a one-click ERC20 consolidation. */
  async startSweepEth(): Promise<SweepJob> {
    return (await apiClient.post<SweepJob>('/admin/payment/crypto/eth-sweep')).data
  },

  /** Sweep job + per-address task progress. */
  async getSweepJob(jobId: number): Promise<{ job: SweepJob; tasks: SweepTask[] }> {
    return (await apiClient.get<{ job: SweepJob; tasks: SweepTask[] }>(
      `/admin/payment/crypto/sweep/${jobId}`
    )).data
  },

  /** Recent sweep jobs. */
  async listSweepJobs(limit = 20): Promise<{ items: SweepJob[] }> {
    return (await apiClient.get<{ items: SweepJob[] }>('/admin/payment/crypto/sweeps', {
      params: { limit }
    })).data
  }
}
