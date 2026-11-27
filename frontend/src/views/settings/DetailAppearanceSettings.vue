<template>
  <div class="detail-appearance-settings">
    <div class="section-header">
      <h2>{{ $t('detailAppearance.title') }}</h2>
      <p class="section-description">
        {{ $t('detailAppearance.description') }}
      </p>
    </div>

    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('detailAppearance.canvasColor') }}</label>
        <p class="desc">{{ $t('detailAppearance.canvasColorDesc') }}</p>
      </div>
      <div class="setting-control">
        <div class="appearance-controls">
          <div class="preset-row">
            <button
              v-for="preset in DETAIL_COLOR_PRESETS"
              :key="preset.value"
              type="button"
              class="preset-swatch"
              :class="{ 'is-active': currentColor === preset.value }"
              :style="{ backgroundColor: preset.value }"
              :title="`${presetLabel(preset.value)} ${preset.value}`"
              :aria-label="presetLabel(preset.value)"
              @click="handleColorChange(preset.value)"
            />
          </div>
          <label class="custom-color">
            <span
              class="custom-color__chip"
              :style="{ backgroundColor: currentColor }"
              aria-hidden="true"
            />
            <input
              type="color"
              class="custom-color__input"
              :value="currentColor"
              :aria-label="$t('detailAppearance.customColor')"
              @input="handleColorInput"
            />
            <span class="custom-color__hex">{{ currentColor }}</span>
          </label>
        </div>
      </div>
    </div>

    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('detailAppearance.transparency') }}</label>
        <p class="desc">{{ $t('detailAppearance.transparencyDesc') }}</p>
      </div>
      <div class="setting-control">
        <div class="transparency-control">
          <t-slider
            v-model="transparencyModel"
            :min="0"
            :max="100"
            :step="1"
            style="width: 240px;"
          />
          <span class="transparency-value">{{ currentTransparency }}%</span>
        </div>
      </div>
    </div>

    <div class="text-color-block">
      <h3>{{ $t('detailAppearance.textColors') }}</h3>
      <p class="text-color-hint">
        {{ $t('detailAppearance.textColorsHint') }}
      </p>
      <div v-for="row in textColorRows" :key="row.level" class="setting-row">
        <div class="setting-info">
          <label>
            {{ row.label }}
            <span v-if="row.isDefault" class="default-tag">{{ $t('detailAppearance.defaultTag') }}</span>
          </label>
          <p class="desc">{{ row.desc }}</p>
        </div>
        <div class="setting-control">
          <label class="custom-color">
            <span
              class="custom-color__chip"
              :style="{ backgroundColor: row.value }"
              aria-hidden="true"
            />
            <input
              type="color"
              class="custom-color__input"
              :value="row.value"
              :aria-label="row.label"
              @input="handleTextColorInput(row.level, $event)"
            />
            <span class="custom-color__hex">{{ row.value }}</span>
          </label>
        </div>
      </div>
    </div>

    <div class="text-color-block">
      <h3>{{ $t('detailAppearance.agentAppearance') }}</h3>
      <p class="text-color-hint">
        {{ $t('detailAppearance.agentAppearanceHint') }}
      </p>
      <div v-for="row in agentAppearanceRows" :key="row.key" class="setting-row">
        <div class="setting-info">
          <label>
            {{ row.label }}
            <span v-if="row.isDefault" class="default-tag">{{ $t('detailAppearance.defaultTag') }}</span>
          </label>
          <p class="desc">{{ row.desc }}</p>
        </div>
        <div class="setting-control">
          <div class="feature-accent-control">
            <label class="custom-color">
              <span
                class="custom-color__chip"
                :style="{ backgroundColor: row.value }"
                aria-hidden="true"
              />
              <input
                type="color"
                class="custom-color__input"
                :value="row.value"
                :aria-label="row.label"
                @input="handleAgentAppearanceInput(row.key, $event)"
              />
              <span class="custom-color__hex">{{ row.value }}</span>
            </label>
            <t-button
              v-if="!row.isDefault"
              variant="text"
              size="small"
              @click="handleAgentAppearanceReset(row.key)"
            >
              {{ $t('detailAppearance.resetFeature') }}
            </t-button>
          </div>
        </div>
      </div>
    </div>

    <div class="text-color-block">
      <h3>{{ $t('detailAppearance.featureColors') }}</h3>
      <p class="text-color-hint">
        {{ $t('detailAppearance.featureColorsHint') }}
      </p>
      <div v-for="row in featureAccentRows" :key="row.key" class="setting-row">
        <div class="setting-info">
          <label>
            {{ row.label }}
            <span v-if="row.isDefault" class="default-tag">{{ $t('detailAppearance.defaultTag') }}</span>
          </label>
          <p class="desc">{{ row.desc }}</p>
        </div>
        <div class="setting-control">
          <div class="feature-accent-control">
            <label class="custom-color">
              <span
                class="custom-color__chip"
                :style="{ backgroundColor: row.value }"
                aria-hidden="true"
              />
              <input
                type="color"
                class="custom-color__input"
                :value="row.value"
                :aria-label="row.label"
                @input="handleFeatureAccentInput(row.key, $event)"
              />
              <span class="custom-color__hex">{{ row.value }}</span>
            </label>
            <t-button
              v-if="!row.isDefault"
              variant="text"
              size="small"
              @click="handleFeatureAccentReset(row.key)"
            >
              {{ $t('detailAppearance.resetFeature') }}
            </t-button>
          </div>
        </div>
      </div>
    </div>

    <div class="appearance-footer">
      <t-button variant="text" size="small" @click="handleReset">{{ $t('detailAppearance.resetAll') }}</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  AGENT_APPEARANCE_KEYS,
  DEFAULT_AGENT_APPEARANCE,
  DEFAULT_FEATURE_ACCENTS,
  DETAIL_COLOR_PRESETS,
  DEFAULT_TEXT_COLORS,
  FEATURE_ACCENT_KEYS,
  useDetailAppearance,
  type AgentAppearanceKey,
  type DetailTextLevel,
  type FeatureAccentKey,
} from '@/composables/useDetailAppearance'

const { t } = useI18n()
const {
  currentColor,
  currentTransparency,
  textPrimary,
  textSecondary,
  textPlaceholder,
  featureAccents,
  agentAppearance,
  setDetailColor,
  setDetailTransparency,
  setDetailTextColor,
  setFeatureAccent,
  resetFeatureAccent,
  setAgentAppearance,
  resetAgentAppearance,
  resetDetailAppearance,
} = useDetailAppearance()

const PRESET_LABEL_KEYS: Record<string, string> = {
  '#f7f5f0': 'detailAppearance.presetWarm',
  '#ffffff': 'detailAppearance.presetWhite',
  '#f2f3f5': 'detailAppearance.presetGray',
  '#eef7ee': 'detailAppearance.presetGreen',
}
const presetLabel = (value: string) => t(PRESET_LABEL_KEYS[value] || 'detailAppearance.canvasColor')

const textColorRows = computed(() => [
  {
    level: 'primary' as DetailTextLevel,
    label: t('detailAppearance.textPrimary'),
    desc: t('detailAppearance.textPrimaryDesc'),
    value: textPrimary.value || DEFAULT_TEXT_COLORS.primary,
    isDefault: !textPrimary.value,
  },
  {
    level: 'secondary' as DetailTextLevel,
    label: t('detailAppearance.textSecondary'),
    desc: t('detailAppearance.textSecondaryDesc'),
    value: textSecondary.value || DEFAULT_TEXT_COLORS.secondary,
    isDefault: !textSecondary.value,
  },
  {
    level: 'placeholder' as DetailTextLevel,
    label: t('detailAppearance.textPlaceholder'),
    desc: t('detailAppearance.textPlaceholderDesc'),
    value: textPlaceholder.value || DEFAULT_TEXT_COLORS.placeholder,
    isDefault: !textPlaceholder.value,
  },
])

const FEATURE_ACCENT_LABEL_KEYS: Record<FeatureAccentKey, string> = {
  chat: 'detailAppearance.featureChat',
  knowledge: 'detailAppearance.featureKnowledge',
  reading: 'detailAppearance.featureReading',
  review: 'detailAppearance.featureReview',
  skills: 'detailAppearance.featureSkills',
  agent: 'detailAppearance.featureAgent',
}

const featureAccentRows = computed(() =>
  FEATURE_ACCENT_KEYS.map((key) => ({
    key,
    label: t(FEATURE_ACCENT_LABEL_KEYS[key]),
    desc: t(`detailAppearance.feature${key[0].toUpperCase()}${key.slice(1)}Desc`),
    value: featureAccents[key] || DEFAULT_FEATURE_ACCENTS[key],
    isDefault: !featureAccents[key],
  })),
)

const AGENT_APPEARANCE_LABEL_KEYS: Record<AgentAppearanceKey, string> = {
  surface: 'detailAppearance.agentSurface',
  raised: 'detailAppearance.agentRaised',
  border: 'detailAppearance.agentBorder',
}

const agentAppearanceRows = computed(() =>
  AGENT_APPEARANCE_KEYS.map((key) => ({
    key,
    label: t(AGENT_APPEARANCE_LABEL_KEYS[key]),
    desc: t(`detailAppearance.agent${key[0].toUpperCase()}${key.slice(1)}Desc`),
    value: agentAppearance[key] || DEFAULT_AGENT_APPEARANCE[key],
    isDefault: !agentAppearance[key],
  })),
)

const transparencyModel = computed<number>({
  get: () => currentTransparency.value,
  set: (value) => setDetailTransparency(value),
})

const handleColorChange = (value: string) => {
  if (!setDetailColor(value)) {
    MessagePlugin.error(t('detailAppearance.invalidColor'))
  }
}

const handleColorInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  handleColorChange(value)
}

const handleTextColorInput = (level: DetailTextLevel, event: Event) => {
  const value = (event.target as HTMLInputElement).value
  if (!setDetailTextColor(level, value)) {
    MessagePlugin.error(t('detailAppearance.invalidColor'))
  }
}

const handleFeatureAccentInput = (key: FeatureAccentKey, event: Event) => {
  const value = (event.target as HTMLInputElement).value
  if (!setFeatureAccent(key, value)) {
    MessagePlugin.error(t('detailAppearance.invalidColor'))
  }
}

const handleFeatureAccentReset = (key: FeatureAccentKey) => {
  resetFeatureAccent(key)
}

const handleAgentAppearanceInput = (key: AgentAppearanceKey, event: Event) => {
  const value = (event.target as HTMLInputElement).value
  if (!setAgentAppearance(key, value)) {
    MessagePlugin.error(t('detailAppearance.invalidColor'))
  }
}

const handleAgentAppearanceReset = (key: AgentAppearanceKey) => {
  resetAgentAppearance(key)
}

const handleReset = () => {
  resetDetailAppearance()
  MessagePlugin.success(t('detailAppearance.resetSuccess'))
}
</script>

<style lang="less" scoped>
.detail-appearance-settings {
  width: 100%;
}

.section-header {
  margin-bottom: 28px;

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 8px;
  }

  .section-description {
    font-size: 14px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-of-type {
    border-bottom: none;
  }
}

.text-color-block {
  margin-top: 8px;

  h3 {
    margin: 0 0 4px;
    font-size: 15px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .text-color-hint {
    margin: 0 0 4px;
    font-size: 13px;
    color: var(--td-text-color-secondary);
    line-height: 1.5;
  }
}

.feature-accent-control {
  display: flex;
  align-items: center;
  gap: 8px;
}

.default-tag {
  margin-left: 6px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 400;
  line-height: 18px;
  border-radius: 9px;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
}

.setting-info {
  flex: 1;
  max-width: 65%;
  padding-right: 24px;

  label {
    font-size: 15px;
    font-weight: 500;
    color: var(--td-text-color-primary);
    display: block;
    margin-bottom: 4px;
  }

  .desc {
    font-size: 13px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.setting-control {
  flex-shrink: 0;
  min-width: 280px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.appearance-controls {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

.preset-row {
  display: flex;
  gap: 8px;
}

.preset-swatch {
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid var(--td-component-stroke);
  border-radius: 50%;
  cursor: pointer;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.03);
  outline: none;
  transition: outline-color 0.15s ease, transform 0.15s ease;

  &:hover {
    transform: scale(1.06);
  }

  &.is-active,
  &:focus-visible {
    outline: 2px solid #07c05f;
    outline-offset: 2px;
  }
}

.custom-color {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 4px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 4px;
  background: var(--td-bg-color-container);
  cursor: pointer;
}

.custom-color__chip {
  width: 22px;
  height: 22px;
  border-radius: 3px;
  border: 1px solid var(--td-component-stroke);
}

.custom-color__input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.custom-color__hex {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  text-transform: lowercase;
}

.transparency-control {
  display: flex;
  align-items: center;
  gap: 12px;
}

.transparency-value {
  min-width: 44px;
  text-align: right;
  font-size: 13px;
  color: var(--td-text-color-secondary);
  font-variant-numeric: tabular-nums;
}

.appearance-footer {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
}
</style>
