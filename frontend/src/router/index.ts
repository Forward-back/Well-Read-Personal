import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getCurrentUser, userInfoFromApi } from '@/api/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: () => import('../views/shell/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'welcome',
          component: () => import('../views/welcome/WelcomeHome.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform',
          component: () => import('../views/chat/ChatWorkspace.vue'),
          children: [
            { path: '', redirect: '/platform/creatChat' },
            {
              path: 'creatChat',
              name: 'globalCreatChat',
              component: () => import('../views/creatChat/creatChat.vue'),
              meta: { requiresAuth: true },
            },
            {
              path: 'chat/:chatid',
              name: 'chat',
              component: () => import('../views/chat/index.vue'),
              meta: { requiresAuth: true },
            },
          ],
        },
        {
          path: 'platform/knowledge-bases',
          name: 'knowledgeBaseList',
          component: () => import('../views/knowledge/KnowledgeBaseList.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/knowledge-bases/new',
          name: 'knowledgeBaseCreate',
          component: () => import('../views/knowledge/KnowledgeBaseCreate.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/knowledge-bases/:kbId',
          name: 'knowledgeBaseDetail',
          component: () => import('../views/knowledge/KnowledgeBase.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/knowledge-bases/:kbId/settings',
          name: 'knowledgeBaseSettings',
          component: () => import('../views/knowledge/KnowledgeBaseSettings.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/knowledge-bases/:kbId/creatChat',
          name: 'kbCreatChat',
          component: () => import('../views/creatChat/creatChat.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/reading',
          name: 'readingRun',
          component: () => import('../views/reading/ReadingRun.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/review',
          name: 'review',
          component: () => import('../views/review/v3/ReviewV3.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/review-v3',
          redirect: to => ({ path: '/platform/review', query: to.query }),
        },
        {
          path: 'platform/review/cluster/:rowGroupId',
          name: 'reviewV3Cluster',
          component: () => import('../views/review/v3/ReviewClusterPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/review-v3/cluster/:rowGroupId',
          redirect: to => ({ path: `/platform/review/cluster/${to.params.rowGroupId}`, query: to.query }),
        },
        {
          path: 'platform/skills',
          name: 'skills',
          component: () => import('../views/skills/Skills.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'platform/settings',
          name: 'settings',
          component: () => import('../views/settings/Settings.vue'),
          meta: { requiresAuth: true },
        },
      ],
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/auth/Login.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/welcome',
      redirect: '/',
    },
    ...(import.meta.env.DEV ? [{
      path: '/platform/dev/markdown',
      name: 'markdownTest',
      component: () => import('../views/dev/MarkdownTestPage.vue'),
      meta: { requiresAuth: false },
    }] : []),
  ],
})

/** Restore user/tenant from the stored token via /auth/me. */
async function hydrateSessionFromToken(authStore: ReturnType<typeof useAuthStore>) {
  const token = localStorage.getItem('gaze_token')
  if (!token) return false

  if (!authStore.token) {
    authStore.setToken(token)
  }
  const storedRefreshToken = localStorage.getItem('gaze_refresh_token')
  if (storedRefreshToken && !authStore.refreshToken) {
    authStore.setRefreshToken(storedRefreshToken)
  }

  try {
    const response = await getCurrentUser()
    const user = response.data?.user
    if (!response.success || !user) return false

    authStore.setUser(userInfoFromApi(user, response.data?.tenant?.id))

    const tenant = response.data?.tenant
    if (tenant) {
      authStore.setTenant({
        id: String(tenant.id) || '',
        name: tenant.name || '',
        owner_id: tenant.owner_id || user.id || '',
        created_at: tenant.created_at || new Date().toISOString(),
        updated_at: tenant.updated_at || new Date().toISOString(),
      })
    } else {
      authStore.setTenant(null)
    }
    return true
  } catch {
    return false
  }
}

router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth === false) {
    if (to.path === '/login' && authStore.isLoggedIn) {
      return '/'
    }
    return true
  }

  if (!authStore.isLoggedIn) {
    const restored = await hydrateSessionFromToken(authStore)
    if (!restored) return { path: '/login', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
