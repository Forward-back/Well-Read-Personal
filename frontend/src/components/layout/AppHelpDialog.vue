<template>
  <t-dialog v-model:visible="visible" header="帮助与关于" :footer="false" width="520px">
    <div class="help-body">
      <section>
        <h3>快捷键</h3>
        <ul>
          <li><kbd>Ctrl K</kbd> 打开全局搜索</li>
          <li><kbd>空格</kbd> 复习时翻面</li>
          <li><kbd>1</kbd> 忘记 · <kbd>2</kbd> 想得慢 · <kbd>3</kbd> 想起来 · <kbd>4</kbd> 秒答</li>
          <li><kbd>Esc</kbd> 暂存退出复习</li>
        </ul>
      </section>
      <section>
        <h3>项目</h3>
        <p>WellRead Personal · 本地单 owner 知识库</p>
        <p class="help-muted">版本 {{ version }} · {{ commit }}</p>
        <a href="https://github.com/Forward-back" target="_blank" rel="noopener noreferrer">github.com/Forward-back</a>
      </section>
      <section>
        <h3>素材</h3>
        <p class="help-muted">
          动态图标与手绘图标来自
          <a href="https://icons8.com" target="_blank" rel="noopener noreferrer">Icons8</a>；
          草地背景来自 PostHog 网站，仅个人本地使用。
        </p>
      </section>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()
const visible = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value),
})
const version = typeof __FRONTEND_VERSION__ !== 'undefined' ? __FRONTEND_VERSION__ : 'dev'
const commit = typeof __FRONTEND_COMMIT__ !== 'undefined' ? __FRONTEND_COMMIT__ : 'local'
</script>

<style scoped>
.help-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  font-size: 13px;
  line-height: 21px;
  color: var(--td-text-color-secondary);
}

h3 {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

ul {
  margin: 0;
  padding-left: 18px;
}

p { margin: 0 0 4px; }

kbd {
  padding: 1px 5px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 4px;
  background: var(--td-bg-color-secondarycontainer);
  font-family: inherit;
  font-size: 11px;
}

.help-muted { color: var(--td-text-color-placeholder); }

a { color: var(--td-brand-color); }
</style>
