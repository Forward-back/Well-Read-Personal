import type { AgentWorkspaceStreamEvent } from '@/api/agent-workspace'

export interface WorkspaceThinkingEvent {
  type: 'thinking'
  event_id?: string
  content: string
  done: boolean
  duration_ms?: number
}

export interface WorkspaceAnswerEvent {
  type: 'answer'
  event_id?: string
  content: string
  done: boolean
}

export interface WorkspaceToolEvent {
  type: 'tool_call'
  tool_call_id: string
  tool_name: string
  arguments?: unknown
  pending: boolean
  success?: boolean
  output?: string
  error?: string
  duration_ms?: number
  tool_data?: Record<string, unknown>
  timestamp?: number
}

export type WorkspaceStreamItem = WorkspaceThinkingEvent | WorkspaceAnswerEvent | WorkspaceToolEvent

export interface WorkspaceTurnState {
  events: WorkspaceStreamItem[]
  content: string
  error?: string
  finished: boolean
  thinking?: WorkspaceThinkingEvent
  pendingTools: Map<string, WorkspaceToolEvent>
}

export interface WorkspaceApplyResult {
  documentChange?: { toolName: string; data: Record<string, unknown> }
}

export function createWorkspaceTurn(): WorkspaceTurnState {
  return { events: [], content: '', finished: false, pendingTools: new Map() }
}

function payloadOf(event: AgentWorkspaceStreamEvent): Record<string, unknown> {
  return (event.data as Record<string, unknown> | undefined) || {}
}

/** Applies one SSE frame to the turn state (mutating), returns side effects. */
export function applyWorkspaceStreamEvent(
  state: WorkspaceTurnState,
  event: AgentWorkspaceStreamEvent,
): WorkspaceApplyResult {
  const payload = payloadOf(event)
  const type = String(event.type || '')
  const asString = (value: unknown): string => (typeof value === 'string' ? value : '')
  switch (type) {
    case 'thinking': {
      if (!state.thinking) {
        state.thinking = { type: 'thinking', content: '', done: false }
        state.events.push(state.thinking)
      }
      const chunk = asString(payload.content ?? event.content)
      if (chunk) state.thinking.content += chunk
      if (payload.done === true || event.done === true) {
        state.thinking.done = true
        const duration = Number(payload.duration_ms)
        if (Number.isFinite(duration) && duration > 0) state.thinking.duration_ms = duration
        state.thinking = undefined
      }
      return {}
    }
    case 'answer': {
      const chunk = asString(payload.content ?? event.content)
      if (chunk) state.content += chunk
      const existing = state.events.find((item) => item.type === 'answer') as WorkspaceAnswerEvent | undefined
      if (existing) {
        existing.content = state.content
        existing.done = payload.done === true || event.done === true
      } else {
        state.events.push({
          type: 'answer',
          content: state.content,
          done: payload.done === true || event.done === true,
        })
      }
      return {}
    }
    case 'tool_call': {
      const toolCallID = asString(payload.tool_call_id) || `tool_${Date.now()}`
      const toolName = asString(payload.tool_name)
      let toolEvent = state.pendingTools.get(toolCallID)
      if (!toolEvent) {
        toolEvent = {
          type: 'tool_call',
          tool_call_id: toolCallID,
          tool_name: toolName,
          arguments: payload.arguments,
          pending: true,
          timestamp: Date.now(),
        }
        state.events.push(toolEvent)
        state.pendingTools.set(toolCallID, toolEvent)
      } else {
        if (toolName) toolEvent.tool_name = toolName
        if (payload.arguments !== undefined) toolEvent.arguments = payload.arguments
      }
      return {}
    }
    case 'tool_result': {
      const toolCallID = asString(payload.tool_call_id)
      const toolName = asString(payload.tool_name)
      let toolEvent = state.pendingTools.get(toolCallID)
      if (!toolEvent) {
        toolEvent = {
          type: 'tool_call',
          tool_call_id: toolCallID || `tool_${Date.now()}`,
          tool_name: toolName,
          pending: false,
          timestamp: Date.now(),
        }
        state.events.push(toolEvent)
      }
      toolEvent.pending = false
      toolEvent.success = payload.success !== false
      toolEvent.output = asString(payload.output)
      toolEvent.error = asString(payload.error) || undefined
      const duration = Number(payload.duration_ms)
      if (Number.isFinite(duration) && duration > 0) toolEvent.duration_ms = duration
      const data = payload.data as Record<string, unknown> | undefined
      if (data) toolEvent.tool_data = data
      state.pendingTools.delete(toolEvent.tool_call_id)
      if (
        toolEvent.success &&
        data &&
        (toolEvent.tool_name === 'edit_document_section' || toolEvent.tool_name === 'create_document')
      ) {
        return { documentChange: { toolName: toolEvent.tool_name, data } }
      }
      return {}
    }
    case 'complete': {
      state.finished = true
      const finalContent = asString(payload.final_content)
      if (finalContent && !state.content) {
        state.content = finalContent
        const existing = state.events.find((item) => item.type === 'answer') as WorkspaceAnswerEvent | undefined
        if (existing) {
          existing.content = finalContent
          existing.done = true
        } else {
          state.events.push({ type: 'answer', content: finalContent, done: true })
        }
      }
      return {}
    }
    case 'error': {
      const message = asString(payload.error) || asString(event.content) || '智能体运行失败'
      state.error = message
      state.finished = true
      return {}
    }
    default:
      return {}
  }
}

/** Replays persisted agent_steps into the same event shape for history. */
export function eventsFromAgentSteps(steps: unknown[] | undefined): WorkspaceStreamItem[] {
  const events: WorkspaceStreamItem[] = []
  if (!Array.isArray(steps)) return events
  for (const raw of steps) {
    const step = (raw || {}) as Record<string, unknown>
    const reasoning = typeof step.reasoning_content === 'string' ? step.reasoning_content.trim() : ''
    if (reasoning) {
      events.push({
        type: 'thinking',
        event_id: `step-${step.iteration}-thought`,
        content: reasoning,
        done: true,
      })
    }
    const toolCalls = Array.isArray(step.tool_calls) ? (step.tool_calls as Array<Record<string, unknown>>) : []
    for (const toolCall of toolCalls) {
      const result = (toolCall.result || {}) as Record<string, unknown>
      const data = result.data as Record<string, unknown> | undefined
      const target = (toolCall.target || {}) as Record<string, unknown>
      events.push({
        type: 'tool_call',
        tool_call_id: String(toolCall.id || `tool-${events.length}`),
        tool_name: String(target.name || toolCall.name || ''),
        arguments: target.args ?? toolCall.args,
        pending: false,
        success: result.success !== false,
        output: typeof result.output === 'string' ? result.output : '',
        error: typeof result.error === 'string' ? result.error : undefined,
        duration_ms: Number(toolCall.duration) || undefined,
        tool_data: data,
      })
    }
  }
  return events
}

/** Successful L1 edits (and their undo pointers) contained in a history event list. */
export function editResultsFromEvents(events: WorkspaceStreamItem[]) {
  return events
    .filter((event): event is WorkspaceToolEvent => event.type === 'tool_call' && event.success === true)
    .filter((event) => event.tool_name === 'edit_document_section' || event.tool_name === 'create_document')
    .map((event) => ({
      toolName: event.tool_name,
      documentId: String(event.tool_data?.document_id || ''),
      title: String(event.tool_data?.title || ''),
      versionLabel: String(event.tool_data?.version_label || ''),
      previousVersionId: String(event.tool_data?.previous_version_id || ''),
      engineSyncStatus: String(event.tool_data?.engine_sync_status || ''),
    }))
}
