<script setup lang="ts">
import { Flag } from '@lucide/vue'

defineProps<{ title: string; detail: string }>()
</script>

<template>
  <div class="empty-state">
    <Flag :size="28" />
    <strong>{{ title }}</strong>
    <span>{{ detail }}</span>
    <slot />
  </div>
</template>
