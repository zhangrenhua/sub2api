<template>
  <div class="code-block group">
    <div class="code-block-header">
      <span class="code-block-lang">{{ language }}</span>
      <button
        type="button"
        @click="copy"
        class="code-block-copy"
        :title="copied ? copiedText : copyText"
      >
        <Icon :name="copied ? 'check' : 'copy'" size="xs" />
        <span>{{ copied ? copiedText : copyText }}</span>
      </button>
    </div>
    <pre><code>{{ code }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  code: string
  language?: string
  copyLabel?: string
  copiedLabel?: string
}>()

const copyText = computed(() => props.copyLabel ?? 'Copy')
const copiedText = computed(() => props.copiedLabel ?? 'Copied')

const copied = ref(false)
let timer: number | null = null

const copy = async () => {
  try {
    await navigator.clipboard.writeText(props.code)
    copied.value = true
    if (timer) window.clearTimeout(timer)
    timer = window.setTimeout(() => { copied.value = false }, 1800)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = props.code
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    try { document.execCommand('copy'); copied.value = true } catch { /* ignore */ }
    document.body.removeChild(ta)
    if (timer) window.clearTimeout(timer)
    timer = window.setTimeout(() => { copied.value = false }, 1800)
  }
}
</script>

<style scoped>
.code-block {
  position: relative;
  margin: 0.75rem 0;
  border-radius: 0.625rem;
  background: rgb(17 24 39);
  overflow: hidden;
  border: 1px solid rgb(31 41 55);
}

.code-block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.4rem 0.875rem;
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.code-block-lang {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.72rem;
  color: rgb(156 163 175);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.code-block-copy {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.75rem;
  color: rgb(209 213 219);
  padding: 0.2rem 0.6rem;
  border-radius: 0.375rem;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.code-block-copy:hover {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.code-block pre {
  margin: 0;
  padding: 1rem 1.125rem;
  overflow-x: auto;
  font-size: 0.85rem;
  line-height: 1.55;
  color: rgb(229 231 235);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: pre;
}

.code-block pre code {
  background: transparent !important;
  color: inherit !important;
  padding: 0 !important;
  font-size: inherit !important;
  border-radius: 0 !important;
}
</style>
