<template>
  <section class="max-w-7xl mx-auto p-4 pb-12">
    <div class="max-w-3xl mx-auto space-y-6">
      <!-- Header: Logo + Version -->
      <div class="flex items-center gap-3">
        <img
          src="/logo.svg"
          alt="Memoh"
          class="size-10 shrink-0 rounded-lg"
        >
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold">
            Memoh
          </p>
          <div class="flex items-center gap-2 mt-0.5">
            <Badge
              v-if="normalizedServerVersion"
              variant="secondary"
            >
              {{ $t('settings.versionTag', { version: normalizedServerVersion }) }}
            </Badge>
            <Badge
              v-if="commitHash"
              variant="outline"
            >
              {{ commitHash }}
            </Badge>
          </div>
        </div>
      </div>

      <section>
        <Separator class="mb-4" />
        <div class="grid grid-cols-[1fr_auto_1fr] gap-4 px-3 mb-3">
          <div class="flex items-center gap-2">
            <Globe class="size-3.5 shrink-0 text-muted-foreground" />
            <span class="flex-1 text-xs">{{ $t('settings.language') }}</span>
            <Select
              :model-value="language"
              @update:model-value="(v) => v && setLanguage(v as Locale)"
            >
              <SelectTrigger
                class="w-24"
                :aria-label="$t('settings.language')"
              >
                <SelectValue :placeholder="$t('settings.languagePlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="zh">
                    {{ $t('settings.langZh') }}
                  </SelectItem>
                  <SelectItem value="en">
                    {{ $t('settings.langEn') }}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <Separator orientation="vertical" />
          <div class="flex items-center gap-2">
            <span class="flex-1 text-xs">{{ $t('settings.theme') }}</span>
            <div class="flex h-9 items-center rounded-md border border-input p-1">
              <button
                class="flex size-7 items-center justify-center rounded-sm transition-colors"
                :class="theme === 'light' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-label="$t('settings.themeLight')"
                @click="setTheme('light')"
              >
                <Sun class="size-4" />
              </button>
              <button
                class="flex size-7 items-center justify-center rounded-sm transition-colors"
                :class="theme === 'dark' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-label="$t('settings.themeDark')"
                @click="setTheme('dark')"
              >
                <Moon class="size-4" />
              </button>
            </div>
          </div>
        </div>
        <div class="space-y-1">
          <a
            href="https://github.com/memohai/memoh"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-9 items-center gap-3 rounded-lg px-3 text-xs text-foreground hover:bg-accent transition-colors"
          >
            <Github class="size-4 text-muted-foreground" />
            {{ $t('about.github') }}
            <ExternalLink class="size-3 ml-auto text-muted-foreground" />
          </a>
          <a
            href="https://docs.memoh.ai"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-9 items-center gap-3 rounded-lg px-3 text-xs text-foreground hover:bg-accent transition-colors"
          >
            <BookOpen class="size-4 text-muted-foreground" />
            {{ $t('about.docs') }}
            <ExternalLink class="size-3 ml-auto text-muted-foreground" />
          </a>
          <a
            href="https://github.com/memohai/memoh/issues"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-9 items-center gap-3 rounded-lg px-3 text-xs text-foreground hover:bg-accent transition-colors"
          >
            <MessageSquare class="size-4 text-muted-foreground" />
            {{ $t('about.feedback') }}
            <ExternalLink class="size-3 ml-auto text-muted-foreground" />
          </a>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ExternalLink, Github, BookOpen, MessageSquare, Globe, Sun, Moon } from 'lucide-vue-next'
import { Badge, Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue, Separator } from '@memohai/ui'
import { useCapabilitiesStore } from '@/store/capabilities'
import { useSettingsStore } from '@/store/settings'
import type { Locale } from '@/i18n'

const capabilitiesStore = useCapabilitiesStore()
const { serverVersion, commitHash } = storeToRefs(capabilitiesStore)
const normalizeVersion = (version?: string | null) => (version ?? '').replace(/^v/i, '')
const normalizedServerVersion = computed(() => normalizeVersion(serverVersion.value))

const settingsStore = useSettingsStore()
const { language, theme } = storeToRefs(settingsStore)
const { setLanguage, setTheme } = settingsStore

onMounted(async () => {
  await capabilitiesStore.load()
})
</script>
