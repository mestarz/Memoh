<template>
  <Dialog v-model:open="open">
    <DialogTrigger as-child>
      <Button
        variant="outline"
        class="w-full mb-4 text-muted-foreground"
      >
        <Plus class="mr-2" />
        {{ $t('speech.add') }}
      </Button>
    </DialogTrigger>
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ $t('speech.add') }}</DialogTitle>
      </DialogHeader>
      <div class="space-y-4 py-4">
        <div class="space-y-2">
          <Label>{{ $t('common.name') }}</Label>
          <Input
            v-model="form.name"
            :placeholder="$t('common.namePlaceholder')"
          />
        </div>
        <div class="space-y-2">
          <Label>{{ $t('speech.providerType') }}</Label>
          <Select v-model:model-value="form.client_type">
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem
                  v-for="ct in speechClientTypes"
                  :key="ct.value"
                  :value="ct.value"
                >
                  {{ ct.label }}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
      </div>
      <DialogFooter>
        <Button
          variant="outline"
          @click="open = false"
        >
          {{ $t('common.cancel') }}
        </Button>
        <Button
          :disabled="!form.name.trim() || !form.client_type || loading"
          @click="handleCreate"
        >
          <Spinner
            v-if="loading"
            class="mr-1.5"
          />
          {{ $t('common.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { reactive, ref } from 'vue'
import {
  Button,
  Input,
  Label,
  Spinner,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem,
} from '@memohai/ui'
import { postProviders } from '@memohai/sdk'
import type { ProvidersCreateRequest } from '@memohai/sdk'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import { useQueryCache } from '@pinia/colada'
import { SPEECH_CLIENT_TYPE_LIST } from '@/constants/client-types'

const open = defineModel<boolean>('open', { default: false })
const { t } = useI18n()
const queryCache = useQueryCache()
const loading = ref(false)

const speechClientTypes = SPEECH_CLIENT_TYPE_LIST

const form = reactive({
  name: '',
  client_type: 'openai-speech',
})

async function handleCreate() {
  loading.value = true
  try {
    await postProviders({
      body: {
        name: form.name.trim(),
        client_type: form.client_type,
        config: {},
      } as ProvidersCreateRequest,
      throwOnError: true,
    })
    toast.success(t('speech.saveSuccess'))
    queryCache.invalidateQueries({ key: ['speech-providers'] })
    queryCache.invalidateQueries({ key: ['speech-models'] })
    open.value = false
    form.name = ''
    form.client_type = 'openai-speech'
  } catch (error) {
    console.error('Failed to create speech provider:', error)
    toast.error(t('common.saveFailed'))
  } finally {
    loading.value = false
  }
}
</script>
