<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowLeft, ArrowRight, Check, Minus, Plus, Trash2 } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Course, Player, Tee } from '@/types'

interface Entry {
  playerId: number
  teeId: number
  scores: (number | null)[]
  handicapUsed: string
}

const router = useRouter()
const route = useRoute()
const editingID = Number(route.params.id) || null
const players = ref<Player[]>([])
const courses = ref<Course[]>([])
const playedOn = ref(new Date().toISOString().slice(0, 10))
const courseId = ref<number | null>(null)
const notes = ref('')
const entries = ref<Entry[]>([])
const playerToAdd = ref<number | null>(null)
const mobileHole = ref(0)
const loading = ref(true)
const saving = ref(false)
const error = ref('')

const course = computed(() => courses.value.find((item) => item.id === courseId.value) ?? null)
const availablePlayers = computed(() => players.value.filter((player) => !entries.value.some((entry) => entry.playerId === player.id)))
const complete = computed(() => entries.value.length > 0 && entries.value.every((entry) => entry.scores.every((score) => score !== null)))

onMounted(async () => {
  try {
    const [loadedPlayers, loadedCourses, existingRound] = await Promise.all([
      api.players(),
      api.courses(),
      editingID ? api.round(editingID) : Promise.resolve(null),
    ])
    players.value = loadedPlayers
    courses.value = loadedCourses
    courseId.value = existingRound?.courseId ?? courses.value[0]?.id ?? null
    if (existingRound) {
      playedOn.value = existingRound.playedOn
      notes.value = existingRound.notes
      entries.value = existingRound.participants.map((participant) => ({
        playerId: participant.playerId,
        teeId: participant.teeId,
        scores: [...participant.scores],
        handicapUsed: participant.handicapUsed?.toString() ?? '',
      }))
    }
    playerToAdd.value = players.value[0]?.id ?? null
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
})

function changeCourse() {
  const firstTee = course.value?.tees[0]
  if (firstTee) entries.value.forEach((entry) => { entry.teeId = firstTee.id })
}

watch(availablePlayers, (available) => {
  if (!available.some((player) => player.id === playerToAdd.value)) playerToAdd.value = available[0]?.id ?? null
})

function player(id: number) {
  return players.value.find((item) => item.id === id)!
}

function tee(id: number): Tee | undefined {
  return course.value?.tees.find((item) => item.id === id)
}

function addPlayer() {
  if (!playerToAdd.value || !course.value?.tees.length) return
  entries.value.push({
    playerId: playerToAdd.value,
    teeId: course.value.tees[0].id,
    scores: Array(18).fill(null),
    handicapUsed: '',
  })
}

function changeScore(entry: Entry, hole: number, delta: number) {
  const base = entry.scores[hole] ?? tee(entry.teeId)?.par[hole] ?? 4
  entry.scores[hole] = Math.min(30, Math.max(1, base + delta))
}

function total(entry: Entry) {
  return entry.scores.reduce<number>((sum, score) => sum + (score ?? 0), 0)
}

function frontNine(entry: Entry) {
  return entry.scores.slice(0, 9).reduce<number>((sum, score) => sum + (score ?? 0), 0)
}

function backNine(entry: Entry) {
  return entry.scores.slice(9).reduce<number>((sum, score) => sum + (score ?? 0), 0)
}

async function submit() {
  if (!courseId.value || !complete.value) {
    error.value = 'Enter a score for every hole and player.'
    return
  }
  saving.value = true
  try {
    const payload = {
      playedOn: playedOn.value,
      courseId: courseId.value,
      notes: notes.value,
      participants: entries.value.map((entry) => ({
        playerId: entry.playerId,
        teeId: entry.teeId,
        scores: entry.scores as number[],
        handicapUsed: entry.handicapUsed === '' ? null : Number(entry.handicapUsed),
      })),
    }
    const round = editingID ? await api.updateRound(editingID, payload) : await api.createRound(payload)
    await router.push(`/rounds/${round.id}`)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="page score-entry-page">
    <RouterLink class="back-link" :to="editingID ? `/rounds/${editingID}` : '/rounds'"><ArrowLeft :size="17" /> {{ editingID ? 'Round' : 'Rounds' }}</RouterLink>
    <header class="page-header">
      <div><p class="eyebrow">Completed scorecard</p><h1>{{ editingID ? 'Edit round' : 'Record round' }}</h1></div>
    </header>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-if="loading" class="loading">Preparing scorecard...</div>
    <template v-else>
      <section class="round-setup">
        <label>Date played<input v-model="playedOn" type="date" required /></label>
        <label>Course
          <select v-model="courseId" required @change="changeCourse">
            <option :value="null" disabled>Select course</option>
            <option v-for="item in courses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label class="notes-field">Notes <span class="optional">Optional</span><input v-model.trim="notes" placeholder="Conditions, event, or context" /></label>
      </section>

      <p v-if="courses.length === 0" class="alert">Add a course and tee before recording a round.</p>
      <p v-else-if="players.length === 0" class="alert">Add at least one player before recording a round.</p>
      <template v-else>
        <section class="player-picker">
          <label>Add player
            <select v-model="playerToAdd" :disabled="availablePlayers.length === 0">
              <option v-for="item in availablePlayers" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <button class="button secondary" :disabled="!playerToAdd" @click="addPlayer"><Plus :size="18" /> Add to round</button>
        </section>

        <section v-if="entries.length > 0" class="desktop-scorecard">
          <div class="score-grid grid-header">
            <span class="player-cell">Player</span><span class="tee-cell">Tee</span><span class="handicap-cell">H'cap</span>
            <span v-for="hole in 18" :key="hole">{{ hole }}</span><span>Out</span><span>In</span><span>Total</span><span></span>
          </div>
          <div v-for="(entry, entryIndex) in entries" :key="entry.playerId" class="score-grid score-grid-row">
            <strong class="player-cell">{{ player(entry.playerId).name }}</strong>
            <select v-model="entry.teeId" class="tee-cell" :aria-label="`${player(entry.playerId).name} tee`">
              <option v-for="item in course?.tees" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
            <input v-model="entry.handicapUsed" class="handicap-cell" type="number" min="-20" max="80" step="0.1" :aria-label="`${player(entry.playerId).name} handicap used`" />
            <input v-for="hole in 18" :key="hole" v-model.number="entry.scores[hole - 1]" type="number" min="0" max="30" required :aria-label="`${player(entry.playerId).name}, hole ${hole}`" />
            <b>{{ frontNine(entry) || '—' }}</b><b>{{ backNine(entry) || '—' }}</b><b>{{ total(entry) || '—' }}</b>
            <button class="icon-button danger" title="Remove player" @click="entries.splice(entryIndex, 1)"><Trash2 :size="17" /></button>
          </div>
        </section>

        <section v-if="entries.length > 0" class="mobile-score-entry">
          <header class="hole-nav">
            <button class="icon-button" title="Previous hole" :disabled="mobileHole === 0" @click="mobileHole--"><ArrowLeft :size="21" /></button>
            <div><span>Hole</span><strong>{{ mobileHole + 1 }}</strong></div>
            <button class="icon-button" title="Next hole" :disabled="mobileHole === 17" @click="mobileHole++"><ArrowRight :size="21" /></button>
          </header>
          <div class="hole-progress"><span v-for="hole in 18" :key="hole" :class="{ active: mobileHole === hole - 1, complete: entries.every((entry) => entry.scores[hole - 1] !== null) }" /></div>
          <article v-for="(entry, entryIndex) in entries" :key="entry.playerId" class="mobile-player-score">
            <header>
              <div><strong>{{ player(entry.playerId).name }}</strong><span>Par {{ tee(entry.teeId)?.par[mobileHole] }} · SI {{ tee(entry.teeId)?.strokeIndex[mobileHole] }}</span></div>
              <select v-model="entry.teeId" :aria-label="`${player(entry.playerId).name} tee`"><option v-for="item in course?.tees" :key="item.id" :value="item.id">{{ item.name }}</option></select>
            </header>
            <label class="mobile-handicap">Handicap used <span class="optional">Optional</span><input v-model="entry.handicapUsed" type="number" min="-20" max="80" step="0.1" inputmode="decimal" /></label>
            <div class="score-stepper">
              <button title="Decrease score" @click="changeScore(entry, mobileHole, -1)"><Minus :size="24" /></button>
              <input v-model.number="entry.scores[mobileHole]" type="number" min="0" max="30" inputmode="numeric" :aria-label="`${player(entry.playerId).name}, hole ${mobileHole + 1}`" />
              <button title="Increase score" @click="changeScore(entry, mobileHole, 1)"><Plus :size="24" /></button>
            </div>
            <footer><span>Running total <b>{{ total(entry) || '—' }}</b></span><button class="text-button danger-text" @click="entries.splice(entryIndex, 1)"><Trash2 :size="15" /> Remove</button></footer>
          </article>
          <button v-if="mobileHole < 17" class="button full-width" @click="mobileHole++">Next hole <ArrowRight :size="18" /></button>
        </section>

        <footer v-if="entries.length > 0" class="sticky-submit">
          <span>{{ complete ? 'Scorecard complete' : 'Complete all 18 holes' }}</span>
          <button class="button" :disabled="saving || !complete" @click="submit"><Check :size="18" /> {{ saving ? 'Saving...' : editingID ? 'Save changes' : 'Save round' }}</button>
        </footer>
      </template>
    </template>
  </div>
</template>
