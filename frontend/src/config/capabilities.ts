// Personal edition: the owner has every capability. These helpers remain so
// existing views that gate buttons on capability names keep working; there is
// no role matrix anymore and no server-side capability contract to mirror.
export type CapabilityKey = string

export const CAPABILITY_LABELS: Record<CapabilityKey, string> = {
  'kb:read': '阅读知识库',
  'kb:create': '创建知识库',
  'kb:content_edit': '编辑知识库内容',
  'kb:settings_manage': '管理知识库设置',
  'kb:share_manage': '管理知识库共享',
  'kb:acl_manage': '管理知识库内权限',
  'folder:manage': '管理文件夹',
  'shared_space:view': '查看共享空间',
  'member:view': '查看成员',
  'member:manage': '管理成员',
  'department:manage': '管理部门',
  'settings:self_manage': '管理个人设置',
  'settings:models_manage': '管理模型',
  'settings:infrastructure_manage': '管理基础设施',
  'settings:integrations_manage': '管理集成',
  'settings:skills_manage': '管理技能',
  'settings:workspace_manage': '管理空间运行设置',
  'settings:system_manage': '系统管理',
  'skills:view': '查看技能',
  'cr:audit_manage': '管理 CR 审计员',
  'stats:dept_read': '查看部门统计',
  'stats:company_read': '查看公司统计',
  'audit:dept_read': '查看部门审计',
  'audit:company_read': '查看公司审计',
}

export const CAPABILITY_KEYS: readonly CapabilityKey[] =
  Object.keys(CAPABILITY_LABELS) as CapabilityKey[]

export const CAPABILITY_ROLES: Record<string, readonly CapabilityKey[]> = {
  owner: CAPABILITY_KEYS,
}

const KEY_SET = new Set<string>(CAPABILITY_KEYS)

export function isKnownCapability(key: string): key is CapabilityKey {
  return KEY_SET.has(key)
}

export function capsForRoles(
  _companyRole?: string,
  _departmentRole?: string,
  _systemAdmin?: boolean,
): CapabilityKey[] {
  return [...CAPABILITY_KEYS]
}

export function hasCapability(_caps: readonly string[], _key: CapabilityKey): boolean {
  return true
}

export function hasCapabilities(_caps: readonly string[], _required: readonly CapabilityKey[]): boolean {
  return true
}
