import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import i18n from '@/i18n'
import grassBackground from '@/assets/img/app-bg/grass.jpg'
import {
  DEFAULT_BACKGROUND_CHOICE,
  builtinBackgroundUrl,
} from '@/constants/backgrounds'
import {
  deleteMyBackground,
  fetchMyBackgroundFile,
  getMyBackgrounds,
  setMyBackgroundChoice,
  uploadMyBackground,
  type UploadedBackground,
} from '@/api/background'

const CHOICE_CACHE_KEY = 'wellread_background_choice'
const UPLOAD_PREFIX = 'upload:'

export const useBackgroundStore = defineStore('background', () => {
  const choice = ref(localStorage.getItem(CHOICE_CACHE_KEY) || DEFAULT_BACKGROUND_CHOICE)
  const uploads = ref<UploadedBackground[]>([])
  const uploadUrls = ref<Record<string, string>>({})
  const maxUploadBytes = ref(10 * 1024 * 1024)
  const maxUploadCount = ref(10)
  const loading = ref(false)
  const loaded = ref(false)

  const imageUrl = computed(() => {
    if (choice.value.startsWith(UPLOAD_PREFIX)) {
      return uploadUrls.value[choice.value.slice(UPLOAD_PREFIX.length)] || grassBackground
    }
    return builtinBackgroundUrl(choice.value) || grassBackground
  })

  const rememberChoice = (value: string) => {
    choice.value = value
    localStorage.setItem(CHOICE_CACHE_KEY, value)
  }

  const ensureUploadUrl = async (id: string): Promise<string> => {
    const cached = uploadUrls.value[id]
    if (cached) return cached
    const blob = await fetchMyBackgroundFile(id)
    const url = URL.createObjectURL(blob)
    uploadUrls.value = { ...uploadUrls.value, [id]: url }
    return url
  }

  const load = async (force = false) => {
    if (loading.value || (loaded.value && !force)) return
    loading.value = true
    try {
      const response = await getMyBackgrounds()
      if (!response?.success || !response.data) return
      uploads.value = response.data.uploads || []
      maxUploadBytes.value = response.data.max_upload_bytes || maxUploadBytes.value
      maxUploadCount.value = response.data.max_upload_count || maxUploadCount.value
      rememberChoice(response.data.choice || DEFAULT_BACKGROUND_CHOICE)
      loaded.value = true
    } catch {
      // Backgrounds are cosmetic: a failed load falls back to the grass default.
    } finally {
      loading.value = false
    }
  }

  const hydrateUploadPreviews = async () => {
    await Promise.allSettled(uploads.value.map((item) => ensureUploadUrl(item.id)))
  }

  const select = async (value: string) => {
    const previous = choice.value
    rememberChoice(value)
    try {
      const response = await setMyBackgroundChoice(value)
      if (!response?.success) throw new Error(response?.message || i18n.global.t('backgroundSettings.saveFailed'))
      if (value.startsWith(UPLOAD_PREFIX)) {
        await ensureUploadUrl(value.slice(UPLOAD_PREFIX.length))
      }
    } catch (error) {
      rememberChoice(previous)
      throw error
    }
  }

  const upload = async (file: File) => {
    const t = i18n.global.t
    if (file.size > maxUploadBytes.value) {
      throw new Error(t('backgroundSettings.imageTooLarge'))
    }
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      throw new Error(t('backgroundSettings.unsupportedType'))
    }
    const response = await uploadMyBackground(file)
    if (!response?.success || !response.data) {
      throw new Error(response?.message || t('backgroundSettings.uploadFailed'))
    }
    uploads.value = [response.data.background, ...uploads.value]
    rememberChoice(response.data.choice)
    await ensureUploadUrl(response.data.background.id)
    return response.data.background
  }

  const remove = async (id: string) => {
    const response = await deleteMyBackground(id)
    if (!response?.success) throw new Error(response?.message || i18n.global.t('backgroundSettings.deleteFailed'))
    uploads.value = uploads.value.filter((item) => item.id !== id)
    const url = uploadUrls.value[id]
    if (url) {
      URL.revokeObjectURL(url)
      const next = { ...uploadUrls.value }
      delete next[id]
      uploadUrls.value = next
    }
    if (choice.value === UPLOAD_PREFIX + id) {
      rememberChoice(DEFAULT_BACKGROUND_CHOICE)
    }
  }

  return {
    choice,
    uploads,
    uploadUrls,
    maxUploadBytes,
    maxUploadCount,
    loading,
    loaded,
    imageUrl,
    load,
    hydrateUploadPreviews,
    ensureUploadUrl,
    select,
    upload,
    remove,
  }
})
