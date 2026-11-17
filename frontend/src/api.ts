import type { Alarm, Machine, WorkOrder, WorkOrderStatus } from './types'

export class ApiError extends Error {}

// 後端回傳的錯誤代碼,轉成看得懂的中文
const MESSAGES: Record<string, string> = {
  invalid_transition: '工單目前的狀態不能做這個操作',
  version_conflict: '這張工單剛剛被修改過,已重新載入最新資料,請再操作一次',
  duplicate: '工單編號已經存在',
  machine_busy: '這台機器已經有一張生產中的工單',
  not_found: '找不到這張工單',
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    let message = `請求失敗 (${res.status})`
    try {
      const j = await res.json()
      const code: string = j.error?.code ?? ''
      message = MESSAGES[code] ?? j.error?.message ?? message
    } catch {
      // 後端沒有回 JSON,就用預設訊息
    }
    throw new ApiError(message)
  }
  return res.json() as Promise<T>
}

export const api = {
  machines: () => request<Machine[]>('GET', '/api/machines'),
  alarms: () => request<Alarm[]>('GET', '/api/alarms'),
  workOrders: () => request<WorkOrder[]>('GET', '/api/work-orders'),

  createWorkOrder: (order_no: string, product_spec: string, planned_qty: number) =>
    request<WorkOrder>('POST', '/api/work-orders', { order_no, product_spec, planned_qty }),

  // 改狀態:派工、開工、完工、取消。一定要帶上目前的版本號
  transition: (wo: WorkOrder, to: WorkOrderStatus, machine_id = '') =>
    request<WorkOrder>('POST', `/api/work-orders/${wo.id}/transition`, {
      to,
      machine_id,
      version: wo.version,
    }),

  report: (wo: WorkOrder, qty: number) =>
    request<WorkOrder>('POST', `/api/work-orders/${wo.id}/report`, {
      qty,
      version: wo.version,
    }),
}