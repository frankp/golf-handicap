<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowRight, Plus, Users, ListChecks } from '@lucide/vue'
import { api } from '@/api'
import type { Player, Round } from '@/types'
import EmptyState from '@/components/EmptyState.vue'

const players = ref<Player[]>([])
const rounds = ref<Round[]>([])
const loading = ref(true)
const error = ref('')

const established = computed(() => players.value.filter((p) => p.groupHandicapIndex !== null).length)
const recentRounds = computed(() => rounds.value.slice(0, 6))

onMounted(async () => {
  try {
    ;[players.value, rounds.value] = await Promise.all([api.players(), api.rounds(200)])
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
})

function formatIndex(value: number | null) {
  return value === null ? 'Pending' : value.toFixed(1)
}
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <p class="eyebrow">Current record</p>
        <h1>Overview</h1>
      </div>
      <RouterLink class="button desktop-only-flex" to="/rounds/new"><Plus :size="18" /> Record round</RouterLink>
    </header>

    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-if="loading" class="loading">Loading record...</div>
    <template v-else>
      <section class="metric-strip">
        <div><Users :size="20" /><span>Players</span><strong>{{ players.length }}</strong></div>
        <div><ListChecks :size="20" /><span>Rounds</span><strong>{{ rounds.length }}</strong></div>
        <div><span class="metric-symbol">HI</span><span>Established</span><strong>{{ established }}</strong></div>
      </section>

      <section class="section">
        <div class="section-heading">
          <h2>Player handicaps</h2>
          <RouterLink to="/players">All players <ArrowRight :size="16" /></RouterLink>
        </div>
        <EmptyState v-if="players.length === 0" title="No players yet" detail="Add the people whose rounds you track.">
          <RouterLink class="button secondary" to="/players">Add player</RouterLink>
        </EmptyState>
        <div v-else class="table-wrap">
          <table>
            <thead><tr><th>Player</th><th>Group HI</th><th>Official</th><th>Rounds</th><th></th></tr></thead>
            <tbody>
              <tr v-for="player in players" :key="player.id">
                <td><RouterLink class="primary-link" :to="`/players/${player.id}`">{{ player.name }}</RouterLink></td>
                <td><span class="index-value" :class="{ pending: player.groupHandicapIndex === null }">{{ formatIndex(player.groupHandicapIndex) }}</span></td>
                <td>{{ player.officialHandicapIndex?.toFixed(1) ?? '—' }}</td>
                <td>{{ player.roundCount }}</td>
                <td class="row-action"><RouterLink class="icon-button" :to="`/players/${player.id}`" title="View player"><ArrowRight :size="17" /></RouterLink></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="section">
        <div class="section-heading">
          <h2>Recent rounds</h2>
          <RouterLink to="/rounds">Round history <ArrowRight :size="16" /></RouterLink>
        </div>
        <EmptyState v-if="rounds.length === 0" title="No rounds recorded" detail="Your group rounds will appear here.">
          <RouterLink class="button" to="/rounds/new">Record round</RouterLink>
        </EmptyState>
        <div v-else class="round-list">
          <RouterLink v-for="round in recentRounds" :key="round.id" class="round-row" :to="`/rounds/${round.id}`">
            <time>{{ new Date(`${round.playedOn}T00:00:00`).toLocaleDateString('en-AU', { day: 'numeric', month: 'short', year: 'numeric' }) }}</time>
            <div><strong>{{ round.courseName }}</strong><span>{{ round.participants.map((p) => p.playerName).join(', ') }}</span></div>
            <span class="count">{{ round.participants.length }}</span>
            <ArrowRight :size="17" />
          </RouterLink>
        </div>
      </section>
    </template>
  </div>
</template>
