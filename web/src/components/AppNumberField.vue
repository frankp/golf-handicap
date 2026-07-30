<script setup lang="ts">
import { computed } from 'vue'
import { NumberFieldInput, NumberFieldRoot } from 'reka-ui'

const props = withDefaults(defineProps<{
  id: string
  modelValue?: number
  name?: string
  min?: number
  max?: number
  step?: number
  required?: boolean
  disabled?: boolean
  ariaLabel?: string
}>(), {
  step: 1,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | undefined]
}>()

const maximumFractionDigits = computed(() => {
  const decimal = props.step.toString().split('.')[1]
  return decimal?.length ?? 0
})
</script>

<template>
  <NumberFieldRoot
    class="number-field"
    :id="id"
    :model-value="modelValue"
    :name="name"
    :min="min"
    :max="max"
    :step="step"
    :required="required"
    :disabled="disabled"
    :disable-wheel-change="true"
    :format-options="{ useGrouping: false, maximumFractionDigits }"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <NumberFieldInput :aria-label="ariaLabel" />
  </NumberFieldRoot>
</template>
