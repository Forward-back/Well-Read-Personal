import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { UserInfo, TenantInfo, KnowledgeBaseInfo } from '@/api/auth'
import { userInfoFromApi } from '@/api/auth'
import i18n from '@/i18n'
import { reloadFontFromStorage } from '@/composables/useFont'
import { reloadThemeFromStorage } from '@/composables/useTheme'
import { resetMigrationLatch } from '@/composables/preferenceStorage'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useEditorResourcesStore } from '@/stores/editorResources'

function clearSessionResourceCaches() {
  useChatResourcesStore().invalidate()
  useEditorResourcesStore().invalidate()
}

function reloadUserPreferences() {
  resetMigrationLatch()
  reloadFontFromStorage()
  reloadThemeFromStorage()
}

// Personal edition auth state: one owner, one internal workspace.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserInfo | null>(null)
  const tenant = ref<TenantInfo | null>(null)
  const token = ref<string>('')
  const refreshToken = ref<string>('')
  const knowledgeBases = ref<KnowledgeBaseInfo[]>([])
  const currentKnowledgeBase = ref<KnowledgeBaseInfo | null>(null)
  const isLiteMode = ref(false)

  const isLoggedIn = computed(() => !!token.value && !!user.value)
  const hasValidTenant = computed(() => !!tenant.value && !!tenant.value.id)
  const currentTenantId = computed(() => tenant.value?.id || '')
  const currentTenantName = computed(() => tenant.value?.name || '')
  const currentUserId = computed(() => user.value?.id || '')
  const isSystemAdmin = computed(() => user.value?.is_system_admin || false)
  const effectiveTenantId = computed(() => (tenant.value?.id ? Number(tenant.value.id) : null))

  const setUser = (userData: UserInfo) => {
    const previousId = user.value?.id
    user.value = userData
    localStorage.setItem('gaze_user', JSON.stringify(userData))
    if (previousId !== userData.id) {
      reloadUserPreferences()
    }
  }

  const setTenant = (tenantData: TenantInfo | null) => {
    tenant.value = tenantData
    if (tenantData) {
      localStorage.setItem('gaze_tenant', JSON.stringify(tenantData))
    } else {
      localStorage.removeItem('gaze_tenant')
    }
  }

  const setToken = (tokenValue: string) => {
    token.value = tokenValue
    localStorage.setItem('gaze_token', tokenValue)
  }

  const setRefreshToken = (refreshTokenValue: string) => {
    refreshToken.value = refreshTokenValue
    localStorage.setItem('gaze_refresh_token', refreshTokenValue)
  }

  const setKnowledgeBases = (kbList: KnowledgeBaseInfo[]) => {
    knowledgeBases.value = Array.isArray(kbList) ? kbList : []
    localStorage.setItem('gaze_knowledge_bases', JSON.stringify(knowledgeBases.value))
  }

  const setCurrentKnowledgeBase = (kb: KnowledgeBaseInfo | null) => {
    currentKnowledgeBase.value = kb
    if (kb) {
      localStorage.setItem('gaze_current_kb', JSON.stringify(kb))
    } else {
      localStorage.removeItem('gaze_current_kb')
    }
  }

  const refreshFromAuthMe = async (): Promise<boolean> => {
    try {
      const { getCurrentUser } = await import('@/api/auth')
      const response = await getCurrentUser()
      const u = response.data?.user
      if (!response.success || !u) return false
      setUser(userInfoFromApi(u, response.data?.tenant?.id))
      const snapshot = response.data?.tenant
      if (snapshot) {
        setTenant({
          id: String(snapshot.id) || '',
          name: snapshot.name || '',
          owner_id: snapshot.owner_id || u.id || '',
          created_at: snapshot.created_at || new Date().toISOString(),
          updated_at: snapshot.updated_at || new Date().toISOString(),
        })
      } else {
        setTenant(null)
      }
      return true
    } catch {
      return false
    }
  }

  const setLiteMode = (value: boolean) => {
    isLiteMode.value = value
  }

  const logout = () => {
    user.value = null
    tenant.value = null
    token.value = ''
    refreshToken.value = ''
    knowledgeBases.value = []
    currentKnowledgeBase.value = null
    isLiteMode.value = false
    clearSessionResourceCaches()

    for (const key of [
      'gaze_user',
      'gaze_tenant',
      'gaze_token',
      'gaze_refresh_token',
      'gaze_knowledge_bases',
      'gaze_current_kb',
      'gaze_lite_mode',
    ]) {
      localStorage.removeItem(key)
    }
    try {
      sessionStorage.removeItem('gaze_lite_last_path')
    } catch {
      /* ignore */
    }
    reloadUserPreferences()
  }

  const initFromStorage = () => {
    const storedUser = localStorage.getItem('gaze_user')
    const storedTenant = localStorage.getItem('gaze_tenant')
    const storedToken = localStorage.getItem('gaze_token')
    const storedRefreshToken = localStorage.getItem('gaze_refresh_token')
    const storedKnowledgeBases = localStorage.getItem('gaze_knowledge_bases')
    const storedCurrentKb = localStorage.getItem('gaze_current_kb')

    if (storedUser) {
      try {
        user.value = userInfoFromApi(JSON.parse(storedUser))
      } catch (error) {
        console.error(i18n.global.t('authStore.errors.parseUserFailed'), error)
      }
    }
    if (storedTenant) {
      try {
        tenant.value = JSON.parse(storedTenant)
      } catch (error) {
        console.error(i18n.global.t('authStore.errors.parseTenantFailed'), error)
      }
    }
    if (storedToken) token.value = storedToken
    if (storedRefreshToken) refreshToken.value = storedRefreshToken
    if (storedKnowledgeBases) {
      try {
        const parsed = JSON.parse(storedKnowledgeBases)
        knowledgeBases.value = Array.isArray(parsed) ? parsed : []
      } catch (error) {
        console.error(i18n.global.t('authStore.errors.parseKnowledgeBasesFailed'), error)
      }
    }
    if (storedCurrentKb) {
      try {
        currentKnowledgeBase.value = JSON.parse(storedCurrentKb)
      } catch (error) {
        console.error(i18n.global.t('authStore.errors.parseCurrentKnowledgeBaseFailed'), error)
      }
    }
    isLiteMode.value = localStorage.getItem('gaze_lite_mode') === 'true'
  }

  initFromStorage()

  return {
    user,
    tenant,
    token,
    refreshToken,
    knowledgeBases,
    currentKnowledgeBase,

    isLoggedIn,
    hasValidTenant,
    currentTenantId,
    currentTenantName,
    currentUserId,
    isSystemAdmin,
    effectiveTenantId,
    isLiteMode,

    setUser,
    setTenant,
    setToken,
    setRefreshToken,
    setKnowledgeBases,
    setCurrentKnowledgeBase,
    refreshFromAuthMe,
    setLiteMode,
    logout,
    initFromStorage,
  }
})
