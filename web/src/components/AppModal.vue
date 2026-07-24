<script setup lang="ts">
import { X } from '@lucide/vue'

defineProps<{ title: string; wide?: boolean }>()
defineEmits<{ close: [] }>()
</script>

<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <section class="modal" :class="{ 'modal-wide': wide }" role="dialog" aria-modal="true" :aria-label="title">
      <header class="modal-header">
        <h2>{{ title }}</h2>
        <button class="icon-button" type="button" title="Close" @click="$emit('close')">
          <X :size="20" />
        </button>
      </header>
      <div class="modal-body"><slot /></div>
    </section>
  </div>
</template>
