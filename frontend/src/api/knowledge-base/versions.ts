import { get, post } from '@/utils/request'

export interface DocVersion {
  id: string
  knowledge_id: string
  version_seq: number
  version_label: string
  source_type: string
  source_id?: string
  title: string
  description: string
  chunk_count: number
  created_by?: string
  created_at: string
  is_current?: boolean
  session_id?: string
  message_id?: string
  tool_name?: string
}

export interface DocVersionChunk {
  id: string
  version_id: string
  chunk_id: string
  chunk_index: number
  content: string
  is_enabled: boolean
}

export interface DocumentContentView {
  knowledge_id: string
  knowledge_base_id: string
  title: string
  file_name: string
  markdown: string
  chunk_count: number
  char_count: number
  truncated: boolean
  engine_sync_status: string
  version?: DocVersion
}

export interface DocVersionCompare {
  from: DocVersion
  to: DocVersion
  from_chunks: DocVersionChunk[]
  to_chunks: DocVersionChunk[]
  from_markdown?: string
  to_markdown?: string
}

export function getKnowledgeContent(id: string, versionId?: string) {
  const suffix = versionId ? `?version_id=${encodeURIComponent(versionId)}` : ''
  return get<{ data: DocumentContentView }>(`/api/v1/knowledge/${encodeURIComponent(id)}/content${suffix}`)
}

export function listDocVersions(id: string) {
  return get<{ data: { versions: DocVersion[] } }>(`/api/v1/knowledge/${encodeURIComponent(id)}/versions`)
}

export function getDocVersion(id: string, versionId: string) {
  return get<{ data: { version: DocVersion; chunks: DocVersionChunk[] } }>(
    `/api/v1/knowledge/${encodeURIComponent(id)}/versions/${encodeURIComponent(versionId)}`,
  )
}

export function compareDocVersions(id: string, from: string, to: string) {
  return get<{ data: DocVersionCompare }>(
    `/api/v1/knowledge/${encodeURIComponent(id)}/versions/compare?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
  )
}

export function rollbackDocVersion(id: string, versionId: string, comment = '') {
  return post<{ data: { version: DocVersion; engine_sync_status: string } }>(
    `/api/v1/knowledge/${encodeURIComponent(id)}/versions/${encodeURIComponent(versionId)}/rollback`,
    { comment },
  )
}
