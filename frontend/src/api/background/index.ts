import { get, put, del, postUpload, getDown } from '@/utils/request'

export interface UploadedBackground {
  id: string
  display_name: string
  mime_type: string
  size_bytes: number
  created_at: string
}

export interface BackgroundList {
  choice: string
  uploads: UploadedBackground[]
  max_upload_bytes: number
  max_upload_count: number
}

export async function getMyBackgrounds(): Promise<{ success: boolean; data: BackgroundList; message?: string }> {
  return get('/api/v1/auth/me/backgrounds') as Promise<any>
}

export async function setMyBackgroundChoice(
  choice: string,
): Promise<{ success: boolean; data?: { choice: string }; message?: string }> {
  return put('/api/v1/auth/me/background', { choice }) as Promise<any>
}

export async function uploadMyBackground(file: File): Promise<{
  success: boolean
  data?: { background: UploadedBackground; choice: string }
  message?: string
}> {
  const form = new FormData()
  form.append('file', file, file.name)
  return postUpload('/api/v1/auth/me/backgrounds', form) as Promise<any>
}

export async function deleteMyBackground(id: string): Promise<{ success: boolean; message?: string }> {
  return del(`/api/v1/auth/me/backgrounds/${encodeURIComponent(id)}`) as Promise<any>
}

export async function fetchMyBackgroundFile(id: string): Promise<Blob> {
  return getDown(`/api/v1/auth/me/backgrounds/${encodeURIComponent(id)}/file`)
}
