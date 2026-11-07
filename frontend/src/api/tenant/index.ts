// 个人版只有单个内部工作区；前端只消费 TenantInfo。
export interface TenantInfo {
  id: number
  name: string
  description?: string
  status?: string
  business?: string
  storage_quota?: number
  storage_used?: number
  created_at: string
  updated_at: string
}
