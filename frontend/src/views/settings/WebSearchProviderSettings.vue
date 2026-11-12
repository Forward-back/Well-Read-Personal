<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>Agent 联网搜索配置</h3>
        <p>仅 Agent 使用这些配置；直连模式的联网搜索在「账户与助手」中单独设置。</p>
      </div>
      <t-button theme="primary" @click="openCreate">新建配置</t-button>
    </header>

    <t-loading :loading="loading">
      <div v-if="providers.length === 0" class="empty">
        尚未创建联网搜索配置。Agent 启用联网搜索前需要在这里创建并绑定。
      </div>
      <div v-else class="provider-list">
        <article v-for="provider in providers" :key="provider.id" class="provider-card">
          <div class="provider-main">
            <div class="provider-title-row">
              <span class="provider-name">{{ provider.name }}</span>
              <t-tag v-if="provider.is_default" theme="primary" variant="light" size="small">默认</t-tag>
              <t-tag variant="outline" size="small">{{ providerLabel(provider.provider) }}</t-tag>
            </div>
            <p v-if="provider.description" class="provider-desc">{{ provider.description }}</p>
            <p class="provider-meta">
              Key：{{ provider.credentials?.api_key?.configured ? '已配置' : '未配置' }}
            </p>
          </div>
          <div class="provider-actions">
            <t-button size="small" variant="text" :loading="testingId === provider.id" @click="testProvider(provider)">
              测试连接
            </t-button>
            <t-button size="small" variant="text" @click="openEdit(provider)">编辑</t-button>
            <t-button
              v-if="!provider.is_default"
              size="small"
              variant="text"
              @click="setDefault(provider)"
            >
              设为默认
            </t-button>
            <t-button size="small" variant="text" theme="danger" @click="removeProvider(provider)">
              删除
            </t-button>
          </div>
        </article>
      </div>
    </t-loading>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑联网搜索配置' : '新建联网搜索配置'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      @confirm="save"
    >
      <div class="form">
        <label class="field">
          <span class="field-label">名称</span>
          <t-input v-model="form.name" placeholder="例如：DeepSeek 搜索" />
        </label>
        <label class="field">
          <span class="field-label">供应商</span>
          <t-select v-model="form.provider" :disabled="editing">
            <t-option value="deepseek" label="DeepSeek" />
            <t-option value="duckduckgo" label="DuckDuckGo" />
          </t-select>
        </label>
        <label class="field">
          <span class="field-label">描述</span>
          <t-input v-model="form.description" placeholder="可选" />
        </label>

        <div v-if="!editing" class="field">
          <span class="field-label">API Key</span>
          <t-input
            v-model="form.apiKey"
            type="password"
            :placeholder="form.provider === 'deepseek' ? 'DeepSeek API Key' : 'DuckDuckGo 可留空'"
          />
        </div>

        <div v-else class="field">
          <span class="field-label">API Key</span>
          <t-radio-group v-model="form.keyAction" variant="default-filled">
            <t-radio-button value="keep">保留</t-radio-button>
            <t-radio-button value="replace">更新</t-radio-button>
            <t-radio-button value="clear" :disabled="!editingKeyConfigured">清除</t-radio-button>
          </t-radio-group>
          <t-input
            v-if="form.keyAction === 'replace'"
            v-model="form.apiKey"
            type="password"
            placeholder="输入新的 API Key"
          />
          <p v-if="form.keyAction === 'clear'" class="danger-hint">
            保存后将清除已存 Key，该配置会立即不可用。
          </p>
        </div>

        <label class="field field--row">
          <span class="field-label">设为默认</span>
          <t-switch v-model="form.isDefault" />
        </label>
      </div>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  createWebSearchProvider,
  deleteWebSearchProvider,
  deleteWebSearchProviderCredentialField,
  listWebSearchProviders,
  putWebSearchProviderCredentials,
  testWebSearchProvider,
  updateWebSearchProvider,
  type WebSearchProviderEntity,
} from '@/api/web-search-provider'

const providers = ref<WebSearchProviderEntity[]>([])
const loading = ref(false)
const saving = ref(false)
const testingId = ref('')
const dialogVisible = ref(false)
const editing = ref<WebSearchProviderEntity | null>(null)
const editingKeyConfigured = ref(false)

const form = reactive({
  name: '',
  provider: 'deepseek' as WebSearchProviderEntity['provider'],
  description: '',
  apiKey: '',
  isDefault: false,
  keyAction: 'keep' as 'keep' | 'replace' | 'clear',
})

const providerLabel = (value: string) => (value === 'deepseek' ? 'DeepSeek' : 'DuckDuckGo')

const resetForm = () => {
  form.name = ''
  form.provider = 'deepseek'
  form.description = ''
  form.apiKey = ''
  form.isDefault = false
  form.keyAction = 'keep'
}

const load = async () => {
  loading.value = true
  try {
    const response: any = await listWebSearchProviders()
    providers.value = Array.isArray(response?.data) ? response.data : []
  } catch (e: any) {
    MessagePlugin.error(e?.message || '读取联网搜索配置失败')
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editing.value = null
  editingKeyConfigured.value = false
  resetForm()
  form.isDefault = providers.value.length === 0
  dialogVisible.value = true
}

const openEdit = (provider: WebSearchProviderEntity) => {
  editing.value = provider
  editingKeyConfigured.value = provider.credentials?.api_key?.configured === true
  resetForm()
  form.name = provider.name
  form.provider = provider.provider
  form.description = provider.description || ''
  form.isDefault = provider.is_default === true
  dialogVisible.value = true
}

const save = async () => {
  if (!form.name.trim()) {
    MessagePlugin.warning('请填写配置名称')
    return
  }
  if (!editing.value && form.provider === 'deepseek' && !form.apiKey.trim()) {
    MessagePlugin.warning('DeepSeek 需要填写 API Key')
    return
  }
  if (editing.value && form.keyAction === 'replace' && !form.apiKey.trim()) {
    MessagePlugin.warning('请输入新的 API Key')
    return
  }

  saving.value = true
  try {
    if (!editing.value) {
      await createWebSearchProvider({
        name: form.name.trim(),
        provider: form.provider,
        description: form.description.trim(),
        parameters: { api_key: form.apiKey.trim() },
        is_default: form.isDefault,
      })
    } else {
      await updateWebSearchProvider(editing.value.id!, {
        name: form.name.trim(),
        provider: form.provider,
        description: form.description.trim(),
        is_default: form.isDefault,
      })
      if (form.keyAction === 'clear') {
        await deleteWebSearchProviderCredentialField(editing.value.id!, 'api_key')
      } else if (form.keyAction === 'replace' && form.apiKey.trim()) {
        await putWebSearchProviderCredentials(editing.value.id!, { api_key: form.apiKey.trim() })
      }
    }
    MessagePlugin.success('联网搜索配置已保存')
    dialogVisible.value = false
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || '保存联网搜索配置失败')
  } finally {
    saving.value = false
  }
}

const setDefault = async (provider: WebSearchProviderEntity) => {
  try {
    await updateWebSearchProvider(provider.id!, { is_default: true })
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || '设置默认失败')
  }
}

const testProvider = async (provider: WebSearchProviderEntity) => {
  testingId.value = provider.id || ''
  try {
    const response: any = await testWebSearchProvider(provider.id)
    const count = response?.data?.results ?? 0
    MessagePlugin.success(`连接正常，返回 ${count} 条结果`)
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || '连接测试失败')
  } finally {
    testingId.value = ''
  }
}

const removeProvider = (provider: WebSearchProviderEntity) => {
  const dialog = DialogPlugin.confirm({
    header: '删除联网搜索配置',
    body: `确定删除「${provider.name}」？被 Agent 引用时无法删除。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteWebSearchProvider(provider.id!)
        MessagePlugin.success('配置已删除')
        dialog.hide()
        await load()
      } catch (e: any) {
        MessagePlugin.error(e?.error?.message || e?.message || '删除失败')
      }
    },
  })
}

onMounted(load)
</script>

<style scoped>
.block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.block__header h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.block__header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.empty {
  padding: 16px;
  border: 1px dashed var(--td-component-border);
  border-radius: 8px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.provider-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.provider-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
}

.provider-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.provider-name {
  font-size: 14px;
  font-weight: 600;
}

.provider-desc,
.provider-meta {
  margin: 6px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
}

.provider-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field--row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.field-label {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.danger-hint {
  margin: 0;
  color: var(--td-error-color);
  font-size: 12px;
}
</style>
