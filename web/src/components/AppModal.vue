<script setup lang="ts">
import { X } from '@lucide/vue'
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'

defineProps<{ title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()

function handleOpenChange(open: boolean) {
  if (!open) emit('close')
}
</script>

<template>
  <DialogRoot :open="true" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="modal-backdrop" />
      <DialogContent
        as="section"
        class="modal"
        :class="{ 'modal-wide': wide }"
        :aria-describedby="undefined"
      >
        <header class="modal-header">
          <DialogTitle as-child><h2>{{ title }}</h2></DialogTitle>
          <DialogClose as-child>
            <button class="icon-button" type="button" title="Close">
              <X :size="20" />
            </button>
          </DialogClose>
        </header>
        <div class="modal-body"><slot /></div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
