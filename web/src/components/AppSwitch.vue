<script setup lang="ts">
import { Label, SwitchRoot, SwitchThumb } from 'reka-ui'

defineProps<{
  id: string
  label: string
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>

<template>
  <div class="switch-control">
    <Label class="switch-label" :for="id">{{ label }}</Label>
    <SwitchRoot
      :id="id"
      class="switch-root"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
    >
      <SwitchThumb class="switch-thumb" />
    </SwitchRoot>
  </div>
</template>
