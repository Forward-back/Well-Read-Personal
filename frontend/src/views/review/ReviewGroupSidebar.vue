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
  <aside class="bg-card flex h-fit flex-col gap-0.5 rounded-md border p-2" data-testid="review-group-sidebar">
    <div class="text-muted-foreground px-2 pt-1 pb-2 text-xs font-medium">
      {{ t('review.groups') }}
    </div>

    <button
      type="button"
      data-testid="review-group-all"
      class="relative flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
      :class="selectedGroupId === '' ? 'bg-primary/10 text-primary before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary' : 'text-foreground hover:bg-muted'"
      @click="emit('select', '')"
    >
      <span class="truncate">{{ t('review.allGroups') }}</span>
      <span class="text-muted-foreground text-xs tabular-nums">{{ totalCards }}</span>
    </button>

    <ContextMenu v-for="item in groups" :key="item.group.id">
      <ContextMenuTrigger as-child>
        <div
          class="group relative flex items-center gap-1 rounded-md pr-1 transition-colors"
          :class="selectedGroupId === item.group.id ? 'bg-primary/10 text-primary before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary' : 'hover:bg-muted'"
          data-testid="review-group-row"
          :data-group-name="item.group.name"
          :data-active="selectedGroupId === item.group.id ? 'true' : 'false'"
        >
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
            :aria-current="selectedGroupId === item.group.id ? 'true' : undefined"
            @click="emit('select', item.group.id)"
          >
            <component :is="item.group.is_default ? Home : Folder" class="size-3.5 shrink-0 opacity-80" />
            <span class="truncate" :style="{ paddingLeft: `${item.depth * 10}px` }">{{ item.group.name }}</span>
          </button>
          <span class="text-muted-foreground shrink-0 text-xs tabular-nums">{{ item.card_count }}</span>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button
                variant="ghost"
                size="icon-xs"
                class="opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100"
                :title="t('review.groupMoreActions')"
              >
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
        <button
          type="button"
          class="text-muted-foreground hover:bg-muted hover:text-foreground mt-1 flex w-full items-center gap-1 rounded-md px-2 py-1.5 text-left text-xs transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
          @click="emit('create', '')"
        >
          <Plus class="size-3.5" />{{ t('review.newGroup') }}
        </button>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem @select="emit('create', '')">
          <Plus />{{ t('review.newGroup') }}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  </aside>
</template>
