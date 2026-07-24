<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowRight, Plus, Trash2 } from '@lucide/vue'
import { api } from '@/api'
import type { Round } from '@/types'
import EmptyState from '@/components/EmptyState.vue'

const rounds = ref<Round[]>([])
const loading = ref(true)
const error = ref('')

onMounted(load)
async function load() {
  loading.value = true
  try {
    rounds.value = await api.rounds(200)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

async function remove(round: Round) {
  if (!confirm(`Delete the round at ${round.courseName} on ${round.playedOn}? Player handicaps will be recalculated.`)) return
  try {
    await api.deleteRound(round.id)
    await load()
  } catch (err) {
    error.value = (err as Error).message
  }
}
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div><p class="eyebrow">Scoring record</p><h1>Rounds</h1></div>
      <RouterLink class="button" to="/rounds/new"><Plus :size="18" /> Record round</RouterLink>
    </header>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-if="loading" class="loading">Loading rounds...</div>
    <EmptyState v-else-if="rounds.length === 0" title="No rounds recorded" detail="Record a completed round for your group.">
      <RouterLink class="button" to="/rounds/new">Record round</RouterLink>
    </EmptyState>
    <div v-else class="round-history">
      <article v-for="round in rounds" :key="round.id" class="history-row">
        <time :datetime="round.playedOn"><span>{{ new Date(`${round.playedOn}T00:00:00`).toLocaleDateString('en-AU', { day: '2-digit' }) }}</span>{{ new Date(`${round.playedOn}T00:00:00`).toLocaleDateString('en-AU', { month: 'short', year: 'numeric' }) }}</time>
        <div class="history-main">
          <RouterLink :to="`/rounds/${round.id}`">{{ round.courseName }}</RouterLink>
          <span>{{ round.participants.map((p) => `${p.playerName} ${p.netScore !== null ? `net ${Math.round(p.netScore)}` : `gross ${p.gross}`}`).join(' · ') }}</span>
        </div>
        <div class="button-cluster">
          <button class="icon-button danger" title="Delete round" @click="remove(round)"><Trash2 :size="17" /></button>
          <RouterLink class="icon-button" :to="`/rounds/${round.id}`" title="View round"><ArrowRight :size="17" /></RouterLink>
        </div>
      </article>
    </div>
  </div>
</template>
