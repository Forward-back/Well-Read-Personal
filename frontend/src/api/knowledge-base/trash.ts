import { del, get, post } from '@/utils/request'

export interface KnowledgeTrashItem {
  id: string
  title: string
  file_name: string
  knowledge_base_id: string
  file_type: string
  deleted_at: string
  purge_after: string
  size: number
}

export interface KnowledgeTrashResult {
  total: number
  items: KnowledgeTrashItem[]
}

export function listKnowledgeTrash(params: { kb_id?: string; keyword?: string; page?: number; page_size?: number }) {
  const query = new URLSearchParams()
  if (params.kb_id) query.set('kb_id', params.kb_id)
  if (params.keyword) query.set('keyword', params.keyword)
  query.set('page', String(params.page || 1))
  query.set('page_size', String(params.page_size || 20))
  return get<{ data: KnowledgeTrashResult }>(`/api/v1/knowledge/trash?${query.toString()}`)
}

export function restoreKnowledgeTrash(id: string) {
  return post<{ data: { restored: string[] } }>(`/api/v1/knowledge/${encodeURIComponent(id)}/restore`, {})
}

export function purgeKnowledgeTrash(id: string) {
  return del<{ data: { purged: number } }>(`/api/v1/knowledge/trash/${encodeURIComponent(id)}`)
}
