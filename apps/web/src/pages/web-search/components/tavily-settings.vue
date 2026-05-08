<template>
  <div class="grid gap-4 md:grid-cols-2">
    <!-- API Key Pool -->
    <div class="space-y-2 md:col-span-2">
      <div class="flex items-center justify-between">
        <Label>{{ t('webSearch.searchProvider.tavilyApiKeys') }}</Label>
        <Button
          v-if="providerId"
          variant="ghost"
          size="sm"
          class="h-7 text-xs px-2"
          :disabled="testLoading"
          @click="testKeys"
        >
          <LoaderCircle
            v-if="testLoading"
            class="mr-1 size-3 animate-spin"
          />
          {{ testLoading ? t('webSearch.searchProvider.testKeysLoading') : t('webSearch.searchProvider.testKeys') }}
        </Button>
      </div>
      <div class="space-y-2">
        <div
          v-for="(_, i) in localConfig.api_keys"
          :key="i"
          class="flex items-center gap-2"
        >
          <Input
            v-model="localConfig.api_keys[i]"
            :type="showKeys[i] ? 'text' : 'password'"
            class="flex-1"
            :placeholder="t('webSearch.searchProvider.tavilyApiKeyPlaceholder')"
          />
          <!-- Show/hide toggle -->
          <Button
            variant="ghost"
            size="icon"
            class="shrink-0 size-8 text-muted-foreground"
            :title="showKeys[i] ? t('webSearch.searchProvider.hideKey') : t('webSearch.searchProvider.showKey')"
            @click="toggleShowKey(i)"
          >
            <EyeOff
              v-if="showKeys[i]"
              class="size-3.5"
            />
            <Eye
              v-else
              class="size-3.5"
            />
          </Button>
          <!-- Remove button -->
          <Button
            variant="ghost"
            size="icon"
            class="shrink-0 size-8 text-muted-foreground hover:text-destructive"
            @click="removeKey(i)"
          >
            <X class="size-3.5" />
          </Button>
          <!-- Usage badge (shown after testing) -->
          <span
            v-if="keyUsages[i] !== undefined"
            class="text-xs shrink-0"
            :class="keyUsages[i].error ? 'text-destructive' : 'text-muted-foreground'"
            :title="keyUsages[i].error || undefined"
          >
            {{
              keyUsages[i].error
                ? `${t('webSearch.searchProvider.creditsError')}: ${keyUsages[i].error}`
                : keyUsages[i].limit == null
                  ? t('webSearch.searchProvider.creditsUnlimitedUsed', { used: keyUsages[i].usage })
                  : t('webSearch.searchProvider.creditsUsed', { used: keyUsages[i].usage, limit: keyUsages[i].limit })
            }}
          </span>
        </div>
      </div>
      <Button
        variant="outline"
        size="sm"
        class="w-full h-8 text-xs"
        @click="addKey"
      >
        <Plus class="mr-1.5 size-3" />
        {{ t('webSearch.searchProvider.addApiKey') }}
      </Button>
    </div>

    <div class="space-y-2 md:col-span-2">
      <Label for="tavily-base-url">Base URL</Label>
      <Input
        id="tavily-base-url"
        v-model="localConfig.base_url"
        aria-label="Base URL"
      />
    </div>
    <div class="space-y-2">
      <Label for="tavily-timeout-seconds">Timeout (seconds)</Label>
      <Input
        id="tavily-timeout-seconds"
        v-model.number="localConfig.timeout_seconds"
        type="number"
        :min="1"
        aria-label="Timeout (seconds)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, watch, ref } from 'vue'
import { Plus, X, Eye, EyeOff, LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Input, Label, Button } from '@memohai/ui'

const props = defineProps<{
  modelValue: Record<string, unknown>
  providerId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, unknown>]
}>()

const { t } = useI18n()

const localConfig = reactive({
  api_keys: [] as string[],
  base_url: 'https://api.tavily.com/search',
  timeout_seconds: 15,
})

// Per-key show/hide state
const showKeys = ref<boolean[]>([])

// Usage results indexed by key position
interface KeyUsageResult {
  usage: number
  limit: number | null
  error?: string
}
const keyUsages = ref<Record<number, KeyUsageResult>>({})
const testLoading = ref(false)

watch(
  () => props.modelValue,
  (val) => {
    // Migrate legacy single api_key → api_keys pool
    const existingPool = Array.isArray(val?.api_keys)
      ? (val.api_keys as unknown[]).map(k => String(k ?? '')).filter(Boolean)
      : []
    const legacyKey = String(val?.api_key ?? '').trim()

    let newNonEmptyKeys: string[]
    if (existingPool.length > 0) {
      newNonEmptyKeys = existingPool
    } else if (legacyKey) {
      newNonEmptyKeys = [legacyKey]
    } else {
      newNonEmptyKeys = []
    }

    // Only reset api_keys when the non-empty content actually changed.
    // This preserves empty placeholder inputs added by addKey().
    const currentNonEmpty = localConfig.api_keys.filter(Boolean)
    if (JSON.stringify(currentNonEmpty) !== JSON.stringify(newNonEmptyKeys)) {
      localConfig.api_keys = newNonEmptyKeys.length > 0 ? [...newNonEmptyKeys] : ['']
      showKeys.value = localConfig.api_keys.map(() => false)
      keyUsages.value = {}
    }

    localConfig.base_url = String(val?.base_url ?? 'https://api.tavily.com/search')
    const timeout = Number(val?.timeout_seconds ?? 15)
    localConfig.timeout_seconds = Number.isFinite(timeout) && timeout > 0 ? timeout : 15
  },
  { immediate: true, deep: true },
)

watch(localConfig, () => {
  const keys = localConfig.api_keys.map(k => k.trim()).filter(Boolean)
  emit('update:modelValue', {
    api_keys: keys,
    base_url: localConfig.base_url,
    timeout_seconds: localConfig.timeout_seconds,
  })
}, { deep: true })

function toggleShowKey(index: number) {
  showKeys.value = showKeys.value.map((v, i) => i === index ? !v : v)
}

function addKey() {
  localConfig.api_keys = [...localConfig.api_keys, '']
  showKeys.value = [...showKeys.value, false]
}

function removeKey(index: number) {
  localConfig.api_keys = localConfig.api_keys.filter((_, i) => i !== index)
  showKeys.value = showKeys.value.filter((_, i) => i !== index)
  const newUsages: Record<number, KeyUsageResult> = {}
  for (const [k, v] of Object.entries(keyUsages.value)) {
    const ki = Number(k)
    if (ki < index) newUsages[ki] = v
    else if (ki > index) newUsages[ki - 1] = v
  }
  keyUsages.value = newUsages
}

async function testKeys() {
  if (!props.providerId || testLoading.value) return
  testLoading.value = true
  keyUsages.value = {}
  try {
    const token = localStorage.getItem('token')
    const res = await fetch(`/api/search-providers/${props.providerId}/probe-keys`, {
      headers: {
        Accept: 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    })
    if (!res.ok) return
    const data = await res.json() as Array<{
      index: number
      masked_key: string
      usage: number
      limit: number | null
      error?: string
    }>
    const newUsages: Record<number, KeyUsageResult> = {}
    for (const item of data) {
      newUsages[item.index] = { usage: item.usage, limit: item.limit, error: item.error }
    }
    keyUsages.value = newUsages
  } catch {
    // silently fail
  } finally {
    testLoading.value = false
  }
}
</script>
