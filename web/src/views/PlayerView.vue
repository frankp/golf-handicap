<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowLeft, Flag } from '@lucide/vue'
import { useRoute } from 'vue-router'
import { api } from '@/api'
import type { PlayerDetail } from '@/types'

const route = useRoute()
const detail = ref<PlayerDetail | null>(null)
const error = ref('')

function formatDecimal(value: number | null | undefined) {
  return value == null ? '—' : value.toFixed(1)
}

function formatWhole(value: number | null | undefined) {
  return value == null ? '—' : Math.round(value).toString()
}

onMounted(async () => {
  try {
    detail.value = await api.player(Number(route.params.id))
  } catch (err) {
    error.value = (err as Error).message
  }
})
</script>

<template>
  <div class="page">
    <RouterLink class="back-link" to="/players"><ArrowLeft :size="17" /> Players</RouterLink>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-else-if="!detail" class="loading">Loading player...</div>
    <template v-else>
      <header class="page-header player-heading">
        <div><p class="eyebrow">Player record</p><h1>{{ detail.player.name }}</h1></div>
      </header>
      <section class="metric-strip player-metrics">
        <div><span class="metric-symbol">HI</span><span>Group index</span><strong>{{ detail.player.groupHandicapIndex?.toFixed(1) ?? 'Pending' }}</strong></div>
        <div><span class="metric-symbol muted">OFF</span><span>Official index</span><strong>{{ detail.player.officialHandicapIndex?.toFixed(1) ?? '—' }}</strong></div>
        <div><span class="metric-symbol muted">#</span><span>Rounds</span><strong>{{ detail.player.roundCount }}</strong></div>
      </section>
      <section class="section">
        <div class="section-heading"><h2>Scoring record</h2></div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>Date</th><th>Course</th><th>Tee</th><th>Gross</th><th>H'cap used</th><th>Net</th><th>Adjusted</th><th>Differential</th><th>Index after</th></tr></thead>
            <tbody>
              <tr v-for="round in detail.rounds" :key="round.id">
                <td>
                  <RouterLink class="primary-link differential-value" :to="`/rounds/${round.id}`">
                    {{ round.playedOn }}
                    <span
                      v-if="round.participants.find((p) => p.playerId === detail!.player.id)?.counting"
                      class="counting-flag counting-mobile"
                      title="Counts toward current group index"
                      aria-label="Counts toward current group index"
                    ><Flag :size="15" /></span>
                  </RouterLink>
                </td>
                <td>{{ round.courseName }}</td>
                <td>{{ round.participants.find((p) => p.playerId === detail!.player.id)?.teeName }}</td>
                <td>{{ round.participants.find((p) => p.playerId === detail!.player.id)?.gross }}</td>
                <td>{{ formatDecimal(round.participants.find((p) => p.playerId === detail!.player.id)?.handicapUsed) }}</td>
                <td>{{ formatWhole(round.participants.find((p) => p.playerId === detail!.player.id)?.netScore) }}</td>
                <td>{{ round.participants.find((p) => p.playerId === detail!.player.id)?.adjustedGross }}</td>
                <td>
                  <span class="differential-value">
                    {{ round.participants.find((p) => p.playerId === detail!.player.id)?.scoreDifferential.toFixed(1) }}
                    <span
                      v-if="round.participants.find((p) => p.playerId === detail!.player.id)?.counting"
                      class="counting-flag counting-desktop"
                      title="Counts toward current group index"
                      aria-label="Counts toward current group index"
                    ><Flag :size="16" /></span>
                  </span>
                </td>
                <td>{{ round.participants.find((p) => p.playerId === detail!.player.id)?.handicapIndexAfter?.toFixed(1) ?? '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>
