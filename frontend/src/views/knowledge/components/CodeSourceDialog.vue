<template>
  <t-dialog
    :visible="visible"
    header="添加代码项目"
    width="620px"
    :confirm-btn="{ content: '开始同步', loading: syncing }"
    @confirm="handleSync"
    @update:visible="(value: boolean) => emit('update:visible', value)"
  >
    <div class="code-source-form">
      <t-form label-width="88px">
        <t-form-item label="来源">
          <t-radio-group v-model="form.kind">
            <t-radio value="git">Git 仓库</t-radio>
            <t-radio value="local">本地目录</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="form.kind === 'git'" label="仓库地址">
          <t-input v-model="form.url" placeholder="https://github.com/Forward-back/Forward-Blog" />
        </t-form-item>
        <t-form-item v-if="form.kind === 'git'" label="分支">
          <t-input v-model="form.branch" placeholder="main" />
        </t-form-item>
        <t-form-item v-else label="本地路径">
          <t-input v-model="form.path" placeholder="/path/to/project（容器内路径）" />
        </t-form-item>
        <t-form-item label="忽略规则">
          <t-textarea
            v-model="ignoreText"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="每行一个路径或 glob，例如 docs/drafts 或 *.min.js"
          />
        </t-form-item>
      </t-form>
      <p class="hint">代码与文本文件由 App 扫描并按函数/类或标题分块，RAG 只负责索引与检索。默认跳过依赖目录、二进制、密钥与大文件。</p>
      <div v-if="result" class="result">
        已同步 {{ result.files }} 个文件、{{ result.chunks }} 个分块，跳过 {{ result.skipped }}，删除 {{ result.deleted }}。
        <span v-if="result.commit"> commit {{ result.commit.slice(0, 12) }}</span>
      </div>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { syncCodeSource, type CodeSourceSyncResult } from '@/api/knowledge-base'

const props = defineProps<{ visible: boolean; kbId: string }>()
const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
  (event: 'synced'): void
}>()

const syncing = ref(false)
const ignoreText = ref('')
const result = ref<CodeSourceSyncResult | null>(null)
const form = reactive({
  kind: 'git' as 'git' | 'local',
  url: '',
  branch: 'main',
  path: '',
})

async function handleSync() {
  if (!props.kbId) return
  if (form.kind === 'git' && !form.url.trim()) {
    MessagePlugin.warning('请填写仓库地址')
    return
  }
  if (form.kind === 'local' && !form.path.trim()) {
    MessagePlugin.warning('请填写本地路径')
    return
  }
  syncing.value = true
  try {
    const response = await syncCodeSource(props.kbId, {
      kind: form.kind,
      url: form.url.trim(),
      branch: form.branch.trim() || 'main',
      path: form.path.trim(),
      ignore: ignoreText.value.split('\n').map((line) => line.trim()).filter(Boolean),
    })
    if (!response.success || !response.data) throw new Error('同步失败')
    result.value = response.data
    MessagePlugin.success('代码项目已同步')
    emit('synced')
  } catch (error: any) {
    MessagePlugin.error(error?.message || '同步代码项目失败')
  } finally {
    syncing.value = false
  }
}
</script>

<style scoped>
.code-source-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.hint {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
.result {
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--td-success-color-light);
  color: var(--td-text-color-primary);
  font-size: 13px;
}
</style>
