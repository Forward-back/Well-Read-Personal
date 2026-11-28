import { fetchEventSource } from '@microsoft/fetch-event-source'
import { del, get, post, put } from '@/utils/request'

export interface AgentWorkspaceSession {
  id: string
  title: string
  surface: string
  model_id: string
  knowledge_base_ids: string[]
  knowledge_ids?: string[]
  created_at: string
  updated_at: string
}

export interface AgentWorkspaceMessage {
  id: string
  session_id: string
  role: 'user' | 'assistant'
  content: string
  agent_steps?: unknown[]
  is_completed?: boolean
  model_id?: string
  created_at: string
}

export interface AgentWorkspaceStreamEvent {
  type: string
  content?: string
  done?: boolean
  data?: Record<string, unknown>
  [key: string]: unknown
}

export interface AgentWorkspaceStreamHandlers {
  onEvent: (event: AgentWorkspaceStreamEvent) => void
  onError: (error: Error) => void
  onClose?: () => void
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('gaze_token') || ''
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export function createAgentWorkspaceSession(payload: {
  title?: string
  model_id?: string
  knowledge_base_ids: string[]
  knowledge_ids?: string[]
}) {
  return post<{ data: AgentWorkspaceSession }>('/api/v1/agent-workspace/sessions', payload)
}

export function listAgentWorkspaceSessions(params: { page?: number; page_size?: number; keyword?: string } = {}) {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.page_size) query.set('page_size', String(params.page_size))
  if (params.keyword) query.set('keyword', params.keyword)
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return get<{ data: AgentWorkspaceSession[]; total: number }>(`/api/v1/agent-workspace/sessions${suffix}`)
}

export function getAgentWorkspaceSession(id: string) {
  return get<{ data: AgentWorkspaceSession }>(`/api/v1/agent-workspace/sessions/${encodeURIComponent(id)}`)
}

export function updateAgentWorkspaceSession(
  id: string,
  payload: { title?: string; model_id?: string; knowledge_base_ids?: string[]; knowledge_ids?: string[] },
) {
  return put<{ data: AgentWorkspaceSession }>(`/api/v1/agent-workspace/sessions/${encodeURIComponent(id)}`, payload)
}

export function deleteAgentWorkspaceSession(id: string) {
  return del<{ success: boolean }>(`/api/v1/agent-workspace/sessions/${encodeURIComponent(id)}`)
}

export function loadAgentWorkspaceMessages(id: string, params: { page?: number; page_size?: number } = {}) {
  const query = new URLSearchParams()
  query.set('page', String(params.page || 1))
  query.set('page_size', String(params.page_size || 100))
  return get<{ data: { messages: AgentWorkspaceMessage[] } }>(
    `/api/v1/agent-workspace/sessions/${encodeURIComponent(id)}/messages?${query.toString()}`,
  )
}

/** Stream one workspace turn; returns an abort handle. */
export function streamAgentWorkspaceMessage(
  id: string,
  query: string,
  handlers: AgentWorkspaceStreamHandlers,
) {
  const controller = new AbortController()
  let settled = false
  const fail = (error: Error) => {
    if (settled) return
    settled = true
    handlers.onError(error)
  }
  fetchEventSource(`/api/v1/agent-workspace/sessions/${encodeURIComponent(id)}/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...authHeaders(),
    },
    body: JSON.stringify({ query }),
    signal: controller.signal,
    openWhenHidden: true,
    onopen: async (response) => {
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
    },
    onmessage: (message) => {
      if (!message.data) return
      try {
        const event = JSON.parse(message.data) as AgentWorkspaceStreamEvent
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
      if (!settled) fail(new Error('连接已关闭'))
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

export interface AgentPendingView {
  id: string
  session_id?: string
  message_id?: string
  tool_name: string
  risk_level: 'L2' | 'L3' | string
  status: string
  expires_at: string
  dry_run?: { tool?: string; plan?: Record<string, unknown>; diff?: string }
  error?: string
  applied_version_ids?: string[]
}

export interface AgentPendingExecuteResult {
  pending_id: string
  status: string
  applied_version_ids: string[]
  message: string
}

export function getAgentPending(id: string) {
  return get<{ data: AgentPendingView }>(`/api/v1/agent-workspace/pending/${encodeURIComponent(id)}`)
}

export function confirmAgentPending(id: string, comment = '') {
  return post<{ data: AgentPendingExecuteResult }>(
    `/api/v1/agent-workspace/pending/${encodeURIComponent(id)}/confirm`,
    { comment },
  )
}

export function rejectAgentPending(id: string, comment = '') {
  return post<{ data: AgentPendingView }>(
    `/api/v1/agent-workspace/pending/${encodeURIComponent(id)}/reject`,
    { comment },
  )
}
