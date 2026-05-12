<template>
  <section>
    <h2 class="mb-2 flex items-center text-xs font-medium">
      <User
        class="mr-2 size-3.5"
      />
      {{ $t('settings.userProfile') }}
    </h2>
    <Separator />
    <div class="mt-4 space-y-4">
      <div class="space-y-2">
        <Label for="settings-user-id">{{ $t('settings.userID') }}</Label>
        <Input
          id="settings-user-id"
          :model-value="displayUserId"
          :aria-label="$t('settings.userID')"
          readonly
        />
      </div>
      <div class="space-y-2">
        <Label for="settings-username">{{ $t('auth.username') }}</Label>
        <Input
          id="settings-username"
          :model-value="displayUsername"
          :aria-label="$t('auth.username')"
          readonly
        />
      </div>
      <div class="space-y-2">
        <Label for="settings-display-name">{{ $t('settings.displayName') }}</Label>
        <Input
          id="settings-display-name"
          :model-value="displayName"
          :aria-label="$t('settings.displayName')"
          @update:model-value="onDisplayNameChange"
        />
      </div>
      <div class="space-y-2">
        <Label for="settings-avatar-url">{{ $t('settings.avatarUrl') }}</Label>
        <div class="flex gap-2">
          <Input
            id="settings-avatar-url"
            :model-value="avatarUrl"
            type="url"
            :aria-label="$t('settings.avatarUrl')"
            class="flex-1"
            @update:model-value="onAvatarUrlChange"
          />
          <input
            ref="avatarFileRef"
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            class="hidden"
            @change="handleAvatarFileChange"
          >
          <Button
            variant="outline"
            size="icon"
            :title="$t('bots.uploadAvatarImage')"
            :disabled="avatarUploading"
            @click="avatarFileRef?.click()"
          >
            <Spinner
              v-if="avatarUploading"
              class="size-4"
            />
            <Upload
              v-else
              class="size-4"
            />
          </Button>
        </div>
        <p
          v-if="avatarUploadError"
          class="text-destructive text-xs"
        >
          {{ avatarUploadError }}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="settings-timezone">{{ $t('settings.timezone') }}</Label>
        <TimezoneSelect
          :model-value="timezone"
          :placeholder="$t('settings.timezonePlaceholder')"
          @update:model-value="onTimezoneChange"
        />
      </div>
      <div class="flex justify-end">
        <Button
          :disabled="saving || loading"
          @click="emit('save')"
        >
          <Spinner v-if="saving" />
          {{ $t('settings.saveProfile') }}
        </Button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  Button,
  Input,
  Label,
  Separator,
  Spinner,
} from '@memohai/ui'
import { Upload, User } from 'lucide-vue-next'
import { ref } from 'vue'
import TimezoneSelect from '@/components/timezone-select/index.vue'
import { uploadAvatarImage } from '@/utils/avatar-upload'

defineProps<{
  displayUserId: string
  displayUsername: string
  displayName: string
  avatarUrl: string
  timezone: string
  saving: boolean
  loading: boolean
}>()

const emit = defineEmits<{
  'update:displayName': [value: string]
  'update:avatarUrl': [value: string]
  'update:timezone': [value: string]
  save: []
}>()

const avatarFileRef = ref<HTMLInputElement | null>(null)
const avatarUploading = ref(false)
const avatarUploadError = ref('')

function onDisplayNameChange(value: string | number) {
  emit('update:displayName', String(value))
}

function onAvatarUrlChange(value: string | number) {
  emit('update:avatarUrl', String(value))
}

function onTimezoneChange(value: string | number | undefined) {
  emit('update:timezone', String(value || 'UTC'))
}

async function handleAvatarFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  input.value = ''

  avatarUploading.value = true
  avatarUploadError.value = ''
  try {
    const url = await uploadAvatarImage(file)
    emit('update:avatarUrl', url)
  }
  catch (err) {
    avatarUploadError.value = err instanceof Error ? err.message : 'Upload failed'
  }
  finally {
    avatarUploading.value = false
  }
}
</script>
