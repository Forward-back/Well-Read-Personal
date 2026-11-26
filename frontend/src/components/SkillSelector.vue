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

          <button
            v-for="skill in filteredEnabled"
            :key="skill.id"
            type="button"
            class="skp-picker-opt"
            :class="{ selected: modelValue === skill.id }"
            @click="select(skill.id)"
          >
            <div class="skp-picker-r1">
              <span class="skp-picker-name">{{ skill.name }}</span>
              <span v-if="skill.version" class="skp-picker-tag ver">{{ skill.version }}</span>
              <span class="skp-picker-tag on"><span class="skp-picker-dot" />{{ $t('skillCatalog.enabled') }}</span>
            </div>
            <div class="skp-picker-desc">{{ skill.description || '—' }}</div>
            <div class="skp-picker-scene">
              <b>{{ $t('skillCatalog.scope', { scope: $t('skillCatalog.scopeQa') }) }}</b>
            </div>
            <div class="skp-picker-extra">
              <b>{{ $t('skillCatalog.pickerWhen') }}</b>{{ skill.description || '—' }}<br />
              <span class="skp-picker-faint">{{ $t('skillCatalog.pickerStats') }}</span>
            </div>
          </button>

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

const select = (id: string) => {
  emit('update:modelValue', id)
  open.value = false
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
  border: 1px solid #e3dcce;
  border-radius: 8px;
  background: #fff;
  color: #7c7264;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;

  &:hover { background: #f8f5ee; border-color: #d3c9b6; }
  &.is-empty { color: #a79c8b; }
}
.skp-picker-trigger-label { max-width: 130px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.skp-picker-caret { font-size: 12px; color: var(--td-text-color-placeholder, #999); }

.skp-picker {
  width: 380px;
  max-height: 520px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border: 1px solid #e3dcce;
  border-radius: 14px 9px 16px 10px / 10px 15px 9px 13px;
  box-shadow: 0 6px 20px rgba(75, 68, 59, 0.12);
  overflow: hidden;
}
.skp-picker-head { padding: 12px 16px 8px; }
.skp-picker-title { font-size: 12px; font-weight: 700; color: #5c6b51; letter-spacing: 0.5px; margin-bottom: 10px; }
.skp-picker-search {
  display: flex; align-items: center; gap: 8px;
  background: #f8f5ee;
  border: 1px solid #eee8dd;
  border-radius: 10px 7px 11px 8px / 8px 11px 7px 10px; padding: 7px 12px;
  color: var(--td-text-color-placeholder, #999);
  input { border: 0; outline: 0; background: none; font-size: 13px; color: var(--td-text-color-primary, #1f2329); width: 100%; font-family: inherit; }
}
.skp-picker-options { padding: 6px 10px 10px; overflow-y: auto; }
.skp-picker-opt {
  display: block; width: 100%; text-align: left; border: 1px solid transparent; background: none;
  border-radius: 12px 8px 13px 9px / 9px 13px 8px 12px; padding: 11px 14px; cursor: pointer;
  font-family: inherit; color: #4b443b;
  & + .skp-picker-opt { margin-top: 4px; }
  &:hover:not(.is-disabled) { background: #f8f5ee; border-color: #d3c9b6; }
  &.selected { border-color: #5c6b51; background: #eef1ea; }
  &.is-off, &.is-disabled { opacity: 0.6; cursor: default; }
}
.skp-picker-r1 { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
.skp-picker-name { font-family: var(--td-font-family-mono, monospace); font-weight: 700; font-size: 13px; }
.skp-picker-desc { margin-top: 3px; font-size: 12.5px; color: #7c7264; line-height: 1.7; }
.skp-picker-scene { margin-top: 3px; font-size: 11.5px; color: #a79c8b; }
.skp-picker-extra { display: none; margin-top: 8px; border-top: 1px dashed #e3dcce; padding-top: 8px; font-size: 12px; color: #7c7264; line-height: 1.7; }
.skp-picker-opt:hover:not(.is-disabled) .skp-picker-extra { display: block; }
.skp-picker-auto { font-size: 11px; color: #46523c; background: #eef1ea; border: 1px solid #d7ddd0; padding: 1px 8px; border-radius: 9px 7px 8px 10px / 10px 8px 7px 9px; }
.skp-picker-tag { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; padding: 1px 8px; border-radius: 9px 7px 8px 10px / 10px 8px 7px 9px; border: 1px solid transparent; }
.skp-picker-tag.on { color: #46523c; background: #eef1ea; border-color: #d7ddd0; }
.skp-picker-tag.off { color: #7c7264; background: #f8f5ee; border-color: #e3dcce; }
.skp-picker-tag.ver { color: #96794b; background: #f6f0e5; border-color: #e2d3b8; }
.skp-picker-dot { width: 6px; height: 6px; border-radius: 50%; background: #5c6b51; }
.skp-picker-faint { color: #a79c8b; }
.skp-picker-empty { padding: 18px 8px; font-size: 12.5px; color: var(--td-text-color-placeholder, #999); text-align: center; }
.skp-picker-foot { display: flex; justify-content: space-between; gap: 10px; flex-wrap: wrap; padding: 8px 16px 12px; border-top: 1px dashed #e3dcce; font-size: 11.5px; color: #a79c8b; }
</style>

<style lang="less">
.skp-picker-popover .t-popup__content { padding: 0; background: transparent; box-shadow: none; }
</style>
