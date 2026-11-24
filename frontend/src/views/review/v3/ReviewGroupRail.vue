<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewGroup } from '@/api/review'
import type { ReviewFlatGroup } from './types'
import ReviewIcon from './ReviewIcon.vue'

const props = defineProps<{
  groups: ReviewFlatGroup[]
  selectedGroupId: string
  allCardCount: number
  allDueCount: number
}>()

const emit = defineEmits<{
  select: [groupId: string]
  create: [parentId: string]
  rename: [group: ReviewGroup]
  delete: [group: ReviewGroup]
}>()

const { t } = useI18n()
const collapsed = ref<Set<string>>(new Set())

const groupMenuOptions = computed(() => [
  { content: t('review.newChildGroup'), value: 'create' },
  { content: t('review.renameGroup'), value: 'rename' },
  { content: t('review.deleteGroup'), value: 'delete', theme: 'error' },
])
const rootMenuOptions = computed(() => [
  { content: t('review.newGroup'), value: 'create' },
])

function groupMenuOptionsFor(group: ReviewGroup) {
  if (group.is_default) return groupMenuOptions.value.slice(0, 1)
  return groupMenuOptions.value
}

function handleGroupMenu(group: ReviewGroup, value: string | number) {
  if (value === 'create') emit('create', group.id)
  else if (value === 'rename') emit('rename', group)
  else if (value === 'delete') emit('delete', group)
}

const childParentIDs = computed(() => {
  const parents = new Set<string>()
  props.groups.forEach(item => {
    if (item.group.parent_id) parents.add(item.group.parent_id)
  })
  return parents
})

const parentByID = computed(() => {
  const map = new Map<string, string>()
  props.groups.forEach(item => map.set(item.group.id, item.group.parent_id))
  return map
})

function hasChildren(groupId: string) {
  return childParentIDs.value.has(groupId)
}

function isVisible(group: ReviewGroup) {
  let cursor = group.parent_id
  while (cursor) {
    if (collapsed.value.has(cursor)) return false
    cursor = parentByID.value.get(cursor) || ''
  }
  return true
}

const visibleGroups = computed(() => props.groups.filter(item => isVisible(item.group)))

function toggle(groupId: string) {
  const next = new Set(collapsed.value)
  if (next.has(groupId)) next.delete(groupId)
  else next.add(groupId)
  collapsed.value = next
}

function select(groupId: string) {
  emit('select', groupId)
}
</script>

<template>
  <aside class="group-rail" data-testid="review-v3-group-rail">
    <div class="group-rail__label">{{ t('review.v3.groupTree') }}</div>
    <div
      class="group-row group-row--root"
      :class="{ 'group-row--active': selectedGroupId === '' }"
      role="treeitem"
      :aria-selected="selectedGroupId === ''"
      tabindex="0"
      @click="select('')"
      @keydown.enter="select('')"
      @keydown.space.prevent="select('')"
    >
      <ReviewIcon name="folder" :size="16" :tilt="-1" class="group-row__icon" />
      <span class="group-row__name">{{ t('review.v3.allGroups') }}</span>
      <span class="group-row__counts">
        <span class="group-row__cards">{{ allCardCount }}</span>
        <span v-if="allDueCount" class="group-row__due" :title="t('review.v3.groupDue', { count: allDueCount })">
          {{ allDueCount }}
        </span>
      </span>
      <t-dropdown
        :options="rootMenuOptions"
        placement="bottom-right"
        attach="body"
        trigger="click"
        @click="(data: any) => data.value === 'create' && emit('create', '')"
      >
        <button
          type="button"
          class="group-row__more"
          :title="t('review.groupMoreActions')"
          data-testid="review-v3-group-menu-root"
          @click.stop
        >
          <ReviewIcon name="more" :size="15" />
        </button>
      </t-dropdown>
    </div>

    <div class="group-rail__tree">
      <div
        v-for="item in visibleGroups"
        :key="item.group.id"
        class="group-row"
        :class="{ 'group-row--active': selectedGroupId === item.group.id }"
        :style="{ paddingLeft: `${8 + item.depth * 14}px` }"
        role="treeitem"
        :aria-selected="selectedGroupId === item.group.id"
        tabindex="0"
        @click="select(item.group.id)"
        @keydown.enter="select(item.group.id)"
        @keydown.space.prevent="select(item.group.id)"
      >
        <button
          v-if="hasChildren(item.group.id)"
          type="button"
          class="group-row__chevron"
          :class="{ 'group-row__chevron--collapsed': collapsed.has(item.group.id) }"
          :aria-label="item.group.name"
          @click.stop="toggle(item.group.id)"
        >
          <ReviewIcon :name="collapsed.has(item.group.id) ? 'chevron-right' : 'chevron-down'" :size="12" />
        </button>
        <span v-else class="group-row__chevron-spacer" />
        <ReviewIcon
          :name="hasChildren(item.group.id) ? 'folder' : 'file'"
          :size="16"
          :tilt="item.depth % 2 === 0 ? 1 : -1"
          class="group-row__icon"
        />
        <span class="group-row__name" :title="item.group.name">{{ item.group.name }}</span>
        <span class="group-row__counts">
          <span class="group-row__cards">{{ item.card_count }}</span>
          <span v-if="item.due_count" class="group-row__due" :title="t('review.v3.groupDue', { count: item.due_count })">
            {{ item.due_count }}
          </span>
        </span>
        <t-dropdown
          :options="groupMenuOptionsFor(item.group)"
          placement="bottom-right"
          attach="body"
          trigger="click"
          @click="(data: any) => handleGroupMenu(item.group, data.value)"
        >
          <button
            type="button"
            class="group-row__more"
            :title="t('review.groupMoreActions')"
            data-testid="review-v3-group-menu"
            @click.stop
          >
            <ReviewIcon name="more" :size="15" />
          </button>
        </t-dropdown>
      </div>
    </div>
  </aside>
</template>

<style lang="less" scoped>
.group-rail,
.group-rail *,
.group-rail *::before,
.group-rail *::after {
  box-sizing: border-box;
}

.group-rail {
  width: 100%;
  flex: none;
  min-width: 0;
  padding: 0;
  background: transparent;
  border: 0;
  border-radius: 0;
  align-self: stretch;
  position: static;
  max-height: none;
  min-height: 0;
  overflow-y: visible;
}

.group-rail__label {
  padding: 0 8px 10px;
  font-size: 19px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--rvn-ink);
}

.group-rail__tree {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.group-row {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-height: 32px;
  padding: 5px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--rvn-ink);
  font-size: 14px;
  text-align: left;
  cursor: pointer;
  outline: none;
  transition: background 0.18s ease, color 0.18s ease;

  &:hover {
    background: var(--rvn-surface);
  }

  &:hover .group-row__more,
  &:focus-within .group-row__more {
    opacity: 1;
  }

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: -2px;
  }
}

.group-row--active {
  background: var(--rvn-surface-deep);

  .group-row__name,
  .group-row__icon,
  .group-row__cards {
    color: var(--rvn-ink);
  }
}

.group-row--root {
  margin-bottom: 4px;
  font-weight: 500;
}

.group-row__chevron,
.group-row__chevron-spacer {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  flex: 0 0 14px;
}

.group-row__chevron {
  padding: 0;
  color: var(--rvn-muted);
  cursor: pointer;
  background: transparent;
  border: 0;

  &:hover {
    color: var(--rvn-ink);
  }
}

.group-row__chevron--collapsed {
  transform: none;
}

.group-row__icon {
  flex: 0 0 auto;
  color: var(--rvn-ink-soft);
}

.group-row__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.group-row__counts {
  display: inline-flex;
  flex: 0 0 auto;
  gap: 5px;
  align-items: center;
}

.group-row__cards {
  min-width: 14px;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  color: var(--rvn-muted);
  text-align: right;
}

.group-row__due {
  min-width: 16px;
  padding: 1px 7px;
  font-size: 14px;
  line-height: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--rvn-red-deep);
  text-align: center;
  background: var(--rvn-red-soft);
  border-radius: 999px;
}

.group-row__more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 18px;
  width: 18px;
  height: 18px;
  padding: 0;
  color: var(--rvn-muted);
  cursor: pointer;
  background: transparent;
  border: 0;
  border-radius: 4px;
  opacity: 0;
  transition: opacity 0.12s ease, background 0.12s ease;

  &:hover {
    background: var(--rvn-surface-deep);
    color: var(--rvn-ink);
  }

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: 1px;
    opacity: 1;
  }
}

@media (max-width: 760px) {
  .group-rail {
    display: none;
  }
}
</style>
