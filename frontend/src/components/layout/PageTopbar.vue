<template>
  <header class="page-topbar">
    <div class="page-topbar__lead">
      <slot name="lead">
        <span class="page-topbar__title" :title="title">{{ title }}</span>
        <span v-if="meta" class="page-topbar__meta" :title="meta">{{ meta }}</span>
      </slot>
    </div>
    <div class="page-topbar__spacer" />
    <slot name="nav" />
    <div v-if="$slots.actions" class="page-topbar__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
defineProps<{ title?: string; meta?: string }>()
</script>

<style scoped>
.page-topbar {
  display: flex;
  flex: none;
  align-items: center;
  gap: 14px;
  min-width: 0;
  height: 48px;
  padding: 0 16px;
  background: var(--page-topbar-bg, #fff);
  border-bottom: 1px solid var(--page-topbar-line, var(--td-component-stroke, #e7e7e7));
  color: var(--page-topbar-title, var(--td-text-color-primary, #1f2329));
}

.page-topbar__lead {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex: 0 1 auto;
  max-width: 55%;
  min-width: 0;
}

.page-topbar__title {
  overflow: hidden;
  min-width: 0;
  font-size: 13.5px;
  font-weight: 600;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-topbar__meta {
  overflow: hidden;
  min-width: 0;
  font-size: 12px;
  line-height: 18px;
  color: var(--page-topbar-meta, var(--td-text-color-placeholder, #999));
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-topbar__spacer {
  flex: 1;
  min-width: 8px;
}

.page-topbar__actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

@media (max-width: 760px) {
  .page-topbar {
    flex-wrap: wrap;
    height: auto;
    min-height: 48px;
    padding: 6px 12px;
    gap: 8px;
  }

  .page-topbar__lead {
    max-width: 100%;
  }

  .page-topbar__actions {
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .page-topbar__actions::-webkit-scrollbar {
    display: none;
  }
}
</style>
