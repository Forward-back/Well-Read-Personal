import assert from 'node:assert/strict'
import test from 'node:test'

import {
  applyWorkspaceStreamEvent,
  createWorkspaceTurn,
  editResultsFromEvents,
  eventsFromAgentSteps,
} from './agentWorkspaceStream'

test('merges tool call and result into one pending-free event', () => {
  const state = createWorkspaceTurn()
  applyWorkspaceStreamEvent(state, {
    type: 'tool_call',
    data: { tool_call_id: 't1', tool_name: 'list_documents', arguments: { limit: 5 } },
  })
  assert.equal(state.events.length, 1)
  assert.equal((state.events[0] as any).pending, true)

  const result = applyWorkspaceStreamEvent(state, {
    type: 'tool_result',
    data: { tool_call_id: 't1', tool_name: 'list_documents', success: true, output: 'ok', duration_ms: 12 },
  })
  assert.equal(result.documentChange, undefined)
  const event = state.events[0] as any
  assert.equal(event.pending, false)
  assert.equal(event.success, true)
  assert.equal(event.output, 'ok')
  assert.equal(event.duration_ms, 12)
})

test('flags document changes for successful edits and creates', () => {
  const state = createWorkspaceTurn()
  applyWorkspaceStreamEvent(state, {
    type: 'tool_call',
    data: { tool_call_id: 'e1', tool_name: 'edit_document_section' },
  })
  const edit = applyWorkspaceStreamEvent(state, {
    type: 'tool_result',
    data: {
      tool_call_id: 'e1',
      tool_name: 'edit_document_section',
      success: true,
      data: { document_id: 'doc-1', version_id: 'v2', previous_version_id: 'v1' },
    },
  })
  assert.equal(edit.documentChange?.toolName, 'edit_document_section')
  assert.equal(edit.documentChange?.data.version_id, 'v2')

  applyWorkspaceStreamEvent(state, {
    type: 'tool_call',
    data: { tool_call_id: 'c1', tool_name: 'create_document' },
  })
  const created = applyWorkspaceStreamEvent(state, {
    type: 'tool_result',
    data: { tool_call_id: 'c1', tool_name: 'create_document', success: true, data: { document_id: 'doc-2', version_id: 'v1' } },
  })
  assert.equal(created.documentChange?.toolName, 'create_document')

  applyWorkspaceStreamEvent(state, {
    type: 'tool_call',
    data: { tool_call_id: 'e2', tool_name: 'edit_document_section' },
  })
  const failed = applyWorkspaceStreamEvent(state, {
    type: 'tool_result',
    data: { tool_call_id: 'e2', tool_name: 'edit_document_section', success: false, error: '冲突' },
  })
  assert.equal(failed.documentChange, undefined)
})

test('accumulates thinking and answer frames and finishes on complete', () => {
  const state = createWorkspaceTurn()
  applyWorkspaceStreamEvent(state, { type: 'thinking', data: { content: '先读', done: false } })
  applyWorkspaceStreamEvent(state, { type: 'thinking', data: { content: '文档', done: true, duration_ms: 30 } })
  applyWorkspaceStreamEvent(state, { type: 'answer', data: { content: '你好', done: false } })
  applyWorkspaceStreamEvent(state, { type: 'answer', data: { content: '，世界', done: true } })
  applyWorkspaceStreamEvent(state, { type: 'complete', data: {} })
  assert.equal(state.content, '你好，世界')
  assert.equal(state.finished, true)
  const thinking = state.events.find((event) => event.type === 'thinking') as any
  assert.equal(thinking.content, '先读文档')
  assert.equal(thinking.done, true)
})

test('captures stream errors', () => {
  const state = createWorkspaceTurn()
  applyWorkspaceStreamEvent(state, { type: 'error', data: { error: '模型不可用' } })
  assert.equal(state.error, '模型不可用')
  assert.equal(state.finished, true)
})

test('replays persisted agent steps and collects edit results', () => {
  const events = eventsFromAgentSteps([
    {
      iteration: 1,
      reasoning_content: '先看结构',
      tool_calls: [
        {
          id: 't1',
          name: 'edit_document_section',
          args: { heading: '背景' },
          duration: 8,
          result: {
            success: true,
            output: '已提交',
            data: {
              document_id: 'doc-1',
              title: '文档',
              version_label: 'V1.2',
              previous_version_id: 'v1',
              engine_sync_status: 'pending',
            },
          },
        },
        { id: 't2', name: 'list_documents', args: {}, result: { success: false, error: '失败' } },
      ],
    },
  ])
  assert.equal(events.filter((event) => event.type === 'thinking').length, 1)
  assert.equal(events.filter((event) => event.type === 'tool_call').length, 2)
  const edits = editResultsFromEvents(events)
  assert.equal(edits.length, 1)
  assert.equal(edits[0].previousVersionId, 'v1')
  assert.equal(edits[0].engineSyncStatus, 'pending')
})
