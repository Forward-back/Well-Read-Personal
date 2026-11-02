import { del, get, post } from '@/utils/request'

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
