import { del, get, post, put } from '@/utils/request'

export interface ReadingChunkSegment {
  id: string
  heading?: string
  content: string
}

export interface ReadingSection {
  id: string
  knowledge_id: string
  title: string
  heading?: string
  index: number
  content: string
  chunk_ids?: string[]
  segments?: ReadingChunkSegment[]
  read: boolean
}

export interface ReadingQuestion {
  text: string
  recommended_answer: string
  why?: string
}

export interface ReadingEvidence {
  knowledge_id: string
  document_title?: string
  chunk_id: string
  section_id?: string
  heading?: string
  quote: string
}

export interface ReadingQuestionNode {
  id: string
  run_id: string
  parent_id?: string
  section_id: string
  round: number
  text: string
  recommended_answer: string
  why?: string
  search_query?: string
  kind?: 'fact' | 'decision' | ''
  gap_type?: string
  evidence?: ReadingEvidence[]
  answer?: string
  answer_source?: string
  status: 'pending' | 'auto_answered' | 'answered' | 'contradicted' | 'resolved' | 'skipped'
}

export interface ReadingConclusionGates {
  references_ok: boolean
  conflict_free: boolean
  cross_doc_consistent: boolean
  user_adjudicated?: boolean
}

export interface ReadingConclusion {
  id: string
  section_id: string
  round: number
  text: string
  evidence?: ReadingEvidence[]
  status: 'candidate' | 'confirmed' | 'contradicted'
  source: 'model' | 'auto' | 'user' | 'legacy'
  gates?: ReadingConclusionGates
}

export interface ReadingTerm {
  id: string
  term: string
  definition: string
  source?: ReadingEvidence[]
  canonical: boolean
  status: 'candidate' | 'confirmed'
}

export interface ReadingContradiction {
  id: string
  description: string
  evidence_a?: ReadingEvidence[]
  evidence_b?: ReadingEvidence[]
  status: 'open' | 'resolved' | 'closed'
  resolution?: string
}

export interface ReadingReport {
  content: string
  edited: boolean
  version: number
  updated_at?: string
}

export interface ReadingDocumentSection {
  id: string
  title?: string
  heading?: string
  read: boolean
  start: number
  end: number
}

export interface ReadingDocument {
  id: string
  title: string
  file_name?: string
  content: string
  sections?: ReadingDocumentSection[]
}

export interface ReadingRunDocuments {
  documents: ReadingDocument[]
}

export interface ReadingNotes {
  confirmed?: string[]
  terms?: string[]
  open_questions?: string[]
  contradictions?: string[]
  decisions?: string[]
  citations?: string[]
  understanding?: string
  duration_ms?: number
  reasoning?: string
  search_trace?: Array<{ query: string; hit_count: number; chunk_ids?: string[] }>
  [key: string]: unknown
}

export interface ReadingRunStep {
  id: string
  seq: number
  action: string
  section_id: string
  question?: ReadingQuestion
  user_feedback?: string
  notes_patch?: ReadingNotes
  citations?: string[]
  status: string
}

export type ReadingDepth = 'quick' | 'standard' | 'deep'

export interface ReadingPlan {
  goal?: string
  depth?: ReadingDepth
  policy?: string
  knowledge_base_id?: string
  state_version?: number
  sections?: ReadingSection[]
}

export interface ReadingRun {
  id: string
  session_id: string
  status: string
  document_ids: string[]
  reading_model_id: string
  plan: ReadingPlan
  notes: ReadingNotes
  final: Record<string, any>
  current_section_id: string
  step_count: number
  steps?: ReadingRunStep[]
  questions?: ReadingQuestionNode[]
  conclusions?: ReadingConclusion[]
  terms?: ReadingTerm[]
  contradictions?: ReadingContradiction[]
}

export function createReadingRun(payload: {
  session_id: string
  document_ids: string[]
  skill_id?: string
  goal?: string
  depth?: ReadingDepth
}) {
  return post<{ data: ReadingRun }>('/api/v1/reading/runs', payload, { timeout: 60_000 })
}

export function getReadingRun(id: string) {
  return get<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}`)
}

export function stepReadingRun(id: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/step`, {}, {
    timeout: 360_000,
  })
}

export function feedbackReadingRun(
  id: string,
  payload: { feedback?: string; question_id?: string; skip?: boolean },
) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/feedback`, payload, {
    timeout: 60_000,
  })
}

export function stopReadingRun(id: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/stop`, {})
}

export function resolveReadingContradiction(
  id: string,
  contradictionId: string,
  payload: { choice: 'a' | 'b' | 'custom'; resolution?: string },
) {
  return post<{ data: ReadingRun }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/contradictions/${encodeURIComponent(contradictionId)}/resolve`,
    payload,
  )
}

export function getReadingReport(id: string) {
  return get<{ data: ReadingReport }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/report`)
}

export function getReadingRunDocuments(id: string) {
  return get<{ data: ReadingRunDocuments }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/documents`,
  )
}

export function saveReadingReport(id: string, content: string, baseVersion: number) {
  return put<{ data: ReadingReport }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/report`, {
    content,
    base_version: baseVersion,
  })
}

export function deleteReadingReport(id: string, baseVersion: number) {
  return del<{ data: ReadingReport }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/report?base_version=${baseVersion}`,
  )
}

export function resumeReadingRun(id: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/resume`, {})
}

export function addReadingDocuments(id: string, documentIds: string[]) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/add-documents`, {
    document_ids: documentIds,
  })
}

export function getReadingConfig() {
  return get<{ data: { reading_model_id: string } }>('/api/v1/settings/kv/reading-config')
}

export function updateReadingConfig(readingModelId: string) {
  return put<{ data: { reading_model_id: string } }>('/api/v1/settings/kv/reading-config', {
    reading_model_id: readingModelId,
  })
}

export interface ReadingExport {
  format: 'markdown' | 'json'
  filename: string
  content: string
  coverage: {
    documents: number
    total_sections: number
    read_sections: number
    step_count: number
    token_usage: number
  }
  generated_at: string
}

export function exportReadingRun(id: string, format: 'markdown' | 'json') {
  return get<{ data: ReadingExport }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/export?format=${format}`)
}
