import { post, get, put } from '@/utils/request'
import i18n from '@/i18n'

const t = (key: string) => i18n.global.t(key)

// 用户登录接口
export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  success: boolean
  message?: string
  user?: {
    id: string
    username: string
    email: string
    avatar?: string
    tenant_id: number
    can_access_all_tenants?: boolean
    is_system_admin?: boolean
    is_active: boolean
    created_at: string
    updated_at: string
  }
  tenant?: {
    id: number
    name: string
    description: string
    status: string
    business: string
    storage_quota: number
    storage_used: number
    created_at: string
    updated_at: string
  } | null
  token?: string
  refresh_token?: string
}

// 用户偏好（与后端 types.UserPreferences 对齐，字段可选 = 没显式设置过）。
// 新加 key 时记得：后端 service.UpdateUserPreferences 也要在 merge 分支里
// 处理；前端调用方按需读 / 默认值降级。
export interface UserPreferences {
  // last_active_tenant_id 持久化「刷新 / 换设备 / 重新登录后回到上次的空间」
  // 偏好；后端在 Login / RefreshToken 时校验 membership 有效后才会沿用，
  // 否则回退到 home 并清掉这个字段。传 0 给 PATCH 表示「清除偏好」。
  last_active_tenant_id?: number | null
  // oidc_only_login 为 true 表示账号由 OIDC 自动开通且用户尚未设置已知密码。
  oidc_only_login?: boolean
}

// 用户信息接口
export interface UserInfo {
  id: string
  username: string
  email: string
  avatar?: string
  tenant_id: string
  can_access_all_tenants?: boolean
  preferences?: UserPreferences
  is_system_admin?: boolean
  // 银联商务融合版：员工标识（注册时工号校验通过）
  is_employee?: boolean
  created_at: string
  updated_at: string
}

/**
 * 把后端返回的 user JSON 规范化成前端 UserInfo。
 *
 * 历史上有 4 处独立的 setUser 调用（Login、autoSetup、token rehydrate、
 * /auth/me 主动 refresh）各自手写字段白名单，每加一个 user 字段都要在
 * 4 处同步——否则该字段就被悄悄过滤掉。is_system_admin 上线时就因为
 * 漏拷一处而看不到「系统管理」入口；这个工厂存在的目的就是杜绝同类
 * 漏拷再发生。**新增 user 字段请只改这里**。
 *
 * fallbackTenantId 是 tenant_id 缺失时的兜底来源——
 *   - autoSetup 响应顶层有 tenant.id，但 user 对象上没有 tenant_id
 *   - /auth/me 偶发只返回 user 不带 tenant 时也走兜底
 * 调用方按需传入；不传则保持空字符串（与历史行为一致）。
 *
 * 字段读取统一走 `=== true` 而不是 `|| false`，对偶发非 boolean
 * 类型（后端某天传 1/0 或字符串）做严格收敛，避免把 truthy 字符串
 * 误判为权限通过。
 */
export function userInfoFromApi(
  u: any,
  fallbackTenantId?: string | number | null,
): UserInfo {
  const rawTenantId =
    u?.tenant_id !== undefined && u?.tenant_id !== null && u.tenant_id !== ''
      ? u.tenant_id
      : fallbackTenantId ?? ''
  const tid = Number(rawTenantId) > 0 ? rawTenantId : ''
  return {
    id: u?.id || '',
    username: u?.username || '',
    email: u?.email || '',
    avatar: u?.avatar,
    tenant_id: String(tid) || '',
    can_access_all_tenants: u?.can_access_all_tenants === true,
    is_system_admin: u?.is_system_admin === true,
    is_employee: u?.is_employee === true,
    preferences: u?.preferences,
    created_at: u?.created_at || new Date().toISOString(),
    updated_at: u?.updated_at || new Date().toISOString(),
  }
}

// 空间信息接口
export interface TenantInfo {
  id: string
  name: string
  description?: string
  status?: string
  business?: string
  owner_id: string
  storage_quota?: number
  storage_used?: number
  created_at: string
  updated_at: string
  knowledge_bases?: KnowledgeBaseInfo[]
}

// 知识库信息接口
export interface KnowledgeBaseInfo {
  id: string
  name: string
  description: string
  tenant_id: string
  // creator_id is the user id of whoever originally created the KB.
  // Set by PR 5 of the multi-tenant RBAC series; nullable for legacy
  // KBs created before that migration backfilled the column.
  creator_id?: string
  // creator_name 由后端 list 接口批量回填（username 优先，退化到 email），
  // 仅用于列表卡片来源徽章；缺失代表无法解析（已删除 / 老数据）。
  creator_name?: string
  created_at: string
  updated_at: string
  document_count?: number
  chunk_count?: number
}

// 模型信息接口
export interface ModelInfo {
  id: string
  name: string
  type: string
  source: string
  description?: string
  is_default?: boolean
  created_at: string
  updated_at: string
}

/**
 * 用户登录
 */
export async function login(data: LoginRequest): Promise<LoginResponse> {
  try {
    const response = await post('/api/v1/auth/login', data)
    return response as unknown as LoginResponse
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.loginFailed')
    }
  }
}

/**
 * 获取认证配置（仅返回前端渲染需要的公开字段）。
 */
export interface AuthConfigResponse {
  success: boolean
  registration_mode: 'self_serve' | 'invite_only' | string
  complex_password_enabled: boolean
}

export async function getAuthConfig(): Promise<AuthConfigResponse> {
  try {
    const response = await get('/api/v1/auth/config')
    return response as unknown as AuthConfigResponse
  } catch {
    return { success: false, registration_mode: 'self_serve', complex_password_enabled: false }
  }
}

/**
 * 获取当前用户信息
 */
export async function getCurrentUser(): Promise<{ success: boolean; data?: { user: UserInfo; tenant?: TenantInfo | null }; message?: string }> {
  try {
    const response = await get('/api/v1/auth/me')
    return response as unknown as { success: boolean; data?: { user: UserInfo; tenant?: TenantInfo | null }; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.getUserFailed')
    }
  }
}

/**
 * 更新当前用户的偏好设置（PATCH 语义：只发要改的字段，后端只覆盖发了的 key，
 * 其它 key 保持不变）。后端会返回更新后的完整 preferences 对象。
 */
export async function updateMyPreferences(
  patch: Partial<UserPreferences>,
): Promise<{ success: boolean; data?: UserPreferences; message?: string }> {
  try {
    const response = await put('/api/v1/auth/me/preferences', patch)
    return response as unknown as { success: boolean; data?: UserPreferences; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.updatePreferencesFailed'),
    }
  }
}

/**
 * 获取当前空间信息
 */
export async function getCurrentTenant(): Promise<{ success: boolean; data?: TenantInfo; message?: string }> {
  try {
    const response = await get('/api/v1/auth/tenant')
    return response as unknown as { success: boolean; data?: TenantInfo; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.getTenantFailed')
    }
  }
}

/**
 * 刷新Token
 */
export async function refreshToken(refreshToken: string): Promise<{ success: boolean; data?: { token: string; refreshToken: string }; message?: string }> {
  try {
    const response: any = await post('/api/v1/auth/refresh', { refreshToken })
    if (response && response.success) {
      if (response.access_token || response.refresh_token) {
        return {
          success: true,
          data: {
            token: response.access_token,
            refreshToken: response.refresh_token,
          }
        }
      }
    }

    // 其他情况直接返回原始消息
    return {
      success: false,
      message: response?.message || t('error.auth.refreshTokenFailed')
    }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.refreshTokenFailed')
    }
  }
}

/**
 * 用户登出
 */
export async function logout(): Promise<{ success: boolean; message?: string }> {
  try {
    await post('/api/v1/auth/logout', {})
    return {
      success: true
    }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.logoutFailed')
    }
  }
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

/** Map change-password API failures to localized UI strings. */
export function resolveChangePasswordError(error: any): string {
  const details =
    typeof error?.error?.details === 'string'
      ? error.error.details
      : typeof error?.details === 'string'
        ? error.details
        : ''
  switch (details) {
    case 'invalid_old_password':
      return t('userProfile.changePassword.failed')
    case 'password_policy':
      return t('userProfile.changePassword.policyFailed')
    case 'same_password':
      return t('userProfile.changePassword.sameAsCurrent')
    default:
      return error?.message || t('userProfile.changePassword.failed')
  }
}

/**
 * Self-service password rotation. On success the backend revokes every
 * outstanding session for the caller, so the client should clear local
 * auth state and send the user back to /login.
 */
export async function changePassword(
  data: ChangePasswordRequest,
): Promise<{ success: boolean; message?: string }> {
  try {
    const response = await post('/api/v1/auth/change-password', data)
    return response as unknown as { success: boolean; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: resolveChangePasswordError(error),
    }
  }
}

/**
 * 验证Token有效性
 */
export async function validateToken(): Promise<{ success: boolean; valid?: boolean; message?: string }> {
  try {
    const response = await get('/api/v1/auth/validate')
    return response as unknown as { success: boolean; valid?: boolean; message?: string }
  } catch (error: any) {
    return {
      success: false,
      valid: false,
      message: error.message || t('error.auth.validateTokenFailed')
    }
  }
}





// ---- share-link registration removed: personal edition has no invitations.
