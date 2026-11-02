<template>
  <div class="about-settings">
    <section class="card">
      <h3>WellRead Personal</h3>
      <p>个人版知识库与精读工作台。单账号、本地部署，保留知识库、问答、精读与技能。</p>
      <dl>
        <div><dt>版本</dt><dd>{{ version }}</dd></div>
        <div><dt>运行模式</dt><dd>个人版（单用户）</dd></div>
        <div><dt>数据目录</dt><dd><code>data/rag/</code>（本地，不进入版本库）</dd></div>
      </dl>
    </section>
    <section class="card">
      <h3>组成服务</h3>
      <ul>
        <li><strong>app</strong>：Go 后端，账号、知识库、模型与队列。</li>
        <li><strong>frontend</strong>：Vue 3 界面。</li>
        <li><strong>rag</strong>：检索与生成侧车。</li>
        <li><strong>mcp</strong>：外部 MCP 只读接入。</li>
        <li><strong>docreader</strong>：文档解析。</li>
      </ul>
    </section>
  </div>
</template>

<script setup lang="ts">
const version = __FRONTEND_VERSION__
</script>

<style lang="less" scoped>
.about-settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.card {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  padding: 20px;

  h3 {
    margin: 0 0 8px;
    font-size: 16px;
    font-weight: 600;
  }

  p,
  li {
    margin: 0;
    font-size: 13px;
    line-height: 1.7;
    color: var(--td-text-color-secondary);
  }

  ul {
    margin: 0;
    padding-left: 18px;
  }
}

dl {
  margin: 16px 0 0;
  display: flex;
  flex-direction: column;
  gap: 8px;

  div {
    display: flex;
    gap: 12px;
    font-size: 13px;
  }

  dt {
    width: 72px;
    color: var(--td-text-color-secondary);
  }

  dd {
    margin: 0;
  }
}
</style>
