<template>
  <!--
    File input is intentionally rendered OUTSIDE the Dialog portal to prevent
    Reka UI's focus-trap from conflicting with the native file picker.
    See: https://github.com/radix-ui/primitives/issues/1666
  -->
  <input
    ref="fileInputRef"
    type="file"
    accept="image/jpeg,image/png,image/gif,image/webp"
    class="sr-only"
    tabindex="-1"
    aria-hidden="true"
    @change="handleFileChange"
  >

  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ $t('bots.editAvatar') }}</DialogTitle>
        <DialogDescription>
          {{ $t('bots.editAvatarDescription') }}
        </DialogDescription>
      </DialogHeader>
      <div class="mt-4 flex flex-col items-center gap-4">
        <Avatar class="size-20 shrink-0 rounded-full">
          <AvatarImage
            v-if="draft.trim()"
            :src="draft.trim()"
            :alt="fallbackText"
          />
          <AvatarFallback class="text-xl">
            {{ fallbackText }}
          </AvatarFallback>
        </Avatar>

        <Button
          variant="outline"
          size="sm"
          :disabled="uploading"
          class="w-full"
          @click="openFilePicker"
        >
          <Spinner
            v-if="uploading"
            class="mr-2 size-4"
          />
          {{ uploading ? $t('common.uploading') : $t('bots.uploadAvatarImage') }}
        </Button>
        <p
          v-if="uploadError"
          class="text-destructive text-xs"
        >
          {{ uploadError }}
        </p>

        <div class="flex w-full items-center gap-2">
          <div class="text-muted-foreground flex-1 border-t" />
          <span class="text-muted-foreground text-xs">{{ $t('common.or') }}</span>
          <div class="text-muted-foreground flex-1 border-t" />
        </div>

        <Input
          v-model="draft"
          type="url"
          class="w-full"
          :placeholder="$t('bots.avatarUrlPlaceholder')"
        />
      </div>
      <DialogFooter class="mt-6">
        <DialogClose as-child>
          <Button variant="outline">
            {{ $t('common.cancel') }}
          </Button>
        </DialogClose>
        <Button
          :disabled="!canConfirm"
          @click="handleConfirm"
        >
          {{ $t('common.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import {
  Avatar,
  AvatarImage,
  AvatarFallback,
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Spinner,
} from '@memohai/ui'
import { ref, computed, watch } from 'vue'
import { uploadAvatarImage } from '@/utils/avatar-upload'

withDefaults(defineProps<{
  fallbackText?: string
}>(), {
  fallbackText: '',
})

const open = defineModel<boolean>('open', { default: false })
const avatarUrl = defineModel<string>('avatarUrl', { default: '' })

const draft = ref('')
const uploading = ref(false)
const uploadError = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)

const canConfirm = computed(() => {
  const next = draft.value.trim()
  const current = (avatarUrl.value || '').trim()
  return next !== current
})

watch(open, (val) => {
  if (val) {
    draft.value = avatarUrl.value || ''
    uploadError.value = ''
  }
})

function openFilePicker() {
  // Click the file input that lives OUTSIDE the dialog portal to avoid
  // Reka UI focus-trap conflicts with the native file picker.
  fileInputRef.value?.click()
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  input.value = ''

  uploading.value = true
  uploadError.value = ''
  try {
    const url = await uploadAvatarImage(file)
    draft.value = url
  }
  catch (err) {
    uploadError.value = err instanceof Error ? err.message : 'Upload failed'
  }
  finally {
    uploading.value = false
  }
}

function handleConfirm() {
  if (!canConfirm.value) return
  avatarUrl.value = draft.value.trim()
  open.value = false
}
</script>
