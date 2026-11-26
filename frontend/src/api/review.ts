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
  card_count: number
  due_count: number
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

export type ReviewRating = 'forgot' | 'fuzzy' | 'remembered' | 'easy'
export type ReviewVulnerability = 'severe' | 'warning' | 'observe' | 'healthy' | 'none'

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
  stability: number
  forget_streak: number
  due_at: string
  lapses: number
  review_count: number
  question_hash: string
  last_reviewed_at?: string | null
  suspended_until?: string | null
  source_updated: boolean
  vulnerability?: ReviewVulnerability
  vulnerability_reason?: string
  created_at: string
  updated_at: string
}

export interface ReviewCardDetail {
  card: ReviewCard
  sources: ReviewCardSource[]
}

export interface ReviewWeakTopic {
  group_id: string
  group_name: string
  reviewed: number
  retention: number
}

export interface ReviewGroupStat {
  group_id: string
  group_name: string
  reviewed: number
  retention: number
}

export interface ReviewCardStat {
  card_id: string
  question: string
  group_id: string
  group_name: string
  easy: number
  remembered: number
  fuzzy: number
  forgot: number
  total: number
  forget_rate: number
  lapses: number
  forget_streak: number
}

export interface ReviewDueForecast {
  date: string
  count: number
}

export interface ReviewRatingTrend {
  date: string
  easy: number
  remembered: number
  fuzzy: number
  forgot: number
  total: number
  retention: number
}

export interface ReviewForgettingPoint {
  day: number
  actual: number
  theoretical: number
  sample: number
}

export interface ReviewStats {
  days: number
  total_cards: number
  reviewed: number
  remembered: number
  retention: number
  rating_distribution: ReviewRatingCounts
  group_ranking: ReviewGroupStat[]
  difficult_cards: ReviewCardStat[]
  due_forecast: ReviewDueForecast[]
  streak_days: number
  rating_trend: ReviewRatingTrend[]
  forgetting_curve: ReviewForgettingPoint[]
  reviewed_30d: number
  remembered_30d: number
  easy_30d: number
  retention_30d: number
  weak_topics?: ReviewWeakTopic[]
}

export interface ReviewSession {
  id: string
  tenant_id: number
  mode: 'daily' | 'manual'
  status: 'active' | 'completed' | 'abandoned' | 'expired'
  group_id: string
  planned_card_ids: string[]
  planned_count: number
  completed_count: number
  remembered_count: number
  easy_count: number
  fuzzy_count: number
  forgot_count: number
  review_day: string
  started_at: string
  completed_at?: string | null
  created_at: string
  updated_at: string
}

export interface ReviewLog {
  id: string
  tenant_id: number
  card_id: string
  session_id: string
  rating: ReviewRating
  level_before: number
  level_after: number
  interval_days: number
  stability_before: number
  stability_after: number
  reviewed_at: string
  client_request_id: string
  created_at: string
}

export interface ReviewSessionState {
  session: ReviewSession
  cards: ReviewCard[]
  logs: ReviewLog[]
}

export interface ReviewRatingCounts {
  easy: number
  remembered: number
  fuzzy: number
  forgot: number
  total: number
}

export interface ReviewCardPerformance {
  card: ReviewCard
  counts: ReviewRatingCounts
  recent_counts: ReviewRatingCounts
  recent_forget_rate: number
  forget_streak: number
  warning: 'none' | 'watch' | 'high'
  warning_reason: string
  logs: ReviewLog[]
}

export interface ReviewQueue {
  due: ReviewCard[]
  new: ReviewCard[]
  completed_today: number
  limit: number
  new_limit: number
  streak_days: number
}

export type ReviewHeatmapColumnKind = 'tag' | 'knowledge_base' | 'source' | 'uncategorized' | 'other'
export type ReviewHeatmapRowKind = 'group' | 'direct' | 'other'

export interface ReviewHeatmapRow {
  id: string
  group_id: string
  name: string
  path: string
  kind: ReviewHeatmapRowKind
  drillable: boolean
  virtual: boolean
  total_cards: number
}

export interface ReviewHeatmapColumn {
  key: string
  title: string
  kind: ReviewHeatmapColumnKind
  source_id: string
  knowledge_base_id: string
}

export interface ReviewHeatmapCell {
  row_id: string
  column_key: string
  level: ReviewVulnerability
  intensity: number
  total: number
  severe: number
  warning: number
  observe: number
  healthy: number
  no_data: number
  clickable: boolean
}

export interface ReviewHeatmapFormula {
  severe_weight: number
  warning_weight: number
  observe_weight: number
  denominator: number
  text: string
}

export interface ReviewHeatmapInsight {
  kind: 'row' | 'column'
  row_id: string
  column_key: string
  message: string
}

export interface ReviewHeatmap {
  rows: ReviewHeatmapRow[]
  columns: ReviewHeatmapColumn[]
  cells: ReviewHeatmapCell[]
  priorities: ReviewHeatmapCell[]
  observe_count: number
  insight?: ReviewHeatmapInsight | null
  scope_group_id: string
  scope_path: string
  row_total: number
  column_total: number
  row_truncated: boolean
  column_truncated: boolean
  formula?: ReviewHeatmapFormula | null
}

export interface ReviewClusterCard {
  card: ReviewCard
  level: ReviewVulnerability
  last_rating: ReviewRating | ''
  suggested_action: 'relearn' | 'observe' | ''
  source?: ReviewCardSource | null
  knowledge_base_id: string
}

export interface ReviewClusterDetail {
  cell: ReviewHeatmapCell
  row: ReviewHeatmapRow
  column: ReviewHeatmapColumn
  cards: ReviewClusterCard[]
  reasons: string[]
  formula?: ReviewHeatmapFormula | null
  scope_group_id: string
}

export interface ReviewRootCause {
  summary: string
  reasons: string[]
  focus_suggestions: string[]
  evidence_card_ids: string[]
  model_id: string
  source: 'llm' | 'rule'
}

export interface ApplyCardSplitResult {
  created: ReviewCard[]
  archived_original: boolean
}

export type ReviewBatchAction = 'delete' | 'move' | 'add_tags' | 'reset'

export interface BatchReviewCardsRequest {
  card_ids: string[]
  action: ReviewBatchAction
  group_id?: string
  tags?: string[]
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
  tag?: string
  due_only?: boolean
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

export function setReviewCardStatus(
  id: string,
  status: 'active' | 'suspended',
  suspendedUntil?: string | null,
) {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}/status`, {
    status,
    ...(suspendedUntil ? { suspended_until: suspendedUntil } : {}),
  })
}

export function deleteReviewCard(id: string) {
  return del<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}`)
}

export function restoreReviewCard(id: string) {
  return post<{ success: boolean }>(`/api/v1/review/cards/${encodeURIComponent(id)}/restore`, {})
}

export function resetReviewCardSchedule(id: string) {
  return post<{ data: ReviewCard }>(`/api/v1/review/cards/${encodeURIComponent(id)}/reset`, {})
}

export function batchReviewCards(payload: BatchReviewCardsRequest) {
  return post<{ data: { affected: number } }>('/api/v1/review/cards/batch', payload)
}

export function getReviewHeatmap(params?: {
  group_id?: string
  row_group_ids?: string[]
  row_limit?: number
  column_limit?: number
  expand?: boolean
}) {
  const query = new URLSearchParams()
  if (params?.group_id) query.set('group_id', params.group_id)
  if (params?.row_group_ids?.length) query.set('row_group_ids', params.row_group_ids.join(','))
  if (params?.row_limit) query.set('row_limit', String(params.row_limit))
  if (params?.column_limit) query.set('column_limit', String(params.column_limit))
  if (params?.expand) query.set('expand', '1')
  const suffix = query.toString() ? `?${query}` : ''
  return get<{ data: ReviewHeatmap }>(`/api/v1/review/heatmap${suffix}`)
}

export function getReviewClusterDetail(params: {
  scope_group_id?: string
  row_group_id: string
  column_key: string
  direct?: boolean
}) {
  const query = new URLSearchParams({ row_group_id: params.row_group_id, column_key: params.column_key })
  if (params.scope_group_id) query.set('scope_group_id', params.scope_group_id)
  if (params.direct) query.set('direct', '1')
  return get<{ data: ReviewClusterDetail }>(`/api/v1/review/clusters/detail?${query}`)
}

export function getReviewRootCause(payload: {
  scope_group_id?: string
  row_group_id: string
  column_key: string
  direct?: boolean
}) {
  return post<{ data: ReviewRootCause }>('/api/v1/review/clusters/root-cause', payload)
}

export function previewCardSplit(cardId: string, count = 2) {
  return post<{ data: ReviewGenerationPreview }>(
    `/api/v1/review/cards/${encodeURIComponent(cardId)}/split/preview`,
    { count },
  )
}

export function applyCardSplit(cardId: string, cards: CreateReviewCardRequest[]) {
  return post<{ data: ApplyCardSplitResult }>(
    `/api/v1/review/cards/${encodeURIComponent(cardId)}/split/apply`,
    { cards },
  )
}

export function previewCardAngles(cardId: string, count = 1) {
  return post<{ data: ReviewGenerationPreview }>(
    `/api/v1/review/cards/${encodeURIComponent(cardId)}/angles/preview`,
    { count },
  )
}

export function getReviewQueue(params?: { group_id?: string; limit?: number }) {
  const query = new URLSearchParams()
  if (params?.group_id) query.set('group_id', params.group_id)
  if (params?.limit) query.set('limit', String(params.limit))
  const suffix = query.toString() ? `?${query}` : ''
  return get<{ data: ReviewQueue }>(`/api/v1/review/queue${suffix}`)
}

export function submitReview(payload: {
  card_id: string
  session_id?: string
  rating: ReviewRating
  client_request_id: string
}) {
  return post<{ data: ReviewCard }>('/api/v1/review/reviews', payload)
}

export function startReviewSession(payload: {
  mode: 'daily' | 'manual'
  group_id?: string
  card_ids?: string[]
  limit?: number
  force_new?: boolean
}) {
  return post<{ data: ReviewSessionState }>('/api/v1/review/sessions', payload)
}

export function getActiveReviewSession() {
  return get<{ data: ReviewSessionState | null }>('/api/v1/review/sessions/active')
}

export function getReviewSession(id: string) {
  return get<{ data: ReviewSessionState }>(`/api/v1/review/sessions/${encodeURIComponent(id)}`)
}

export function abandonReviewSession(id: string) {
  return post<{ success: boolean }>(`/api/v1/review/sessions/${encodeURIComponent(id)}/abandon`, {})
}

export function listReviewSessions(limit = 50) {
  return get<{ data: ReviewSession[] }>(`/api/v1/review/sessions?limit=${limit}`)
}

export function getReviewCardPerformance(id: string) {
  return get<{ data: ReviewCardPerformance }>(`/api/v1/review/cards/${encodeURIComponent(id)}/performance`)
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

export function getReviewStats(days = 30) {
  return get<{ data: ReviewStats }>(`/api/v1/review/stats?days=${days}`)
}

export interface GeneratedReviewCard {
  question: string
  answer: string
  explanation: string
  source_knowledge_id: string
  source_chunk_id: string
  source_title: string
  source_excerpt: string
  duplicate: boolean
  existing_card_id: string
  sources: ReviewCardSourceInput[]
}

export interface ReviewGenerationPreview {
  cards: GeneratedReviewCard[]
  model_id: string
}

export function previewReviewGeneration(payload: {
  knowledge_base_id: string
  knowledge_ids?: string[]
  group_id?: string
  count: number
}) {
  return post<{ data: ReviewGenerationPreview }>('/api/v1/review/generations/preview', payload)
}

export function importReviewCards(payload: {
  group_id?: string
  cards: CreateReviewCardRequest[]
}) {
  return post<{ data: { imported: number } }>('/api/v1/review/generations/import', payload)
}
