<template>
  <t-popup
    v-model:visible="open"
    trigger="click"
    placement="top-left"
    :show-arrow="false"
    overlay-class-name="skp-picker-popover"
  >
    <button type="button" class="skp-picker-trigger" :class="{ 'is-empty': !selected }">
      <t-icon name="book" />
      <span class="skp-picker-trigger-label">{{ selected ? selected.name : $t('skillCatalog.noSkill') }}</span>
      <t-icon name="chevron-up" class="skp-picker-caret" />
    </button>

    <template #content>
      <div class="skp-picker">
        <div class="skp-picker-head">
          <div class="skp-picker-title">{{ $t('skillCatalog.pickerTitle') }}</div>
          <label class="skp-picker-search">
            <t-icon name="search" />
            <input v-model="keyword" :placeholder="$t('skillCatalog.pickerSearch')" />
          </label>
        </div>

        <div class="skp-picker-options">
          <div class="skp-picker-opt is-disabled" :title="$t('skillCatalog.autoDesc')">
            <div class="skp-picker-r1">
              <span class="skp-picker-name">{{ $t('skillCatalog.autoMatch') }}</span>
              <span class="skp-picker-auto">{{ $t('skillCatalog.autoDefault') }}</span>
            </div>
            <div class="skp-picker-desc">{{ $t('skillCatalog.autoDesc') }}</div>
          </div>

          <button
            type="button"
            class="skp-picker-opt"
            :class="{ selected: !modelValue }"
            @click="select('')"
          >
            <div class="skp-picker-r1"><span class="skp-picker-name">{{ $t('skillCatalog.noSkill') }}</span></div>
            <div class="skp-picker-desc">{{ $t('skillCatalog.noSkillDesc') }}</div>
          </button>

          <div
            v-for="skill in filteredEnabled"
            :key="skill.id"
            class="skp-picker-opt"
            :class="{ selected: modelValue === skill.id, 'is-expanded': expandedId === skill.id }"
            role="button"
            tabindex="0"
            :aria-expanded="expandedId === skill.id"
            @click="toggleExpand(skill.id)"
            @keydown.enter.prevent="toggleExpand(skill.id)"
            @keydown.space.prevent="toggleExpand(skill.id)"
          >
            <div class="skp-picker-r1">
              <span class="skp-picker-name">{{ skill.name }}</span>
              <span v-if="skill.version" class="skp-picker-tag ver">{{ skill.version }}</span>
              <span class="skp-picker-tag on"><span class="skp-picker-dot" />{{ $t('skillCatalog.enabled') }}</span>
              <t-icon name="chevron-down" class="skp-picker-chevron" />
            </div>
            <div class="skp-picker-desc">{{ skill.description || '—' }}</div>
            <div class="skp-picker-scene">
              <span>{{ $t('skillCatalog.scope', { scope: $t('skillCatalog.scopeQa') }) }}</span>
            </div>
            <div v-if="expandedId === skill.id" class="skp-picker-extra">
              <button
                type="button"
                class="skp-picker-use"
                :disabled="modelValue === skill.id"
                @click.stop="select(skill.id)"
              >
                {{ modelValue === skill.id ? $t('skillCatalog.pickerSelected') : $t('skillCatalog.pickerUse') }}
              </button>
            </div>
          </div>

          <button
            v-for="skill in filteredDisabled"
            :key="skill.id"
            type="button"
            class="skp-picker-opt is-off is-disabled"
            disabled
          >
            <div class="skp-picker-r1">
              <span class="skp-picker-name">{{ skill.name }}</span>
              <span class="skp-picker-tag off">{{ $t('skillCatalog.disabled') }}</span>
            </div>
            <div class="skp-picker-desc">{{ $t('skillCatalog.pickerDisabledHint') }}</div>
          </button>

          <div v-if="!filteredEnabled.length && !filteredDisabled.length" class="skp-picker-empty">
            {{ keyword.trim() ? $t('skillCatalog.pickerEmpty') : $t('skillCatalog.pickerNoSkills') }}
          </div>
        </div>

        <div class="skp-picker-foot">
          <span>{{ $t('skillCatalog.pickerFootLeft') }}</span>
          <span>{{ $t('skillCatalog.pickerFootRight') }}</span>
        </div>
      </div>
    </template>
  </t-popup>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listSelectableSkills, listSkillCatalog, type SelectableSkill, type SkillCatalogItem } from '@/api/skill'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const open = ref(false)
const keyword = ref('')
const enabled = ref<SelectableSkill[]>([])
const catalog = ref<SkillCatalogItem[]>([])
const expandedId = ref<string | null>(null)

const selected = computed(() => {
  const all = [...enabled.value, ...catalog.value]
  return all.find((skill) => skill.id === props.modelValue)
})
const filteredEnabled = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return enabled.value
  return enabled.value.filter((skill) => `${skill.name} ${skill.description || ''}`.toLowerCase().includes(q))
})
const filteredDisabled = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return catalog.value
    .filter((skill) => skill.enabled === false)
    .filter((skill) => !q || `${skill.name} ${skill.description || ''}`.toLowerCase().includes(q))
})

const toggleExpand = (id: string) => {
  expandedId.value = expandedId.value === id ? null : id
}

const select = (id: string) => {
  emit('update:modelValue', id)
  open.value = false
  expandedId.value = null
  keyword.value = ''
}

onMounted(async () => {
  const [selectableResult, catalogResult] = await Promise.allSettled([
    listSelectableSkills(),
    listSkillCatalog(),
  ])
  if (selectableResult.status === 'fulfilled') {
    enabled.value = Array.isArray(selectableResult.value.data) ? selectableResult.value.data : []
  }
  if (catalogResult.status === 'fulfilled') {
    catalog.value = Array.isArray(catalogResult.value.data) ? catalogResult.value.data : []
  }
})
</script>

<style lang="less" scoped>
.skp-picker-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--w-control-radius, 8px);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  font-size: 13px;
  cursor: pointer;
  font-family: inherit;
  transition: border-color 0.15s ease, background-color 0.15s ease, color 0.15s ease;

  &:hover {
    border-color: var(--w-page-accent, #07c05f);
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }

  &.is-empty { color: var(--td-text-color-placeholder); }
}
.skp-picker-trigger-label { max-width: 130px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.skp-picker-caret { font-size: 12px; color: var(--td-text-color-placeholder); }

.skp-picker {
  --skp-picker-accent: var(--w-accent-chat, var(--w-page-accent, #07c05f));
  width: min(460px, calc(100vw - 24px));
  max-height: 520px;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--w-card-radius, 12px);
  box-shadow: var(--td-shadow-2);
  color: var(--td-text-color-primary);
  overflow: hidden;
}

.skp-picker-head { padding: 14px 16px 10px; }
.skp-picker-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);

  &::before {
    content: '';
    width: 3px;
    height: 13px;
    border-radius: 2px;
    background: var(--skp-picker-accent);
  }
}
.skp-picker-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid transparent;
  border-radius: var(--w-control-radius, 8px);
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-placeholder);
  transition: border-color 0.15s ease, background-color 0.15s ease;

  &:focus-within {
    border-color: var(--skp-picker-accent);
    background: var(--td-bg-color-container);
  }

  input {
    width: 100%;
    border: 0;
    outline: 0;
    background: none;
    color: var(--td-text-color-primary);
    font-size: 13px;
    font-family: inherit;
  }
}

.skp-picker-options {
  padding: 8px 0 10px;
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-width: none;
  -ms-overflow-style: none;

  &::-webkit-scrollbar { display: none; }
}
.skp-picker-opt {
  display: block;
  width: 100%;
  padding: 12px 14px;
  border: 1px solid transparent;
  border-radius: 0;
  background: none;
  color: var(--td-text-color-primary);
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  overflow-wrap: anywhere;
  transition: border-color 0.15s ease, background-color 0.15s ease;

  & + .skp-picker-opt { margin-top: 4px; }
  &:hover:not(.is-disabled) { background: var(--td-bg-color-secondarycontainer); }
  &:focus-visible {
    outline: none;
    border-color: var(--skp-picker-accent);
  }
  &.is-expanded { background: var(--td-bg-color-secondarycontainer); }
  &.selected {
    border-color: var(--skp-picker-accent);
    background: color-mix(in srgb, var(--skp-picker-accent) 10%, var(--td-bg-color-container));
  }
  &.is-off, &.is-disabled { opacity: 0.55; cursor: default; }
}
.skp-picker-r1 { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.skp-picker-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  overflow-wrap: anywhere;
}
.skp-picker-chevron {
  margin-left: auto;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
  transition: transform 0.15s ease;
}
.skp-picker-opt.is-expanded .skp-picker-chevron { transform: rotate(180deg); }
.skp-picker-desc {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.skp-picker-opt.is-expanded .skp-picker-desc {
  display: block;
  overflow: visible;
  -webkit-line-clamp: unset;
}
.skp-picker-scene { margin-top: 4px; font-size: 12px; color: var(--td-text-color-placeholder); }
.skp-picker-extra {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--td-component-stroke);
}
.skp-picker-use {
  height: 28px;
  padding: 0 14px;
  border: 1px solid var(--skp-picker-accent);
  border-radius: var(--w-control-radius, 8px);
  background: var(--skp-picker-accent);
  color: #fff;
  font-size: 13px;
  font-family: inherit;
  cursor: pointer;

  &:hover:not(:disabled) { filter: brightness(0.94); }
  &:disabled {
    border-color: var(--td-component-stroke);
    background: transparent;
    color: var(--td-text-color-secondary);
    cursor: default;
  }
}
.skp-picker-auto {
  padding: 1px 8px;
  border: 1px solid color-mix(in srgb, var(--skp-picker-accent) 30%, var(--td-bg-color-container));
  border-radius: 7px;
  background: color-mix(in srgb, var(--skp-picker-accent) 10%, var(--td-bg-color-container));
  color: var(--skp-picker-accent);
  font-size: 12px;
}
.skp-picker-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  font-size: 12px;
}
.skp-picker-tag.on { color: var(--td-success-color); background: var(--td-success-color-light); }
.skp-picker-tag.off { color: var(--td-text-color-secondary); background: var(--td-bg-color-secondarycontainer); border-color: var(--td-component-stroke); }
.skp-picker-tag.ver { color: var(--td-warning-color); background: var(--td-warning-color-light); }
.skp-picker-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--td-success-color); }
.skp-picker-empty { padding: 18px 8px; font-size: 13px; color: var(--td-text-color-placeholder); text-align: center; }
.skp-picker-foot {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 16px 12px;
  border-top: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>

<style lang="less">
.skp-picker-popover .t-popup__content {
  padding: 0;
  background: transparent;
  box-shadow: none;
  border-radius: var(--w-card-radius, 12px);
}
</style>
