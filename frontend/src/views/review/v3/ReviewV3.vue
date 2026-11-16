<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createReviewGroup,
  deleteReviewGroup,
  getActiveReviewSession,
  getReviewQueue,
  listReviewGroups,
  updateReviewGroup,
  type ReviewGroup,
  type ReviewGroupNode,
  type ReviewQueue,
  type ReviewSessionState,
} from '@/api/review'
import ReviewGroupRail from './ReviewGroupRail.vue'
import ReviewTodayView from './ReviewTodayView.vue'
import type { ReviewFlatGroup } from './types'

const { t } = useI18n()

const activeView = ref<'today' | 'cards' | 'stats'>('today')
const groups = ref<ReviewGroupNode[]>([])
const flatGroups = ref<ReviewFlatGroup[]>([])
const selectedGroupId = ref('')
const queue = ref<ReviewQueue | null>(null)
const activeSession = ref<ReviewSessionState | null>(null)
const loading = ref(true)
const queueLoading = ref(false)
const error = ref('')
let queueRequestSeq = 0

const groupDialogVisible = ref(false)
const groupDialogMode = ref<'create' | 'rename'>('create')
const groupSaving = ref(false)
const groupForm = reactive({ name: '' })
const groupParentId = ref('')
const groupTarget = ref<ReviewGroup | null>(null)
const groupDeleteVisible = ref(false)
const groupDeleting = ref(false)
const groupDeleteTarget = ref<ReviewGroup | null>(null)
const groupDeleteCards = ref(false)

function flattenGroups(nodes: ReviewGroupNode[], depth = 0): ReviewFlatGroup[] {
  return nodes.flatMap(node => [
    {
      group: node.group,
      depth,
      card_count: node.card_count || 0,
      due_count: node.due_count || 0,
    },
    ...flattenGroups(node.children || [], depth + 1),
  ])
}

const selectedGroupName = computed(() => {
  if (!selectedGroupId.value) return ''
  return flatGroups.value.find(item => item.group.id === selectedGroupId.value)?.group.name || ''
})

const allCardCount = computed(() =>
  groups.value.reduce((total, node) => total + (node.card_count || 0), 0),
)
const allDueCount = computed(() =>
  groups.value.reduce((total, node) => total + (node.due_count || 0), 0),
)

const groupOptions = computed(() => [
  { label: t('review.v3.allGroups'), value: '' },
  ...flatGroups.value.map(item => ({
    label: `${'　'.repeat(item.depth)}${item.group.name}`,
    value: item.group.id,
  })),
])

async function loadGroups() {
  const response = await listReviewGroups()
  groups.value = response.data || []
  flatGroups.value = flattenGroups(groups.value)
}

async function loadQueue() {
  const seq = ++queueRequestSeq
  queueLoading.value = true
  try {
    const response = await getReviewQueue({
      group_id: selectedGroupId.value || undefined,
    })
    if (seq !== queueRequestSeq) return
    queue.value = response.data || null
    error.value = ''
  } catch (err: any) {
    if (seq !== queueRequestSeq) return
    error.value = err?.message || t('review.loadFailed')
  } finally {
    if (seq === queueRequestSeq) queueLoading.value = false
  }
}

async function loadActiveSession() {
  try {
    const response = await getActiveReviewSession()
    activeSession.value = response.data || null
  } catch {
    activeSession.value = null
  }
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadGroups(), loadQueue(), loadActiveSession()])
  } catch (err: any) {
    error.value = err?.message || t('review.loadFailed')
  } finally {
    loading.value = false
  }
}

function selectGroup(groupId: string) {
  selectedGroupId.value = groupId
}

function handleStart() {
  MessagePlugin.info(t('review.v3.sessionPending'))
}

function openCreateGroup(parentId: string) {
  groupDialogMode.value = 'create'
  groupParentId.value = parentId
  groupTarget.value = null
  groupForm.name = ''
  groupDialogVisible.value = true
}

function openRenameGroup(group: ReviewGroup) {
  groupDialogMode.value = 'rename'
  groupTarget.value = group
  groupParentId.value = group.parent_id
  groupForm.name = group.name
  groupDialogVisible.value = true
}

function openDeleteGroup(group: ReviewGroup) {
  groupDeleteTarget.value = group
  groupDeleteCards.value = false
  groupDeleteVisible.value = true
}

async function saveGroup() {
  const name = groupForm.name.trim()
  if (!name) {
    MessagePlugin.warning(t('review.groupNameRequired'))
    return
  }
  groupSaving.value = true
  try {
    if (groupDialogMode.value === 'create') {
      const response = await createReviewGroup({ parent_id: groupParentId.value || undefined, name })
      MessagePlugin.success(t('review.groupCreated'))
      groupDialogVisible.value = false
      await loadGroups()
      if (response.data?.id) selectedGroupId.value = response.data.id
    } else if (groupTarget.value) {
      await updateReviewGroup(groupTarget.value.id, { name })
      MessagePlugin.success(t('review.groupUpdated'))
      groupDialogVisible.value = false
      await loadGroups()
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.groupFailed'))
  } finally {
    groupSaving.value = false
  }
}

async function confirmDeleteGroup() {
  const target = groupDeleteTarget.value
  if (!target) return
  groupDeleting.value = true
  try {
    await deleteReviewGroup(target.id, groupDeleteCards.value)
    MessagePlugin.success(t('review.groupDeleted'))
    groupDeleteVisible.value = false
    if (selectedGroupId.value === target.id) selectedGroupId.value = ''
    await loadGroups()
    await loadQueue()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.groupFailed'))
  } finally {
    groupDeleting.value = false
  }
}

watch(selectedGroupId, () => {
  void loadQueue()
})

onMounted(() => {
  void refresh()
})
</script>

<template>
  <div class="review-v3">
    <div class="review-v3__inner">
      <header class="review-v3__head">
        <div>
          <div class="review-v3__title-row">
            <h1>{{ t('review.title') }}</h1>
            <t-tag theme="primary" variant="light" size="small">{{ t('review.v3.preview') }}</t-tag>
          </div>
          <p>{{ t('review.subtitle') }}</p>
        </div>
      </header>

      <t-tabs v-model="activeView" class="review-v3__tabs">
        <t-tab-panel value="today" :label="t('review.v3.todayTab')">
          <div class="review-v3__today">
            <ReviewGroupRail
              :groups="flatGroups"
              :selected-group-id="selectedGroupId"
              :all-card-count="allCardCount"
              :all-due-count="allDueCount"
              @select="selectGroup"
              @create="openCreateGroup"
              @rename="openRenameGroup"
              @delete="openDeleteGroup"
            />
            <main class="review-v3__main">
              <ReviewTodayView
                :queue="queue"
                :active-session="activeSession"
                :selected-group-name="selectedGroupName"
                :selected-group-id="selectedGroupId"
                :group-options="groupOptions"
                :loading="loading || queueLoading"
                :error="error"
                @start="handleStart"
                @retry="refresh"
                @update:group-id="selectGroup"
              />
            </main>
          </div>
        </t-tab-panel>

        <t-tab-panel value="cards" :label="t('review.v3.cardsTab')">
          <div class="review-v3__placeholder">
            <t-empty :description="t('review.v3.cardsPending')" />
            <p class="review-v3__note">{{ t('review.v3.emptyPreview') }}</p>
          </div>
        </t-tab-panel>

        <t-tab-panel value="stats" :label="t('review.v3.statsTab')">
          <div class="review-v3__placeholder">
            <t-empty :description="t('review.v3.statsPending')" />
            <p class="review-v3__note">{{ t('review.v3.emptyPreview') }}</p>
          </div>
        </t-tab-panel>
      </t-tabs>
    </div>

    <t-dialog
      v-model:visible="groupDialogVisible"
      :header="groupDialogMode === 'create' ? t('review.newGroup') : t('review.renameGroup')"
      :confirm-btn="{ content: t('review.save'), loading: groupSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="420px"
      @confirm="saveGroup"
    >
      <t-input
        v-model="groupForm.name"
        :placeholder="t('review.groupNamePlaceholder')"
        :maxlength="200"
        @enter="saveGroup"
      />
    </t-dialog>

    <t-dialog
      v-model:visible="groupDeleteVisible"
      :header="t('review.deleteGroup')"
      :confirm-btn="{ content: t('review.confirm'), theme: 'danger', loading: groupDeleting }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="440px"
      @confirm="confirmDeleteGroup"
    >
      <p class="review-v3__dialog-text">
        {{ t('review.groupDeleteConfirm', { name: groupDeleteTarget?.name || '' }) }}
      </p>
      <t-checkbox v-model="groupDeleteCards">{{ t('review.groupDeleteWithCards') }}</t-checkbox>
    </t-dialog>
  </div>
</template>

<style lang="less" scoped>
.review-v3,
.review-v3 *,
.review-v3 *::before,
.review-v3 *::after {
  box-sizing: border-box;
}

.review-v3 {
  height: 100%;
  overflow-y: auto;
  background: var(--td-bg-color-page);
}

.review-v3__inner {
  max-width: 1440px;
  margin: 0 auto;
  padding: 24px 28px 48px;
}

.review-v3__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;

  h1 {
    margin: 0;
    font-size: 22px;
    line-height: 30px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  p {
    margin: 4px 0 0;
    font-size: 13px;
    line-height: 20px;
    color: var(--td-text-color-secondary);
  }
}

.review-v3__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.review-v3__tabs {
  margin-top: 14px;

  :deep(.t-tabs__content) {
    padding: 16px 0 0;
  }
}

.review-v3__today {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.review-v3__main {
  flex: 1;
  min-width: 0;
}

.review-v3__placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 360px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.review-v3__note {
  margin: 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.review-v3__dialog-text {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 21px;
  color: var(--td-text-color-secondary);
}

@media (max-width: 760px) {
  .review-v3__inner {
    padding: 16px 14px 32px;
  }

  .review-v3__today {
    display: block;
  }
}
</style>
