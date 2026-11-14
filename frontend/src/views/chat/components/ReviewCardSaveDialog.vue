<template>
  <t-dialog :visible="visible" :header="t('review.saveDialogTitle')" :footer="false" width="560px"
    destroy-on-close @close="close">
    <div class="review-save-body">
      <div class="review-save-field">
        <label>{{ t('review.groupName') }}</label>
        <t-select v-model="groupId" :options="groupOptions" :loading="loadingGroups" />
      </div>
      <div class="review-save-field">
        <label>{{ t('review.sourceLabel') }}</label>
        <div v-if="sourceOptions.length" class="review-save-sources">
          <t-checkbox-group v-model="selectedSourceKeys">
            <t-checkbox v-for="source in sourceOptions" :key="source.key" :value="source.key">
              {{ source.label }}
            </t-checkbox>
          </t-checkbox-group>
        </div>
        <p v-else class="review-save-empty">{{ t('review.noSources') }}</p>
      </div>
      <div class="review-save-field">
        <label>{{ t('review.selfTests') }}</label>
        <div class="review-save-candidates">
          <div v-for="(candidate, index) in candidates" :key="index" class="review-save-candidate">
            <div class="review-save-question">{{ index + 1 }}. {{ candidate.question }}</div>
            <div class="review-save-answer">{{ candidate.answer }}</div>
          </div>
        </div>
      </div>
      <div class="review-save-footer">
        <t-button variant="outline" @click="close">{{ t('review.cancel') }}</t-button>
        <t-button theme="primary" :loading="saving" :disabled="!canSave" @click="save">
          {{ t('review.save') }}
        </t-button>
      </div>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import {
  createReviewCard,
  listReviewGroups,
  type ReviewGroupNode,
  type ReviewCardSourceInput,
} from '@/api/review'
import type { ReviewSelfTestCandidate } from '@/utils/reviewSelfTest'

const props = defineProps<{
  visible: boolean
  candidates: ReviewSelfTestCandidate[]
  references: any[]
}>()

const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
  (event: 'saved'): void
}>()

const { t } = useI18n()
const loadingGroups = ref(false)
const saving = ref(false)
const groupId = ref('')
const groupOptions = ref<Array<{ label: string; value: string }>>([])
const selectedSourceKeys = ref<string[]>([])

interface SourceOption {
  key: string
  label: string
  source: ReviewCardSourceInput
}

const sourceOptions = computed<SourceOption[]>(() => {
  const items: SourceOption[] = []
  for (const [index, ref] of (props.references || []).entries()) {
    if (!ref || ref.chunk_type === 'web_search' || !ref.knowledge_id) continue
    const key = `${ref.knowledge_id}:${ref.chunk_id || ref.id || index}`
    items.push({
      key,
      label: ref.knowledge_title || ref.knowledge_id,
      source: {
        source_type: 'knowledge_chunk',
        knowledge_id: ref.knowledge_id,
        chunk_id: ref.chunk_id || ref.id || '',
        excerpt: String(ref.content || '').slice(0, 1000),
        path: ref.path || ref.metadata?.path || '',
        symbol: ref.symbol || ref.metadata?.symbol || '',
      },
    })
  }
  return items
})

const canSave = computed(() =>
  Boolean(groupId.value && props.candidates.length && selectedSourceKeys.value.length && !saving.value),
)

function flatten(nodes: ReviewGroupNode[]): Array<{ label: string; value: string }> {
  return nodes.flatMap(node => [
    { label: node.group.name, value: node.group.id },
    ...flatten(node.children || []),
  ])
}

async function loadGroups() {
  if (groupOptions.value.length) return
  loadingGroups.value = true
  try {
    const response = await listReviewGroups()
    groupOptions.value = flatten(response.data || [])
    if (!groupId.value && groupOptions.value.length) {
      groupId.value = groupOptions.value[0].value
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.loadFailed'))
  } finally {
    loadingGroups.value = false
  }
}

watch(
  () => props.visible,
  visible => {
    if (!visible) return
    void loadGroups()
    selectedSourceKeys.value = sourceOptions.value.slice(0, 1).map(source => source.key)
  },
)

function close() {
  emit('update:visible', false)
}

async function save() {
  if (!canSave.value) return
  const sources = sourceOptions.value
    .filter(source => selectedSourceKeys.value.includes(source.key))
    .map(source => source.source)
  saving.value = true
  let saved = 0
  try {
    for (const candidate of props.candidates) {
      await createReviewCard({
        group_id: groupId.value,
        question: candidate.question,
        answer: candidate.answer,
        sources,
      })
      saved += 1
    }
    MessagePlugin.success(t('review.savedCount', { count: saved }))
    emit('saved')
    close()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.review-save-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.review-save-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.review-save-field > label {
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.review-save-sources {
  max-height: 120px;
  overflow-y: auto;
  padding: 8px 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
}
.review-save-empty {
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
.review-save-candidates {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 280px;
  overflow-y: auto;
}
.review-save-candidate {
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
}
.review-save-question {
  font-weight: 550;
  line-height: 1.5;
}
.review-save-answer {
  margin-top: 6px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}
.review-save-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
