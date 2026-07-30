<script setup lang="ts">
import { Label } from 'reka-ui'

defineProps<{
  inputId: string
  label: string
  hint?: string
}>()
</script>

<template>
  <div class="form-field">
    <div class="field-heading">
      <Label class="field-label" :for="inputId">{{ label }}</Label>
      <span v-if="hint" class="optional">{{ hint }}</span>
    </div>
    <slot />
  </div>
</template>
