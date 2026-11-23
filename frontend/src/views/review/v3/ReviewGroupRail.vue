<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewGroup } from '@/api/review'
import type { ReviewFlatGroup } from './types'
import bookIcon from '@/assets/icons/static/book.png'
import folderIcon from '@/assets/icons/static/folder.png'
import arrowIcon from '@/assets/icons/static/arrow-right.png'
import moreIcon from '@/assets/icons/static/more.png'

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
    <div class="group-rail__label"><img :src="bookIcon" alt="" />{{ t('review.groups') }}</div>
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
      <img class="group-row__icon group-row__hand" :src="folderIcon" alt="" />
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
          <img class="group-row__hand" :src="moreIcon" alt="" />
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
          <img class="group-row__arrow" :src="arrowIcon" alt="" />
        </button>
        <span v-else class="group-row__chevron-spacer" />
        <img v-if="collapsed.has(item.group.id)" class="group-row__icon group-row__hand" :src="folderIcon" alt="" />
        <img v-else class="group-row__icon group-row__hand" :src="folderIcon" alt="" />
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
            <img class="group-row__hand" :src="moreIcon" alt="" />
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
  width: 200px;
  flex: 0 0 200px;
  min-width: 0;
  padding: 10px 8px;
  background: var(--rvn-panel);
  border: 1px solid var(--rvn-border);
  border-radius: var(--rvn-radius);
  align-self: stretch;
  position: static;
  max-height: none;
  min-height: 100%;
  overflow-y: auto;
}

.group-rail__label {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px 8px;
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-placeholder);

  img {
    width: 18px;
    height: 18px;
    object-fit: contain;
  }
}

.group-rail__tree {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.group-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 30px;
  padding: 4px 8px;
  border: 0;
  border-radius: var(--td-radius-default);
  background: transparent;
  color: var(--td-text-color-primary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  outline: none;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &:hover .group-row__more,
  &:focus-within .group-row__more {
    opacity: 1;
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color-focus);
    outline-offset: -2px;
  }
}

.group-row--active {
  background: var(--rvn-sage-bg);

  .group-row__name,
  .group-row__icon,
  .group-row__cards {
    color: var(--rvn-sage-deep);
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
  border: 0;
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: pointer;
  transition: transform 0.15s ease;

  &:hover {
    color: var(--td-text-color-primary);
  }
}

.group-row__chevron--collapsed {
  transform: rotate(-90deg);
}

.group-row__icon {
  flex: 0 0 auto;
  color: var(--td-text-color-secondary);
}

.group-row__hand {
  width: 15px;
  height: 15px;
  object-fit: contain;
  flex: 0 0 auto;
}

.group-row__arrow {
  width: 12px;
  height: 12px;
  object-fit: contain;
  transition: transform 0.15s ease;
}

.group-row__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-row__counts {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: 0 0 auto;
}

.group-row__cards {
  min-width: 14px;
  text-align: right;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-placeholder);
}

.group-row__due {
  min-width: 16px;
  padding: 0 4px;
  border-radius: var(--td-radius-round);
  background: var(--rvn-sage);
  color: #fff;
  font-size: 10px;
  line-height: 16px;
  font-variant-numeric: tabular-nums;
  text-align: center;
}

.group-row__more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 18px;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  border-radius: var(--td-radius-default);
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease, background 0.12s ease;

  &:hover {
    background: var(--td-bg-color-container-active);
    color: var(--td-text-color-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color-focus);
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
