<template>
    <div class="refer" :class="{ 'refer-timeline': timelineMode }"
        v-if="session.knowledge_references && session.knowledge_references.length">
        <div class="refer_header" v-if="!contentOnly" @click="referBoxSwitch">
            <div class="refer_title">
                <t-icon v-if="!timelineMode" name="file" class="refer-title-icon" />
                <span>{{ headerText }}</span>
                <div class="refer_show_icon">
                    <t-icon :name="showReferBox ? 'chevron-down' : 'chevron-right'" />
                </div>
            </div>
        </div>
        <div class="refer_box" v-show="contentOnly || showReferBox">
            <!-- Web search references (ungrouped) -->
            <div v-for="(item, index) in webSearchRefs" :key="'web-' + index">
                <a :href="getWebSearchUrl(item)" target="_blank" rel="noopener noreferrer" class="doc doc-web"
                    @click.stop>
                    {{ webSearchRefs.length < 2 ? getWebSearchDisplayText(item) : `${index + 1}.
                        ${getWebSearchDisplayText(item)}` }} </a>
            </div>

            <!-- Knowledge references grouped by document -->
            <div v-for="(group, gIdx) in groupedKnowledgeRefs" :key="'grp-' + gIdx" class="doc-group">
                <div class="doc-group-header" @click="toggleGroup(group.key)">
                    <div class="doc-group-left">
                        <t-icon name="file" size="14px" class="doc-group-icon" />
                        <span v-if="group.number" class="doc-group-number">[{{ group.number }}]</span>
                        <span class="doc-group-title" :title="group.title">{{ group.title }}</span>
                        <span class="doc-group-count">{{ $t('chat.referenceChunkCount', { count: group.chunks.length })
                            }}</span>
                    </div>
                    <div class="doc-group-actions" v-if="group.knowledgeBaseId" @click.stop>
                        <t-tooltip :content="$t('chat.navigateToDocument')">
                            <a
                                class="doc-group-navigate"
                                :href="getDocumentHref(group)"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                <t-icon name="jump" size="14px" />
                            </a>
                        </t-tooltip>
                    </div>
                </div>
                <div class="doc-group-chunks" v-show="expandedGroups[group.key]">
                    <div v-for="(chunk, cIdx) in group.chunks" :key="'chunk-' + cIdx" class="doc-chunk-item">
                        <div class="doc-chunk-row">
                            <t-popup overlayClassName="refer-to-layer" placement="bottom-left" width="400"
                                :showArrow="false" trigger="click">
                                <template #content>
                                    <ContentPopup :content="safeProcessContent(chunk.content)" :is-html="true" />
                                </template>
                                <span class="doc-chunk-text">
                                    <span class="doc-chunk-index">{{ $t('chat.chunkLabel', { index: cIdx + 1 }) }}</span>
                                    {{ truncateContent(chunk.content, 80) }}
                                </span>
                            </t-popup>
                        </div>
                        <div v-if="chunkMetaBadges(chunk).length" class="doc-chunk-meta">
                            <span v-for="(b, bIdx) in chunkMetaBadges(chunk)" :key="'b-' + bIdx"
                                class="doc-chunk-badge" :class="{ 'doc-chunk-badge--danger': b.danger }">{{ b.text }}</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
<script setup>
import { computed, ref, reactive } from "vue";
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { sanitizeHTML } from '@/utils/security';
import ContentPopup from './tool-results/ContentPopup.vue';
import { useChatReferencesDrawer } from '@/composables/useChatReferencesDrawer';
import { buildCitationNumberMap, citationDocDisplayName, citationDocKey, selectCitedRefs } from '@/utils/citationMarkdown';

const router = useRouter();
const { t } = useI18n();
const referencesDrawer = useChatReferencesDrawer();

const props = defineProps({
    content: {
        type: String,
        required: false
    },
    session: {
        type: Object,
        required: false
    },
    timelineMode: {
        type: Boolean,
        default: false
    },
    contentOnly: {
        type: Boolean,
        default: false
    },
    userQuery: {
        type: String,
        default: ''
    }
});

const showReferBox = ref(false);
const expandedGroups = reactive({});

const referBoxSwitch = () => {
    const refs = props.session?.knowledge_references;
    if (referencesDrawer && refs?.length) {
        referencesDrawer.open({ references: refs });
        return;
    }
    showReferBox.value = !showReferBox.value;
};

const toggleGroup = (key) => {
    expandedGroups[key] = !expandedGroups[key];
};

const webSearchRefs = computed(() => {
    if (!props.session?.knowledge_references) return [];
    return props.session.knowledge_references.filter(item => item.chunk_type === 'web_search');
});

const knowledgeRefs = computed(() => {
    if (!props.session?.knowledge_references) return [];
    // 只统计实际被答案引用的文档（有对齐位置时过滤未命中的检索结果）
    return selectCitedRefs(props.session.knowledge_references.filter(item => item.chunk_type !== 'web_search'));
});

const groupedKnowledgeRefs = computed(() => {
    const refs = knowledgeRefs.value;
    if (!refs.length) return [];

    // [n] 编号与答案内联上标共用 refs 顺序,保证两侧编号一致
    const numberMap = buildCitationNumberMap(props.session?.knowledge_references);
    const groupMap = new Map();
    for (const item of refs) {
        const key = citationDocKey(item) || item.knowledge_id || item.knowledge_title || item.id;
        if (!groupMap.has(key)) {
            groupMap.set(key, {
                key,
                number: numberMap.get(key) || 0,
                title: citationDocDisplayName(item) || item.knowledge_title || item.knowledge_filename || key,
                knowledgeId: item.knowledge_id,
                knowledgeBaseId: item.knowledge_base_id,
                chunks: [],
            });
        }
        groupMap.get(key).chunks.push(item);
    }
    return Array.from(groupMap.values());
});

const headerText = computed(() => {
    const total = knowledgeRefs.value.length + webSearchRefs.value.length;
    const docCount = groupedKnowledgeRefs.value.length;
    const webCount = webSearchRefs.value.length;
    if (docCount > 0 && webCount > 0) {
        return t('chat.referencesDocAndWebCount', { docCount, webCount });
    }
    if (docCount > 0) {
        return t('chat.referencesDocCount', { count: docCount });
    }
    if (webCount > 0) {
        return t('chat.referencesWebCount', { count: webCount });
    }
    return t('chat.referencesTitle', { count: total });
});

const safeProcessContent = (content) => {
    if (!content) return '';
    const sanitized = sanitizeHTML(content);
    return sanitized.replace(/\n/g, '<br/>');
};

const truncateContent = (content, maxLen) => {
    if (!content) return '';
    const text = content.replace(/\n/g, ' ').trim();
    if (text.length <= maxLen) return text;
    return text.slice(0, maxLen) + '...';
};

// 机构/文号/生命周期徽标（§2.14 元数据，YLSW references 透传）
const chunkMetaBadges = (chunk) => {
    const md = chunk.metadata || {};
    const badges = [];
    if (md.org_name) badges.push({ text: md.org_name });
    if (md.doc_number) badges.push({ text: md.doc_number });
    if (chunk.version_label) badges.push({ text: t('chat.referenceVersion', { version: chunk.version_label }) });
    if (md.lifecycle_status === 'expired') badges.push({ text: t('chat.referenceExpired'), danger: true });
    return badges;
};

const getDocumentHref = (group) => {
    if (!group.knowledgeBaseId) return '';
    const query = {};
    if (group.knowledgeId) {
        query.knowledge_id = group.knowledgeId;
    }
    return router.resolve({
        path: `/platform/knowledge-bases/${group.knowledgeBaseId}`,
        query
    }).href;
};

// QA 引用卡片双入口：跳到文档抽屉并直接停留在「协作」标签页。
const getDocumentCollabHref = (group) => {
    if (!group.knowledgeBaseId) return '';
    const query = { doc_tab: 'collab' };
    if (group.knowledgeId) {
        query.knowledge_id = group.knowledgeId;
    }
    return router.resolve({
        path: `/platform/knowledge-bases/${group.knowledgeBaseId}`,
        query
    }).href;
};

const getWebSearchUrl = (item) => {
    if (item.metadata?.url) {
        return item.metadata.url;
    }
    if (item.id && (item.id.startsWith('http://') || item.id.startsWith('https://'))) {
        return item.id;
    }
    return '#';
};

const getWebSearchDisplayText = (item) => {
    if (item.knowledge_title) {
        return item.knowledge_title;
    }
    if (item.metadata?.title) {
        return item.metadata.title;
    }
    const url = getWebSearchUrl(item);
    if (url && url !== '#') {
        try {
            const urlObj = new URL(url);
            return urlObj.hostname;
        } catch {
            return url;
        }
    }
    return 'Web Search Result';
};
</script>
<style lang="less" scoped>
.refer {
    --refer-brand-4: color-mix(in srgb, var(--td-brand-color) 4%, transparent);
    --refer-brand-8: color-mix(in srgb, var(--td-brand-color) 8%, transparent);
    --refer-brand-12: color-mix(in srgb, var(--td-brand-color) 12%, transparent);

    display: flex;
    flex-direction: column;
    font-size: 12px;
    width: 100%;
    border-radius: 8px;
    background-color: var(--td-bg-color-container);
    border: .5px solid var(--td-component-stroke);
    box-shadow: 0 2px 4px var(--refer-brand-8);
    box-sizing: border-box;
    overflow: hidden;
    box-sizing: border-box;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    margin-bottom: 8px;

    &.refer-timeline {
        --td-brand-color: var(--td-text-color-placeholder);
        --embed-primary: var(--td-text-color-placeholder);
        --refer-brand-4: color-mix(in srgb, var(--td-text-color-primary) 4%, transparent);
        --refer-brand-8: color-mix(in srgb, var(--td-text-color-primary) 6%, transparent);
        --refer-brand-12: color-mix(in srgb, var(--td-text-color-primary) 8%, transparent);

        width: 100%;
        border-radius: 0;
        background-color: transparent;
        border: 0;
        box-shadow: none;
        margin-bottom: 0;
        font-size: 13px;

        .refer_header {
            padding: 0;
            font-weight: 400;
            color: var(--td-text-color-secondary);

            .refer_title span {
                font-size: 14px;
                white-space: normal;
            }
        }

        .refer_header:hover {
            background-color: transparent;
        }

        .refer_show_icon {
            color: var(--td-text-color-placeholder);
        }

        .refer_box {
            padding: 2px 0 0 0;
            border-top: 0;
        }

        .doc-group {
            margin-top: 2px;
        }

        .doc-group-header {
            padding: 2px 0;
        }

        .doc-group-title {
            color: var(--td-text-color-secondary);
            font-weight: 400;
            font-size: 13px;
            max-width: min(520px, 100%);
        }

        .doc-group-count,
        .doc-chunk-text,
        .doc-chunk-index {
            font-size: 12px;
        }

        .doc-group-icon {
            color: var(--td-text-color-placeholder);
        }

        .doc-group-navigate {
            color: var(--td-text-color-placeholder);
        }

        .doc {
            color: var(--td-text-color-secondary);
            border-bottom-color: color-mix(in srgb, var(--td-text-color-secondary) 30%, transparent);
        }

        .doc-chunk-item .doc-chunk-text:hover {
            background-color: var(--refer-brand-4);
            color: var(--td-text-color-secondary);
        }
    }

    .refer_header {
        display: flex;
        align-items: center;
        padding: 6px 14px;
        color: var(--td-text-color-primary);
        font-weight: 500;

        .refer_title {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            min-width: 0;

            span {
                white-space: nowrap;
                font-size: 12px;
            }
        }

        .refer_show_icon {
            font-size: 14px;
            padding: 0 2px 1px 2px;
            color: var(--td-brand-color);
            flex-shrink: 0;
        }
    }

    .refer_header:hover {
        background-color: var(--refer-brand-4);
        cursor: pointer;
    }

    .refer_box {
        padding: 4px 14px 8px 14px;
        flex-direction: column;
        border-top: 1px solid var(--td-bg-color-secondarycontainer);
    }
}

.refer-title-icon {
    width: 16px;
    height: 16px;
    margin-right: 8px;
    flex-shrink: 0;
    color: var(--embed-primary, var(--td-brand-color));
}

.doc {
    text-decoration: none;
    color: var(--td-brand-color);
    cursor: pointer;
    display: inline-block;
    white-space: nowrap;
    max-width: calc(100% - 24px);
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 20px;
    padding: 2px 0;
    transition: all 0.2s ease;
    border-bottom: 1px dashed var(--td-brand-color);

    &:hover {
        background-color: var(--refer-brand-8);
        border-radius: 3px;
        padding-right: 4px;
    }

    &.doc-web {
        white-space: normal;
        word-break: break-all;
    }
}

.doc-group {
    margin-top: 4px;

    .doc-group-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 4px 4px;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.15s ease;

        &:hover {
            background-color: var(--refer-brand-4);
        }

        .doc-group-left {
            display: flex;
            align-items: center;
            min-width: 0;
            flex: 1;
        }

        .doc-group-arrow {
            color: var(--td-text-color-placeholder);
            flex-shrink: 0;
            margin-left: 4px;
        }

        .doc-group-icon {
            color: var(--td-brand-color);
            flex-shrink: 0;
            margin-right: 6px;
        }

        .doc-group-title {
            color: var(--td-text-color-primary);
            font-weight: 500;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            max-width: 200px;
        }

        .doc-group-number {
            color: var(--td-brand-color);
            font-size: 11px;
            font-weight: 600;
            margin-right: 4px;
            flex-shrink: 0;
        }

        .doc-group-count {
            color: var(--td-text-color-placeholder);
            font-size: 11px;
            margin-left: 6px;
            white-space: nowrap;
            flex-shrink: 0;
        }

        .doc-group-actions {
            flex-shrink: 0;
            margin-left: 8px;
        }

        .doc-group-navigate {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 22px;
            height: 22px;
            border-radius: 4px;
            color: var(--td-brand-color);
            cursor: pointer;
            text-decoration: none;
            transition: all 0.15s ease;

            &:hover {
                background-color: var(--refer-brand-12);
            }
        }
    }

    .doc-group-chunks {
        padding-left: 22px;
    }
}

.doc-chunk-item {
    .doc-chunk-row {
        display: flex;
        align-items: center;
        gap: 4px;
        min-width: 0;
    }

    .doc-chunk-text {
        display: block;
        flex: 1;
        min-width: 0;
        color: var(--td-text-color-secondary);
        font-size: 12px;
        line-height: 18px;
        padding: 3px 6px;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.15s ease;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;

        &:hover {
            background-color: var(--refer-brand-4);
            color: var(--td-brand-color);
        }

        .doc-chunk-index {
            color: var(--td-text-color-placeholder);
            font-size: 11px;
            margin-right: 4px;
        }
    }

    .doc-chunk-actions {
        display: flex;
        align-items: center;
        gap: 2px;
        flex-shrink: 0;
        opacity: 0;
        transition: opacity 0.15s ease;
    }

    &:hover .doc-chunk-actions,
    .doc-chunk-actions:focus-within {
        opacity: 1;
    }

    .doc-chunk-action {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 22px;
        height: 22px;
        padding: 0;
        border: 0;
        border-radius: 4px;
        background: transparent;
        color: var(--td-text-color-placeholder);
        cursor: pointer;
        text-decoration: none;
        transition: color 0.15s ease, background-color 0.15s ease;

        &:hover {
            color: var(--td-brand-color);
            background-color: var(--refer-brand-12);
        }
    }

    .doc-chunk-meta {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
        padding: 1px 6px 2px;
    }

    .doc-chunk-badge {
        display: inline-flex;
        align-items: center;
        padding: 0 6px;
        border-radius: 6px;
        background: var(--td-brand-color-light);
        font-size: 10px;
        font-weight: 600;
        line-height: 1.7;
        color: var(--td-brand-color);

        &--danger {
            background: var(--td-error-color-1);
            color: var(--td-error-color);
        }
    }
}
</style>

<style>
.refer-to-layer {
    width: 400px;
    max-width: 500px;

    .t-popup__content {
        max-height: 400px;
        max-width: 500px;
        overflow-y: auto;
        overflow-x: hidden;
        word-wrap: break-word;
        word-break: break-word;
    }
}
</style>
