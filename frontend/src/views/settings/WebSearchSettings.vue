<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>联网搜索</h3>
        <p>直连模式可显式开启联网搜索；网页来源由后端根据搜索结果生成，不依赖模型编造 URL。</p>
      </div>
    </header>

    <div class="provider-list">
      <button
        v-for="provider in providers"
        :key="provider.value"
        type="button"
        class="provider-card"
        :class="{ active: form.provider === provider.value }"
        @click="form.provider = provider.value"
      >
        <span class="provider-title">{{ provider.label }}</span>
        <span class="provider-desc">{{ provider.description }}</span>
      </button>
    </div>

    <div v-if="form.provider === 'deepseek'" class="key-row">
      <t-input
        v-model="apiKeyInput"
        type="password"
        clearable
        class="key-input"
        :placeholder="keyConfigured ? '已保存 Key，留空表示保留' : '留空则复用直连模型 Key'"
      />
      <span class="key-status">{{ keyConfigured ? 'Key 已配置' : '未单独配置 Key' }}</span>
    </div>

    <div class="action-row">
      <span class="current">当前搜索引擎：{{ currentProviderLabel }}</span>
      <t-button theme="primary" :loading="saving" :disabled="saving || loading" @click="save">保存</t-button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getTenantWebSearchConfig,
  updateTenantWebSearchConfig,
  type WebSearchConfig,
} from '@/api/web-search'

const providers = [
  { value: 'deepseek', label: 'DeepSeek', description: '使用 DeepSeek Search API；留空 Key 时复用直连模型凭据。' },
  { value: 'duckduckgo', label: 'DuckDuckGo', description: '无需 API Key；受网络环境影响，部分部署可能不可达。' },
] as const

const form = reactive({
  provider: 'deepseek',
  maxResults: 10,
  includeDate: false,
  compressionMethod: 'none',
  blacklist: [] as string[],
})

const apiKeyInput = ref('')
const keyConfigured = ref(false)
const loading = ref(false)
const saving = ref(false)

const currentProviderLabel = computed(
  () => providers.find((item) => item.value === form.provider)?.label || form.provider,
)

const load = async () => {
  loading.value = true
  try {
    const response = await getTenantWebSearchConfig()
    const config: WebSearchConfig = response?.data || {}
    form.provider = config.provider || 'deepseek'
    form.maxResults = config.max_results || 10
    form.includeDate = config.include_date || false
    form.compressionMethod = config.compression_method || 'none'
    form.blacklist = config.blacklist || []
    keyConfigured.value = config.api_key_configured === true
    apiKeyInput.value = ''
  } catch (e: any) {
    MessagePlugin.error(e?.message || '读取联网搜索配置失败')
  } finally {
    loading.value = false
  }
}

const save = async () => {
  const apiKey = apiKeyInput.value.trim()
  saving.value = true
  try {
    await updateTenantWebSearchConfig({
      provider: form.provider,
      api_key: apiKey,
      max_results: form.maxResults,
      include_date: form.includeDate,
      compression_method: form.compressionMethod,
      blacklist: form.blacklist,
    })
    if (apiKey) {
      keyConfigured.value = true
    }
    apiKeyInput.value = ''
    MessagePlugin.success('联网搜索配置已保存')
  } catch (e: any) {
    MessagePlugin.error(e?.message || '保存联网搜索配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.block {
  display: flex;
  flex-direction: column;
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

.provider-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.provider-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
}

.provider-card:hover {
  border-color: var(--td-brand-color);
}

.provider-card.active {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.provider-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.provider-desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.key-input {
  width: 360px;
}

.key-status,
.current {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>
