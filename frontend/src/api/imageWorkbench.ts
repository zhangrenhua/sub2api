/**
 * 画图工作台（fork）API：提交异步生图任务 / 查询任务 / 历史 / 删除。
 * 任务在服务端 worker 异步执行（刷新页面不影响），前端轮询 /tasks 获取状态与结果。
 * 图片文件经 token 鉴权由 url 直接给 <img src> 使用（免 JWT）。
 */
import { apiClient } from './client'

export interface WorkbenchImage {
  id: number
  prompt: string
  revised_prompt: string
  model: string
  size: string
  quality: string
  session_id: string
  url: string
  mime: string
  bytes: number
  created_at: string
  expires_at: string
}

export type WorkbenchTaskStatus = 'queued' | 'running' | 'done' | 'error'

export interface WorkbenchTask {
  id: number
  status: WorkbenchTaskStatus
  prompt: string
  model: string
  size: string
  n: number
  error: string
  images: WorkbenchImage[]
  created_at: string
  updated_at: string
}

export interface GenerateParams {
  api_key_id: number
  prompt: string
  model: string
  size?: string
  quality?: string
  n?: number
  session_id?: string
  base_image_id?: number
  base_images_b64?: string[]
}

/** 提交一个异步生图任务，返回新建任务（queued）。 */
export async function generate(params: GenerateParams): Promise<WorkbenchTask | null> {
  // 上传多张底图时请求体可能较大，单独放宽超时（覆盖 apiClient 默认）
  const { data } = await apiClient.post<{ task: WorkbenchTask }>('/image-workbench/generate', params, {
    timeout: 120000
  })
  return (data as unknown as { task?: WorkbenchTask })?.task ?? null
}

/** 列出当前用户的任务（可按 status 过滤），用于轮询与任务队列页。 */
export async function listTasks(status = '', limit = 50, offset = 0): Promise<WorkbenchTask[]> {
  const params: Record<string, string | number> = { limit, offset }
  if (status) params.status = status
  const { data } = await apiClient.get<{ tasks: WorkbenchTask[] }>('/image-workbench/tasks', { params })
  return (data as unknown as { tasks?: WorkbenchTask[] })?.tasks ?? []
}

export async function history(limit = 50, offset = 0): Promise<WorkbenchImage[]> {
  const { data } = await apiClient.get<{ images: WorkbenchImage[] }>('/image-workbench/history', {
    params: { limit, offset }
  })
  return (data as unknown as { images?: WorkbenchImage[] })?.images ?? []
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/image-workbench/${id}`)
}

export const imageWorkbenchAPI = { generate, listTasks, history, remove }
export default imageWorkbenchAPI
