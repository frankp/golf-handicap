<script setup lang="ts" generic="T extends string | number">
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
  modelValue?: T | null
  options: ReadonlyArray<{ value: T; label: string; disabled?: boolean }>
  placeholder?: string
  disabled?: boolean
  required?: boolean
  ariaLabel?: string
  triggerClass?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()

function update(value: AcceptableInputValue | null) {
  if (typeof value === 'string' || typeof value === 'number') emit('update:modelValue', value as T)
}
</script>

<template>
  <SelectRoot
    :model-value="modelValue"
    :disabled="disabled"
    :required="required"
    @update:model-value="update"
  >
    <SelectTrigger :id="id" class="select-trigger" :class="triggerClass" :aria-label="ariaLabel">
      <SelectValue :placeholder="placeholder" />
      <SelectIcon class="select-icon"><ChevronDown :size="16" /></SelectIcon>
    </SelectTrigger>
    <SelectPortal>
      <SelectContent class="select-content" position="popper" :side-offset="5">
        <SelectViewport class="select-viewport">
          <SelectItem v-for="option in options" :key="option.value" class="select-item" :value="option.value" :disabled="option.disabled">
            <SelectItemText>{{ option.label }}</SelectItemText>
            <SelectItemIndicator class="select-item-indicator"><Check :size="15" /></SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
