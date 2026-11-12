<template>
  <div class="personal-integration">
    <section class="block">
      <header class="block__header">
        <div>
          <h3>MCP 只读密钥</h3>
          <p>供外部 MCP 客户端以只读权限访问本机知识库。个人版仅保留一把密钥。</p>
        </div>
        <t-button v-if="!key" theme="primary" :loading="creating" @click="handleCreate">生成密钥</t-button>
        <div v-else class="actions">
          <t-button variant="outline" theme="default" :loading="resetting" @click="handleReset">重置</t-button>
          <t-button variant="outline" theme="danger" :loading="deleting" @click="handleDelete">删除</t-button>
        </div>
      </header>

      <t-loading :loading="loading">
        <div v-if="key" class="key-card">
          <div class="key-row">
            <span class="key-label">名称</span>
            <span>{{ key.name }}</span>
          </div>
          <div class="key-row">
            <span class="key-label">密钥</span>
            <code>{{ key.masked_key }}</code>
          </div>
          <div class="key-row">
            <span class="key-label">创建时间</span>
            <span>{{ formatDate(key.created_at) }}</span>
          </div>
          <div class="key-row">
            <span class="key-label">最近使用</span>
            <span>{{ key.last_used_at ? formatDate(key.last_used_at) : '从未使用' }}</span>
          </div>
        </div>
        <div v-else class="empty">尚未生成密钥。生成后请立即保存，明文只显示一次。</div>
      </t-loading>
    </section>

    <section class="block">
      <header class="block__header">
        <div>
          <h3>调用方式</h3>
          <p>MCP 客户端使用请求头 <code>X-API-Key</code> 访问；该密钥只具备检索、问答与会话读取权限。</p>
        </div>
      </header>
      <pre class="snippet">X-API-Key: {{ key ? '&lt;你的密钥&gt;' : '&lt;先生成密钥&gt;' }}</pre>
      <p class="hint">MCP 服务地址以 <code>mcp/</code> 实际部署为准；管理接口不接受该密钥，密钥本身也只能在登录后管理。</p>
    </section>

    <WebSearchProviderSettings />

    <t-dialog
      v-model:visible="tokenVisible"
      header="请立即保存密钥"
      :footer="false"
      width="560px"
    >
      <p class="token-warning">关闭后将无法再次查看明文，只能重置生成新密钥。</p>
      <div class="token-box">
        <code>{{ issuedToken }}</code>
        <t-button size="small" variant="outline" @click="copyToken">复制</t-button>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  createPersonalAPIKey,
  deletePersonalAPIKey,
  getPersonalAPIKey,
  resetPersonalAPIKey,
  type PersonalAPIKey,
} from '@/api/settings'
import WebSearchProviderSettings from './WebSearchProviderSettings.vue'

const loading = ref(false)
const creating = ref(false)
const resetting = ref(false)
const deleting = ref(false)
const key = ref<PersonalAPIKey | null>(null)
const issuedToken = ref('')
const tokenVisible = ref(false)

const formatDate = (value?: string) => (value ? new Date(value).toLocaleString() : '—')

const refresh = async () => {
  loading.value = true
  try {
    const response = await getPersonalAPIKey()
    key.value = response.success && response.data?.exists ? response.data.key ?? null : null
  } catch {
    MessagePlugin.error('读取密钥失败')
  } finally {
    loading.value = false
  }
}

const showToken = (token: string) => {
  issuedToken.value = token
  tokenVisible.value = true
}

const handleCreate = async () => {
  creating.value = true
  try {
    const response = await createPersonalAPIKey()
    if (response.success && response.data) {
      key.value = response.data
      showToken(response.data.token)
      MessagePlugin.success('密钥已生成')
    } else {
      MessagePlugin.error('生成密钥失败')
    }
  } catch {
    MessagePlugin.error('生成密钥失败')
  } finally {
    creating.value = false
  }
}

const handleReset = () => {
  const dialog = DialogPlugin.confirm({
    header: '重置密钥',
    body: '旧密钥会立即失效，使用旧密钥的 MCP 客户端需要更新配置。',
    confirmBtn: { content: '重置', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      resetting.value = true
      try {
        const response = await resetPersonalAPIKey()
        if (response.success && response.data) {
          key.value = response.data
          showToken(response.data.token)
          MessagePlugin.success('密钥已重置')
        } else {
          MessagePlugin.error('重置密钥失败')
        }
      } catch {
        MessagePlugin.error('重置密钥失败')
      } finally {
        resetting.value = false
      }
    },
  })
}

const handleDelete = () => {
  const dialog = DialogPlugin.confirm({
    header: '删除密钥',
    body: '删除后外部 MCP 客户端将无法访问，需重新生成密钥。',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      deleting.value = true
      try {
        await deletePersonalAPIKey()
        key.value = null
        MessagePlugin.success('密钥已删除')
      } catch {
        MessagePlugin.error('删除密钥失败')
      } finally {
        deleting.value = false
      }
    },
  })
}

const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(issuedToken.value)
    MessagePlugin.success('已复制')
  } catch {
    MessagePlugin.error('复制失败，请手动选择文本')
  }
}

onMounted(refresh)
</script>

<style lang="less" scoped>
.personal-integration {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.block {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  padding: 20px;
  background: var(--td-bg-color-container);
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;

  h3 {
    margin: 0 0 6px;
    font-size: 16px;
    font-weight: 600;
  }

  p {
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    color: var(--td-text-color-secondary);
  }
}

.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.key-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;

  code {
    font-family: var(--td-font-family-mono, monospace);
  }
}

.key-label {
  width: 72px;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.empty {
  padding: 20px;
  text-align: center;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  border: 1px dashed var(--td-component-stroke);
  border-radius: 8px;
}

.snippet {
  margin: 0;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
  font-family: var(--td-font-family-mono, monospace);
  font-size: 13px;
  overflow-x: auto;
}

.hint {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.token-warning {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--td-warning-color);
}

.token-box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);

  code {
    flex: 1;
    word-break: break-all;
    font-family: var(--td-font-family-mono, monospace);
    font-size: 13px;
  }
}
</style>
