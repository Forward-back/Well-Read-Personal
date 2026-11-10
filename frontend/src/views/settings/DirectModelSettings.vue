<template>
  <div class="direct-model-settings">
    <header class="block__header">
      <div>
        <h3>直连模型</h3>
        <p>直连模式不检索知识库，直接调用该模型回答，可另行开启联网搜索。必须选择支持思考模式的问答模型；未配置时输入栏的直连模式不可用。</p>
      </div>
    </header>

    <div class="form-row">
      <t-select
        v-model="selectedModelId"
        :options="options"
        :loading="loading"
        :placeholder="options.length ? '选择直连模型' : '暂无支持思考模式的问答模型'"
        clearable
        class="model-select"
      />
      <t-button theme="primary" :loading="saving" :disabled="!dirty || saving" @click="save">保存</t-button>
    </div>
    <p v-if="currentModelId" class="current">当前直连模型：{{ currentModelName || currentModelId }}</p>
    <p v-else class="current current--empty">当前未配置直连模型</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { listModels, type ModelConfig } from '@/api/model'
import { getDirectConfig, updateDirectConfig } from '@/api/settings'
import { modelSupportsThinking } from '@/utils/thinkingControl'

const loading = ref(false)
const saving = ref(false)
const models = ref<ModelConfig[]>([])
const currentModelId = ref('')
const selectedModelId = ref('')

const thinkingModels = computed(() => models.value.filter((m) => m.id && modelSupportsThinking(m)))
const options = computed(() =>
  thinkingModels.value.map((m) => ({ label: m.display_name || m.name, value: m.id as string })),
)
const currentModelName = computed(() => {
  const hit = models.value.find((m) => m.id === currentModelId.value)
  return hit ? hit.display_name || hit.name : ''
})
const dirty = computed(() => (selectedModelId.value || '') !== currentModelId.value)

const load = async () => {
  loading.value = true
  try {
    const [modelList, config] = await Promise.all([listModels(), getDirectConfig()])
    models.value = modelList || []
    currentModelId.value = config?.data?.direct_model_id || ''
    selectedModelId.value = currentModelId.value
  } catch {
    MessagePlugin.error('读取直连配置失败')
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    await updateDirectConfig(selectedModelId.value || '')
    currentModelId.value = selectedModelId.value || ''
    MessagePlugin.success(selectedModelId.value ? '直连模型已保存' : '已清除直连模型')
  } catch (e: any) {
    MessagePlugin.error(e?.message || '保存直连模型失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style lang="less" scoped>
.direct-model-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block__header {
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

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.model-select {
  width: 360px;
}

.current {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.current--empty {
  color: var(--td-warning-color);
}
</style>
