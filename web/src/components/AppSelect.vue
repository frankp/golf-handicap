<script setup lang="ts" generic="T extends string">
import { Check, ChevronDown } from '@lucide/vue'
import {
  SelectContent,
  SelectIcon,
  SelectItem,
  SelectItemIndicator,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableInputValue,
} from 'reka-ui'

defineProps<{
  id: string
  modelValue: T
  options: ReadonlyArray<{ value: T; label: string }>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()

function update(value: AcceptableInputValue) {
  if (typeof value === 'string') emit('update:modelValue', value as T)
}
</script>

<template>
  <SelectRoot :model-value="modelValue" @update:model-value="update">
    <SelectTrigger :id="id" class="select-trigger">
      <SelectValue />
      <SelectIcon class="select-icon"><ChevronDown :size="16" /></SelectIcon>
    </SelectTrigger>
    <SelectPortal>
      <SelectContent class="select-content" position="popper" :side-offset="5">
        <SelectViewport class="select-viewport">
          <SelectItem v-for="option in options" :key="option.value" class="select-item" :value="option.value">
            <SelectItemText>{{ option.label }}</SelectItemText>
            <SelectItemIndicator class="select-item-indicator"><Check :size="15" /></SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
