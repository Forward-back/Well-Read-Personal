import { del, get, patch, post, put } from '@/utils/request'

export type DocCollaborationType = 'issue' | 'cr'
export type DocIssueStatus = 'open' | 'closed'
export type DocIssueType = 'data_error' | 'outdated' | 'unclear' | 'format' | 'other'
export type DocCRStatus = 'draft' | 'pending_review' | 'changes_requested' | 'merged' | 'closed'
export type DocCRReviewerStatus = 'pending' | 'approved' | 'changes_requested'

export interface CollaborationPage {
  total: number
  page: number
  page_size: number
}

export interface DocIssue {
  id: string
  seq_id: number
  tenant_id: number
  knowledge_base_id: string
  knowledge_id: string
  chunk_id: string
  issue_type: DocIssueType
  anchor_text: string
  anchor_start: number
  anchor_end: number
  anchor_line_start: number
  anchor_line_end: number
  title: string
  body: string
  status: DocIssueStatus
  anchor_orphaned: boolean
  created_by: string
  created_at: string
  updated_at: string
  closed_by?: string
  closed_at?: string | null
  created_by_name?: string
  knowledge_title?: string
  chunk_excerpt?: string
  chunk_index?: number
  comments_count?: number
}

export interface DocCRItem {
  id: string
  tenant_id: number
  cr_id: string
  chunk_id: string
  original_content: string
  suggested_content: string
  original_revision: number
  viewed_by: string[]
  anchor_text: string
  seq: number
  created_at: string
  updated_at: string
  chunk_index?: number
  anchor_orphaned?: boolean
  conflict?: boolean
  current_content?: string
}

export interface DocCRReviewEntry {
  id: string
  tenant_id: number
  cr_id: string
  action: 'accept' | 'reject'
  comment: string
  reviewed_by: string
  created_at: string
  reviewed_by_name?: string
}

export interface DocCRReviewer {
  id: string
  tenant_id: number
  cr_id: string
  user_id: string
  status: DocCRReviewerStatus
  comment: string
  decided_at?: string | null
  created_at: string
  updated_at: string
  user_name?: string
}

export interface DocChangeRequest {
  id: string
  seq_id: number
  tenant_id: number
  knowledge_base_id: string
  knowledge_id: string
  title: string
  description: string
  status: DocCRStatus
  base_version_id: string
  merged_version_id: string
  merged_at?: string | null
  closed_by?: string
  closed_at?: string | null
  tags: string[]
  draft_updated_at?: string | null
  anchor_orphaned: boolean
  created_by: string
  created_at: string
  updated_at: string
  reviewed_by?: string
  can_review?: boolean
  reviewed_at?: string | null
  review_comment?: string
  created_by_name?: string
  reviewed_by_name?: string
  knowledge_title?: string
  item_count?: number
  items?: DocCRItem[]
  reviews?: DocCRReviewEntry[]
  reviewers?: DocCRReviewer[]
  linked_issues?: DocIssue[]
}

export interface DocComment {
  id: string
  tenant_id: number
  knowledge_base_id: string
  target_type: DocCollaborationType
  target_id: string
  body: string
  created_by: string
  created_at: string
  deleted_by?: string
  deleted_at?: string | null
  created_by_name?: string
  deleted?: boolean
}

export interface DocAnchorComment {
  id: string
  tenant_id: number
  knowledge_base_id: string
  knowledge_id: string
  chunk_id: string
  parent_id: string
  anchor_text: string
  anchor_start: number
  anchor_end: number
  anchor_line_start: number
  anchor_line_end: number
  body: string
  resolved_at?: string | null
  resolved_by?: string
  created_by: string
  created_at: string
  deleted_by?: string
  deleted_at?: string | null
  created_by_name?: string
}

export interface DocVersion {
  id: string
  tenant_id: number
  knowledge_base_id: string
  knowledge_id: string
  version_seq: number
  version_label: string
  source_type: 'upload' | 'manual_edit' | 'cr_merge' | 'rollback'
  source_id: string
  title: string
  description: string
  chunk_count: number
  created_by: string
  created_at: string
  is_current: boolean
  created_by_name?: string
}

export interface DocVersionChunk {
  id: string
  tenant_id: number
  version_id: string
  chunk_id: string
  chunk_index: number
  content: string
  is_enabled: boolean
}

export interface DocCRChunkLock {
  chunk_id: string
  cr_id: string
  seq_id: number
  title: string
  status: DocCRStatus
}

export interface ChunkCitationStat {
  chunk_id: string
  total: number
  last_30_days: number
}

export interface CollaborationItem {
  type: DocCollaborationType
  id: string
  seq_id: number
  title: string
  status: DocIssueStatus | DocCRStatus
  knowledge_id: string
  knowledge_base_id: string
  knowledge_title?: string
  chunk_id?: string
  created_by: string
  created_by_name?: string
  created_at: string
  updated_at: string
  anchor_orphaned: boolean
  comments_count: number
  item_count?: number
  reviewed_at?: string | null
}

export interface DocIssueListResult extends CollaborationPage {
  items: DocIssue[]
}

export interface DocCRListResult extends CollaborationPage {
  items: DocChangeRequest[]
}

export interface CollaborationListResult extends CollaborationPage {
  items: CollaborationItem[]
}

export interface CollaborationListParams {
  knowledge_id?: string
  chunk_id?: string
  type?: DocCollaborationType | ''
  status?: string
  created_by?: string
  page?: number
  page_size?: number
}

export interface DocCRItemPayload {
  chunk_id: string
  suggested_content: string
  anchor_text?: string
}

export interface CreateDocCRPayload {
  knowledge_id: string
  title: string
  description?: string
  items: DocCRItemPayload[]
  reviewer_ids?: string[]
  tags?: string[]
  linked_issue_ids?: string[]
  base_version_id?: string
  as_draft?: boolean
}

function queryString(params?: Record<string, unknown>): string {
  if (!params) return ''
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  })
  const qs = query.toString()
  return qs ? `?${qs}` : ''
}

function kbPath(kbId: string, suffix: string): string {
  return `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}${suffix}`
}

function docPath(kbId: string, knowledgeId: string, suffix: string): string {
  return kbPath(kbId, `/documents/${encodeURIComponent(knowledgeId)}${suffix}`)
}

export function listDocCollaborations(kbId: string, params?: CollaborationListParams) {
  return get(kbPath(kbId, `/collaborations${queryString(params as Record<string, unknown>)}`))
}

export function listDocIssues(kbId: string, params?: CollaborationListParams) {
  return get(kbPath(kbId, `/issues${queryString(params as Record<string, unknown>)}`))
}

export function createDocIssue(kbId: string, data: {
  knowledge_id: string
  chunk_id?: string
  issue_type?: DocIssueType
  anchor_text?: string
  anchor_start?: number
  anchor_end?: number
  anchor_line_start?: number
  anchor_line_end?: number
  title: string
  body?: string
}) {
  return post(kbPath(kbId, '/issues'), data)
}

export function getDocIssue(kbId: string, issueId: string) {
  return get(kbPath(kbId, `/issues/${encodeURIComponent(issueId)}`))
}

export function updateDocIssue(kbId: string, issueId: string, data: {
  title?: string
  body?: string
  status?: DocIssueStatus
  issue_type?: DocIssueType
}) {
  return patch(kbPath(kbId, `/issues/${encodeURIComponent(issueId)}`), data)
}

export function deleteDocIssue(kbId: string, issueId: string) {
  return del(kbPath(kbId, `/issues/${encodeURIComponent(issueId)}`))
}

export function listDocChangeRequests(kbId: string, params?: CollaborationListParams) {
  return get(kbPath(kbId, `/change-requests${queryString(params as Record<string, unknown>)}`))
}

export function createDocChangeRequest(kbId: string, data: CreateDocCRPayload) {
  return post(kbPath(kbId, '/change-requests'), data)
}

export function getDocChangeRequest(kbId: string, crId: string) {
  return get(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}`))
}

export function resubmitDocChangeRequest(kbId: string, crId: string, data: CreateDocCRPayload) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/resubmit`), data)
}

export function reviewDocChangeRequest(kbId: string, crId: string, data: {
  action: 'approve' | 'request_changes' | 'reject'
  comment?: string
}) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/review`), data)
}

export function mergeDocChangeRequest(kbId: string, crId: string, data: {
  comment?: string
  close_issue_ids?: string[]
}) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/merge`), data)
}

export function closeDocChangeRequest(kbId: string, crId: string, comment = '') {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/close`), { comment })
}

export function updateDocCRDraft(kbId: string, crId: string, data: Partial<CreateDocCRPayload>) {
  return patch(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/draft`), data)
}

export function submitDocCRDraft(kbId: string, crId: string, data: CreateDocCRPayload) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/submit`), data)
}

export function updateDocCRReviewers(kbId: string, crId: string, reviewerIds: string[]) {
  return put(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/reviewers`), {
    reviewer_ids: reviewerIds,
  })
}

export function markDocCRViewed(kbId: string, crId: string) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/viewed`), {})
}

export function listIssueComments(kbId: string, issueId: string) {
  return get(kbPath(kbId, `/issues/${encodeURIComponent(issueId)}/comments`))
}

export function createIssueComment(kbId: string, issueId: string, body: string) {
  return post(kbPath(kbId, `/issues/${encodeURIComponent(issueId)}/comments`), { body })
}

export function listCRComments(kbId: string, crId: string) {
  return get(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/comments`))
}

export function createCRComment(kbId: string, crId: string, body: string) {
  return post(kbPath(kbId, `/change-requests/${encodeURIComponent(crId)}/comments`), { body })
}

export function deleteDocComment(kbId: string, commentId: string) {
  return del(kbPath(kbId, `/comments/${encodeURIComponent(commentId)}`))
}

export function listAnchorComments(kbId: string, knowledgeId: string) {
  return get(docPath(kbId, knowledgeId, '/anchor-comments'))
}

export function listDocCRChunkLocks(kbId: string, knowledgeId: string) {
  return get(docPath(kbId, knowledgeId, '/chunk-locks'))
}

export function listChunkCitationStats(kbId: string, chunkIds: string[]) {
  return get(kbPath(kbId, `/citation-stats${queryString({ chunk_ids: chunkIds.join(',') })}`))
}

export function createAnchorComment(kbId: string, knowledgeId: string, data: {
  knowledge_id: string
  chunk_id?: string
  parent_id?: string
  anchor_text?: string
  anchor_start?: number
  anchor_end?: number
  anchor_line_start?: number
  anchor_line_end?: number
  body: string
}) {
  return post(docPath(kbId, knowledgeId, '/anchor-comments'), data)
}

export function updateAnchorComment(kbId: string, commentId: string, data: {
  body?: string
  resolved?: boolean
}) {
  return patch(kbPath(kbId, `/anchor-comments/${encodeURIComponent(commentId)}`), data)
}

export function deleteAnchorComment(kbId: string, commentId: string) {
  return del(kbPath(kbId, `/anchor-comments/${encodeURIComponent(commentId)}`))
}

export function listDocVersions(kbId: string, knowledgeId: string) {
  return get(docPath(kbId, knowledgeId, '/versions'))
}

export function getDocVersion(kbId: string, knowledgeId: string, versionId: string) {
  return get(docPath(kbId, knowledgeId, `/versions/${encodeURIComponent(versionId)}`))
}

export function compareDocVersions(kbId: string, knowledgeId: string, from: string, to: string) {
  return get(docPath(kbId, knowledgeId, `/versions/compare${queryString({ from, to })}`))
}

export function rollbackDocVersion(kbId: string, knowledgeId: string, versionId: string, comment = '') {
  return post(docPath(kbId, knowledgeId, `/versions/${encodeURIComponent(versionId)}/rollback`), { comment })
}

export function triggerKBSync(kbId: string, knowledgeIds: string[] = []) {
  return post(kbPath(kbId, '/engine-sync'), knowledgeIds.length ? { knowledge_ids: knowledgeIds } : {})
}

export interface EngineSyncConfig {
  enabled: boolean
}

export function getTenantEngineSyncConfig() {
  return get('/api/v1/settings/kv/engine-sync-config')
}

export function updateTenantEngineSyncConfig(config: EngineSyncConfig) {
  return put('/api/v1/settings/kv/engine-sync-config', config)
}
