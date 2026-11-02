// 一台機器長什麼樣子(跟後端 /api/machines 回傳的一樣)
export interface Machine {
  id: string
  name: string
  online: boolean
  values: Partial<Record<string, number>>
  updated_at: string | null
}