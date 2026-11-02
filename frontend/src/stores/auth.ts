import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { UserInfo, TenantInfo, KnowledgeBaseInfo } from '@/api/auth'
import { userInfoFromApi } from '@/api/auth'
import type { TenantInfo as TenantInfoFromAPI } from '@/api/tenant'
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
//
// Compatibility fields/methods keep the not-yet-deleted enterprise views
// compiling while they are removed in later passes. They never grant access:
// the backend is the security boundary, and every authenticated request is
// already scoped to the owner's tenant.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserInfo | null>(null)
  const tenant = ref<TenantInfo | null>(null)
  const token = ref<string>('')
  const refreshToken = ref<string>('')
  const knowledgeBases = ref<KnowledgeBaseInfo[]>([])
  const currentKnowledgeBase = ref<KnowledgeBaseInfo | null>(null)
  const isLiteMode = ref(false)

  const selectedTenantId = ref<number | null>(null)
  const selectedTenantName = ref<string | null>(null)
  const allTenants = ref<TenantInfoFromAPI[]>([])
  const memberships = ref<Array<{
    tenant_id: number
    tenant_name?: string
    role: string
    department_id?: string | null
    department_role?: string
    caps?: string[]
  }>>([])
  const pendingInvitationCount = ref(0)
  const canCreateTenant = ref(false)
  const autoAcceptInvitation = ref(false)

  const isLoggedIn = computed(() => !!token.value && !!user.value)
  const hasValidTenant = computed(() => !!tenant.value && !!tenant.value.id)
  const currentTenantId = computed(() => tenant.value?.id || '')
  const currentTenantName = computed(() => tenant.value?.name || '')
  const currentUserId = computed(() => user.value?.id || '')
  const canAccessAllTenants = computed(() => false)
  const isSystemAdmin = computed(() => user.value?.is_system_admin || false)
  const isEmployee = computed(() => false)
  const currentTenantRole = computed(() => 'owner')
  const currentDepartmentRole = computed(() => 'manager')
  const currentDepartmentId = computed(() => String(tenant.value?.id || ''))
  const isCompanyAdmin = computed(() => false)
  const currentCaps = computed<readonly string[]>(() => [])
  const companyCaps = computed<readonly string[]>(() => [])
  const effectiveTenantId = computed(() => (tenant.value?.id ? Number(tenant.value.id) : null))

  const hasRole = (_min: 'viewer' | 'contributor' | 'admin' | 'owner'): boolean => true
  const hasDepartmentRole = (_min: 'viewer' | 'editor' | 'knowledge_admin' | 'manager'): boolean => true
  const hasCaps = (_required: readonly string[]): boolean => true
  const hasCompanyCaps = (_required: readonly string[]): boolean => true

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

  const setSelectedTenant = (tenantId: number | null, tenantName: string | null = null) => {
    selectedTenantId.value = tenantId
    selectedTenantName.value = tenantName
    if (tenantId === null) {
      localStorage.removeItem('gaze_selected_tenant_id')
      localStorage.removeItem('gaze_selected_tenant_name')
      return
    }
    localStorage.setItem('gaze_selected_tenant_id', String(tenantId))
    if (tenantName) localStorage.setItem('gaze_selected_tenant_name', tenantName)
  }

  const setAllTenants = (tenants: TenantInfoFromAPI[]) => {
    allTenants.value = tenants
  }

  const setMemberships = (list: typeof memberships.value) => {
    memberships.value = Array.isArray(list) ? list : []
  }

  const setPendingInvitationCount = (n: number) => {
    pendingInvitationCount.value = n
  }

  const setCanCreateTenant = (allowed: boolean) => {
    canCreateTenant.value = allowed
  }

  const setAutoAcceptInvitation = (enabled: boolean) => {
    autoAcceptInvitation.value = enabled
  }

  const fetchPendingInvitationCount = async () => {
    pendingInvitationCount.value = 0
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

  const acceptInvitationByTokenAndRefresh = async (
    _invitationToken: string,
  ): Promise<{ ok: boolean; tenantId?: number; tenantName?: string }> => {
    return { ok: false }
  }

  const getSelectedTenant = () => selectedTenantId.value

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
    selectedTenantId.value = null
    selectedTenantName.value = null
    allTenants.value = []
    memberships.value = []
    pendingInvitationCount.value = 0
    canCreateTenant.value = false
    autoAcceptInvitation.value = false
    isLiteMode.value = false
    clearSessionResourceCaches()

    for (const key of [
      'gaze_user',
      'gaze_tenant',
      'gaze_token',
      'gaze_refresh_token',
      'gaze_knowledge_bases',
      'gaze_current_kb',
      'gaze_selected_tenant_id',
      'gaze_selected_tenant_name',
      'gaze_memberships',
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
    selectedTenantId,
    selectedTenantName,
    allTenants,
    memberships,
    pendingInvitationCount,
    canCreateTenant,
    autoAcceptInvitation,

    isLoggedIn,
    hasValidTenant,
    currentTenantId,
    currentTenantName,
    currentUserId,
    canAccessAllTenants,
    isSystemAdmin,
    isEmployee,
    currentTenantRole,
    currentDepartmentRole,
    currentDepartmentId,
    isCompanyAdmin,
    hasRole,
    hasDepartmentRole,
    currentCaps,
    companyCaps,
    hasCaps,
    hasCompanyCaps,
    effectiveTenantId,
    isLiteMode,

    setUser,
    setTenant,
    setToken,
    setRefreshToken,
    setKnowledgeBases,
    setCurrentKnowledgeBase,
    setSelectedTenant,
    setAllTenants,
    setMemberships,
    setPendingInvitationCount,
    setCanCreateTenant,
    setAutoAcceptInvitation,
    fetchPendingInvitationCount,
    refreshFromAuthMe,
    acceptInvitationByTokenAndRefresh,
    getSelectedTenant,
    setLiteMode,
    logout,
    initFromStorage,
  }
})
