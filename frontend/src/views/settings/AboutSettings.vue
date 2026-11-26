<template>
  <div class="about-settings">
    <section class="card">
      <h3>WellRead Personal</h3>
      <p>{{ $t('settingsPages.about.intro') }}</p>
      <dl>
        <div><dt>{{ $t('settingsPages.about.version') }}</dt><dd>{{ version }}</dd></div>
        <div><dt>{{ $t('settingsPages.about.mode') }}</dt><dd>{{ $t('settingsPages.about.modeValue') }}</dd></div>
        <div><dt>{{ $t('settingsPages.about.storage') }}</dt><dd><code>data/rag/</code>{{ $t('settingsPages.about.storageNote') }}</dd></div>
      </dl>
    </section>
    <section class="card">
      <h3>{{ $t('settingsPages.about.services') }}</h3>
      <ul>
        <li><strong>app</strong>: {{ $t('settingsPages.about.appDesc') }}</li>
        <li><strong>frontend</strong>: {{ $t('settingsPages.about.frontendDesc') }}</li>
        <li><strong>rag</strong>: {{ $t('settingsPages.about.ragDesc') }}</li>
        <li><strong>mcp</strong>: {{ $t('settingsPages.about.mcpDesc') }}</li>
        <li><strong>docreader</strong>: {{ $t('settingsPages.about.docreaderDesc') }}</li>
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
