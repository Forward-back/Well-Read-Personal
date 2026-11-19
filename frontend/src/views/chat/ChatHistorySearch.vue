<template>
  <section class="chat-search" @keydown.esc="$emit('close')">
    <div class="chat-search__bar">
      <button type="button" class="chat-search__back" aria-label="返回对话" @click="$emit('close')">
        <t-icon name="chevron-left" size="18px" />
      </button>
      <t-input
        ref="inputRef"
        v-model="keyword"
        class="chat-search__input"
        size="large"
        clearable
        placeholder="搜索历史对话"
        @input="onInput"
      >
        <template #prefix-icon><t-icon name="search" /></template>
      </t-input>
    </div>

    <h2 class="chat-search__title">对话历史</h2>

    <div v-if="loading" class="chat-search__state">
      <t-loading size="small" />
      <span>加载中…</span>
    </div>
    <div v-else-if="error" class="chat-search__state">
      <span>{{ error }}</span>
      <t-button size="small" variant="outline" @click="load">重试</t-button>
    </div>
    <div v-else-if="!rows.length" class="chat-search__state">
      <span>{{ keyword.trim() ? '没有找到相关对话' : '暂无历史对话' }}</span>
    </div>
    <div v-else class="chat-search__list">
      <button
        v-for="row in rows"
        :key="row.id"
        type="button"
        class="chat-search__card"
        @click="openSession(row)"
      >
        <div class="chat-search__card-head">
          <strong>{{ row.title }}</strong>
          <span v-if="row.updated_at">{{ formatDate(row.updated_at) }}</span>
        </div>
        <p v-if="row.first_user_message">{{ row.first_user_message }}</p>
        <p v-else class="chat-search__empty-desc">暂无提问</p>
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getSessionsList } from '@/api/chat'

type SessionRow = {
  id: string
  title: string
  first_user_message?: string
  updated_at?: string
}

const emit = defineEmits<{ (e: 'close'): void }>()
const router = useRouter()
const inputRef = ref<{ focus?: () => void } | null>(null)
const keyword = ref('')
const rows = ref<SessionRow[]>([])
const loading = ref(false)
const error = ref('')
let timer: ReturnType<typeof setTimeout> | null = null
let requestSeq = 0

function mapRow(item: any): SessionRow {
  return {
    id: String(item.id),
    title: item.title || '未命名对话',
    first_user_message: item.first_user_message || '',
    updated_at: item.updated_at || item.created_at || '',
  }
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

async function load() {
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  try {
    const res: any = await getSessionsList(1, 50, keyword.value.trim() || undefined)
    if (seq !== requestSeq) return
    rows.value = (res?.data || []).map(mapRow)
  } catch {
    if (seq !== requestSeq) return
    rows.value = []
    error.value = '加载历史对话失败'
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function onInput() {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => { void load() }, 250)
}

function openSession(row: SessionRow) {
  emit('close')
  router.push(`/platform/chat/${row.id}`)
}

onMounted(() => {
  void load()
  nextTick(() => inputRef.value?.focus?.())
})
</script>

<style lang="less" scoped>
.chat-search {
  position: absolute;
  inset: 0;
  z-index: 20;
  overflow-y: auto;
  padding: 32px 40px 48px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
}

.chat-search__bar {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 980px;
  margin: 0 auto;
}

.chat-search__back {
  display: grid;
  flex: none;
  place-items: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-primary);
  }
}

.chat-search__input {
  flex: 1;
}

.chat-search__title {
  max-width: 980px;
  margin: 22px auto 16px;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--td-brand-color);
  color: var(--td-text-color-primary);
  font-size: 18px;
  font-weight: 600;
  line-height: 28px;
  width: fit-content;
}

.chat-search__list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 980px;
  margin: 0 auto;
}

.chat-search__card {
  display: block;
  width: 100%;
  padding: 16px 20px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;

  &:hover {
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container);
  }

  p {
    display: -webkit-box;
    overflow: hidden;
    margin: 6px 0 0;
    color: var(--td-text-color-secondary);
    font-size: 13px;
    line-height: 22px;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }
}

.chat-search__card-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;

  strong {
    overflow: hidden;
    font-size: 15px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  span {
    flex: none;
    color: var(--td-text-color-placeholder);
    font-size: 12px;
  }
}

.chat-search__empty-desc {
  color: var(--td-text-color-placeholder) !important;
}

.chat-search__state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  max-width: 980px;
  min-height: 160px;
  margin: 0 auto;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
</style>
