import { del, get, post, put } from '@/utils/request'
import { fetchEventSource } from '@microsoft/fetch-event-source'

export interface ReadingChunkSegment {
  id: string
  heading?: string
  content: string
}

export interface ReadingSection {
  id: string
  knowledge_id: string
  knowledge_base_id?: string
  title: string
  heading?: string
  index: number
  content: string
  chunk_ids?: string[]
  segments?: ReadingChunkSegment[]
  read: boolean
}

export interface ReadingQuestionOption {
  id: string
  label: string
  text: string
  recommended?: boolean
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
  options?: ReadingQuestionOption[]
  evidence?: ReadingEvidence[]
  answer?: string
  answer_source?: string
  judgment?: 'correct' | 'partial' | 'wrong' | ''
  judgment_reason?: string
  judged_at?: string
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
  status: 'candidate' | 'confirmed' | 'contradicted' | 'rejected'
  source: 'model' | 'auto' | 'user' | 'legacy'
  gates?: ReadingConclusionGates
}

export interface ReadingTerm {
  id: string
  term: string
  definition: string
  source?: ReadingEvidence[]
  canonical: boolean
  status: 'candidate' | 'confirmed' | 'rejected'
}

export interface ReadingContradiction {
  id: string
  description: string
  evidence_a?: ReadingEvidence[]
  evidence_b?: ReadingEvidence[]
  status: 'open' | 'resolved' | 'closed'
  resolution?: string
}

export interface ReadingUnderstanding {
  id: string
  run_id: string
  section_id: string
  round: number
  text: string
  decision: 'pending' | 'confirmed' | 'corrected' | 'skipped'
  correction?: string
}

export interface ReadingDraft {
  id: string
  name: string
  document_ids: string[]
  depth: ReadingDepth
  goal: string
  created_at?: string
  updated_at: string
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

export type ReadingKnowledgeType =
  | 'concept'
  | 'argument'
  | 'process'
  | 'comparison'
  | 'example'
  | 'reference'
  | 'unknown'

export interface ReadingSectionStructure {
  section_id: string
  thesis: string
  knowledge_type: ReadingKnowledgeType
}

export interface ReadingPlan {
  goal?: string
  depth?: ReadingDepth
  policy?: string
  knowledge_base_id?: string
  knowledge_base_ids?: string[]
  state_version?: number
  structure?: ReadingSectionStructure[]
  structure_degraded?: boolean
  structure_error?: string
  sections?: ReadingSection[]
}

export interface ReadingSelfTest {
  id: string
  run_id: string
  section_id: string
  question: string
  answer: string
  explanation: string
  evidence?: ReadingEvidence[]
  status: 'candidate' | 'imported' | 'rejected'
  created_at?: string
  updated_at?: string
}

export interface ReadingSelfTestImportResult {
  imported: number
  skipped: number
}

export interface ReadingRun {
  id: string
  session_id: string
  status: string
  phase?: string
  quiet?: boolean
  document_ids: string[]
  reading_model_id: string
  extraction_model_id?: string
  synthesis_model_id?: string
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
  understandings?: ReadingUnderstanding[]
  annotations?: ReadingAnnotation[]
  feynman_attempts?: ReadingFeynmanAttempt[]
  argument_chain_mermaid?: string
  argument_chain_evidence?: Array<{ id: string; evidence: ReadingEvidence }>
}

export interface ReadingFeynmanPoint {
  text: string
  evidence: ReadingEvidence[]
}

export interface ReadingFeynmanAnswer {
  angle_id: string
  label: string
  prompt: string
  text: string
}

export interface ReadingFeynmanAngleFeedback {
  angle_id: string
  status: 'strong' | 'partial' | 'missing' | 'skipped'
  summary: string
  omissions: ReadingFeynmanPoint[]
  errors: ReadingFeynmanPoint[]
}

export interface ReadingFeynmanAttempt {
  id: string
  run_id: string
  section_id: string
  text: string
  score: number
  coverage: number
  accuracy: number
  evidence_score: number
  summary: string
  omissions: ReadingFeynmanPoint[]
  errors: ReadingFeynmanPoint[]
  evidence: ReadingEvidence[]
  answers?: ReadingFeynmanAnswer[]
  angle_feedback?: ReadingFeynmanAngleFeedback[]
  prompt_version?: number
  created_at: string
}

export interface ReadingAnnotation {
  id: string
  run_id: string
  section_id: string
  knowledge_id: string
  chunk_id: string
  quote: string
  comment: string
  status: string
  created_at: string
  updated_at: string
}

export interface ReadingHistoryDocument {
  id: string
  title: string
  file_name?: string
  knowledge_base_id?: string
}

export interface ReadingHistoryItem {
  id: string
  session_id: string
  title: string
  status: string
  phase?: string
  goal?: string
  depth?: ReadingDepth
  documents: ReadingHistoryDocument[]
  read_sections: number
  total_sections: number
  created_at: string
  updated_at: string
}

export function listReadingHistory(limit = 50) {
  return get<{ data: ReadingHistoryItem[] }>(`/api/v1/reading/runs/history?limit=${limit}`)
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

export interface ReadingProgressEvent {
  type: 'phase' | 'reasoning' | 'search' | 'heartbeat' | 'complete' | 'error'
  phase?: string
  status?: string
  section_id?: string
  section_title?: string
  section_heading?: string
  round?: number
  query?: string
  hit_count?: number
  content?: string
  elapsed_ms?: number
  run?: ReadingRun
}

export interface ReadingStepStreamHandlers {
  onEvent: (event: ReadingProgressEvent) => void
  onError: (error: Error) => void
  onClose?: () => void
}

/** Stream one reading step. Returns an abort handle for route changes/unmounts. */
export function streamReadingStep(id: string, handlers: ReadingStepStreamHandlers) {
  const controller = new AbortController()
  let settled = false
  const fail = (error: Error) => {
    if (settled) return
    settled = true
    handlers.onError(error)
  }
  const token = localStorage.getItem('gaze_token') || ''
  fetchEventSource(`/api/v1/reading/runs/${encodeURIComponent(id)}/step/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: '{}',
    signal: controller.signal,
    openWhenHidden: true,
    onopen: async (response) => {
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
    },
    onmessage: (message) => {
      if (!message.data) return
      try {
        const event = JSON.parse(message.data) as ReadingProgressEvent
        if (event.type === 'error') {
          settled = true
          handlers.onError(new Error(event.content || '精读步骤失败'))
          return
        }
        if (event.type === 'complete') settled = true
        handlers.onEvent(event)
      } catch (error) {
        fail(error instanceof Error ? error : new Error(String(error)))
      }
    },
    onerror: (error) => {
      fail(error instanceof Error ? error : new Error(String(error)))
      throw error
    },
    onclose: () => {
      if (!settled) fail(new Error('精读步骤连接已关闭'))
      handlers.onClose?.()
    },
  }).catch((error) => {
    fail(error instanceof Error ? error : new Error(String(error)))
  })
  return {
    close: () => {
      settled = true
      controller.abort()
    },
  }
}

export function feedbackReadingRun(
  id: string,
  payload: { feedback?: string; question_id?: string; option_id?: string; skip?: boolean },
) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/feedback`, payload, {
    timeout: 120_000,
  })
}

export function judgeReadingQuestion(id: string, questionId: string) {
  return post<{ data: ReadingRun }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/questions/${encodeURIComponent(questionId)}/judge`,
    {},
    { timeout: 60_000 },
  )
}

export function stopReadingRun(id: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/stop`, {})
}

export function abandonReadingRun(id: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/abandon`, {})
}

export function jumpReadingSection(id: string, sectionId: string) {
  return post<{ data: ReadingRun }>(`/api/v1/reading/runs/${encodeURIComponent(id)}/jump`, {
    section_id: sectionId,
  })
}

export function updateReadingUnderstanding(
  id: string,
  payload: { decision: 'confirmed' | 'corrected' | 'skipped'; correction?: string },
) {
  return put<{ data: ReadingRun }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/understanding`,
    payload,
  )
}

export function updateReadingTerm(
  id: string,
  termId: string,
  payload: { status?: 'candidate' | 'confirmed' | 'rejected'; text?: string },
) {
  return put<{ data: ReadingTerm }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/terms/${encodeURIComponent(termId)}`,
    payload,
  )
}

export function updateReadingConclusion(
  id: string,
  conclusionId: string,
  payload: { status?: 'candidate' | 'confirmed' | 'rejected'; text?: string },
) {
  return put<{ data: ReadingConclusion }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/conclusions/${encodeURIComponent(conclusionId)}`,
    payload,
  )
}

export function listReadingDrafts() {
  return get<{ data: ReadingDraft[] }>('/api/v1/reading/drafts')
}

export function createReadingDraft(payload: {
  name?: string
  document_ids: string[]
  depth: ReadingDepth
  goal?: string
}) {
  return post<{ data: ReadingDraft }>('/api/v1/reading/drafts', payload)
}

export function updateReadingDraft(
  draftId: string,
  payload: { name?: string; document_ids?: string[]; depth?: ReadingDepth; goal?: string },
) {
  return put<{ data: ReadingDraft }>(
    `/api/v1/reading/drafts/${encodeURIComponent(draftId)}`,
    payload,
  )
}

export function deleteReadingDraft(draftId: string) {
  return del<{ success: boolean }>(`/api/v1/reading/drafts/${encodeURIComponent(draftId)}`)
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

export function listReadingSelfTests(id: string) {
  return get<{ data: ReadingSelfTest[] }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/self-tests`,
  )
}

export function importReadingSelfTests(id: string, groupId: string, selfTestIds: string[]) {
  return post<{ data: ReadingSelfTestImportResult }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/self-tests/import`,
    { group_id: groupId, self_test_ids: selfTestIds },
    { timeout: 120_000 },
  )
}

export interface ReadingConfig {
  reading_model_id: string
  reading_extraction_model_id: string
  reading_synthesis_model_id: string
}

export function getReadingConfig() {
  return get<{ data: ReadingConfig }>('/api/v1/settings/kv/reading-config')
}

export function updateReadingConfig(payload: Partial<ReadingConfig>) {
  return put<{ data: ReadingConfig }>('/api/v1/settings/kv/reading-config', payload)
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

export function submitReadingFeynman(
  id: string,
  payload: {
    section_id: string
    text?: string
    answers?: Array<{ angle_id: string; label: string; prompt: string; text: string }>
    freeform?: string
  },
) {
  return post<{ data: ReadingFeynmanAttempt }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/feynman`,
    payload,
    { timeout: 180_000 },
  )
}

export function listReadingFeynman(id: string, sectionId = '') {
  const query = sectionId ? `?section_id=${encodeURIComponent(sectionId)}` : ''
  return get<{ data: ReadingFeynmanAttempt[] }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/feynman${query}`,
  )
}

export function createReadingAnnotation(
  id: string,
  payload: { section_id: string; quote: string; comment: string },
) {
  return post<{ data: ReadingAnnotation }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/annotations`,
    payload,
  )
}

export function deleteReadingAnnotation(id: string, annotationId: string) {
  return del<{ success: boolean }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/annotations/${encodeURIComponent(annotationId)}`,
  )
}

export function createCardFromReadingAnnotation(
  id: string,
  annotationId: string,
  payload: { group_id: string; question?: string; answer?: string },
) {
  return post<{ data: { imported: number } }>(
    `/api/v1/reading/runs/${encodeURIComponent(id)}/annotations/${encodeURIComponent(annotationId)}/card`,
    payload,
  )
}
