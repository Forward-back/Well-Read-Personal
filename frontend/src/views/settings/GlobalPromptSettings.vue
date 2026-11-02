<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>全局系统提示词</h3>
        <p>作用于问答与精读助手的全局指令。技能可在其范围内补充风格与任务要求，但不能覆盖引用与不编造规则。</p>
      </div>
      <div class="actions">
        <t-button variant="outline" theme="default" :loading="loading" @click="handleReset">恢复默认</t-button>
        <t-button theme="primary" :loading="saving" :disabled="loading" @click="handleSave">保存</t-button>
      </div>
    </header>

    <t-loading :loading="loading">
      <t-textarea
        v-model="prompt"
        class="prompt-editor"
        :autosize="{ minRows: 10, maxRows: 24 }"
        placeholder="加载内置提示词中…"
      />
      <p class="hint">
        留空即使用内置提示词；保存后对新会话与后续问答生效。
        <span v-if="isCustom" class="custom-badge">当前为自定义提示词</span>
      </p>
    </t-loading>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { getPromptTemplates, saveAgentSystemPrompt, type PromptTemplate } from '@/api/settings'

const loading = ref(false)
const saving = ref(false)
const prompt = ref('')
const isCustom = ref(false)

function displayContent(templates?: PromptTemplate[]): string {
  if (!templates || templates.length === 0) return ''
  return (templates.find((item) => item.default) ?? templates[0]).content || ''
}

async function load() {
  loading.value = true
  try {
    const response = await getPromptTemplates()
    const templates = response?.data?.agent_system_prompt
    prompt.value = displayContent(templates)
    isCustom.value = templates?.length === 1 && templates[0]?.id === 'personal_global_override'
  } catch (error: any) {
    MessagePlugin.error(error?.message || '加载提示词失败')
  } finally {
    loading.value = false
  }
}

async function persist(value: string) {
  saving.value = true
  try {
    const response = await saveAgentSystemPrompt(value)
    isCustom.value = Boolean(response?.data?.is_custom)
    if (!value.trim()) {
      await load()
    } else {
      prompt.value = response?.data?.agent_system_prompt || value
    }
    MessagePlugin.success(value.trim() ? '提示词已保存' : '已恢复默认提示词')
  } catch (error: any) {
    MessagePlugin.error(error?.message || '保存提示词失败')
  } finally {
    saving.value = false
  }
}

function handleSave() {
  void persist(prompt.value)
}

function handleReset() {
  const dialog = DialogPlugin.confirm({
    header: '恢复默认提示词',
    body: '将丢弃当前自定义内容并恢复内置提示词，是否继续？',
    onConfirm: async () => {
      dialog.destroy()
      await persist('')
    },
    onCancel: () => dialog.destroy(),
  })
}

onMounted(load)
</script>

<style scoped>
.block {
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  padding: 20px;
  background: var(--td-bg-color-container);
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.block__header h3 {
  margin: 0 0 6px;
  font-size: 16px;
}

.block__header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.prompt-editor {
  width: 100%;
}

.hint {
  margin: 8px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.custom-badge {
  margin-left: 8px;
  color: var(--td-brand-color);
}
</style>
