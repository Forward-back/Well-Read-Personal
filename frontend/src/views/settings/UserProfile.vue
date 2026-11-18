<template>
  <div class="user-profile">
    <div class="section-header">
      <h2>{{ $t('userProfile.title') }}</h2>
      <p class="section-description">{{ $t('userProfile.description') }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-inline">
      <t-loading size="small" />
      <span>{{ $t('tenant.loadingInfo') }}</span>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="error-inline">
      <t-alert theme="error" :message="error">
        <template #operation>
          <t-button size="small" @click="loadInfo">{{ $t('tenant.retry') }}</t-button>
        </template>
      </t-alert>
    </div>

    <!-- Content -->
    <div v-else class="settings-group">
      <!-- 头像 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ t('userProfile.avatar.label') }}</label>
          <p class="desc">{{ t('userProfile.avatar.description') }}</p>
        </div>
        <div class="setting-control avatar-control">
          <div class="avatar-preview" :class="{ 'is-empty': !displayAvatar }">
            <img v-if="displayAvatar" :src="displayAvatar" alt="" />
            <span v-else>{{ avatarInitial }}</span>
          </div>
          <div class="avatar-actions">
            <t-button theme="default" variant="outline" size="small" @click="pickAvatar">
              {{ displayAvatar ? t('userProfile.avatar.change') : t('userProfile.avatar.choose') }}
            </t-button>
            <t-button
              v-if="pendingAvatar"
              theme="primary"
              size="small"
              :loading="avatarSaving"
              @click="saveAvatar"
            >
              {{ avatarSaving ? t('userProfile.avatar.saving') : t('userProfile.avatar.save') }}
            </t-button>
            <t-button
              v-if="displayAvatar"
              theme="danger"
              variant="text"
              size="small"
              :disabled="avatarSaving"
              @click="removeAvatar"
            >
              {{ t('userProfile.avatar.remove') }}
            </t-button>
          </div>
          <p class="avatar-hint">{{ t('userProfile.avatar.hint') }}</p>
          <input
            ref="avatarInput"
            class="avatar-file-input"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            @change="onAvatarFile"
          />
        </div>
      </div>

      <!-- 用户 ID -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('tenant.api.userIdLabel') }}</label>
          <p class="desc">{{ $t('tenant.api.userIdDescription') }}</p>
        </div>
        <div class="setting-control">
          <span class="info-value">{{ userInfo?.id || '-' }}</span>
        </div>
      </div>

      <!-- 用户名 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('tenant.api.usernameLabel') }}</label>
          <p class="desc">{{ $t('tenant.api.usernameDescription') }}</p>
        </div>
        <div class="setting-control">
          <span class="info-value">{{ userInfo?.username || '-' }}</span>
        </div>
      </div>

      <!-- 邮箱 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('tenant.api.emailLabel') }}</label>
          <p class="desc">{{ $t('tenant.api.emailDescription') }}</p>
        </div>
        <div class="setting-control">
          <span class="info-value">{{ userInfo?.email || '-' }}</span>
        </div>
      </div>

      <!-- 注册时间 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('tenant.api.createdAtLabel') }}</label>
          <p class="desc">{{ $t('tenant.api.createdAtDescription') }}</p>
        </div>
        <div class="setting-control">
          <span class="info-value">{{ formatDate(userInfo?.created_at) }}</span>
        </div>
      </div>

      <!-- 修改密码：与其它 setting-row 同款只读行 + 编辑入口，表单进原地 popup -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('userProfile.changePassword.label') }}</label>
          <p class="desc">
            {{ oidcOnlyLogin
              ? $t('userProfile.changePassword.oidcOnlyDescription')
              : $t('userProfile.changePassword.description') }}
          </p>
        </div>
        <div class="setting-control">
          <template v-if="oidcOnlyLogin">
            <span class="info-value info-value--muted">—</span>
          </template>
          <template v-else>
            <span class="info-value password-mask" aria-hidden="true">••••••••</span>
            <t-popup
              v-model="passwordPopupVisible"
              trigger="click"
              placement="bottom-end"
              destroy-on-close
              overlay-class-name="user-profile-password-popup-overlay"
            >
              <t-button
                theme="default"
                variant="text"
                shape="square"
                size="small"
                class="edit-btn"
                :title="$t('userProfile.changePassword.label')"
                :aria-label="$t('userProfile.changePassword.label')"
              >
                <template #icon>
                  <t-icon name="edit" />
                </template>
              </t-button>
              <template #content>
                <div class="password-popup-inner" @click.stop>
                  <div class="password-popup-title">{{ $t('userProfile.changePassword.label') }}</div>
                  <p class="password-popup-hint">{{ $t('userProfile.changePassword.description') }}</p>
                  <t-form
                    ref="passwordFormRef"
                    :data="passwordForm"
                    :rules="passwordRules"
                    label-align="top"
                    class="password-popup-form"
                    @submit.prevent
                  >
                    <t-form-item :label="$t('userProfile.changePassword.currentLabel')" name="oldPassword">
                      <t-input
                        v-model="passwordForm.oldPassword"
                        type="password"
                        autocomplete="current-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.currentPlaceholder')"
                      />
                    </t-form-item>
                    <t-form-item :label="$t('userProfile.changePassword.newLabel')" name="newPassword">
                      <t-input
                        v-model="passwordForm.newPassword"
                        type="password"
                        autocomplete="new-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.newPlaceholder')"
                      />
                    </t-form-item>
                    <t-form-item :label="$t('userProfile.changePassword.confirmLabel')" name="confirmPassword">
                      <t-input
                        v-model="passwordForm.confirmPassword"
                        type="password"
                        autocomplete="new-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.confirmPlaceholder')"
                        @enter="submitPasswordChange"
                      />
                    </t-form-item>
                  </t-form>
                  <div class="password-popup-footer">
                    <t-button
                      variant="outline"
                      :disabled="passwordSubmitting"
                      @click="closePasswordPopup"
                    >
                      {{ $t('common.cancel') }}
                    </t-button>
                    <t-button
                      theme="primary"
                      :loading="passwordSubmitting"
                      @click="submitPasswordChange"
                    >
                      {{ $t('userProfile.changePassword.submit') }}
                    </t-button>
                  </div>
                </div>
              </template>
            </t-popup>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'
import {
  getCurrentUser,
  getAuthConfig,
  changePassword,
  updateMyAvatar,
  logout as logoutApi,
  type UserInfo,
} from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { newPasswordRules } from '@/utils/passwordPolicy'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const userInfo = ref<UserInfo | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)
const pendingAvatar = ref<string | null>(null)
const avatarSaving = ref(false)
const displayAvatar = computed(() => pendingAvatar.value ?? userInfo.value?.avatar ?? '')
const avatarInitial = computed(() => {
  const source = userInfo.value?.username || userInfo.value?.email || 'W'
  return source.trim().slice(0, 1).toUpperCase()
})
const complexPasswordEnabled = ref(false)
const loading = ref(true)
const error = ref('')

const passwordPopupVisible = ref(false)
const passwordFormRef = ref<FormInstanceFunctions | null>(null)
const passwordSubmitting = ref(false)
const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const loadPasswordPolicy = async () => {
  try {
    const resp = await getAuthConfig()
    complexPasswordEnabled.value = !!resp.complex_password_enabled
  } catch {
    complexPasswordEnabled.value = false
  }
}

const oidcOnlyLogin = computed(
  () => userInfo.value?.preferences?.oidc_only_login === true,
)

watch(passwordPopupVisible, (open) => {
  if (!open) {
    resetPasswordForm()
    return
  }
  resetPasswordForm()
  void loadPasswordPolicy()
})

const passwordRules = computed<Record<string, FormRule[]>>(() => ({
  oldPassword: [
    { required: true, message: t('userProfile.changePassword.currentRequired'), type: 'error' },
  ],
  newPassword: newPasswordRules(t, complexPasswordEnabled.value, [
    {
      validator: (val: string) => val !== passwordForm.oldPassword,
      message: t('userProfile.changePassword.sameAsCurrent'),
      type: 'error',
    },
  ]),
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    {
      validator: (val: string) => val === passwordForm.newPassword,
      message: t('auth.passwordMismatch'),
      type: 'error',
      trigger: 'blur',
    },
  ],
}))

const loadInfo = async () => {
  try {
    loading.value = true
    error.value = ''
    const resp = await getCurrentUser()
    if ((resp as any).success && resp.data) {
      userInfo.value = resp.data.user
    } else {
      error.value = resp.message || t('tenant.messages.fetchFailed')
    }
  } catch (err: any) {
    error.value = err?.message || t('tenant.messages.networkError')
  } finally {
    loading.value = false
  }
}

const pickAvatar = () => {
  if (avatarSaving.value) return
  const input = avatarInput.value
  if (!input) return
  input.value = ''
  input.click()
}

const cropSquare = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('read'))
    reader.onload = () => {
      const image = new Image()
      image.onerror = () => reject(new Error('image'))
      image.onload = () => {
        const side = Math.min(image.width, image.height)
        const sx = (image.width - side) / 2
        const sy = (image.height - side) / 2
        const canvas = document.createElement('canvas')
        canvas.width = 256
        canvas.height = 256
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          reject(new Error('canvas'))
          return
        }
        ctx.drawImage(image, sx, sy, side, side, 0, 0, 256, 256)
        resolve(canvas.toDataURL('image/jpeg', 0.88))
      }
      image.src = String(reader.result || '')
    }
    reader.readAsDataURL(file)
  })
}

const onAvatarFile = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    MessagePlugin.warning(t('userProfile.avatar.invalidType'))
    input.value = ''
    return
  }
  if (file.size > 5 * 1024 * 1024) {
    MessagePlugin.warning(t('userProfile.avatar.tooLarge'))
    input.value = ''
    return
  }
  try {
    pendingAvatar.value = await cropSquare(file)
  } catch {
    MessagePlugin.error(t('userProfile.avatar.readFailed'))
  } finally {
    input.value = ''
  }
}

const saveAvatar = async () => {
  if (!pendingAvatar.value || avatarSaving.value) return
  avatarSaving.value = true
  try {
    const resp = await updateMyAvatar(pendingAvatar.value)
    if (!resp.success) {
      MessagePlugin.error(resp.message || t('userProfile.avatar.failed'))
      return
    }
    const saved = resp.data?.avatar ?? pendingAvatar.value
    if (userInfo.value) userInfo.value = { ...userInfo.value, avatar: saved }
    authStore.setAvatar(saved)
    pendingAvatar.value = null
    MessagePlugin.success(t('userProfile.avatar.success'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('userProfile.avatar.failed'))
  } finally {
    avatarSaving.value = false
  }
}

const removeAvatar = async () => {
  if (avatarSaving.value) return
  avatarSaving.value = true
  try {
    const resp = await updateMyAvatar('')
    if (!resp.success) {
      MessagePlugin.error(resp.message || t('userProfile.avatar.failed'))
      return
    }
    if (userInfo.value) userInfo.value = { ...userInfo.value, avatar: '' }
    authStore.setAvatar('')
    pendingAvatar.value = null
    MessagePlugin.success(t('userProfile.avatar.removed'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('userProfile.avatar.failed'))
  } finally {
    avatarSaving.value = false
  }
}

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return t('tenant.unknown')
  try {
    const d = new Date(dateStr)
    const fmt = new Intl.DateTimeFormat(locale.value || 'zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
    return fmt.format(d)
  } catch {
    return t('tenant.formatError')
  }
}

const resetPasswordForm = () => {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordFormRef.value?.clearValidate?.()
}

const closePasswordPopup = () => {
  if (passwordSubmitting.value) return
  passwordPopupVisible.value = false
  resetPasswordForm()
}

const submitPasswordChange = async () => {
  if (passwordSubmitting.value) return
  const result = await passwordFormRef.value?.validate?.()
  if (result !== true) return

  passwordSubmitting.value = true
  try {
    const resp = await changePassword({
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword,
    })
    if (!resp.success) {
      MessagePlugin.error(resp.message || t('userProfile.changePassword.failed'))
      return
    }

    passwordPopupVisible.value = false
    MessagePlugin.success(t('userProfile.changePassword.success'))
    resetPasswordForm()

    // Backend revokes all sessions on success; mirror that locally and
    // force a fresh login with the new credential.
    try {
      await logoutApi()
    } catch {
      /* ignore — local cleanup still proceeds */
    }
    authStore.logout()
    router.push('/login')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('userProfile.changePassword.failed'))
  } finally {
    passwordSubmitting.value = false
  }
}

onMounted(loadInfo)
</script>

<style lang="less" scoped>
.user-profile {
  width: 100%;
}

.section-header {
  margin-bottom: 32px;

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 8px 0;
  }

  .section-description {
    font-size: 14px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.loading-inline {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 40px 0;
  justify-content: center;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}

.error-inline {
  padding: 20px 0;
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child {
    border-bottom: none;
  }
}

.setting-info {
  flex: 1;
  max-width: 65%;
  padding-right: 24px;

  label {
    font-size: 15px;
    font-weight: 500;
    color: var(--td-text-color-primary);
    display: block;
    margin-bottom: 4px;
  }

  .desc {
    font-size: 13px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.setting-control {
  flex-shrink: 0;
  min-width: 280px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;

  .info-value {
    font-size: 14px;
    color: var(--td-text-color-primary);
    text-align: right;
    word-break: break-word;
  }

  .info-value--muted {
    color: var(--td-text-color-placeholder);
  }

  .edit-btn {
    flex-shrink: 0;
  }
}

.password-mask {
  letter-spacing: 0.12em;
  color: var(--td-text-color-secondary);
}

.avatar-control {
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.avatar-preview {
  width: 64px;
  height: 64px;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--td-component-stroke);
  border-radius: 14px;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
  font-size: 24px;
  font-weight: 700;
  line-height: 1;

  img {
    width: 100%;
    height: 100%;
    display: block;
    object-fit: cover;
  }

  &.is-empty {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);
  }
}

.avatar-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.avatar-hint {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 18px;
  text-align: right;
}

.avatar-file-input {
  display: none;
}

.password-popup-inner {
  max-width: 100%;
}

.password-popup-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 8px;
  line-height: 1.35;
}

.password-popup-hint {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--td-text-color-secondary);
}

.password-popup-form {
  :deep(.t-form__item) {
    margin-bottom: 14px;

    &:last-child {
      margin-bottom: 4px;
    }
  }
}

.password-popup-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>

<style lang="less">
/* t-popup 挂到 body，需全局样式；z-index 需高于设置全屏遮罩（2000）。 */
.user-profile-password-popup-overlay {
  z-index: 3050 !important;

  .t-popup__content {
    padding: 14px 16px !important;
    min-width: 300px;
    max-width: min(392px, calc(100vw - 24px));
    border-radius: 12px !important;
    background: var(--td-bg-color-container) !important;
    border: 0.5px solid var(--td-component-stroke) !important;
    box-shadow:
      0 0 0 0.5px rgba(0, 0, 0, 0.03),
      0 2px 4px rgba(0, 0, 0, 0.04),
      0 8px 24px rgba(0, 0, 0, 0.1) !important;
    backdrop-filter: blur(20px) saturate(180%) !important;
    -webkit-backdrop-filter: blur(20px) saturate(180%) !important;
  }
}

:root[theme-mode='dark'] .user-profile-password-popup-overlay .t-popup__content {
  background: rgba(36, 36, 36, 0.92) !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
  box-shadow:
    0 0 0 0.5px rgba(255, 255, 255, 0.05),
    0 2px 4px rgba(0, 0, 0, 0.12),
    0 8px 32px rgba(0, 0, 0, 0.28) !important;
}
</style>
