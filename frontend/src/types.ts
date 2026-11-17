export interface Machine {
  id: string
  name: string
  online: boolean
  values: Partial<Record<string, number>>
  updated_at: string | null
}

export type WorkOrderStatus = 'created' | 'released' | 'running' | 'completed' | 'cancelled'

export interface WorkOrder {
  id: number
  order_no: string
  product_spec: string
  planned_qty: number
  produced_qty: number
  machine_id: string
  status: WorkOrderStatus
  version: number
  created_at: string
  updated_at: string
}

export interface Alarm {
  id: number
  machine_id: string
  kind: string
  message: string
  value: number
  active: boolean
  occurred_at: string
}