<template>
  <template v-if="block.t === 'p'">
    <p v-html="block.html" />
  </template>

  <template v-else-if="block.t === 'h3'">
    <h3 :id="anchorId">{{ block.text }}</h3>
  </template>

  <template v-else-if="block.t === 'h4'">
    <h4>{{ block.text }}</h4>
  </template>

  <template v-else-if="block.t === 'ul'">
    <ul>
      <li v-for="(item, idx) in block.items" :key="idx">
        <template v-if="typeof item === 'string'">
          <span v-html="item" />
        </template>
        <template v-else>
          <span v-html="item.html" />
          <template v-if="item.children">
            <HelpBlock
              v-for="(child, ci) in item.children"
              :key="ci"
              :block="child"
              :copy-label="copyLabel"
              :copied-label="copiedLabel"
            />
          </template>
        </template>
      </li>
    </ul>
  </template>

  <template v-else-if="block.t === 'ol'">
    <ol>
      <li v-for="(item, idx) in block.items" :key="idx" v-html="item" />
    </ol>
  </template>

  <template v-else-if="block.t === 'steps'">
    <ol class="help-steps">
      <li v-for="(item, idx) in block.items" :key="idx" v-html="item" />
    </ol>
  </template>

  <template v-else-if="block.t === 'code'">
    <CodeBlock
      :language="block.lang"
      :code="block.code"
      :copy-label="copyLabel"
      :copied-label="copiedLabel"
    />
  </template>

  <template v-else-if="block.t === 'callout'">
    <div class="help-callout" :class="`help-callout-${block.variant}`" v-html="block.html" />
  </template>

  <template v-else-if="block.t === 'table'">
    <div class="help-table-wrap">
      <table>
        <thead v-if="block.head">
          <tr>
            <th v-for="(h, i) in block.head" :key="i" v-html="h" />
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, ri) in block.rows" :key="ri">
            <td v-for="(cell, ci) in row" :key="ci" v-html="cell" />
          </tr>
        </tbody>
      </table>
    </div>
  </template>

  <template v-else-if="block.t === 'faq'">
    <details
      v-for="(item, idx) in block.items"
      :key="idx"
      class="help-faq"
      :open="idx === 0"
    >
      <summary v-html="item.q" />
      <HelpBlock
        v-for="(child, ci) in item.blocks"
        :key="ci"
        :block="child"
        :copy-label="copyLabel"
        :copied-label="copiedLabel"
      />
    </details>
  </template>
</template>

<script setup lang="ts">
import CodeBlock from '@/components/help/CodeBlock.vue'
import type { Block } from './content'

defineProps<{
  block: Block
  copyLabel?: string
  copiedLabel?: string
  anchorId?: string
}>()
</script>
