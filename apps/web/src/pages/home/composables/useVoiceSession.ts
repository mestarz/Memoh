import { ref, watch, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useStorage } from '@vueuse/core'
import { useChatStore } from '@/store/chat-list'

const SENTENCE_ENDINGS = new Set(['。', '！', '？', '!', '?', '\n'])
const MIN_SENTENCE_LEN = 4

function buildApiUrl(path: string): string {
  const base = String((import.meta.env.VITE_API_URL ?? '').trim() || '/api')
  return `${base.replace(/\/+$/, '')}${path}`
}

export function useVoiceSession(
  botId: () => string | null | undefined,
  isActive: () => boolean = () => true,
) {
  const enabled = useStorage('voice-session-enabled', false)

  const chatStore = useChatStore()
  const { messages, streaming } = storeToRefs(chatStore)

  const synthesizing = ref(false)
  const queueLength = ref(0)

  let textBuffer = ''
  const lastSeenContent = new Map<number, string>()
  let lastAssistantTurnId: string | null = null
  let audioQueue: string[] = []
  let currentAudio: HTMLAudioElement | null = null
  let playing = false
  // Generation token: incremented on reset() to discard in-flight synthesis requests
  let generation = 0
  // Synthesis chain: serializes all synthesis calls so audio is enqueued in sentence order
  let synthesisChain = Promise.resolve()

  function playNext() {
    if (playing || audioQueue.length === 0) return
    const url = audioQueue.shift()!
    queueLength.value = audioQueue.length
    playing = true
    const audio = new Audio(url)
    currentAudio = audio
    // Capture generation so that stale done() calls (from an audio paused by reset())
    // don't corrupt the playing flag or trigger playNext() for the new turn.
    const myGen = generation
    let cleaned = false
    const done = () => {
      if (cleaned || generation !== myGen) return
      cleaned = true
      URL.revokeObjectURL(url)
      playing = false
      currentAudio = null
      playNext()
    }
    audio.onended = done
    audio.onerror = done
    audio.play().catch(done)
  }

  async function synthesizeNow(text: string, myGeneration: number) {
    const id = botId()
    if (!id || generation !== myGeneration) return
    synthesizing.value = true
    try {
      const token = localStorage.getItem('token')
      const resp = await fetch(buildApiUrl(`/bots/${encodeURIComponent(id)}/tts/render`), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ text }),
      })
      if (!resp.ok || generation !== myGeneration) return
      const blob = await resp.blob()
      if (generation !== myGeneration) return
      const url = URL.createObjectURL(blob)
      audioQueue.push(url)
      queueLength.value = audioQueue.length
      playNext()
    } catch {
      // Ignore network errors
    } finally {
      if (generation === myGeneration) synthesizing.value = false
    }
  }

  // Enqueue a sentence: serialized via synthesisChain to guarantee playback order
  function synthesizeAndEnqueue(text: string) {
    const myGeneration = generation
    synthesisChain = synthesisChain.then(() => synthesizeNow(text, myGeneration))
  }

  function processDelta(delta: string) {
    textBuffer += delta
    let pos = 0
    for (let i = 0; i < textBuffer.length; i++) {
      if (SENTENCE_ENDINGS.has(textBuffer[i])) {
        const sentence = textBuffer.slice(pos, i + 1).trim()
        if (sentence.length >= MIN_SENTENCE_LEN) {
          synthesizeAndEnqueue(sentence)
        }
        pos = i + 1
      }
    }
    textBuffer = textBuffer.slice(pos)
  }

  function flushBuffer() {
    const text = textBuffer.trim()
    if (text.length >= MIN_SENTENCE_LEN) {
      synthesizeAndEnqueue(text)
    }
    textBuffer = ''
  }

  // Watch streaming text blocks. Also detects new assistant turns so that
  // lastSeenContent is reset before processing any content from the new turn,
  // preventing the race where streaming watcher from the old turn fires late and
  // clears tracking state that already belongs to the new turn.
  watch(
    () => {
      const lastMsg = messages.value[messages.value.length - 1]
      if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.streaming) return null
      return {
        turnId: lastMsg.id,
        blocks: lastMsg.messages
          .filter((m): m is { id: number; type: 'text'; content: string } => m.type === 'text')
          .map((m) => ({ id: m.id, content: m.content })),
      }
    },
    (current) => {
      if (!enabled.value || !isActive() || !current) return
      // New turn: reset per-turn tracking before processing any blocks
      if (current.turnId !== lastAssistantTurnId) {
        lastAssistantTurnId = current.turnId
        textBuffer = ''
        lastSeenContent.clear()
      }
      for (const block of current.blocks) {
        const prev = lastSeenContent.get(block.id) ?? ''
        if (block.content.length > prev.length) {
          const delta = block.content.slice(prev.length)
          lastSeenContent.set(block.id, block.content)
          processDelta(delta)
        }
      }
    },
    { deep: true },
  )

  // Flush remaining text when streaming ends (no lastSeenContent.clear here —
  // that would race with the watcher above on fast consecutive turns)
  watch(streaming, (isStreaming) => {
    if (!isStreaming && enabled.value && isActive()) {
      flushBuffer()
    }
  })

  function reset() {
    generation++
    synthesisChain = Promise.resolve() // abandon pending synthesis chain
    textBuffer = ''
    lastSeenContent.clear()
    lastAssistantTurnId = null
    if (currentAudio) {
      currentAudio.pause()
      currentAudio = null
    }
    for (const url of audioQueue) URL.revokeObjectURL(url)
    audioQueue = []
    queueLength.value = 0
    playing = false
    synthesizing.value = false
  }

  const active = computed(() => enabled.value && (synthesizing.value || queueLength.value > 0 || playing))

  return {
    enabled,
    synthesizing,
    queueLength,
    active,
    reset,
  }
}
