<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowLeft, Flag, Pencil } from '@lucide/vue'
import { useRoute } from 'vue-router'
import { api } from '@/api'
import type { Course, Round, Tee } from '@/types'
import { authState } from '@/auth'
import AppSwitch from '@/components/AppSwitch.vue'

const route = useRoute()
const round = ref<Round | null>(null)
const courses = ref<Course[]>([])
const error = ref('')
const showNet = ref(true)

onMounted(async () => {
  try {
    ;[round.value, courses.value] = await Promise.all([
      api.round(Number(route.params.id)),
      api.courses(),
    ])
  } catch (err) {
    error.value = (err as Error).message
  }
})

function teeFor(teeId: number): Tee | undefined {
  for (const course of courses.value) {
    const tee = course.tees.find((item) => item.id === teeId)
    if (tee) return tee
  }
}

function holePar(teeId: number, holeIndex: number) {
  return teeFor(teeId)?.par[holeIndex] ?? 0
}

function ninePar(teeId: number, start: number) {
  let total = 0
  for (let i = start; i < start + 9; i++) total += holePar(teeId, i)
  return total
}

function nineScore(scores: number[], start: number) {
  return scores.slice(start, start + 9).reduce((total, score) => total + score, 0)
}

function scoreClass(score: number, par: number) {
  if (score <= 0 || par <= 0) return ''
  const difference = score - par
  if (difference <= -2) return 'score-eagle'
  if (difference === -1) return 'score-birdie'
  if (difference === 0) return 'score-par'
  if (difference === 1) return 'score-bogey'
  return 'score-double-bogey'
}

function scoreLabel(score: number, par: number) {
  if (score <= 0 || par <= 0) return 'No score'
  const difference = score - par
  if (difference <= -3) return 'Albatross or better'
  if (difference === -2) return 'Eagle'
  if (difference === -1) return 'Birdie'
  if (difference === 0) return 'Par'
  if (difference === 1) return 'Bogey'
  if (difference === 2) return 'Double bogey'
  if (difference === 3) return 'Triple bogey'
  return `${difference} over par`
}
</script>

<template>
  <div class="page round-view-page">
    <RouterLink class="back-link" to="/rounds"><ArrowLeft :size="17" /> Rounds</RouterLink>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-else-if="!round" class="loading">Loading round...</div>
    <template v-else>
      <header class="page-header">
        <div><p class="eyebrow">{{ round.playedOn }}</p><h1>{{ round.courseName }}</h1><p v-if="round.notes" class="subtle">{{ round.notes }}</p></div>
        <RouterLink v-if="authState.authenticated" class="button secondary" :to="`/rounds/${round.id}/edit`"><Pencil :size="17" /> Edit round</RouterLink>
      </header>
      <div class="score-toolbar">
        <AppSwitch id="show-net-scores" v-model="showNet" label="Show net" />
        <div class="score-legend" aria-label="Score legend">
          <span><i class="score-mark score-eagle">−2</i>Eagle+</span>
          <span><i class="score-mark score-birdie">−1</i>Birdie</span>
          <span><i class="score-mark score-par">E</i>Par</span>
          <span><i class="score-mark score-bogey">+1</i>Bogey</span>
          <span><i class="score-mark score-double-bogey">+2</i>Double+</span>
        </div>
      </div>
      <section v-for="participant in round.participants" :key="participant.id" class="score-section">
        <header>
          <div><RouterLink :to="`/players/${participant.playerId}`">{{ participant.playerName }}</RouterLink><span>{{ participant.teeName }} tee</span></div>
          <div class="score-summary">
            <span>Gross <b>{{ participant.gross }}</b></span>
            <span>Net <b>{{ Math.round(participant.netScore) }}</b></span>
            <span>Adjusted <b>{{ participant.adjustedGross }}</b></span>
            <span>Diff <b class="differential-value">{{ participant.scoreDifferential.toFixed(1) }} <span v-if="participant.counting" class="counting-flag" title="Counts toward current group index" aria-label="Counts toward current group index"><Flag :size="14" /></span></b></span>
            <span>HI <b>{{ participant.handicapIndexAfter?.toFixed(1) ?? '—' }}</b></span>
          </div>
        </header>
        <div class="round-scorecard">
          <section v-for="nine in [0, 9]" :key="nine" class="nine-card">
            <header>
              <strong>{{ nine === 0 ? 'Front nine' : 'Back nine' }}</strong>
              <span>{{ nineScore(participant.scores, nine) }}</span>
            </header>
            <div class="nine-row nine-holes">
              <span>Hole</span>
              <b v-for="hole in 9" :key="hole">{{ hole + nine }}</b>
              <b>{{ nine === 0 ? 'Out' : 'In' }}</b>
            </div>
            <div class="nine-row nine-pars">
              <span>Par</span>
              <b v-for="hole in 9" :key="hole">{{ holePar(participant.teeId, hole + nine - 1) }}</b>
              <b>{{ ninePar(participant.teeId, nine) }}</b>
            </div>
            <div class="nine-row nine-scores">
              <span>Score</span>
              <b v-for="(score, i) in participant.scores.slice(nine, nine + 9)" :key="i">
                <i
                  class="score-mark"
                  :class="scoreClass(score, holePar(participant.teeId, i + nine))"
                  :title="`${scoreLabel(score, holePar(participant.teeId, i + nine))} on hole ${i + nine + 1}`"
                >{{ score || '–' }}</i>
              </b>
              <b>{{ nineScore(participant.scores, nine) }}</b>
            </div>
            <div v-if="showNet" class="nine-row nine-net">
              <span>Net</span>
              <b v-for="(score, i) in participant.netScores.slice(nine, nine + 9)" :key="i">
                <i
                  class="score-mark net-score-mark"
                  :class="scoreClass(score, holePar(participant.teeId, i + nine))"
                  :title="`Net ${scoreLabel(score, holePar(participant.teeId, i + nine)).toLowerCase()} on hole ${i + nine + 1}`"
                >{{ score || '–' }}</i>
              </b>
              <b>{{ nineScore(participant.netScores, nine) }}</b>
            </div>
          </section>
          <div class="scorecard-total">
            <span>Round total</span>
            <strong>{{ participant.gross }}</strong>
            <template v-if="showNet"><span>Net</span><strong>{{ Math.round(participant.netScore) }}</strong></template>
          </div>
        </div>
        <p v-if="participant.initialParFiveCapUsed || participant.startingHandicapUsed" class="calculation-note">
          {{ participant.startingHandicapUsed ? `Starting Daily Handicap ${participant.dailyHandicap} applied.` : 'Initial Par + 5 limits applied.' }}
        </p>
        <p v-if="showNet" class="calculation-note">
          {{ participant.handicapUsed !== null
            ? `Net score uses the supplied handicap of ${participant.handicapUsed.toFixed(1)}.`
            : `Net score uses the Daily Handicap of ${participant.dailyHandicap}.` }}
        </p>
      </section>
    </template>
  </div>
</template>
