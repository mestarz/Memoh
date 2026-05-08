<template>
  <div class="flex flex-col h-full min-w-0">
    <div class="p-2 shrink-0">
      <InputGroup class="h-[30px]">
        <InputGroupAddon class="pl-2.5">
          <Search
            class="size-[11px] text-muted-foreground"
          />
        </InputGroupAddon>
        <InputGroupInput
          v-model="searchQuery"
          :placeholder="t('chat.searchSessionPlaceholder')"
          class="text-xs h-[30px]"
        />
      </InputGroup>
    </div>

    <div class="px-1.5 shrink-0">
      <Button
        v-if="!selectionMode"
        variant="ghost"
        class="w-full h-12 justify-start gap-4.5 text-xs font-medium"
        :disabled="!currentBotId"
        @click="handleNewSession"
      >
        <Plus class="size-3" />
        {{ t('chat.newSession') }}
      </Button>
      <!-- Selection mode header actions -->
      <div
        v-else
        class="flex items-center h-12 gap-1.5"
      >
        <Button
          variant="ghost"
          size="xs"
          class="text-xs h-7 px-2"
          @click="toggleSelectAll"
        >
          {{ allVisibleSelected ? t('chat.deselectAll') : t('chat.selectAll') }}
        </Button>
        <span class="text-xs text-muted-foreground flex-1 text-center">
          {{ t('chat.selectedCount', { n: selectedIds.length }) }}
        </span>
        <Button
          variant="ghost"
          size="xs"
          class="text-xs h-7 px-2"
          @click="exitSelectionMode"
        >
          {{ t('chat.cancelSelect') }}
        </Button>
      </div>
    </div>

    <div class="px-3.5 h-[38px] flex items-center justify-between shrink-0">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button class="flex items-center gap-1">
            <component
              :is="filterIconComponent"
              class="size-2.5"
              :class="filterIconClass"
            />
            <span class="text-[10px] font-medium text-muted-foreground uppercase tracking-[0.7px]">
              {{ t('chat.sessionSourcePrefix') }}{{ filterLabel }}
            </span>
            <ChevronDown class="size-2.5 text-muted-foreground" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem
            v-for="opt in filterOptions"
            :key="opt.value ?? 'all'"
            class="relative"
            @click="filterType = opt.value"
          >
            <Check
              v-if="filterType === opt.value"
              class="size-3 mr-2 absolute"
            />
            <span class="ml-5">{{ opt.label }}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Select mode toggle button -->
      <button
        v-if="!selectionMode && filteredSessions.length > 0"
        class="text-[10px] text-muted-foreground hover:text-foreground transition-colors"
        @click="enterSelectionMode"
      >
        {{ t('chat.selectSessions') }}
      </button>
    </div>

    <div class="flex-1 relative min-h-0">
      <div class="absolute inset-0">
        <ScrollArea class="h-full">
          <div class="flex flex-col gap-1 px-1.5">
            <SessionItem
              v-for="session in filteredSessions"
              :key="session.id"
              :session="session"
              :is-active="sessionId === session.id"
              :selectable="selectionMode"
              :selected="selectedIds.includes(session.id)"
              @select="handleSelect"
              @delete="confirmDeleteSession"
              @toggle-select="toggleSelectItem"
            />
          </div>

          <div
            v-if="currentBotId && !loadingChats && filteredSessions.length === 0"
            class="px-3 py-6 text-center text-xs text-muted-foreground"
          >
            {{ t('chat.noSessions') }}
          </div>

          <div
            v-if="loadingChats"
            class="flex justify-center py-4"
          >
            <LoaderCircle class="size-4 animate-spin text-muted-foreground" />
          </div>
        </ScrollArea>
      </div>
    </div>

    <!-- Bulk delete action bar -->
    <div
      v-if="selectionMode"
      class="shrink-0 px-2 py-2 border-t border-border"
    >
      <Button
        variant="destructive"
        class="w-full h-8 text-xs"
        :disabled="selectedIds.length === 0 || bulkDeleteLoading"
        @click="bulkDeleteDialogOpen = true"
      >
        <LoaderCircle
          v-if="bulkDeleteLoading"
          class="mr-1 size-3 animate-spin"
        />
        {{ t('chat.deleteSelected') }}
      </Button>
    </div>

    <!-- Single delete dialog -->
    <Dialog v-model:open="deleteSessionDialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('chat.deleteSession') }}</DialogTitle>
          <DialogDescription>{{ t('chat.deleteSessionConfirm') }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            :disabled="deleteSessionLoading"
            @click="deleteSessionDialogOpen = false"
          >
            {{ t('common.cancel') }}
          </Button>
          <Button
            variant="destructive"
            :disabled="deleteSessionLoading"
            @click="handleDeleteSession"
          >
            <LoaderCircle
              v-if="deleteSessionLoading"
              class="mr-1 size-3 animate-spin"
            />
            {{ t('common.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Bulk delete confirm dialog -->
    <Dialog v-model:open="bulkDeleteDialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('chat.deleteSelected') }}</DialogTitle>
          <DialogDescription>
            {{ t('chat.deleteSelectedConfirm', { n: selectedIds.length }) }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            :disabled="bulkDeleteLoading"
            @click="bulkDeleteDialogOpen = false"
          >
            {{ t('common.cancel') }}
          </Button>
          <Button
            variant="destructive"
            :disabled="bulkDeleteLoading"
            @click="handleBulkDelete"
          >
            <LoaderCircle
              v-if="bulkDeleteLoading"
              class="mr-1 size-3 animate-spin"
            />
            {{ t('common.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, type Component } from 'vue'
import { Search, Plus, ChevronDown, Check, LoaderCircle, MessageSquare, MessageCircle, HeartPulse, Clock, GitBranch } from 'lucide-vue-next'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/store/chat-list'
import { useWorkspaceTabsStore } from '@/store/workspace-tabs'
import type { SessionSummary } from '@/composables/api/useChat'
import {
  Button,
  ScrollArea,
  InputGroup,
  InputGroupInput,
  InputGroupAddon,
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@memohai/ui'
import SessionItem from './session-item.vue'

const { t } = useI18n()
const chatStore = useChatStore()
const workspaceTabs = useWorkspaceTabsStore()
const { sessions, sessionId, currentBotId, loadingChats } = storeToRefs(chatStore)

const searchQuery = ref('')
const filterType = ref<string>('chat')

const filterOptions = computed(() => [
  { value: 'chat', label: t('chat.sessionTypeChat') },
  { value: 'discuss', label: t('chat.sessionTypeDiscuss') },
  { value: 'heartbeat', label: t('chat.sessionTypeHeartbeat') },
  { value: 'schedule', label: t('chat.sessionTypeSchedule') },
  { value: 'subagent', label: t('chat.sessionTypeSubagent') },
])

const filterLabel = computed(() => {
  const opt = filterOptions.value.find(o => o.value === filterType.value)
  return opt?.label ?? t('chat.sessionTypeChat')
})

const filterIconComponent = computed<Component>(() => {
  switch (filterType.value) {
    case 'discuss': return MessageCircle
    case 'heartbeat': return HeartPulse
    case 'schedule': return Clock
    case 'subagent': return GitBranch
    default: return MessageSquare
  }
})

const filterIconClass = computed(() => {
  switch (filterType.value) {
    case 'discuss': return 'text-sky-400'
    case 'heartbeat': return 'text-rose-400'
    case 'schedule': return 'text-amber-400'
    case 'subagent': return 'text-violet-400'
    default: return 'text-muted-foreground'
  }
})

const filteredSessions = computed(() => {
  let list = sessions.value
  if (filterType.value === 'chat') {
    list = list.filter(s => s.type === 'chat' || s.type === 'discuss')
  } else {
    list = list.filter(s => s.type === filterType.value)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(s =>
      (s.title ?? '').toLowerCase().includes(q)
      || (s.id ?? '').toLowerCase().includes(q),
    )
  }
  return list
})

function handleSelect(session: SessionSummary) {
  workspaceTabs.openChat(session.id, session.title ?? '')
}

function handleNewSession() {
  workspaceTabs.openDraft()
}

// --- Single delete ---
const deleteSessionDialogOpen = ref(false)
const deleteSessionLoading = ref(false)
const sessionPendingDelete = ref<SessionSummary | null>(null)

function confirmDeleteSession(session: SessionSummary) {
  sessionPendingDelete.value = session
  deleteSessionDialogOpen.value = true
}

async function handleDeleteSession() {
  const target = sessionPendingDelete.value
  if (!target || deleteSessionLoading.value) return
  deleteSessionLoading.value = true
  try {
    await chatStore.removeSession(target.id)
    workspaceTabs.closeChatBySession(target.id)
    deleteSessionDialogOpen.value = false
    sessionPendingDelete.value = null
  } finally {
    deleteSessionLoading.value = false
  }
}

// --- Bulk delete ---
const selectionMode = ref(false)
const selectedIds = ref<string[]>([])
const bulkDeleteDialogOpen = ref(false)
const bulkDeleteLoading = ref(false)

const allVisibleSelected = computed(() =>
  filteredSessions.value.length > 0
  && filteredSessions.value.every(s => selectedIds.value.includes(s.id)),
)

function enterSelectionMode() {
  selectionMode.value = true
  selectedIds.value = []
}

function exitSelectionMode() {
  selectionMode.value = false
  selectedIds.value = []
}

function toggleSelectItem(session: SessionSummary) {
  const idx = selectedIds.value.indexOf(session.id)
  if (idx >= 0) {
    selectedIds.value = selectedIds.value.filter(id => id !== session.id)
  } else {
    selectedIds.value = [...selectedIds.value, session.id]
  }
}

function toggleSelectAll() {
  if (allVisibleSelected.value) {
    selectedIds.value = []
  } else {
    selectedIds.value = filteredSessions.value.map(s => s.id)
  }
}

async function handleBulkDelete() {
  if (bulkDeleteLoading.value || selectedIds.value.length === 0) return
  bulkDeleteLoading.value = true
  const ids = [...selectedIds.value]
  try {
    for (const id of ids) workspaceTabs.closeChatBySession(id)
    await chatStore.removeSessions(ids)
    bulkDeleteDialogOpen.value = false
    exitSelectionMode()
  } finally {
    bulkDeleteLoading.value = false
  }
}
</script>
