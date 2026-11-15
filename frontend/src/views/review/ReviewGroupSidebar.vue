<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Folder, Home, MoreHorizontal, Pencil, Plus, Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { ReviewGroup } from '@/api/review'

interface FlatGroupItem {
  group: ReviewGroup
  depth: number
  card_count: number
}

defineProps<{
  groups: FlatGroupItem[]
  selectedGroupId: string
  totalCards: number
}>()

const emit = defineEmits<{
  (e: 'select', groupId: string): void
  (e: 'create', parentId: string): void
  (e: 'rename', group: ReviewGroup): void
  (e: 'delete', group: ReviewGroup): void
}>()

const { t } = useI18n()
</script>

<template>
  <aside class="bg-card flex h-fit flex-col gap-1 rounded-lg border p-2" data-testid="review-group-sidebar">
    <div class="text-muted-foreground flex items-center justify-between px-2 py-1 text-xs">
      <span>{{ t('review.groups') }}</span>
      <Button variant="ghost" size="icon-xs" :title="t('review.newGroup')" @click="emit('create', '')">
        <Plus />
      </Button>
    </div>

    <button
      type="button"
      data-testid="review-group-all"
      class="flex w-full items-center justify-between rounded-md border-0 bg-transparent px-2 py-1.5 text-sm transition-colors"
      :class="selectedGroupId === '' ? 'bg-primary/10 text-primary' : 'hover:bg-muted'"
      @click="emit('select', '')"
    >
      <span class="truncate">{{ t('review.allGroups') }}</span>
      <span class="text-muted-foreground text-xs">{{ totalCards }}</span>
    </button>

    <ContextMenu v-for="item in groups" :key="item.group.id">
      <ContextMenuTrigger as-child>
        <div
          class="flex items-center gap-1 rounded-md pr-1 transition-colors"
          :class="selectedGroupId === item.group.id ? 'bg-primary/10 text-primary' : 'hover:bg-muted'"
          data-testid="review-group-row"
          :data-group-name="item.group.name"
          :data-active="selectedGroupId === item.group.id ? 'true' : 'false'"
        >
          <button type="button" class="flex min-w-0 flex-1 items-center gap-1.5 border-0 bg-transparent px-2 py-1.5 text-left text-sm" @click="emit('select', item.group.id)">
            <component :is="item.group.is_default ? Home : Folder" class="size-3.5 shrink-0" />
            <span class="truncate" :style="{ paddingLeft: `${item.depth * 10}px` }">{{ item.group.name }}</span>
          </button>
          <span class="text-muted-foreground shrink-0 text-xs">{{ item.card_count }}</span>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-xs" :title="t('review.groupMoreActions')">
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @select="emit('create', item.group.id)">
                <Plus />{{ t('review.newChildGroup') }}
              </DropdownMenuItem>
              <DropdownMenuItem v-if="!item.group.is_default" @select="emit('rename', item.group)">
                <Pencil />{{ t('review.renameGroup') }}
              </DropdownMenuItem>
              <DropdownMenuSeparator v-if="!item.group.is_default" />
              <DropdownMenuItem v-if="!item.group.is_default" variant="destructive" @select="emit('delete', item.group)">
                <Trash2 />{{ t('review.deleteGroup') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem @select="emit('create', item.group.id)">
          <Plus />{{ t('review.newChildGroup') }}
        </ContextMenuItem>
        <ContextMenuItem v-if="!item.group.is_default" @select="emit('rename', item.group)">
          <Pencil />{{ t('review.renameGroup') }}
        </ContextMenuItem>
        <ContextMenuSeparator v-if="!item.group.is_default" />
        <ContextMenuItem v-if="!item.group.is_default" variant="destructive" @select="emit('delete', item.group)">
          <Trash2 />{{ t('review.deleteGroup') }}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>

    <ContextMenu>
      <ContextMenuTrigger as-child>
        <div class="min-h-16 rounded-md" :title="t('review.groupBlankHint')" />
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem @select="emit('create', '')">
          <Plus />{{ t('review.newGroup') }}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  </aside>
</template>
