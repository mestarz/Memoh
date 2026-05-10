<template>
  <section class="max-w-3xl mx-auto p-4 pb-12">
    <div class="space-y-4">
      <header class="space-y-1">
        <h2 class="text-sm font-semibold flex items-center gap-2">
          <Network class="size-3.5" />
          {{ $t('networkProxy.title') }}
        </h2>
        <p class="text-xs text-muted-foreground">
          {{ $t('networkProxy.description') }}
        </p>
      </header>

      <Separator />

      <form
        class="space-y-4"
        @submit.prevent="onSave"
      >
        <div class="space-y-1.5">
          <label class="text-xs font-medium">
            {{ $t('networkProxy.urlLabel') }}
          </label>
          <Input
            v-model="urlInput"
            type="text"
            :placeholder="$t('networkProxy.urlPlaceholder')"
            :disabled="loading"
            spellcheck="false"
            autocomplete="off"
          />
          <p class="text-xs text-muted-foreground">
            {{ $t('networkProxy.urlHint') }}
          </p>
          <p
            v-if="errorMessage"
            class="text-xs text-destructive"
          >
            {{ errorMessage }}
          </p>
        </div>

        <div class="flex items-center gap-2">
          <Button
            type="submit"
            :disabled="!canSave || saving"
          >
            <Spinner
              v-if="saving"
              class="mr-2 size-3"
            />
            {{ $t('common.save') }}
          </Button>
          <Button
            v-if="urlInput"
            type="button"
            variant="ghost"
            :disabled="saving"
            @click="urlInput = ''"
          >
            {{ $t('networkProxy.clear') }}
          </Button>
          <Button
            type="button"
            variant="outline"
            :disabled="testing || saving || !urlInput.trim()"
            @click="onTest"
          >
            <Spinner
              v-if="testing"
              class="mr-2 size-3"
            />
            {{ testing ? $t('networkProxy.testing') : $t('networkProxy.test') }}
          </Button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { useQuery } from '@pinia/colada'
import { Button, Input, Separator, Spinner } from '@memohai/ui'
import { Network } from 'lucide-vue-next'
import { getAppSettingsNetwork, postAppSettingsNetworkTest, putAppSettingsNetwork } from '@memohai/sdk'

const { t } = useI18n()

const urlInput = ref('')
const saving = ref(false)
const testing = ref(false)
const errorMessage = ref('')

const { data, asyncStatus, refetch } = useQuery({
  key: () => ['app-settings', 'network'],
  query: async () => {
    const { data } = await getAppSettingsNetwork({ throwOnError: true })
    return data ?? { http_proxy_url: '' }
  },
})

const loading = computed(() => asyncStatus.value === 'loading')

watch(data, (val) => {
  urlInput.value = (val?.http_proxy_url ?? '').trim()
}, { immediate: true })

const canSave = computed(() => {
  const trimmed = urlInput.value.trim()
  const original = (data.value?.http_proxy_url ?? '').trim()
  return trimmed !== original
})

function validate(value: string): string {
  if (!value) return ''
  try {
    const u = new URL(value)
    if (!['http:', 'https:', 'socks5:', 'socks5h:'].includes(u.protocol)) {
      return t('networkProxy.errorScheme')
    }
    if (!u.hostname) {
      return t('networkProxy.errorHost')
    }
  } catch {
    return t('networkProxy.errorInvalid')
  }
  return ''
}

function extractErrorMessage(error: unknown): string {
  if (!error) return ''
  if (typeof error === 'string') return error
  if (typeof error === 'object') {
    const obj = error as Record<string, unknown>
    const candidates: unknown[] = [obj.message, obj.error, obj.detail, obj.msg]
    for (const c of candidates) {
      if (typeof c === 'string' && c.trim()) return c
    }
    try {
      return JSON.stringify(error)
    } catch {
      return ''
    }
  }
  return String(error)
}

async function onTest() {
  const value = urlInput.value.trim()
  errorMessage.value = validate(value)
  if (errorMessage.value) return
  testing.value = true
  try {
    const { data, error } = await postAppSettingsNetworkTest({
      body: { http_proxy_url: value },
    })
    if (error) {
      const detail = extractErrorMessage(error)
      throw new Error(detail || t('networkProxy.testFailed', { error: 'unknown' }))
    }
    if (data?.ok) {
      toast.success(t('networkProxy.testSuccess', {
        status: data.status_code ?? '?',
        latency: data.latency_ms ?? 0,
      }))
    } else {
      const reason = data?.error || `HTTP ${data?.status_code ?? '?'}`
      const msg = t('networkProxy.testFailed', { error: reason })
      errorMessage.value = msg
      toast.error(msg)
    }
  } catch (err: unknown) {
    const reason = err instanceof Error && err.message ? err.message : 'unknown'
    const msg = t('networkProxy.testFailed', { error: reason })
    errorMessage.value = msg
    toast.error(msg)
  } finally {
    testing.value = false
  }
}

async function onSave() {
  const value = urlInput.value.trim()
  errorMessage.value = validate(value)
  if (errorMessage.value) return
  saving.value = true
  try {
    const { error } = await putAppSettingsNetwork({
      body: { http_proxy_url: value },
    })
    if (error) {
      const detail = extractErrorMessage(error)
      throw new Error(detail || t('networkProxy.saveFailed'))
    }
    toast.success(t('networkProxy.saveSuccess'))
    await refetch()
  } catch (err: unknown) {
    const msg = err instanceof Error && err.message
      ? err.message
      : t('networkProxy.saveFailed')
    errorMessage.value = msg
    toast.error(msg)
  } finally {
    saving.value = false
  }
}
</script>
