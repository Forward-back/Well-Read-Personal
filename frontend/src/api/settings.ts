import { del, get, post, put } from '@/utils/request'

export interface PersonalAPIKey {
  id: number
  name: string
  masked_key: string
  last_used_at?: string
  created_at: string
}

export interface PersonalAPIKeyState {
  exists: boolean
  key?: PersonalAPIKey
}

export interface PersonalAPIKeyCreated extends PersonalAPIKey {
  token: string
}

export async function getPersonalAPIKey() {
  return await get('/api/v1/settings/api-key') as unknown as {
    success: boolean
    data?: PersonalAPIKeyState
  }
}

export async function createPersonalAPIKey() {
  return await post('/api/v1/settings/api-key') as unknown as {
    success: boolean
    data?: PersonalAPIKeyCreated
  }
}

export async function resetPersonalAPIKey() {
  return await post('/api/v1/settings/api-key/reset') as unknown as {
    success: boolean
    data?: PersonalAPIKeyCreated
  }
}

export async function deletePersonalAPIKey() {
  return await del('/api/v1/settings/api-key') as unknown as { success: boolean }
}

export interface PromptTemplate {
  id: string
  name: string
  description: string
  content: string
  user?: string
  mode?: string
  default?: boolean
}

export interface PromptTemplateState {
  agent_system_prompt?: PromptTemplate[]
}

export async function getPromptTemplates() {
  return await get('/api/v1/settings/kv/prompt-templates') as unknown as {
    success: boolean
    data?: PromptTemplateState
  }
}

export async function saveAgentSystemPrompt(agentSystemPrompt: string) {
  return await put('/api/v1/settings/kv/prompt-templates', {
    agent_system_prompt: agentSystemPrompt,
  }) as unknown as {
    success: boolean
    data?: { agent_system_prompt: string; is_custom: boolean }
  }
}
