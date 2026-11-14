import { del, get, post, put } from '@/utils/request'

export interface ReviewGroup {
  id: string
  tenant_id: number
  parent_id: string
  name: string
  sort_order: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface ReviewGroupNode {
  group: ReviewGroup
  children: ReviewGroupNode[]
}

export interface ReviewCardSource {
  id: string
  tenant_id: number
  card_id: string
  source_type: string
  knowledge_id: string
  chunk_id: string
  message_id: string
  path: string
  symbol: string
  start_line: number
  end_line: number
  content_hash: string
  excerpt: string
  is_primary: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface ReviewCard {
  id: string
  tenant_id: number
  group_id: string
  card_type: string
  question: string
  answer: string
  explanation: string
  tags: string[] | string
  status: 'active' | 'suspended' | 'orphaned' | 'deleted'
  review_level: number
  due_at: string
  lapses: number
  review_count: number
  question_hash: string
  last_reviewed_at?: string | null
  source_updated: boolean
  created_at: string
  updated_at: string
}

export interface ReviewCardDetail {
  card: ReviewCard
  sources: ReviewCardSource[]
}

export interface ReviewStats {
  total_cards: number
  reviewed_30d: number
  remembered_30d: number
  retention_30d: number
}

export interface ReviewQueue {
  due: ReviewCard[]
  new: ReviewCard[]
  completed_today: number
  limit: number
}

export interface ReviewCardSourceInput {
  source_type?: string
  knowledge_id?: string
  chunk_id?: string
  message_id?: string
  path?: string
  symbol?: string
  start_line?: number
  end_line?: number
  content_hash?: string
  excerpt?: string
  is_primary?: boolean
}

export interface CreateReviewCardRequest {
  group_id?: string
  question: string
  answer: string
  explanation?: string
  tags?: string[]
  sources: ReviewCardSourceInput[]
}

export function listReviewGroups() {
  return get<{ data: ReviewGroupNode[] }>('/api/v1/review/groups')
}

export function createReviewGroup(payload: { parent_id?: string; name: string; sort_order?: number }) {
  return post<{ data: ReviewGroup }>('/api/v1/review/groups', payload)
}

export function updateReviewGroup(id: string, payload: { name?: string; sort_order?: number }) {
  return put<{ data: ReviewGroup }>(`/api/v1/review/groups/${encodeURIComponent(id)}`, payload)
}

export function moveReviewGroup(id: string, parentId: string) {
  return post<{ success: boolean }>(`/api/v1/review/groups/${encodeURIComponent(id)}/move`, { parent_id: parentId })
}

export function deleteReviewGroup(id: string, deleteCards = false) {
  return del<{ success: boolean }>(`/api/v1/review/groups/${encodeURIComponent(id)}?delete_cards=${deleteCards}`)
}

export function listReviewCards(params: {
  group_id?: string
  include_subtree?: boolean
  status?: string
  keyword?: string
  limit?: number
  offset?: number
}) {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') query.set(key, String(value))
  })
  const suffix = query.toString() ? `?${query}` : ''
  return get<{ data: ReviewCard[] }>(`/api/v1/review/cards${suffix}`)
}

export function createReviewCard(payload: CreateReviewCardRequest) {
  return post<{ data: ReviewCard }>('/api/v1/review/cards', payload)
}

export function getReviewCard(id: string) {
  return get<{ data: ReviewCardDetail }>(`/api/v1/review/cards/${encodeURIComponent(id)}`)
}

export function updateReviewCard(id: string, payload: {
  question?: string
  answer?: string
  explanation?: string
  tags?: string[]
}) {
  return put<{ data: ReviewCard }>(`/api/v1/review/cards/${encodeURIComponent(id)}`, payload)
}

export function moveReviewCard(id: string, groupId: string) {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}/move`, { group_id: groupId })
}

export function setReviewCardStatus(id: string, status: 'active' | 'suspended') {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}/status`, { status })
}

export function deleteReviewCard(id: string) {
  return del<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}`)
}

export function restoreReviewCard(id: string) {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}/restore`, {})
}

export function getReviewQueue(params?: { group_id?: string; limit?: number }) {
  const query = new URLSearchParams()
  if (params?.group_id) query.set('group_id', params.group_id)
  if (params?.limit) query.set('limit', String(params.limit))
  const suffix = query.toString() ? `?${query}` : ''
  return get<{ data: ReviewQueue }>(`/api/v1/review/queue${suffix}`)
}

export function submitReview(payload: { card_id: string; rating: 'forgot' | 'fuzzy' | 'remembered'; client_request_id: string }) {
  return post<{ data: ReviewCard }>('/api/v1/review/reviews', payload)
}

export function markReviewSourceUnavailable(cardId: string, sourceId: string) {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(cardId)}/sources/${encodeURIComponent(sourceId)}/unavailable`, {})
}

export function mergeReviewCards(payload: { target_card_id: string; source_card_ids: string[] }) {
  return post<{ success: boolean }>('/api/v1/review/cards/merge', payload)
}

export function rebindReviewSource(cardId: string, sourceId: string, payload: {
  knowledge_id: string
  chunk_id?: string
  path?: string
  symbol?: string
  start_line?: number
  end_line?: number
  content_hash?: string
  excerpt?: string
}) {
  return put<{ success: boolean }>(
    `/api/v1/review/cards/${encodeURIComponent(cardId)}/sources/${encodeURIComponent(sourceId)}`,
    payload,
  )
}

export function exportReviewData() {
  return get<{ success: boolean; data: unknown }>('/api/v1/review/export')
}

export function getReviewStats() {
  return get<{ data: ReviewStats }>('/api/v1/review/stats')
}
