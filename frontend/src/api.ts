import type { Machine } from './types'

// 向後端要機台資料
export async function getMachines(): Promise<Machine[]> {
  const res = await fetch('/api/machines')
  if (!res.ok) {
    throw new Error(`無法取得機台資料 (${res.status})`)
  }
  return res.json()
}